package voicecodec

import (
	"math"
	"testing"
)

func TestOpusRoundTrip(t *testing.T) {
	enc, err := NewOpusEncoder()
	if err != nil {
		t.Fatalf("NewOpusEncoder() error = %v", err)
	}
	defer enc.Close()

	dec, err := NewOpusDecoder()
	if err != nil {
		t.Fatalf("NewOpusDecoder() error = %v", err)
	}
	defer dec.Close()

	pcm := make([]int16, FrameSamples)
	for i := range pcm {
		v := math.Sin(2 * math.Pi * 440 * float64(i) / SampleRate)
		pcm[i] = int16(v * 12000)
	}

	packet, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(packet) == 0 {
		t.Fatal("Encode() returned empty packet")
	}

	out, err := dec.Decode(packet)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(out) != FrameSamples {
		t.Fatalf("decoded samples=%d want=%d", len(out), FrameSamples)
	}
	nonZero := false
	for _, sample := range out {
		if sample != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("decoded output is silent")
	}
}
