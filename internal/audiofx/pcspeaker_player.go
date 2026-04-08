package audiofx

import (
	"io"
	"math"
	"sync"

	"gddoom/internal/music"
	"gddoom/internal/sound"
)

// PCSpeakerPlayer is a single-channel player that streams PC speaker audio.
// Starting a new sound always interrupts the current one, matching real hardware.
type PCSpeakerPlayer struct {
	mu     sync.Mutex
	player ebitenPlayer
	src    *pcSpeakerSource
	volume float64
}

// ebitenPlayer is the subset of *audio.Player we need, allowing test fakes.
type ebitenPlayer interface {
	Play()
	Pause()
	Rewind() error
	SetVolume(float64)
	IsPlaying() bool
	Close() error
}

// caseReverb is a small Schroeder reverb tuned for a 5150-sized metal PC case
// (~40×17×18 cm).  Four comb filters in parallel feed two allpass stages.
// All delay lines are sized for 44100 Hz.
type caseReverb struct {
	// Comb filter delay lines and read positions.
	// Delay lengths chosen from case dimensions + small primes to decorrelate:
	//   40cm → ~102 smp, 35cm → ~90 smp, 28cm → ~72 smp, 23cm → ~59 smp
	comb    [4][128]float64 // longest comb needs 102 samples
	combPos [4]int
	combLen [4]int
	combFB  [4]float64 // feedback gain (controls RT60)

	// Allpass stages (Schroeder): decorrelate and smear the tail.
	ap    [2][64]float64
	apPos [2]int
	apLen [2]int
	apG   [2]float64 // allpass gain ≈ 0.7
}

func newCaseReverb() caseReverb {
	r := caseReverb{}
	// Comb delays (samples at 44100 Hz) and feedback.
	// RT60 ≈ -3·delay/(log10(fb)·rate) — targeting ~25ms.
	r.combLen = [4]int{102, 90, 72, 59}
	r.combFB = [4]float64{0.86, 0.84, 0.82, 0.80}
	// Allpass delays and gain.
	r.apLen = [2]int{47, 23}
	r.apG = [2]float64{0.7, 0.7}
	return r
}

func (r *caseReverb) process(in float64) float64 {
	// Four parallel comb filters.
	out := 0.0
	for i := 0; i < 4; i++ {
		delayed := r.comb[i][r.combPos[i]]
		r.comb[i][r.combPos[i]] = in + delayed*r.combFB[i]
		r.combPos[i]++
		if r.combPos[i] >= r.combLen[i] {
			r.combPos[i] = 0
		}
		out += delayed
	}
	out *= 0.25 // normalise comb sum

	// Two series allpass filters.
	for i := 0; i < 2; i++ {
		delayed := r.ap[i][r.apPos[i]]
		v := out + delayed*(-r.apG[i])
		r.ap[i][r.apPos[i]] = out + delayed*r.apG[i]
		r.apPos[i]++
		if r.apPos[i] >= r.apLen[i] {
			r.apPos[i] = 0
		}
		out = v + delayed*r.apG[i]
	}
	return out
}

func (r *caseReverb) reset() {
	*r = newCaseReverb()
}

// pcSpeakerSource is an io.ReadSeeker that streams stereo s16 LE PCM by
// simulating the PC speaker physics from a compact []PCSpeakerTone sequence.
type pcSpeakerSource struct {
	mu   sync.Mutex
	seq  []sound.PCSpeakerTone
	rate int // output sample rate

	// playback position
	samplePos int
	phase     float64 // square-wave phase [0,1) within the current tone

	// mass-spring-damper state
	vel  float64
	disp float64

	// RC high-pass state (33Ω + 0.01µF, f_c ≈ 482 Hz)
	rcPrev float64
	rcOut  float64

	// Acoustic short-circuit high-pass state (~1909 Hz, no enclosure)
	hpPrev float64
	hpOut  float64

	// Case reverb: Schroeder reverb tuned for a 5150-sized metal enclosure.
	reverb caseReverb
}

// Physical speaker model constants — derived from real PC speaker hardware.
//
// Circuit: 8253 PIT → 75475 Darlington driver → 33Ω series resistor
//
//	(with 0.01µF cap to GND before resistor) → 57mm/8Ω speaker, 0.25W
//
// Speaker geometry:
//
//	Diameter = 2.25 inches = 57.15mm
//	Radius   = 1.125 inches = 0.028575m  (effective piston radius)
//
// 75475 open-collector output: Vce_sat ≈ 1V when sinking.
// Drive voltage: 5V - 1V = 4V
//
// RC high-pass (0.01µF cap to GND before 33Ω resistor):
//
//	f_c = 1/(2π·R·C) = 1/(2π · 33 · 0.00000001) ≈ 482 Hz
//
// Current and power at high frequency (cap fully charged):
//
//	I      = 4V / (33Ω + 8Ω) = 97.6mA
//	V_coil = 0.0976 × 8Ω    = 0.78V
//	P_coil = 0.0976² × 8    = 0.076W  (within 0.25W rating)
//	Force  = Bl × I ≈ 0.5 T·m × 0.0976 = 0.049N
//	Accel  = F/m ≈ 0.049 / 0.002kg     = 24.4 m/s²
//
// Acoustic short-circuit (no enclosure/baffle — piston in free air):
//
//	f_sc = c / (2π·a) = 343 / (2π · 0.028575) ≈ 1909 Hz
//	Bass below ~1.9 kHz is cancelled by front/back wave interference.
//
// 57mm cone mechanical resonance: Fs ≈ 500 Hz, Q ≈ 0.8
//
// Integration at sample rate; k and d are dimensionless-per-sample:
//
//	k = (2π·500/44100)² ≈ 0.00508
//	d = √k / Q          ≈ 0.0891
//
// Signal chain:
//
//	PIT square wave → RC HP (482 Hz) → mechanical model → velocity output
//	→ acoustic short-circuit HP (1909 Hz) → int16
const (
	// Speaker physical dimensions.
	spkDiameterInches = 2.25
	spkRadiusMetres   = spkDiameterInches * 0.0254 / 2 // 0.028575m

	// Acoustic short-circuit cutoff: f_sc = c/(2π·a), c=343 m/s.
	// Computed value: 343/(2π·0.028575) ≈ 1909 Hz
	spkSpeedOfSound   = 343.0
	spkAcousticCutoff = spkSpeedOfSound / (2 * math.Pi * spkRadiusMetres) // ≈ 1909 Hz

	spkK     = 0.00508   // stiffness: (2π·500/44100)²
	spkD     = 0.0891    // damping: Q≈0.8, no enclosure
	spkDrive = 0.00176   // 97.6mA × Bl(0.5) / mass(0.002) normalised per-sample
	spkGain      = 4000000.0 // velocity → int16 range; high gain compensates acoustic short-circuit HP attenuation
	spkReverbMix = 0.8       // wet mix for case reverb (0=dry, 1=full wet)

	// RC high-pass: 33Ω + 0.01µF → f_c ≈ 482 Hz
	// alpha = exp(-2π·482/44100) ≈ 0.9336
	spkRCAlpha = 0.9336
)

// spkHPAlpha is the first-order IIR coefficient for the acoustic short-circuit
// high-pass at f_sc ≈ 1909 Hz: alpha = exp(-2π·f_sc/rate).
// Also models the lack of enclosure/baffle: speaker operated in free air inside
// a metal PC case, so bass cancels via front/back wave interference below ~1.9 kHz.
var spkHPAlpha = math.Exp(-2 * math.Pi * spkAcousticCutoff / 44100)


func (s *pcSpeakerSource) totalSamples() int {
	if s.rate <= 0 || len(s.seq) == 0 {
		return 0
	}
	return int(math.Round(float64(len(s.seq)) * float64(s.rate) / 140.0))
}

func (s *pcSpeakerSource) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	frames := len(p) / 4 // stereo s16 LE = 4 bytes per frame
	if frames == 0 {
		return 0, nil
	}

	total := s.totalSamples()
	if s.samplePos >= total {
		clear(p)
		return len(p), io.EOF
	}

	samplesPerTick := float64(s.rate) / 140.0
	written := 0

	for i := 0; i < frames; i++ {
		if s.samplePos >= total {
			// zero-fill remainder of the buffer
			for j := written; j < len(p); j++ {
				p[j] = 0
			}
			return len(p), io.EOF
		}

		// Which DMX tick does this sample belong to?
		tickIdx := int(float64(s.samplePos) / samplesPerTick)
		if tickIdx >= len(s.seq) {
			tickIdx = len(s.seq) - 1
		}
		tone := s.seq[tickIdx]

		// PIT square wave: single-polarity (1 = drive, 0 = off).
		var pitOut float64
		if tone.Active {
			freq := tone.ToneFrequency()
			if freq > 0 {
				s.phase += freq / float64(s.rate)
				if s.phase >= 1.0 {
					s.phase -= math.Floor(s.phase)
				}
				if s.phase < 0.5 {
					pitOut = 1.0
				}
			}
		} else {
			s.phase = 0
		}

		// RC high-pass (33Ω + 0.01µF, f_c ≈ 482 Hz): filters the drive signal
		// before it reaches the speaker coil.
		rcOut := spkRCAlpha * (s.rcOut + pitOut - s.rcPrev)
		s.rcPrev = pitOut
		s.rcOut = rcOut

		force := rcOut * spkDrive
		s.samplePos++

		// Mass-spring-damper Euler step.
		accel := force - spkK*s.disp - spkD*s.vel
		s.vel += accel
		s.disp += s.vel

		// Output cone velocity (∝ SPL), not displacement.
		// Velocity naturally rolls off bass (v ∝ f·x), giving the thin/harsh
		// character of a real PC speaker.
		rawVel := s.vel * spkGain

		// High-pass at 300 Hz: removes residual DC and low-end from
		// single-polarity drive and mechanical low-frequency response.
		hpOut := spkHPAlpha * (s.hpOut + rawVel - s.hpPrev)
		s.hpPrev = rawVel
		s.hpOut = hpOut

		// Case reverb: mix dry + wet for small metal enclosure.
		wet := s.reverb.process(hpOut)
		raw := hpOut + wet*spkReverbMix
		if raw > math.MaxInt16 {
			raw = math.MaxInt16
		} else if raw < math.MinInt16 {
			raw = math.MinInt16
		}
		v := int16(raw)

		// Stereo: same value left and right.
		p[written] = byte(v)
		p[written+1] = byte(v >> 8)
		p[written+2] = byte(v)
		p[written+3] = byte(v >> 8)
		written += 4
	}
	return written, nil
}

func (s *pcSpeakerSource) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := int64(s.totalSamples()) * 4
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = int64(s.samplePos)*4 + offset
	case io.SeekEnd:
		abs = total + offset
	}
	if abs < 0 {
		abs = 0
	}
	s.samplePos = int(abs / 4)
	s.phase = 0
	s.vel = 0
	s.disp = 0
	s.rcPrev = 0
	s.rcOut = 0
	s.hpPrev = 0
	s.hpOut = 0
	s.reverb.reset()
	return abs, nil
}

func (s *pcSpeakerSource) load(seq []sound.PCSpeakerTone, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq = seq
	s.rate = rate
	s.samplePos = 0
	s.phase = 0
	s.vel = 0
	s.disp = 0
	s.rcPrev = 0
	s.rcOut = 0
	s.hpPrev = 0
	s.hpOut = 0
	s.reverb.reset()
}

// NewPCSpeakerPlayer creates a player. Returns nil if the audio context is unavailable.
func NewPCSpeakerPlayer(volume float64) *PCSpeakerPlayer {
	ctx := sharedOrNewAudioContext(music.OutputSampleRate)
	if ctx == nil {
		return nil
	}
	src := &pcSpeakerSource{reverb: newCaseReverb()}
	ap, err := ctx.NewPlayer(src)
	if err != nil {
		return nil
	}
	return &PCSpeakerPlayer{
		player: ap,
		src:    src,
		volume: clampVolume(volume),
	}
}

// Play interrupts any current sound and starts the new tone sequence immediately.
func (p *PCSpeakerPlayer) Play(seq []sound.PCSpeakerTone) {
	if p == nil || p.player == nil || len(seq) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.player.Pause()
	p.src.load(seq, music.OutputSampleRate)
	if err := p.player.Rewind(); err != nil {
		return
	}
	p.player.SetVolume(p.volume)
	p.player.Play()
}

func (p *PCSpeakerPlayer) Stop() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.player.Pause()
}

func (p *PCSpeakerPlayer) SetVolume(v float64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = clampVolume(v)
	if p.player != nil {
		p.player.SetVolume(p.volume)
	}
}
