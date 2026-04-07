package voicecodec

import (
	"encoding/binary"
	"fmt"

	"gddoom/internal/voicecodec/silkc"
)

type SilkEncoder struct {
	sampleRate           int
	packetDurationMillis int
	bitrate              int
	state                *silkc.Encoder
}

type SilkDecoder struct {
	sampleRate int
	state      *silkc.Decoder
}

func NewSilkEncoder(sampleRate, packetDurationMillis, bitrate int) *SilkEncoder {
	if sampleRate <= 0 {
		sampleRate = SampleRate
	}
	if packetDurationMillis <= 0 {
		packetDurationMillis = SilkPacketDurationMillis
	}
	if bitrate <= 0 {
		bitrate = SilkDefaultBitrate
	}
	state, err := silkc.NewEncoder(sampleRate, bitrate)
	if err != nil {
		state = nil
	}
	return &SilkEncoder{
		sampleRate:           sampleRate,
		packetDurationMillis: packetDurationMillis,
		bitrate:              bitrate,
		state:                state,
	}
}

func NewSilkDecoder(sampleRate int) *SilkDecoder {
	if sampleRate <= 0 {
		sampleRate = SampleRate
	}
	state, err := silkc.NewDecoder(sampleRate)
	if err != nil {
		state = nil
	}
	return &SilkDecoder{sampleRate: sampleRate, state: state}
}

func (e *SilkEncoder) SetSampleRate(sampleRate int) {
	if e == nil || sampleRate <= 0 {
		return
	}
	e.sampleRate = sampleRate
	if e.state != nil {
		_ = e.state.SetSampleRate(sampleRate)
	}
}

func (e *SilkEncoder) SetPacketDurationMillis(packetDurationMillis int) {
	if e == nil || packetDurationMillis <= 0 {
		return
	}
	e.packetDurationMillis = packetDurationMillis
}

func (e *SilkEncoder) SetBitrate(bitrate int) {
	if e == nil || bitrate <= 0 {
		return
	}
	e.bitrate = bitrate
	if e.state != nil {
		e.state.SetBitrate(bitrate)
	}
}

func (d *SilkDecoder) SetSampleRate(sampleRate int) {
	if d == nil || sampleRate <= 0 {
		return
	}
	d.sampleRate = sampleRate
	if d.state != nil {
		_ = d.state.SetSampleRate(sampleRate)
	}
}

func (e *SilkEncoder) Reset() error {
	if e == nil || e.state == nil {
		return fmt.Errorf("silk encoder is nil")
	}
	return e.state.Reset()
}

func (d *SilkDecoder) Reset() error {
	if d == nil || d.state == nil {
		return fmt.Errorf("silk decoder is nil")
	}
	return d.state.Reset()
}

func (e *SilkEncoder) Encode(pcm []int16) ([]byte, error) {
	if e == nil || e.state == nil {
		return nil, fmt.Errorf("silk encoder is nil")
	}
	packetSamples, err := PacketSamplesFor(e.sampleRate, e.packetDurationMillis)
	if err != nil {
		return nil, err
	}
	if len(pcm) < packetSamples*Channels {
		return nil, fmt.Errorf("pcm samples=%d want at least %d", len(pcm), packetSamples*Channels)
	}

	raw := make([]byte, packetSamples*Channels*2)
	for i, sample := range pcm[:packetSamples*Channels] {
		binary.LittleEndian.PutUint16(raw[i*2:i*2+2], uint16(sample))
	}
	encoded, err := e.state.EncodePCM16LE(raw)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (d *SilkDecoder) Decode(packet []byte) ([]int16, error) {
	if d == nil || d.state == nil {
		return nil, fmt.Errorf("silk decoder is nil")
	}
	decoded, err := d.state.DecodePCM16LE(packet)
	if err != nil {
		return nil, err
	}
	if len(decoded)%2 != 0 {
		return nil, fmt.Errorf("silk decoded pcm len=%d must be even", len(decoded))
	}
	out := make([]int16, len(decoded)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(decoded[i*2 : i*2+2]))
	}
	return out, nil
}
