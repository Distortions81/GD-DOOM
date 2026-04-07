package voicecodec

import (
	"math"
	"testing"
)

func TestSilkEncodeDecodeRoundTrip(t *testing.T) {
	packetSamples, err := PacketSamplesFor(24000, SilkPacketDurationMillis)
	if err != nil {
		t.Fatalf("PacketSamplesFor() error = %v", err)
	}
	pcm := make([]int16, packetSamples)
	for i := range pcm {
		v := 0.35 * math.Sin(2*math.Pi*440*float64(i)/24000)
		pcm[i] = int16(v * 32767)
	}

	enc := NewSilkEncoder(24000, SilkPacketDurationMillis, SilkDefaultBitrate)
	packet, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if len(packet) == 0 {
		t.Fatal("Encode() produced empty packet")
	}

	dec := NewSilkDecoder(24000)
	got, err := dec.Decode(packet)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(got) != len(pcm) {
		t.Fatalf("decoded samples=%d want %d", len(got), len(pcm))
	}
	if silkRMSInt16(got) <= 0 {
		t.Fatal("decoded packet is silent")
	}
}

func silkRMSInt16(src []int16) float64 {
	if len(src) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range src {
		v := float64(sample)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(src)))
}
