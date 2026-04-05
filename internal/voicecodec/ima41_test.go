package voicecodec

import (
	"math"
	"testing"
)

func TestIMA41RoundTrip(t *testing.T) {
	enc := NewIMA41Encoder()
	dec := NewIMA41Decoder()

	pcm := make([]int16, FrameSamples)
	for i := range pcm {
		v := math.Sin(2 * math.Pi * 440 * float64(i) / SampleRate)
		pcm[i] = int16(v * 12000)
	}

	packet, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got, want := len(packet), 4+(FrameSamples-1+1)/2; got != want {
		t.Fatalf("packet len=%d want=%d", got, want)
	}

	out, err := dec.Decode(packet)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(out) != FrameSamples {
		t.Fatalf("decoded samples=%d want=%d", len(out), FrameSamples)
	}
	nonZero := false
	var totalErr float64
	for i := range out {
		if out[i] != 0 {
			nonZero = true
		}
		totalErr += math.Abs(float64(out[i]) - float64(pcm[i]))
	}
	if !nonZero {
		t.Fatal("decoded output is silent")
	}
	if avgErr := totalErr / float64(len(out)); avgErr > 2500 {
		t.Fatalf("average absolute error=%0.2f too large", avgErr)
	}
}
