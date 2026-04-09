//go:build js && wasm

package voicecodec

import "fmt"

var errSilkUnsupported = fmt.Errorf("silk codec is not supported on js/wasm")

type SilkEncoder struct {
	sampleRate           int
	packetDurationMillis int
	bitrate              int
}

type SilkDecoder struct {
	sampleRate int
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
	return &SilkEncoder{
		sampleRate:           sampleRate,
		packetDurationMillis: packetDurationMillis,
		bitrate:              bitrate,
	}
}

func NewSilkDecoder(sampleRate int) *SilkDecoder {
	if sampleRate <= 0 {
		sampleRate = SampleRate
	}
	return &SilkDecoder{sampleRate: sampleRate}
}

func (e *SilkEncoder) SetSampleRate(sampleRate int) {
	if e == nil || sampleRate <= 0 {
		return
	}
	e.sampleRate = sampleRate
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
}

func (d *SilkDecoder) SetSampleRate(sampleRate int) {
	if d == nil || sampleRate <= 0 {
		return
	}
	d.sampleRate = sampleRate
}

func (e *SilkEncoder) Reset() error {
	if e == nil {
		return fmt.Errorf("silk encoder is nil")
	}
	return errSilkUnsupported
}

func (d *SilkDecoder) Reset() error {
	if d == nil {
		return fmt.Errorf("silk decoder is nil")
	}
	return errSilkUnsupported
}

func (e *SilkEncoder) Encode(pcm []int16) ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("silk encoder is nil")
	}
	return nil, errSilkUnsupported
}

func (d *SilkDecoder) Decode(packet []byte) ([]int16, error) {
	if d == nil {
		return nil, fmt.Errorf("silk decoder is nil")
	}
	return nil, errSilkUnsupported
}
