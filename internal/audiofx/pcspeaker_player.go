package audiofx

import (
	"encoding/binary"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"gddoom/internal/music"
	"gddoom/internal/sound"
)

const pcSpeakerPCMUpdateRate = 11025

const pcSpeakerPCMCompactThresholdBytes = 64 * 1024
const pcSpeakerPlayerBuffer = 30 * time.Millisecond

// PCSpeakerPlayer is a single-channel player that streams PC speaker audio.
// Starting a new sound always interrupts the current one, matching real hardware.
type PCSpeakerPlayer struct {
	mu     sync.Mutex
	player ebitenPlayer
	src    *pcSpeakerSource
	volume float64
}

type PCSpeakerVariant int

const (
	PCSpeakerVariantClean PCSpeakerVariant = iota
	PCSpeakerVariantSmallSpeaker
	PCSpeakerVariantPiezo
)

func (v PCSpeakerVariant) String() string {
	switch v {
	case PCSpeakerVariantClean:
		return "passthrough"
	case PCSpeakerVariantPiezo:
		return "small-buzzer"
	default:
		return "paper-speaker"
	}
}

func ParsePCSpeakerVariant(s string) PCSpeakerVariant {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "passthrough", "clean", "dry", "none":
		return PCSpeakerVariantClean
	case "small-buzzer", "piezo", "pizeo", "moving-iron", "movingiron", "tiny":
		return PCSpeakerVariantPiezo
	default:
		return PCSpeakerVariantSmallSpeaker
	}
}

// ebitenPlayer is the subset of *audio.Player we need, allowing test fakes.
type ebitenPlayer interface {
	Play()
	Pause()
	Rewind() error
	SetBufferSize(time.Duration)
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
	mu      sync.Mutex
	rate    int // output sample rate
	variant PCSpeakerVariant
	model   pcSpeakerModel

	// playback position
	effectSeq         []sound.PCSpeakerTone
	effectTickRate    int
	effectSamplePos   int
	effectMixPhase    float64
	effectMixDivisor  uint16
	effectMixActive   bool
	musicSeq          []sound.PCSpeakerTone
	musicTickRate     int
	musicSamplePos    int
	musicLoop         bool
	musicPCM          []byte
	musicPCMPos       int
	musicPCMRate      int
	musicPCMPhase     float64
	musicPCMTarget    float64
	musicPCMTargetOK  bool
	musicPCMError     float64
	musicPCMActive    bool
	musicPCMClosed    bool
	musicPCMLoop      bool
	musicPCMEnv       float64
	musicPCMAGain     float64
	musicPCMHPPrevIn  float64
	musicPCMHPPrevOut float64
	musicPCMLPState   float64
	streamGain        float64
	pitPhase          float64 // PIT input clocks elapsed within the current divisor period
	lastDivisor       uint16
	lastActive        bool
	lastDirect        bool

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

	// DC-blocking state for the clean passthrough preset.
	dcPrevIn  float64
	dcPrevOut float64

	// Piezo resonator state.
	lpState      float64
	envState     float64
	driveState   float64
	resY1        float64
	resY2        float64
	res2Y1       float64
	res2Y2       float64
	acY1         float64
	acY2         float64
	hpState      float64
	driftPhase   float64
	piezoPhase   float64
	freqSmooth   float64
	lastFreq     float64
	lastPiezoIn  float64
	piezoTickAge int
	noiseState   uint32
}

type pcSpeakerModel struct {
	hpAlpha       float64
	gain          float64
	reverbMix     float64
	restLength    float64
	restStiffness float64
	restDamping   float64
	coneRadius    float64
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

	spkGain      = 0.00176 * 4_000_000.0 / spkDrive * 2 // velocity (SI, per-sample) → int16; ×2 compensates 2nd-order HP loss
	spkReverbMix = 1.0                                  // wet mix for case reverb (0=dry, 1=full wet)
)

// spkHPAlpha is the first-order IIR coefficient for the acoustic short-circuit
// high-pass at f_sc ≈ 2729 Hz: alpha = 1/(1 + 2π·f_sc/rate).
// Unbaffled speaker in free air inside a metal PC case — bass cancels via
// front/back wave interference. Applied twice (cascaded) for 12dB/oct dipole rolloff.
var spkHPAlpha = 1.0 / (1.0 + 2*math.Pi*spkAcousticCutoff/44100)
var cleanDCBlockAlpha = 1.0 / (1.0 + 2*math.Pi*20.0/44100)

// Moving-iron buzzer model: Re/Le electrical roll-off feeding one dominant
// diaphragm mode plus a weaker enclosure/vent mode.
const (
	piezoReOhms        = 4.5
	piezoLeHenries     = 0.0053
	piezoF0Hz          = 2400.0
	piezoQ             = 7.0
	piezoCabF0Hz       = 4100.0
	piezoCabQ          = 4.8
	piezoHoleF0Hz      = 2850.0
	piezoHoleQ         = 6.8
	piezoRadiationHPHz = 2400.0
	piezoDriveGain     = 44.0
	piezoPrimaryMix    = 0.18
	piezoSecondaryMix  = 0.08
	piezoHoleMix       = 3.8
	piezoDriveAsymPos  = 1.15
	piezoDriveAsymNeg  = 0.90
	piezoDriftHz       = 0.55
	piezoDriftRangeHz  = 10.0
	piezoNoiseMix      = 0.0003

	pcSpeakerAGCTarget     = 0.995
	pcSpeakerAGCMinGain    = 1.0
	pcSpeakerAGCMaxGain    = 192.0
	pcSpeakerPCMHighPassHz = 180.0
	pcSpeakerPCMLowPassHz  = 3200.0
	pcSpeakerAGCAttackMS   = 0.75
	pcSpeakerAGCReleaseMS  = 10.0
	pcSpeakerAGCGainRiseMS = 0.50
	pcSpeakerAGCGainFallMS = 6.0
)

var piezoElectricalAlpha = math.Exp(-2 * math.Pi * (piezoReOhms / (2 * math.Pi * piezoLeHenries)) / 44100)
var piezoHPAlpha = math.Exp(-2 * math.Pi * piezoRadiationHPHz / 44100)

func modelForVariant(v PCSpeakerVariant) pcSpeakerModel {
	switch v {
	case PCSpeakerVariantClean:
		return pcSpeakerModel{
			hpAlpha:       1.0,
			gain:          math.MaxInt16 * 0.85,
			reverbMix:     0,
			restLength:    0,
			restStiffness: 0,
			restDamping:   0,
			coneRadius:    0,
		}
	case PCSpeakerVariantPiezo:
		return pcSpeakerModel{
			hpAlpha:       1.0,
			gain:          1.0,
			reverbMix:     spkReverbMix,
			restLength:    0,
			restStiffness: 0,
			restDamping:   0,
			coneRadius:    0,
		}
	default:
		return pcSpeakerModel{
			hpAlpha:       spkHPAlpha,
			gain:          spkGain,
			reverbMix:     spkReverbMix,
			restLength:    0.01300,
			restStiffness: 0.02850,
			restDamping:   0.0,
			coneRadius:    spkRadiusMetres,
		}
	}
}

func (s *pcSpeakerSource) totalSamples() int {
	if s.rate <= 0 {
		return 0
	}
	return max(s.effectTotalSamples(), max(s.musicTotalSamples(), s.musicPCMTotalSamples()))
}

func (s *pcSpeakerSource) effectTotalSamples() int {
	if s.rate <= 0 || len(s.effectSeq) == 0 {
		return 0
	}
	return totalSamplesForToneSeq(len(s.effectSeq), s.rate, s.effectTickRate)
}

func (s *pcSpeakerSource) musicTotalSamples() int {
	if s.rate <= 0 || len(s.musicSeq) == 0 {
		return 0
	}
	return totalSamplesForToneSeq(len(s.musicSeq), s.rate, s.musicTickRate)
}

func (s *pcSpeakerSource) musicPCMTotalSamples() int {
	if s.rate <= 0 || len(s.musicPCM) == 0 {
		return 0
	}
	return len(s.musicPCM) / 4
}

func (s *pcSpeakerSource) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	frames := len(p) / 4 // stereo s16 LE = 4 bytes per frame
	if frames == 0 {
		return 0, nil
	}

	written := 0

	for i := 0; i < frames; i++ {
		tone, direct, directDrive := s.currentToneLocked()
		if !tone.Active && !direct && !s.hasPlaybackLocked() {
			for j := written; j < len(p); j++ {
				p[j] = 0
			}
			if written == 0 {
				return 0, io.EOF
			}
			return len(p), io.EOF
		}

		var pitOut float64
		if direct {
			if !s.lastDirect {
				s.pitPhase = 0
				s.lastDivisor = 0
				s.lastActive = false
			}
			pitOut = directDrive
			s.lastDirect = true
		} else {
			divisorNow := tone.ToneDivisor()

			// Reload the PIT whenever the programmed tone byte changes.
			if tone.Active != s.lastActive || divisorNow != s.lastDivisor || s.lastDirect {
				s.pitPhase = 0
				s.lastDivisor = divisorNow
				s.lastActive = tone.Active
			}

			// PIT mode 3 square wave: high for ceil(divisor/2) clocks, low for
			// floor(divisor/2) clocks. Model it in PIT input clocks so odd
			// divisors and reload edges match hardware more closely than a
			// generic 50% duty oscillator.
			if tone.Active {
				divisor := float64(divisorNow)
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
			pitOut *= s.streamGain
			s.lastDirect = false
		}

		if s.variant == PCSpeakerVariantClean {
			raw := pitOut * s.model.gain
			in := raw
			raw = cleanDCBlockAlpha * (s.dcPrevOut + in - s.dcPrevIn)
			s.dcPrevIn = in
			s.dcPrevOut = raw
			if raw > math.MaxInt16 {
				raw = math.MaxInt16
			} else if raw < math.MinInt16 {
				raw = math.MinInt16
			}
			v := int16(raw)
			p[written] = byte(v)
			p[written+1] = byte(v >> 8)
			p[written+2] = byte(v)
			p[written+3] = byte(v >> 8)
			written += 4
			continue
		}

		var rawVel float64
		if s.variant == PCSpeakerVariantPiezo {
			signedIn := 0.0
			if direct {
				signedIn = math.Max(-1, math.Min(1, directDrive))
			} else if tone.Active {
				if pitOut > 0 {
					signedIn = 1.0
				} else {
					signedIn = -1.0
				}
			}

			// Coil current is limited by Re/Le before the armature can respond.
			s.lpState = piezoElectricalAlpha*s.lpState + (1-piezoElectricalAlpha)*signedIn
			coilCurrent := s.lpState
			s.lastPiezoIn = signedIn
			s.noiseState = s.noiseState*1664525 + 1013904223
			noise := (float64((s.noiseState>>16)&0xffff)/32767.5 - 1.0) * piezoNoiseMix
			s.driftPhase += 2 * math.Pi * piezoDriftHz / float64(s.rate)
			if s.driftPhase > 2*math.Pi {
				s.driftPhase -= 2 * math.Pi
			}

			// Magnetic drive is slightly asymmetric near saturation.
			drive := coilCurrent
			if drive >= 0 {
				drive = math.Tanh(drive * piezoDriveAsymPos)
			} else {
				drive = math.Tanh(drive * piezoDriveAsymNeg)
			}
			s.driveState = 0.995*s.driveState + 0.005*math.Abs(drive)

			// Moving-iron diaphragm: one dominant armature/diaphragm mode and a
			// weaker cavity mode from the tiny plastic enclosure and vent hole.
			f0 := piezoF0Hz*(1.0+0.004*math.Tanh(s.driveState)) + piezoDriftRangeHz*math.Sin(s.driftPhase)
			r1 := math.Exp(-math.Pi * f0 / (piezoQ * float64(s.rate)))
			w1 := 2 * math.Pi * f0 / float64(s.rate)
			y1 := 2*r1*math.Cos(w1)*s.resY1 - r1*r1*s.resY2 + (1-r1)*drive
			s.resY2 = s.resY1
			s.resY1 = y1

			f1 := piezoCabF0Hz + 36.0*math.Sin(s.driftPhase*0.61+0.4)
			r2 := math.Exp(-math.Pi * f1 / (piezoCabQ * float64(s.rate)))
			w2 := 2 * math.Pi * f1 / float64(s.rate)
			y2 := 2*r2*math.Cos(w2)*s.res2Y1 - r2*r2*s.res2Y2 + (1-r2)*drive
			s.res2Y2 = s.res2Y1
			s.res2Y1 = y2

			mainBand := y1 - s.resY2
			upperBand := y2 - s.res2Y2

			// Tiny plastic case + vent hole: cavity compliance and hole air mass
			// form the second narrow acoustic band-pass that dominates the tone.
			acF := piezoHoleF0Hz + 22.0*math.Sin(s.driftPhase*0.47+0.2)
			acR := math.Exp(-math.Pi * acF / (piezoHoleQ * float64(s.rate)))
			acW := 2 * math.Pi * acF / float64(s.rate)
			acDrive := mainBand + upperBand*0.35
			acY := 2*acR*math.Cos(acW)*s.acY1 - acR*acR*s.acY2 + (1-acR)*acDrive
			s.acY2 = s.acY1
			s.acY1 = acY
			holeBand := acY - s.acY2

			rawVel = mainBand*piezoPrimaryMix + holeBand*piezoHoleMix + upperBand*piezoSecondaryMix + noise

			s.hpState = piezoHPAlpha*s.hpState + (1-piezoHPAlpha)*rawVel
			rawVel -= s.hpState
			s.freqSmooth = 0.52*s.freqSmooth + 0.48*rawVel
			rawVel = s.freqSmooth
			if rawVel >= 0 {
				rawVel = rawVel / (1.0 + 0.24*rawVel)
			} else {
				rawVel = rawVel / (1.0 + 0.34*math.Abs(rawVel))
			}
			rawVel *= math.MaxInt16 * (piezoDriveGain * 4.0)
		} else {
			// Keep the original paper-speaker path exactly on the committed constants.
			force := pitOut * spkDrive
			// Mass-spring-damper Euler step.
			accel := force - spkK*s.disp - spkD*s.vel
			s.vel += accel
			s.disp += s.vel

			// Output cone velocity (∝ SPL), not displacement.
			// Velocity naturally rolls off bass (v ∝ f·x), giving the thin/harsh
			// character of a real PC speaker.
			rawVel = s.vel * spkGain
		}

		hpOut := rawVel
		if s.variant == PCSpeakerVariantSmallSpeaker {
			// Original paper-speaker dipole high-pass.
			hp1 := spkHPAlpha * (s.hp1Out + rawVel - s.hp1Prev)
			s.hp1Prev = rawVel
			s.hp1Out = hp1
			hp2 := spkHPAlpha * (s.hp2Out + hp1 - s.hp2Prev)
			s.hp2Prev = hp1
			s.hp2Out = hp2
			hpOut = hp2
		} else if s.model.hpAlpha < 1.0 {
			// Acoustic short-circuit HP (~1909 Hz): two cascaded first-order stages
			// model dipole radiation rolloff (12dB/oct below f_sc).
			hp1 := s.model.hpAlpha * (s.hp1Out + rawVel - s.hp1Prev)
			s.hp1Prev = rawVel
			s.hp1Out = hp1
			hp2 := s.model.hpAlpha * (s.hp2Out + hp1 - s.hp2Prev)
			s.hp2Prev = hp1
			s.hp2Out = hp2
			hpOut = hp2
		}

		// Case reverb: mix dry + wet for small metal enclosure.
		raw := hpOut
		if s.variant == PCSpeakerVariantSmallSpeaker {
			wet := s.reverb.process(hpOut)
			raw = hpOut + wet*spkReverbMix
		} else if s.model.reverbMix > 0 {
			wet := s.reverb.process(hpOut)
			raw = hpOut + wet*s.model.reverbMix
		}
		if s.model.coneRadius == 0 && s.variant != PCSpeakerVariantPiezo {
			in := raw
			raw = cleanDCBlockAlpha * (s.dcPrevOut + in - s.dcPrevIn)
			s.dcPrevIn = in
			s.dcPrevOut = raw
		}
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

func (s *pcSpeakerSource) currentToneLocked() (sound.PCSpeakerTone, bool, float64) {
	if s.musicPCMActive {
		if drive, ok := s.nextMusicPCMDriveLocked(); ok {
			return sound.PCSpeakerTone{}, true, drive
		}
		return sound.PCSpeakerTone{}, true, 0
	}
	if tone, ok, _ := s.nextEffectToneLocked(); ok {
		return tone, false, 0
	}
	if tone, ok := s.nextMusicToneLocked(); ok {
		return tone, false, 0
	}
	s.pitPhase = 0
	return sound.PCSpeakerTone{}, false, 0
}

func (s *pcSpeakerSource) nextEffectToneLocked() (sound.PCSpeakerTone, bool, bool) {
	total := s.effectTotalSamples()
	if total <= 0 || s.effectSamplePos >= total {
		s.effectSeq = nil
		s.effectSamplePos = 0
		return sound.PCSpeakerTone{}, false, false
	}
	tone := toneAtSample(s.effectSeq, s.rate, s.effectTickRate, s.effectSamplePos)
	s.effectSamplePos++
	if s.effectSamplePos >= total {
		s.effectSeq = nil
		s.effectSamplePos = 0
	}
	return tone, tone.Active, false
}

func (s *pcSpeakerSource) nextMusicToneLocked() (sound.PCSpeakerTone, bool) {
	total := s.musicTotalSamples()
	if total <= 0 {
		s.musicSeq = nil
		s.musicSamplePos = 0
		return sound.PCSpeakerTone{}, false
	}
	if s.musicSamplePos >= total {
		if !s.musicLoop {
			s.musicSeq = nil
			s.musicSamplePos = 0
			return sound.PCSpeakerTone{}, false
		}
		s.musicSamplePos = 0
	}
	tone := toneAtSample(s.musicSeq, s.rate, s.musicTickRate, s.musicSamplePos)
	s.musicSamplePos++
	if s.musicLoop && s.musicSamplePos >= total {
		s.musicSamplePos = 0
	}
	return tone, true
}

func (s *pcSpeakerSource) nextMusicPCMDriveLocked() (float64, bool) {
	target, ok := s.nextPreEncoderMixedTargetLocked()
	if !ok {
		return 0, false
	}
	target = s.filterMusicPCMTargetLocked(target)
	absTarget := math.Abs(target)
	attack := agcBlendForMs(s.musicPCMRate, pcSpeakerAGCAttackMS)
	release := agcBlendForMs(s.musicPCMRate, pcSpeakerAGCReleaseMS)
	if absTarget > s.musicPCMEnv {
		s.musicPCMEnv += (absTarget - s.musicPCMEnv) * attack
	} else {
		s.musicPCMEnv += (absTarget - s.musicPCMEnv) * release
	}
	if s.musicPCMEnv < 1e-5 {
		s.musicPCMEnv = 1e-5
	}
	desiredGain := pcSpeakerAGCTarget / s.musicPCMEnv
	if desiredGain < pcSpeakerAGCMinGain {
		desiredGain = pcSpeakerAGCMinGain
	} else if desiredGain > pcSpeakerAGCMaxGain {
		desiredGain = pcSpeakerAGCMaxGain
	}
	gainBlend := agcBlendForMs(s.musicPCMRate, pcSpeakerAGCGainFallMS)
	if desiredGain > s.musicPCMAGain {
		gainBlend = agcBlendForMs(s.musicPCMRate, pcSpeakerAGCGainRiseMS)
	}
	s.musicPCMAGain += (desiredGain - s.musicPCMAGain) * gainBlend
	boosted := target * s.musicPCMAGain
	if boosted > 1 {
		boosted = 1
	} else if boosted < -1 {
		boosted = -1
	}
	if s.musicPCMError+boosted >= 0 {
		s.musicPCMError += boosted - 1
		return 1, true
	}
	s.musicPCMError += boosted + 1
	return -1, true
}

func (s *pcSpeakerSource) nextPreEncoderMixedTargetLocked() (float64, bool) {
	target := 0.0
	haveTarget := false
	if musicTarget, ok := s.nextMusicPCMTargetLocked(); ok {
		target = musicTarget
		haveTarget = true
	}
	if effectDrive, ok := s.nextEffectMixedDriveLocked(); ok {
		if haveTarget {
			target = mixPreEncoderSignals(target, effectDrive)
		} else {
			target = effectDrive
		}
		haveTarget = true
	}
	if !haveTarget {
		return 0, false
	}
	if target > 1 {
		target = 1
	} else if target < -1 {
		target = -1
	}
	return target, true
}

func (s *pcSpeakerSource) nextMusicPCMTargetLocked() (float64, bool) {
	total := s.musicPCMTotalSamples()
	if total <= 0 {
		if s.musicPCMClosed {
			s.musicPCMActive = false
		}
		return 0, false
	}
	if s.musicPCMRate <= 0 {
		s.musicPCMRate = pcSpeakerPCMUpdateRate
	}
	if !s.musicPCMTargetOK {
		if !s.loadNextMusicPCMTargetLocked() {
			return 0, false
		}
	}
	step := float64(s.musicPCMRate) / float64(s.rate)
	if step <= 0 {
		step = 1
	}
	s.musicPCMPhase += step
	for s.musicPCMPhase >= 1 {
		s.musicPCMPhase -= 1
		if !s.advanceMusicPCMFrameLocked() {
			break
		}
	}
	target := s.musicPCMTarget
	if target > 1 {
		target = 1
	} else if target < -1 {
		target = -1
	}
	return target, true
}

func (s *pcSpeakerSource) nextEffectMixedDriveLocked() (float64, bool) {
	total := s.effectTotalSamples()
	if total <= 0 || s.effectSamplePos >= total {
		s.effectSeq = nil
		s.effectSamplePos = 0
		s.effectMixPhase = 0
		s.effectMixDivisor = 0
		s.effectMixActive = false
		return 0, false
	}
	tone := toneAtSample(s.effectSeq, s.rate, s.effectTickRate, s.effectSamplePos)
	divisorNow := tone.ToneDivisor()
	if tone.Active != s.effectMixActive || divisorNow != s.effectMixDivisor {
		s.effectMixPhase = 0
		s.effectMixDivisor = divisorNow
		s.effectMixActive = tone.Active
	}
	drive := 0.0
	if tone.Active && divisorNow > 0 {
		divisor := float64(divisorNow)
		highClocks := math.Ceil(divisor / 2.0)
		drive = -1.0
		if s.effectMixPhase < highClocks {
			drive = 1.0
		}
		s.effectMixPhase += float64(sound.PCSpeakerPITHz()) / float64(s.rate)
		if s.effectMixPhase >= divisor {
			s.effectMixPhase = math.Mod(s.effectMixPhase, divisor)
		}
	}
	s.effectSamplePos++
	if s.effectSamplePos >= total {
		s.effectSeq = nil
		s.effectSamplePos = 0
		s.effectMixPhase = 0
		s.effectMixDivisor = 0
		s.effectMixActive = false
	}
	return drive, true
}

func (s *pcSpeakerSource) hasPlaybackLocked() bool {
	return len(s.effectSeq) > 0 || len(s.musicSeq) > 0 || len(s.musicPCM) > 0 || s.musicPCMActive
}

func (s *pcSpeakerSource) loadNextMusicPCMTargetLocked() bool {
	total := s.musicPCMTotalSamples()
	if total <= 0 || s.musicPCMPos >= total {
		if !s.musicPCMLoop {
			if s.musicPCMClosed {
				s.musicPCMTargetOK = false
				s.musicPCMActive = false
				return false
			}
			// Streaming PCM ran out of buffered frames. Hold the last target
			// until more bytes arrive instead of dropping to silence.
			return s.musicPCMTargetOK
		}
		s.musicPCMPos = 0
	}
	base := s.musicPCMPos * 4
	if base+3 >= len(s.musicPCM) {
		return false
	}
	l := int16(binary.LittleEndian.Uint16(s.musicPCM[base : base+2]))
	r := int16(binary.LittleEndian.Uint16(s.musicPCM[base+2 : base+4]))
	s.musicPCMTarget = float64(int(l)+int(r)) / (2.0 * float64(math.MaxInt16))
	s.musicPCMTargetOK = true
	return true
}

func (s *pcSpeakerSource) advanceMusicPCMFrameLocked() bool {
	total := s.musicPCMTotalSamples()
	if total <= 0 {
		if s.musicPCMClosed {
			s.musicPCMTargetOK = false
			s.musicPCMActive = false
			return false
		}
		return s.musicPCMTargetOK
	}
	nextPos := s.musicPCMPos + 1
	if nextPos < total {
		s.musicPCMPos = nextPos
		s.musicPCMTargetOK = false
		return s.loadNextMusicPCMTargetLocked()
	}
	if s.musicPCMLoop {
		s.musicPCMPos = 0
		s.musicPCMTargetOK = false
		return s.loadNextMusicPCMTargetLocked()
	}
	if s.musicPCMClosed {
		s.musicPCMPos = nextPos
		s.musicPCMTargetOK = false
		s.musicPCMActive = false
		return false
	}
	// Buffered PCM is temporarily exhausted. Keep driving the last sample
	// until more stream data is appended.
	return true
}

func toneAtSample(seq []sound.PCSpeakerTone, rate int, tickRate int, samplePos int) sound.PCSpeakerTone {
	if len(seq) == 0 || rate <= 0 {
		return sound.PCSpeakerTone{}
	}
	if tickRate <= 0 {
		tickRate = 140
	}
	samplesPerTick := float64(rate) / float64(tickRate)
	tickIdx := int(float64(samplePos) / samplesPerTick)
	if tickIdx >= len(seq) {
		tickIdx = len(seq) - 1
	}
	return seq[tickIdx]
}

func totalSamplesForToneSeq(seqLen int, sampleRate int, tickRate int) int {
	if seqLen <= 0 || sampleRate <= 0 {
		return 0
	}
	if tickRate <= 0 {
		tickRate = 140
	}
	return int(math.Round(float64(seqLen) * float64(sampleRate) / float64(tickRate)))
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
		cur := s.effectSamplePos
		if len(s.effectSeq) == 0 {
			cur = s.musicSamplePos
		}
		abs = int64(cur)*4 + offset
	case io.SeekEnd:
		abs = total + offset
	}
	if abs < 0 {
		abs = 0
	}
	s.effectSamplePos = int(abs / 4)
	s.musicSamplePos = int(abs / 4)
	s.resetStateLocked()
	return abs, nil
}

func (s *pcSpeakerSource) load(seq []sound.PCSpeakerTone, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.effectSeq = seq
	s.rate = rate
	s.effectTickRate = 140
	s.effectSamplePos = 0
	s.resetStateLocked()
}

func (s *pcSpeakerSource) setEffectMixed(seq []sound.PCSpeakerTone, rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.effectSeq = seq
	s.rate = rate
	s.effectTickRate = 140
	s.effectSamplePos = 0
	s.effectMixPhase = 0
	s.effectMixDivisor = 0
	s.effectMixActive = false
}

func (s *pcSpeakerSource) musicPCMIsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.musicPCMActive
}

func (s *pcSpeakerSource) musicIsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.musicPCMActive || len(s.musicSeq) > 0
}

func (s *pcSpeakerSource) setMusicPCM(pcm []byte, rate int, loop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.musicPCM = pcm
	s.rate = rate
	s.musicPCMRate = pcSpeakerPCMUpdateRate
	s.musicPCMLoop = loop
	s.musicPCMPos = 0
	s.musicPCMPhase = 0
	s.musicPCMTarget = 0
	s.musicPCMTargetOK = false
	s.musicPCMError = 0
	s.musicPCMEnv = 0
	s.musicPCMAGain = 1
	s.musicPCMHPPrevIn = 0
	s.musicPCMHPPrevOut = 0
	s.musicPCMLPState = 0
	s.musicPCMActive = true
	s.musicPCMClosed = true
}

func (s *pcSpeakerSource) beginMusicPCM(rate int, loop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = rate
	s.musicPCM = s.musicPCM[:0]
	s.musicPCMPos = 0
	s.musicPCMRate = pcSpeakerPCMUpdateRate
	s.musicPCMPhase = 0
	s.musicPCMTarget = 0
	s.musicPCMTargetOK = false
	s.musicPCMError = 0
	s.musicPCMEnv = 0
	s.musicPCMAGain = 1
	s.musicPCMHPPrevIn = 0
	s.musicPCMHPPrevOut = 0
	s.musicPCMLPState = 0
	s.musicPCMActive = true
	s.musicPCMClosed = false
	s.musicPCMLoop = loop
}

func (s *pcSpeakerSource) appendMusicPCM(pcm []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(pcm) == 0 {
		return
	}
	s.compactMusicPCMLocked(false)
	s.musicPCM = append(s.musicPCM, pcm...)
	s.musicPCMActive = true
}

func (s *pcSpeakerSource) finishMusicPCM() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.musicPCMClosed = true
}

func (s *pcSpeakerSource) bufferedMusicPCMBytes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactMusicPCMLocked(false)
	if len(s.musicPCM) == 0 {
		return 0
	}
	pos := s.musicPCMPos * 4
	if pos >= len(s.musicPCM) {
		return 0
	}
	return len(s.musicPCM) - pos
}

func (s *pcSpeakerSource) setMusic(seq []sound.PCSpeakerTone, rate int, tickRate int, loop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.musicSeq = seq
	s.rate = rate
	s.musicTickRate = tickRate
	s.musicLoop = loop
	s.musicSamplePos = 0
	s.musicPCM = nil
	s.musicPCMPos = 0
	s.musicPCMRate = 0
	s.musicPCMPhase = 0
	s.musicPCMTarget = 0
	s.musicPCMTargetOK = false
	s.musicPCMError = 0
	s.musicPCMEnv = 0
	s.musicPCMAGain = 1
	s.musicPCMHPPrevIn = 0
	s.musicPCMHPPrevOut = 0
	s.musicPCMLPState = 0
	s.musicPCMActive = false
	s.musicPCMClosed = false
	s.musicPCMLoop = false
}

func (s *pcSpeakerSource) clearMusic() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.musicSeq = nil
	s.musicTickRate = 0
	s.musicSamplePos = 0
	s.musicLoop = false
	s.musicPCM = nil
	s.musicPCMPos = 0
	s.musicPCMRate = 0
	s.musicPCMPhase = 0
	s.musicPCMTarget = 0
	s.musicPCMTargetOK = false
	s.musicPCMError = 0
	s.musicPCMEnv = 0
	s.musicPCMAGain = 1
	s.musicPCMHPPrevIn = 0
	s.musicPCMHPPrevOut = 0
	s.musicPCMLPState = 0
	s.musicPCMActive = false
	s.musicPCMClosed = false
	s.musicPCMLoop = false
}

func (s *pcSpeakerSource) compactMusicPCMLocked(force bool) {
	if len(s.musicPCM) == 0 || s.musicPCMPos <= 0 {
		return
	}
	consumedBytes := s.musicPCMPos * 4
	if consumedBytes <= 0 {
		return
	}
	if !force {
		if consumedBytes < pcSpeakerPCMCompactThresholdBytes {
			return
		}
		if consumedBytes*2 < len(s.musicPCM) {
			return
		}
	}
	if consumedBytes >= len(s.musicPCM) {
		s.musicPCM = s.musicPCM[:0]
		s.musicPCMPos = 0
		return
	}
	remaining := len(s.musicPCM) - consumedBytes
	copy(s.musicPCM[:remaining], s.musicPCM[consumedBytes:])
	s.musicPCM = s.musicPCM[:remaining]
	s.musicPCMPos = 0
}

func (s *pcSpeakerSource) filterMusicPCMTargetLocked(v float64) float64 {
	if s.musicPCMRate <= 0 {
		return v
	}
	hpAlpha := highPassAlphaForHz(float64(s.musicPCMRate), pcSpeakerPCMHighPassHz)
	hpOut := hpAlpha * (s.musicPCMHPPrevOut + v - s.musicPCMHPPrevIn)
	s.musicPCMHPPrevIn = v
	s.musicPCMHPPrevOut = hpOut

	lpAlpha := lowPassBlendForHz(float64(s.musicPCMRate), pcSpeakerPCMLowPassHz)
	s.musicPCMLPState += (hpOut - s.musicPCMLPState) * lpAlpha
	return s.musicPCMLPState
}

func agcBlendForMs(rate int, ms float64) float64 {
	if rate <= 0 || ms <= 0 {
		return 1
	}
	return 1 - math.Exp(-1000.0/(float64(rate)*ms))
}

func highPassAlphaForHz(rate float64, cutoffHz float64) float64 {
	if rate <= 0 || cutoffHz <= 0 {
		return 1
	}
	rc := 1.0 / (2 * math.Pi * cutoffHz)
	dt := 1.0 / rate
	return rc / (rc + dt)
}

func lowPassBlendForHz(rate float64, cutoffHz float64) float64 {
	if rate <= 0 || cutoffHz <= 0 {
		return 1
	}
	return 1 - math.Exp(-2*math.Pi*cutoffHz/rate)
}

func mixPreEncoderSignals(a float64, b float64) float64 {
	return (a + b) * 0.5
}

func (s *pcSpeakerSource) setGain(v float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamGain = clampVolume(v)
}

func (s *pcSpeakerSource) resetStateLocked() {
	s.pitPhase = 0
	s.lastDivisor = 0
	s.lastActive = false
	s.effectMixPhase = 0
	s.effectMixDivisor = 0
	s.effectMixActive = false
	s.vel = 0
	s.disp = 0
	s.hp1Prev = 0
	s.hp1Out = 0
	s.hp2Prev = 0
	s.hp2Out = 0
	s.dcPrevIn = 0
	s.dcPrevOut = 0
	s.lpState = 0
	s.envState = 0
	s.driveState = 0
	s.resY1 = 0
	s.resY2 = 0
	s.res2Y1 = 0
	s.res2Y2 = 0
	s.acY1 = 0
	s.acY2 = 0
	s.hpState = 0
	s.driftPhase = 0
	s.freqSmooth = 0
	s.lastFreq = 0
	s.lastPiezoIn = 0
	s.piezoTickAge = 0
	s.noiseState = 1
	s.reverb.reset()
}

func (s *pcSpeakerSource) setVariant(v PCSpeakerVariant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.variant = v
	s.model = modelForVariant(v)
}

// NewPCSpeakerPlayer creates a player. Returns nil if the audio context is unavailable.
func NewPCSpeakerPlayer(volume float64, variant PCSpeakerVariant) *PCSpeakerPlayer {
	ctx := sharedOrNewAudioContext(music.OutputSampleRate)
	if ctx == nil {
		return nil
	}
	src := &pcSpeakerSource{reverb: newCaseReverb()}
	src.setVariant(variant)
	src.setGain(volume)
	ap, err := ctx.NewPlayer(src)
	if err != nil {
		return nil
	}
	ap.SetBufferSize(pcSpeakerPlayerBuffer)
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
	if p.src.musicIsActive() {
		p.src.setEffectMixed(seq, music.OutputSampleRate)
		p.player.SetVolume(p.volume)
		if !p.player.IsPlaying() {
			p.player.Play()
		}
		return
	}
	p.player.Pause()
	p.src.load(seq, music.OutputSampleRate)
	if err := p.player.Rewind(); err != nil {
		return
	}
	p.player.SetVolume(p.volume)
	p.player.Play()
}

func (p *PCSpeakerPlayer) SetMusic(seq []sound.PCSpeakerTone, tickRate int, loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.setMusic(seq, music.OutputSampleRate, tickRate, loop)
	if err := p.player.Rewind(); err != nil {
		return
	}
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) SetMusicPCM(pcm []byte, loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.setMusicPCM(pcm, music.OutputSampleRate, loop)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) BeginMusicPCM(loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.beginMusicPCM(music.OutputSampleRate, loop)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) AppendMusicPCM(pcm []byte) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.appendMusicPCM(pcm)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) FinishMusicPCM() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.finishMusicPCM()
}

func (p *PCSpeakerPlayer) BufferedMusicPCMBytes() int {
	if p == nil || p.player == nil {
		return 0
	}
	return p.src.bufferedMusicPCMBytes()
}

func (p *PCSpeakerPlayer) ClearMusic() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.clearMusic()
}

func (p *PCSpeakerPlayer) SetEffectVolume(v float64) {
	p.SetVolume(v)
}

func (p *PCSpeakerPlayer) SetMusicVolume(v float64) {
	p.SetVolume(v)
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
	p.src.setGain(p.volume)
	if p.player != nil {
		p.player.SetVolume(p.volume)
	}
}
