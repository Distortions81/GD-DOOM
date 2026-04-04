package sessionvoice

import "math"

const (
	agcTargetRMS        = 9000.0
	agcMinGain          = 0.5
	agcMaxGain          = 6.0
	agcVoiceRMSFloor    = 350.0
	agcAttack           = 0.08
	agcRelease          = 0.02
	agcIdleReturn       = 0.01
	agcLowPassCutoffHz  = 4000.0
	agcHighPassCutoffHz = 120.0
)

type micAGC struct {
	gain        float64
	lpState     float64
	lowState    float64
	prevBand    float64
	havePrevBand bool
}

func newMicAGC() *micAGC {
	return &micAGC{gain: 1}
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
	voiced := rms >= agcVoiceRMSFloor && zeroCrossings >= 4 && zeroCrossings <= 160 && crest <= 12
	if voiced {
		target := clampFloat(agcTargetRMS/max(rms, 1), agcMinGain, agcMaxGain)
		rate := agcRelease
		if target > a.gain {
			rate = agcAttack
		}
		a.gain += (target - a.gain) * rate
	} else {
		a.gain += (1 - a.gain) * agcIdleReturn
	}
	a.gain = clampFloat(a.gain, agcMinGain, agcMaxGain)
	for i, sample := range pcm {
		v := int(math.Round(float64(sample) * a.gain))
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

