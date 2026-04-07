package voicecodec

import "math"

const encoderLowPassCutoffHz = 6500.0

type lowPassFilter struct {
	alpha float64
	yPrev float64
}

func newLowPassFilter(cutoffHz float64, sampleRate int) lowPassFilter {
	if sampleRate <= 0 {
		return lowPassFilter{}
	}
	nyquistSafeCutoff := 0.45 * float64(sampleRate)
	if nyquistSafeCutoff <= 0 {
		return lowPassFilter{}
	}
	if cutoffHz <= 0 || cutoffHz > nyquistSafeCutoff {
		cutoffHz = nyquistSafeCutoff
	}
	rc := 1.0 / (2.0 * math.Pi * cutoffHz)
	dt := 1.0 / float64(sampleRate)
	return lowPassFilter{alpha: dt / (rc + dt)}
}

func (f *lowPassFilter) Reset() {
	if f == nil {
		return
	}
	f.yPrev = 0
}

func (f *lowPassFilter) ProcessSample(sample int16) int16 {
	if f == nil || f.alpha <= 0 {
		return sample
	}
	x := float64(sample)
	f.yPrev += f.alpha * (x - f.yPrev)
	return clampPCM16(f.yPrev)
}

func encoderSampleRate(packetSamples int) int {
	if packetSamples <= 0 || PacketDurationMillis <= 0 {
		return SampleRate
	}
	sampleRate := packetSamples * 1000 / PacketDurationMillis
	if sampleRate <= 0 {
		return SampleRate
	}
	return sampleRate
}

func clampPCM16(v float64) int16 {
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
