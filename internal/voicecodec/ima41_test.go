package voicecodec

import (
	"math"
	"testing"
)

func TestIMA41RoundTrip(t *testing.T) {
	enc := NewIMA41Encoder()
	dec := NewIMA41Decoder()

	pcm := make([]int16, FrameSamples*2)
	for i := range pcm {
		v := math.Sin(2 * math.Pi * 440 * float64(i) / SampleRate)
		pcm[i] = int16(v * 12000)
	}

	first, err := enc.Encode(pcm[:FrameSamples])
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got, want := len(first), IMA41PacketBytes+4; got != want {
		t.Fatalf("first packet len=%d want=%d", got, want)
	}

	second, err := enc.Encode(pcm[FrameSamples:])
	if err != nil {
		t.Fatalf("Encode() second frame error = %v", err)
	}
	if got, want := len(second), IMA41PacketBytes; got != want {
		t.Fatalf("second packet len=%d want=%d", got, want)
	}

	out, err := dec.Decode(first)
	if err != nil {
		t.Fatalf("Decode() first frame error = %v", err)
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

	out, err = dec.Decode(second)
	if err != nil {
		t.Fatalf("Decode() second frame error = %v", err)
	}
	if len(out) != FrameSamples {
		t.Fatalf("decoded second-frame samples=%d want=%d", len(out), FrameSamples)
	}
}

func TestIMA41ResetProducesSeededPacket(t *testing.T) {
	enc := NewIMA41Encoder()
	pcm := make([]int16, FrameSamples)

	packet, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if got, want := len(packet), IMA41PacketBytes+4; got != want {
		t.Fatalf("seed packet len=%d want=%d", got, want)
	}

	packet, err = enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() second error = %v", err)
	}
	if got, want := len(packet), IMA41PacketBytes; got != want {
		t.Fatalf("delta packet len=%d want=%d", got, want)
	}

	enc.Reset()
	packet, err = enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() after reset error = %v", err)
	}
	if got, want := len(packet), IMA41PacketBytes+4; got != want {
		t.Fatalf("reset packet len=%d want=%d", got, want)
	}
}
