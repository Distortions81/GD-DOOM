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
	// RT60 ≈ -3·delay/(log10(fb)·rate) — ranging ~80–110ms across the four combs.
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

	// Two series allpass filters (Schroeder topology).
	for i := 0; i < 2; i++ {
		delayed := r.ap[i][r.apPos[i]]
		v := out - r.apG[i]*delayed
		r.ap[i][r.apPos[i]] = v
		r.apPos[i]++
		if r.apPos[i] >= r.apLen[i] {
			r.apPos[i] = 0
		}
		out = delayed + r.apG[i]*v
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
	pitPhase  float64 // PIT input clocks elapsed within the current divisor period
	lastTone  byte

	// mass-spring-damper state
	vel  float64
	disp float64

	// Acoustic short-circuit high-pass state (~1909 Hz, unbaffled dipole).
	// Two cascaded first-order stages give 12dB/oct rolloff below f_sc,
	// matching dipole radiation: SPL ∝ ω²·v below f_sc, ∝ v above.
	hp1Prev float64
	hp1Out  float64
	hp2Prev float64
	hp2Out  float64

	// Case reverb: Schroeder reverb tuned for a 5150-sized metal enclosure.
	reverb caseReverb
}

// Physical speaker model constants — derived from real PC speaker hardware.
//
// Circuit: 8253 PIT → 75475 driver → 33Ω + 57mm/8Ω speaker (0.25W)
// Drive: 5V - Vce_sat(1V) = 4V, I = 4V/41Ω = 97.6mA
// Force = Bl(0.5) × I = 0.0488N, Accel = F/m(0.002kg) = 24.4 m/s²
//
// Mechanical resonance: Fs ≈ 800 Hz, Q ≈ 4 (stiff, lightly damped paper cone)
//
//	k = (2π·800/44100)² ≈ 0.01300
//	d = √k / Q          ≈ 0.02850
//
// Acoustic short-circuit (unbaffled, free air):
//
//	Effective cone diameter ≈ 40mm (frame is 57mm, cone is ~70% of frame)
//	f_sc = c/(2π·a) = 343/(2π·0.020) ≈ 2729 Hz
//
// Signal chain:
//
//	PIT square wave → mechanical model → velocity output
//	→ acoustic short-circuit HP (2729 Hz, 12dB/oct dipole) → int16
const (
	// Speaker physical dimensions.
	// Frame is 2.25" (57mm), effective cone piston ~40mm diameter.
	spkConeDiameterMM = 40.0
	spkRadiusMetres   = spkConeDiameterMM / 1000.0 / 2 // 0.020m effective piston radius

	// Acoustic short-circuit cutoff: f_sc = c/(2π·a), c=343 m/s.
	// Computed value: 343/(2π·0.020) ≈ 2729 Hz
	spkSpeedOfSound   = 343.0
	spkAcousticCutoff = spkSpeedOfSound / (2 * math.Pi * spkRadiusMetres) // ≈ 2729 Hz

	spkK = 0.01300 // stiffness: (2π·800/44100)²
	spkD = 0.02850 // damping: √k / Q, Q≈4 (stiff paper cone)

	// Drive circuit (SI units)
	spkBl     = 0.5   // voice coil force factor (T·m)
	spkVdrive = 4.0   // drive voltage: 5V - Vce_sat(1V)
	spkRtotal = 41.0  // total resistance: 33Ω series + 8Ω coil
	spkMass   = 0.002 // cone + voice coil mass (kg)

	// Per-sample² drive acceleration: Bl·V / (R·m·rate²)
	spkDrive = spkBl * spkVdrive / (spkRtotal * spkMass * 44100 * 44100)

	spkGain      = 0.00176 * 4_000_000.0 / spkDrive * 8 // velocity (SI, per-sample) → int16; ×8 compensates 2nd-order HP loss
	spkReverbMix = 1.0                                  // wet mix for case reverb (0=dry, 1=full wet)
)

// spkHPAlpha is the first-order IIR coefficient for the acoustic short-circuit
// high-pass at f_sc ≈ 2729 Hz: alpha = 1/(1 + 2π·f_sc/rate).
// Unbaffled speaker in free air inside a metal PC case — bass cancels via
// front/back wave interference. Applied twice (cascaded) for 12dB/oct dipole rolloff.
var spkHPAlpha = 1.0 / (1.0 + 2*math.Pi*spkAcousticCutoff/44100)

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

		// Reload the PIT whenever the programmed tone byte changes.
		if tone.ToneValue != s.lastTone {
			s.pitPhase = 0
			s.lastTone = tone.ToneValue
		}

		// PIT mode 3 square wave: high for ceil(divisor/2) clocks, low for
		// floor(divisor/2) clocks. Model it in PIT input clocks so odd
		// divisors and reload edges match hardware more closely than a
		// generic 50% duty oscillator.
		var pitOut float64
		if tone.Active {
			divisor := float64(tone.ToneDivisor())
			if divisor > 0 {
				highClocks := math.Ceil(divisor / 2.0)
				if s.pitPhase < highClocks {
					pitOut = 1.0
				}
				s.pitPhase += float64(sound.PCSpeakerPITHz()) / float64(s.rate)
				if s.pitPhase >= divisor {
					s.pitPhase = math.Mod(s.pitPhase, divisor)
				}
			}
		} else {
			s.pitPhase = 0
		}

		force := pitOut * spkDrive
		s.samplePos++

		// Mass-spring-damper Euler step.
		accel := force - spkK*s.disp - spkD*s.vel
		s.vel += accel
		s.disp += s.vel

		// Output cone velocity (∝ SPL), not displacement.
		// Velocity naturally rolls off bass (v ∝ f·x), giving the thin/harsh
		// character of a real PC speaker.
		rawVel := s.vel * spkGain

		// Acoustic short-circuit HP (~1909 Hz): two cascaded first-order stages
		// model dipole radiation rolloff (12dB/oct below f_sc).
		hp1 := spkHPAlpha * (s.hp1Out + rawVel - s.hp1Prev)
		s.hp1Prev = rawVel
		s.hp1Out = hp1
		hp2 := spkHPAlpha * (s.hp2Out + hp1 - s.hp2Prev)
		s.hp2Prev = hp1
		s.hp2Out = hp2
		hpOut := hp2

		// Case reverb: mix dry + wet for small metal enclosure.
		wet := s.reverb.process(hpOut)
		raw := hpOut + wet*spkReverbMix
		// Soft-clip via tanh: fills int16 range without hard clipping.
		v := int16(math.Tanh(raw/math.MaxInt16) * math.MaxInt16)

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
	s.pitPhase = 0
	s.lastTone = 0
	s.vel = 0
	s.disp = 0
	s.hp1Prev = 0
	s.hp1Out = 0
	s.hp2Prev = 0
	s.hp2Out = 0
	s.reverb.reset()
	return abs, nil
}

func (s *pcSpeakerSource) load(seq []sound.PCSpeakerTone, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq = seq
	s.rate = rate
	s.samplePos = 0
	s.pitPhase = 0
	s.lastTone = 0
	s.vel = 0
	s.disp = 0
	s.hp1Prev = 0
	s.hp1Out = 0
	s.hp2Prev = 0
	s.hp2Out = 0
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
