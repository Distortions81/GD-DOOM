package sessionvoice

import "math"

type highPassFilter struct {
	alpha float64
	xPrev float64
	yPrev float64
}

func newHighPassFilter(cutoffHz float64, sampleRate int) *highPassFilter {
	if cutoffHz <= 0 || sampleRate <= 0 {
		return &highPassFilter{}
	}
	rc := 1.0 / (2.0 * math.Pi * cutoffHz)
	dt := 1.0 / float64(sampleRate)
	return &highPassFilter{
		alpha: rc / (rc + dt),
	}
}

func (f *highPassFilter) ProcessInt16(pcm []int16) {
	if f == nil || f.alpha <= 0 {
		return
	}
	for i, sample := range pcm {
		x := float64(sample)
		y := f.alpha * (f.yPrev + x - f.xPrev)
		f.xPrev = x
		f.yPrev = y
		pcm[i] = clampFilterSample(y)
	}
}

func clampFilterSample(v float64) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	if v >= 0 {
		return int16(v + 0.5)
	}
	return int16(v - 0.5)
}
