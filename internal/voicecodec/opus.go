package voicecodec

import (
	"fmt"

	opus "github.com/kazzmir/opus-go/opus"
)

const (
	CodecOpus           byte = 1
	SampleRate               = 48000
	Channels                 = 1
	FrameDurationMillis      = 20
	FrameSamples             = SampleRate * FrameDurationMillis / 1000
	DefaultBitrate           = 20000
	MaxPacketBytes           = 1500
)

type OpusEncoder struct {
	enc    *opus.Encoder
	packet []byte
}

func NewOpusEncoder() (*OpusEncoder, error) {
	enc, err := opus.NewEncoder(SampleRate, Channels, opus.ApplicationVoIP)
	if err != nil {
		return nil, err
	}
	if err := enc.SetBitrate(DefaultBitrate); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.SetVBR(true); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.SetComplexity(5); err != nil {
		_ = enc.Close()
		return nil, err
	}
	return &OpusEncoder{
		enc:    enc,
		packet: make([]byte, MaxPacketBytes),
	}, nil
}

func (e *OpusEncoder) Close() error {
	if e == nil || e.enc == nil {
		return nil
	}
	return e.enc.Close()
}

func (e *OpusEncoder) Encode(pcm []int16) ([]byte, error) {
	if e == nil || e.enc == nil {
		return nil, fmt.Errorf("opus encoder is not initialized")
	}
	if len(pcm) < FrameSamples*Channels {
		return nil, fmt.Errorf("pcm samples=%d want at least %d", len(pcm), FrameSamples*Channels)
	}
	n, err := e.enc.Encode(pcm[:FrameSamples*Channels], FrameSamples, e.packet)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), e.packet[:n]...), nil
}

type OpusDecoder struct {
	dec *opus.Decoder
	pcm []int16
}

func NewOpusDecoder() (*OpusDecoder, error) {
	dec, err := opus.NewDecoder(SampleRate, Channels)
	if err != nil {
		return nil, err
	}
	return &OpusDecoder{
		dec: dec,
		pcm: make([]int16, FrameSamples*Channels),
	}, nil
}

func (d *OpusDecoder) Close() error {
	if d == nil || d.dec == nil {
		return nil
	}
	return d.dec.Close()
}

func (d *OpusDecoder) Decode(packet []byte) ([]int16, error) {
	if d == nil || d.dec == nil {
		return nil, fmt.Errorf("opus decoder is not initialized")
	}
	n, err := d.dec.Decode(packet, d.pcm, FrameSamples, false)
	if err != nil {
		return nil, err
	}
	return append([]int16(nil), d.pcm[:n*Channels]...), nil
}
