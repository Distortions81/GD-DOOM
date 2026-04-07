package voicecodec

import (
	"math"
	"testing"
)

func TestEncoderLowPassSuppressesSibilantBandMoreThanVoiceBand(t *testing.T) {
	low := makeSine(2000, 12000, SampleRate)
	high := makeSine(8000, 12000, SampleRate)
	lowPassLow := newLowPassFilter(encoderLowPassCutoffHz, SampleRate)
	lowPassHigh := newLowPassFilter(encoderLowPassCutoffHz, SampleRate)

	lowOut := make([]int16, len(low))
	for i, sample := range low {
		lowOut[i] = lowPassLow.ProcessSample(sample)
	}
	highOut := make([]int16, len(high))
	for i, sample := range high {
		highOut[i] = lowPassHigh.ProcessSample(sample)
	}

	lowRMS := rmsInt16(lowOut)
	highRMS := rmsInt16(highOut)
	if highRMS >= lowRMS*0.85 {
		t.Fatalf("high-band rms after encode low-pass=%.1f want < %.1f", highRMS, lowRMS*0.85)
	}
}

func makeSine(freq float64, amp int16, sampleRate int) []int16 {
	out := make([]int16, sampleRate/5)
	for i := range out {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
		out[i] = int16(v * float64(amp))
	}
	return out
}

func rmsInt16(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range samples {
		v := float64(sample)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}
