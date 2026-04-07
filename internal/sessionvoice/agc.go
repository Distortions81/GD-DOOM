package sessionvoice

import (
	"fmt"
	"math"
	"os"
	"sync"
)

const (
	agcTargetRMS        = 9000.0
	agcPeakLimit        = 26000.0
	agcMinGain          = 0.5
	agcMaxGain          = 10.0
	agcVoiceRMSFloor    = 220.0
	agcVoiceNoiseRatio  = 1.08
	agcVoiceHoldFrames  = 12
	agcAverageSmoothing = 0.01
	agcNoiseSmoothing   = 0.03
	agcGainAttack       = 0.03
	agcGainRelease      = 0.06
	agcGainDeadband     = 0.2
	agcIdleReturn       = 0.005
	agcGateFloorRatio   = 1.05
	agcGateOpenRatio    = 2.2
	agcGateMinGain      = 0.0
	agcGateAttack       = 0.55
	agcGateRelease      = 0.25
	agcGateSilentGain   = 0.08
	agcGateGroupSamples = 80
	agcGateLookahead    = 2
	agcGateSpeechGroups = 3
	agcLowPassCutoffHz  = 4000.0
	agcHighPassCutoffHz = 120.0
)

type micAGC struct {
	mu           sync.RWMutex
	enabled      bool
	gateEnabled  bool
	gateScale    float64
	gain         float64
	voiceRMSAvg  float64
	noiseRMSAvg  float64
	lpState      float64
	lowState     float64
	prevBand     float64
	havePrevBand bool
	voiceHold    int
	frameCount   int
	gateGain     float64
	gateActive   bool
	lastLogGain  float64
	lastLogGate  float64
	lastLogVoice bool
	logEnabled   bool
}

func newMicAGC() *micAGC {
	return &micAGC{
		gain:        1,
		enabled:     true,
		gateEnabled: true,
		gateScale:   1,
		gateGain:    0,
		noiseRMSAvg: agcVoiceRMSFloor * 0.5,
		lastLogGain: 1,
		lastLogGate: 0,
		logEnabled:  os.Getenv("GD_DOOM_VOICE_AGC_LOG") != "",
	}
}

func (a *micAGC) SetEnabled(enabled bool) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.enabled = enabled
	a.mu.Unlock()
}

func (a *micAGC) SetGate(enabled bool, threshold float64) {
	if a == nil {
		return
	}
	if threshold <= 0 {
		threshold = 1
	}
	a.mu.Lock()
	a.gateEnabled = enabled
	a.gateScale = threshold
	a.mu.Unlock()
}

func (a *micAGC) GateActive() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.gateActive
}

func (a *micAGC) config() (agcEnabled bool, gateEnabled bool, scale float64) {
	if a == nil {
		return true, true, 1
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	agcEnabled = a.enabled
	gateEnabled = a.gateEnabled
	if a.gateScale <= 0 {
		return agcEnabled, gateEnabled, 1
	}
	return agcEnabled, gateEnabled, a.gateScale
}

func (a *micAGC) ProcessFrame(pcm []int16, sampleRate int) bool {
	if a == nil || len(pcm) == 0 || sampleRate <= 0 {
		return false
	}
	agcEnabled, gateEnabled, gateScale := a.config()
	lpAlpha := onePoleAlpha(agcLowPassCutoffHz, sampleRate)
	lowAlpha := onePoleAlpha(agcHighPassCutoffHz, sampleRate)
	var sumSq float64
	var peak float64
	zeroCrossings := 0
	groupRMS := make([]float64, groupCount(len(pcm), agcGateGroupSamples))
	groupSumSq := make([]float64, len(groupRMS))
	groupLens := make([]int, len(groupRMS))
	for i, sample := range pcm {
		x := float64(sample)
		a.lpState += lpAlpha * (x - a.lpState)
		a.lowState += lowAlpha * (a.lpState - a.lowState)
		band := a.lpState - a.lowState
		absBand := math.Abs(band)
		sumSq += band * band
		group := i / agcGateGroupSamples
		groupSumSq[group] += band * band
		groupLens[group]++
		if absBand > peak {
			peak = absBand
		}
		if a.havePrevBand && ((band >= 0 && a.prevBand < 0) || (band < 0 && a.prevBand >= 0)) {
			zeroCrossings++
		}
		a.prevBand = band
		a.havePrevBand = true
	}
	for i := range groupRMS {
		if groupLens[i] > 0 {
			groupRMS[i] = math.Sqrt(groupSumSq[i] / float64(groupLens[i]))
		}
	}
	rms := math.Sqrt(sumSq / float64(len(pcm)))
	crest := 0.0
	if rms > 0 {
		crest = peak / rms
	}
	noiseGate := max(a.noiseRMSAvg, 1)
	rawVoiced := rms >= max(agcVoiceRMSFloor, noiseGate*agcVoiceNoiseRatio) &&
		zeroCrossings >= 4 && zeroCrossings <= 160 && crest <= 12
	if rawVoiced {
		a.voiceHold = agcVoiceHoldFrames
	} else if a.voiceHold > 0 {
		a.voiceHold--
	}
	voiced := rawVoiced || a.voiceHold > 0
	if agcEnabled {
		if voiced {
			if a.voiceRMSAvg <= 0 {
				a.voiceRMSAvg = rms
			} else {
				a.voiceRMSAvg += (rms - a.voiceRMSAvg) * agcAverageSmoothing
			}
			target := clampFloat(agcTargetRMS/max(a.voiceRMSAvg, 1), agcMinGain, agcMaxGain)
			if peak > 0 && peak*a.gain > agcPeakLimit {
				a.gain = clampFloat(agcPeakLimit/peak, agcMinGain, agcMaxGain)
			} else if target > a.gain+agcGainDeadband {
				a.gain += (target - a.gain) * agcGainAttack
			} else if target < a.gain-agcGainDeadband {
				a.gain += (target - a.gain) * agcGainRelease
			}
		} else {
			if a.noiseRMSAvg <= 0 {
				a.noiseRMSAvg = rms
			} else {
				a.noiseRMSAvg += (rms - a.noiseRMSAvg) * agcNoiseSmoothing
			}
			a.gain += (1 - a.gain) * agcIdleReturn
		}
		a.gain = clampFloat(a.gain, agcMinGain, agcMaxGain)
	} else {
		a.gain = 1
		if !voiced {
			if a.noiseRMSAvg <= 0 {
				a.noiseRMSAvg = rms
			} else {
				a.noiseRMSAvg += (rms - a.noiseRMSAvg) * agcNoiseSmoothing
			}
		}
	}
	noiseAvg := max(a.noiseRMSAvg, 1)
	targetGate := gateGainForFrame(rawVoiced, a.voiceHold, rms, noiseAvg, gateEnabled, gateScale)
	groupGates := make([]float64, len(groupRMS))
	if !gateEnabled {
		a.gateGain = 1
		for i := range groupGates {
			groupGates[i] = 1
		}
	} else if targetGate == 0 && rms == 0 {
		a.gateGain = 0
	} else {
		desiredGates := desiredGateByGroup(a.gateGain, targetGate, rawVoiced, groupRMS, noiseAvg, gateEnabled, gateScale)
		for i := range desiredGates {
			if desiredGates[i] > a.gateGain {
				a.gateGain += (desiredGates[i] - a.gateGain) * agcGateAttack
			} else {
				a.gateGain += (desiredGates[i] - a.gateGain) * agcGateRelease
			}
			a.gateGain = clampFloat(a.gateGain, agcGateMinGain, 1)
			groupGates[i] = a.gateGain
		}
	}
	gateGain := a.gateGain
	a.mu.Lock()
	a.gateActive = gateEnabled && gateGain < 0.999
	a.mu.Unlock()
	a.frameCount++
	if a.shouldLog(voiced, gateGain) {
		fmt.Printf("mic-agc frame=%d voiced=%t hold=%d rms=%.1f voice_avg=%.1f noise=%.1f peak=%.1f gain=%.3f gate=%.3f\n",
			a.frameCount, voiced, a.voiceHold, rms, a.voiceRMSAvg, a.noiseRMSAvg, peak, a.gain, gateGain)
		a.lastLogGain = a.gain
		a.lastLogGate = gateGain
		a.lastLogVoice = voiced
	}
	if !rawVoiced && a.voiceHold <= 0 && gateGain <= agcGateSilentGain {
		a.gateGain = 0
		a.mu.Lock()
		a.gateActive = gateEnabled
		a.mu.Unlock()
		clear(pcm)
		return true
	}
	silent := true
	for i, sample := range pcm {
		group := i / agcGateGroupSamples
		groupGate := gateGain
		if len(groupGates) > 0 {
			groupGate = groupGates[group]
			if next := group + 1; next < len(groupGates) && groupLens[group] > 1 {
				pos := i - group*agcGateGroupSamples
				t := float64(pos) / float64(groupLens[group]-1)
				groupGate += (groupGates[next] - groupGate) * t
			}
		}
		if groupGate > 0 {
			silent = false
		}
		v := int(math.Round(float64(sample) * a.gain * groupGate))
		switch {
		case v > math.MaxInt16:
			pcm[i] = math.MaxInt16
		case v < math.MinInt16:
			pcm[i] = math.MinInt16
		default:
			pcm[i] = int16(v)
		}
	}
	return silent
}

func onePoleAlpha(cutoffHz float64, sampleRate int) float64 {
	if cutoffHz <= 0 || sampleRate <= 0 {
		return 1
	}
	dt := 1 / float64(sampleRate)
	rc := 1 / (2 * math.Pi * cutoffHz)
	return dt / (rc + dt)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func groupCount(samples, groupSize int) int {
	if samples <= 0 || groupSize <= 0 {
		return 0
	}
	return (samples + groupSize - 1) / groupSize
}

func (a *micAGC) shouldLog(voiced bool, gateGain float64) bool {
	if a == nil {
		return false
	}
	if !a.logEnabled {
		return false
	}
	if a.frameCount <= 12 {
		return true
	}
	if voiced != a.lastLogVoice {
		return true
	}
	if math.Abs(a.gain-a.lastLogGain) >= 0.1 {
		return true
	}
	if math.Abs(gateGain-a.lastLogGate) >= 0.08 {
		return true
	}
	return a.frameCount%50 == 0
}

func softGateGain(rms, noiseAvg, scale float64) float64 {
	if scale <= 0 {
		scale = 1
	}
	noiseAvg = max(noiseAvg, 1)
	floor := noiseAvg * agcGateFloorRatio * scale
	open := noiseAvg * agcGateOpenRatio * scale
	if open <= floor {
		return 1
	}
	if rms <= floor {
		return agcGateMinGain
	}
	if rms >= open {
		return 1
	}
	x := (rms - floor) / (open - floor)
	x = x * x * (3 - 2*x)
	return agcGateMinGain + (1-agcGateMinGain)*x
}

func gateGainForFrame(rawVoiced bool, voiceHold int, rms, noiseAvg float64, enabled bool, scale float64) float64 {
	if !enabled {
		return 1
	}
	if rawVoiced {
		return 1
	}
	knee := softGateGain(rms, noiseAvg, scale)
	if voiceHold <= 0 || agcVoiceHoldFrames <= 0 {
		return knee
	}
	blend := float64(voiceHold) / float64(agcVoiceHoldFrames)
	if blend < 0 {
		blend = 0
	} else if blend > 1 {
		blend = 1
	}
	return knee + (1-knee)*blend
}

func desiredGateByGroup(startGate, targetGate float64, rawVoiced bool, groupRMS []float64, noiseAvg float64, enabled bool, scale float64) []float64 {
	if len(groupRMS) == 0 {
		return nil
	}
	out := make([]float64, len(groupRMS))
	for i := range out {
		out[i] = targetGate
	}
	if !rawVoiced || targetGate <= startGate {
		return out
	}
	if !enabled {
		return out
	}
	if scale <= 0 {
		scale = 1
	}
	onsetThreshold := max(agcVoiceRMSFloor*scale, noiseAvg*agcVoiceNoiseRatio*scale)
	onset, ok := sustainedSpeechOnset(groupRMS, onsetThreshold, agcGateSpeechGroups)
	if !ok {
		for i := range out {
			out[i] = startGate
		}
		return out
	}
	rampStart := onset - agcGateLookahead
	if rampStart < 0 {
		rampStart = 0
	}
	for i := 0; i < rampStart; i++ {
		out[i] = startGate
	}
	span := onset - rampStart + 1
	if span < 1 {
		span = 1
	}
	for i := rampStart; i <= onset && i < len(out); i++ {
		x := float64(i-rampStart+1) / float64(span+1)
		out[i] = startGate + (targetGate-startGate)*x
	}
	return out
}

func sustainedSpeechOnset(groupRMS []float64, threshold float64, minGroups int) (int, bool) {
	if len(groupRMS) == 0 {
		return 0, false
	}
	if minGroups <= 1 {
		for i, rms := range groupRMS {
			if rms >= threshold {
				return i, true
			}
		}
		return 0, false
	}
	runStart := -1
	runLen := 0
	for i, rms := range groupRMS {
		if rms >= threshold {
			if runLen == 0 {
				runStart = i
			}
			runLen++
			if runLen >= minGroups {
				return runStart, true
			}
			continue
		}
		runStart = -1
		runLen = 0
	}
	return 0, false
}
