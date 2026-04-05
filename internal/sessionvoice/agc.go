package sessionvoice

import (
	"fmt"
	"math"
	"os"
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
	agcLowPassCutoffHz  = 4000.0
	agcHighPassCutoffHz = 120.0
)

type micAGC struct {
	gain         float64
	voiceRMSAvg  float64
	noiseRMSAvg  float64
	lpState      float64
	lowState     float64
	prevBand     float64
	havePrevBand bool
	voiceHold    int
	frameCount   int
	lastLogGain  float64
	lastLogGate  float64
	lastLogVoice bool
	logEnabled   bool
}

func newMicAGC() *micAGC {
	return &micAGC{
		gain:        1,
		noiseRMSAvg: agcVoiceRMSFloor * 0.5,
		lastLogGain: 1,
		lastLogGate: 1,
		logEnabled:  os.Getenv("GD_DOOM_VOICE_AGC_LOG") != "",
	}
}

func (a *micAGC) ProcessFrame(pcm []int16, sampleRate int) {
	if a == nil || len(pcm) == 0 || sampleRate <= 0 {
		return
	}
	lpAlpha := onePoleAlpha(agcLowPassCutoffHz, sampleRate)
	lowAlpha := onePoleAlpha(agcHighPassCutoffHz, sampleRate)
	var sumSq float64
	var peak float64
	zeroCrossings := 0
	for _, sample := range pcm {
		x := float64(sample)
		a.lpState += lpAlpha * (x - a.lpState)
		a.lowState += lowAlpha * (a.lpState - a.lowState)
		band := a.lpState - a.lowState
		absBand := math.Abs(band)
		sumSq += band * band
		if absBand > peak {
			peak = absBand
		}
		if a.havePrevBand && ((band >= 0 && a.prevBand < 0) || (band < 0 && a.prevBand >= 0)) {
			zeroCrossings++
		}
		a.prevBand = band
		a.havePrevBand = true
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
	gateGain := gateGainForFrame(rawVoiced, a.voiceHold, rms, max(a.noiseRMSAvg, 1))
	a.frameCount++
	if a.shouldLog(voiced, gateGain) {
		fmt.Printf("mic-agc frame=%d voiced=%t hold=%d rms=%.1f voice_avg=%.1f noise=%.1f peak=%.1f gain=%.3f gate=%.3f\n",
			a.frameCount, voiced, a.voiceHold, rms, a.voiceRMSAvg, a.noiseRMSAvg, peak, a.gain, gateGain)
		a.lastLogGain = a.gain
		a.lastLogGate = gateGain
		a.lastLogVoice = voiced
	}
	for i, sample := range pcm {
		v := int(math.Round(float64(sample) * a.gain * gateGain))
		switch {
		case v > math.MaxInt16:
			pcm[i] = math.MaxInt16
		case v < math.MinInt16:
			pcm[i] = math.MinInt16
		default:
			pcm[i] = int16(v)
		}
	}
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

func softGateGain(rms, noiseAvg float64) float64 {
	noiseAvg = max(noiseAvg, 1)
	floor := noiseAvg * agcGateFloorRatio
	open := noiseAvg * agcGateOpenRatio
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

func gateGainForFrame(rawVoiced bool, voiceHold int, rms, noiseAvg float64) float64 {
	if rawVoiced {
		return 1
	}
	knee := softGateGain(rms, noiseAvg)
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
