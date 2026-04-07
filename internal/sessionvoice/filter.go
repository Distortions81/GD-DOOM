package sessionvoice

import (
	"math"

	"gddoom/internal/voicecodec"
)

type highPassFilter struct {
	alpha float64
	xPrev float64
	yPrev float64
}

type lowPassFilter struct {
	alpha float64
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

func newLowPassFilter(cutoffHz float64, sampleRate int) *lowPassFilter {
	if cutoffHz <= 0 || sampleRate <= 0 {
		return &lowPassFilter{}
	}
	rc := 1.0 / (2.0 * math.Pi * cutoffHz)
	dt := 1.0 / float64(sampleRate)
	return &lowPassFilter{
		alpha: dt / (rc + dt),
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

func (f *lowPassFilter) ProcessInt16(pcm []int16) {
	if f == nil || f.alpha <= 0 {
		return
	}
	for i, sample := range pcm {
		x := float64(sample)
		f.yPrev += f.alpha * (x - f.yPrev)
		pcm[i] = clampFilterSample(f.yPrev)
	}
}

func decimateBy2LowPass(src []int16, f1, f2 *lowPassFilter) []int16 {
	if len(src) == 0 {
		return nil
	}
	work := append([]int16(nil), src...)
	if f1 != nil {
		f1.ProcessInt16(work)
	}
	if f2 != nil {
		f2.ProcessInt16(work)
	}
	out := make([]int16, (len(work)+1)/2)
	write := 0
	for i := 0; i < len(work); i += 2 {
		out[write] = work[i]
		write++
	}
	return out
}

func downsampleCaptureToVoice(src []int16, targetSampleRate int, filters ...*lowPassFilter) []int16 {
	if len(src) == 0 {
		return nil
	}
	if voiceSampleRatesAreExactHalf(targetSampleRate) {
		work := append([]int16(nil), src...)
		for _, f := range filters {
			if f != nil {
				f.ProcessInt16(work)
			}
		}
		out := make([]int16, (len(work)+1)/2)
		write := 0
		for i := 0; i < len(work); i += 2 {
			out[write] = work[i]
			write++
		}
		return out
	}
	return resampleMonoLinear(src, voicecodec.CaptureSampleRate, targetSampleRate)
}

func voiceSampleRatesAreExactHalf(targetSampleRate int) bool {
	return targetSampleRate > 0 && voicecodec.CaptureSampleRate == targetSampleRate*2
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
