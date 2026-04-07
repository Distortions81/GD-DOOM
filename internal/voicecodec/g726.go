package voicecodec

import (
	"fmt"

	"github.com/lkmio/g726"
)

type G726Encoder struct {
	packetSamples int
	bitsPerSample int
	state         *g726.G726_state
}

type G726Decoder struct {
	packetSamples int
	bitsPerSample int
	state         *g726.G726_state
}

func NewG726Encoder(packetSamples, bitsPerSample int) *G726Encoder {
	if packetSamples <= 0 {
		packetSamples = PacketSamples
	}
	enc := &G726Encoder{packetSamples: packetSamples, bitsPerSample: NormalizeG726BitsPerSample(bitsPerSample)}
	enc.Reset()
	return enc
}

func NewG726Decoder(packetSamples, bitsPerSample int) *G726Decoder {
	if packetSamples <= 0 {
		packetSamples = PacketSamples
	}
	dec := &G726Decoder{packetSamples: packetSamples, bitsPerSample: NormalizeG726BitsPerSample(bitsPerSample)}
	dec.Reset()
	return dec
}

func NewG72632Encoder(packetSamples int) *G726Encoder {
	return NewG726Encoder(packetSamples, 4)
}

func NewG72632Decoder(packetSamples int) *G726Decoder {
	return NewG726Decoder(packetSamples, 4)
}

func (e *G726Encoder) PacketSamples() int {
	if e == nil || e.packetSamples <= 0 {
		return PacketSamples
	}
	return e.packetSamples
}

func (d *G726Decoder) PacketSamples() int {
	if d == nil || d.packetSamples <= 0 {
		return PacketSamples
	}
	return d.packetSamples
}

func (e *G726Encoder) BitsPerSample() int {
	if e == nil {
		return 4
	}
	return NormalizeG726BitsPerSample(e.bitsPerSample)
}

func (d *G726Decoder) BitsPerSample() int {
	if d == nil {
		return 4
	}
	return NormalizeG726BitsPerSample(d.bitsPerSample)
}

func (e *G726Encoder) SetPacketSamples(packetSamples int) {
	if e == nil {
		return
	}
	if packetSamples <= 0 {
		packetSamples = PacketSamples
	}
	e.packetSamples = packetSamples
	e.Reset()
}

func (d *G726Decoder) SetPacketSamples(packetSamples int) {
	if d == nil {
		return
	}
	if packetSamples <= 0 {
		packetSamples = PacketSamples
	}
	d.packetSamples = packetSamples
	d.Reset()
}

func (e *G726Encoder) SetBitsPerSample(bitsPerSample int) {
	if e == nil {
		return
	}
	e.bitsPerSample = NormalizeG726BitsPerSample(bitsPerSample)
	e.Reset()
}

func (d *G726Decoder) SetBitsPerSample(bitsPerSample int) {
	if d == nil {
		return
	}
	d.bitsPerSample = NormalizeG726BitsPerSample(bitsPerSample)
	d.Reset()
}

func (e *G726Encoder) Reset() {
	if e == nil {
		return
	}
	e.state = g726.G726_init_state(g726RateForBits(e.BitsPerSample()))
}

func (d *G726Decoder) Reset() {
	if d == nil {
		return
	}
	d.state = g726.G726_init_state(g726RateForBits(d.BitsPerSample()))
}

func (e *G726Encoder) Encode(pcm []int16) ([]byte, error) {
	packetSamples := e.PacketSamples()
	if len(pcm) < packetSamples*Channels {
		return nil, fmt.Errorf("pcm samples=%d want at least %d", len(pcm), packetSamples*Channels)
	}
	if e.state == nil {
		e.Reset()
	}
	return e.state.Encode(pcm[:packetSamples*Channels])
}

func (d *G726Decoder) Decode(packet []byte) ([]int16, error) {
	packetSamples := d.PacketSamples()
	packetBytes, err := G726PacketBytes(packetSamples, Channels, d.BitsPerSample())
	if err != nil {
		return nil, err
	}
	if len(packet) != packetBytes {
		return nil, fmt.Errorf("g726 packet len=%d want=%d", len(packet), packetBytes)
	}
	if d.state == nil {
		d.Reset()
	}
	out, err := d.state.Decode(packet)
	if err != nil {
		return nil, err
	}
	if len(out) != packetSamples*Channels {
		return nil, fmt.Errorf("g726 decoded samples=%d want=%d", len(out), packetSamples*Channels)
	}
	return out, nil
}

func g726RateForBits(bitsPerSample int) g726.G726Rate {
	switch NormalizeG726BitsPerSample(bitsPerSample) {
	case 2:
		return g726.G726Rate16kbps
	case 3:
		return g726.G726Rate24kbps
	case 5:
		return g726.G726Rate40kbps
	default:
		return g726.G726Rate32kbps
	}
}
