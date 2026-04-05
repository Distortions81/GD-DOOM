package sessionvoice

import (
	"math"
	"testing"

	"gddoom/internal/voicecodec"
)

func makeSine(freq float64, amp int16) []int16 {
	out := make([]int16, voicecodec.CaptureFrameSamples*8)
	for i := range out {
		v := math.Sin(2 * math.Pi * freq * float64(i) / float64(voicecodec.CaptureSampleRate))
		out[i] = int16(v * float64(amp))
	}
	return out
}

func TestHighPassFilterSuppressesLowRumbleMoreThanVoiceBand(t *testing.T) {
	filter := newHighPassFilter(50, voicecodec.CaptureSampleRate)
	low := makeSine(20, 12000)
	high := makeSine(220, 12000)

	lowBefore := rmsInt16(low)
	highBefore := rmsInt16(high)
	filter.ProcessInt16(low)
	filter = newHighPassFilter(50, voicecodec.CaptureSampleRate)
	filter.ProcessInt16(high)
	lowAfter := rmsInt16(low)
	highAfter := rmsInt16(high)

	if lowAfter >= lowBefore*0.55 {
		t.Fatalf("20 Hz rms after=%.1f want < %.1f", lowAfter, lowBefore*0.55)
	}
	if highAfter <= highBefore*0.75 {
		t.Fatalf("220 Hz rms after=%.1f want > %.1f", highAfter, highBefore*0.75)
	}
}

func TestResampleMonoLinearDownTo32kUsesExpectedLength(t *testing.T) {
	src := makeSine(3000, 12000)
	out := resampleMonoLinear(src, voicecodec.CaptureSampleRate, voicecodec.SampleRate)
	want := len(src) * voicecodec.SampleRate / voicecodec.CaptureSampleRate
	if len(out) != want {
		t.Fatalf("len(out)=%d want %d", len(out), want)
	}
}
