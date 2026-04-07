package voicecodec

import (
	"math"
	"testing"
)

func TestG726RoundTripAcrossBitDepths(t *testing.T) {
	for _, bits := range []int{2, 3, 4, 5} {
		enc := NewG726Encoder(PacketSamples, bits)
		dec := NewG726Decoder(PacketSamples, bits)

		pcm := make([]int16, PacketSamples)
		for i := range pcm {
			v := math.Sin(2 * math.Pi * 440 * float64(i) / SampleRate)
			pcm[i] = int16(v * 12000)
		}

		packet, err := enc.Encode(pcm)
		if err != nil {
			t.Fatalf("bits=%d Encode() error = %v", bits, err)
		}
		wantLen, err := G726PacketBytes(PacketSamples, Channels, bits)
		if err != nil {
			t.Fatalf("bits=%d packet bytes: %v", bits, err)
		}
		if got := len(packet); got != wantLen {
			t.Fatalf("bits=%d packet len=%d want=%d", bits, got, wantLen)
		}

		out, err := dec.Decode(packet)
		if err != nil {
			t.Fatalf("bits=%d Decode() error = %v", bits, err)
		}
		if len(out) != PacketSamples {
			t.Fatalf("bits=%d decoded samples=%d want=%d", bits, len(out), PacketSamples)
		}
	}
}

func TestG726RoundTripWithNonDefaultPacketSamples(t *testing.T) {
	const packetSamples = 480
	for _, bits := range []int{2, 3, 4, 5} {
		enc := NewG726Encoder(packetSamples, bits)
		dec := NewG726Decoder(packetSamples, bits)

		pcm := make([]int16, packetSamples)
		for i := range pcm {
			v := math.Sin(2 * math.Pi * 330 * float64(i) / 16000.0)
			pcm[i] = int16(v * 9000)
		}

		packet, err := enc.Encode(pcm)
		if err != nil {
			t.Fatalf("bits=%d Encode() error = %v", bits, err)
		}
		wantLen, err := G726PacketBytes(packetSamples, Channels, bits)
		if err != nil {
			t.Fatalf("bits=%d packet bytes: %v", bits, err)
		}
		if got := len(packet); got != wantLen {
			t.Fatalf("bits=%d packet len=%d want=%d", bits, got, wantLen)
		}
		out, err := dec.Decode(packet)
		if err != nil {
			t.Fatalf("bits=%d Decode() error = %v", bits, err)
		}
		if len(out) != packetSamples {
			t.Fatalf("bits=%d decoded samples=%d want=%d", bits, len(out), packetSamples)
		}
	}
}
