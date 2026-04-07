package silkc

/*
#cgo CFLAGS: -Wno-shift-negative-value -Wno-constant-conversion
#include "SKP_Silk_SDK_API.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

const (
	defaultSampleRate    = 24000
	maxBytesPerFrame     = 250
	maxInputFrames       = 5
	frameLengthMillis    = 20
	maxAPISampleRateKHz  = 48
	defaultComplexity    = 2
	defaultBitrate       = 25000
	maxDecoderFrameBytes = (frameLengthMillis * maxAPISampleRateKHz) << 2
)

type Encoder struct {
	state      unsafe.Pointer
	free       func()
	sampleRate int
	bitrate    int
}

type Decoder struct {
	state      unsafe.Pointer
	free       func()
	sampleRate int
}

func NewEncoder(sampleRate, bitrate int) (*Encoder, error) {
	if sampleRate <= 0 {
		sampleRate = defaultSampleRate
	}
	if sampleRate > maxAPISampleRateKHz*1000 {
		return nil, fmt.Errorf("sample rate %d out of range", sampleRate)
	}
	if bitrate <= 0 {
		bitrate = defaultBitrate
	}
	size := getEncoderSize()
	state, free := malloc(size)
	C.SKP_Silk_SDK_InitEncoder(state, (*C.SKP_SILK_SDK_EncControlStruct)(unsafe.Pointer(&C.SKP_SILK_SDK_EncControlStruct{})))
	return &Encoder{
		state:      state,
		free:       free,
		sampleRate: sampleRate,
		bitrate:    bitrate,
	}, nil
}

func NewDecoder(sampleRate int) (*Decoder, error) {
	if sampleRate <= 0 {
		sampleRate = defaultSampleRate
	}
	if sampleRate > maxAPISampleRateKHz*1000 {
		return nil, fmt.Errorf("sample rate %d out of range", sampleRate)
	}
	size := getDecoderSize()
	state, free := malloc(size)
	C.SKP_Silk_SDK_InitDecoder(state)
	return &Decoder{
		state:      state,
		free:       free,
		sampleRate: sampleRate,
	}, nil
}

func (e *Encoder) Close() {
	if e == nil || e.free == nil {
		return
	}
	e.free()
	e.free = nil
	e.state = nil
}

func (d *Decoder) Close() {
	if d == nil || d.free == nil {
		return
	}
	d.free()
	d.free = nil
	d.state = nil
}

func (e *Encoder) SetSampleRate(sampleRate int) error {
	if e == nil {
		return fmt.Errorf("encoder is nil")
	}
	if sampleRate <= 0 || sampleRate > maxAPISampleRateKHz*1000 {
		return fmt.Errorf("sample rate %d out of range", sampleRate)
	}
	if e.sampleRate == sampleRate {
		return nil
	}
	e.sampleRate = sampleRate
	return e.reset()
}

func (e *Encoder) SetBitrate(bitrate int) {
	if e == nil || bitrate <= 0 {
		return
	}
	e.bitrate = bitrate
}

func (d *Decoder) SetSampleRate(sampleRate int) error {
	if d == nil {
		return fmt.Errorf("decoder is nil")
	}
	if sampleRate <= 0 || sampleRate > maxAPISampleRateKHz*1000 {
		return fmt.Errorf("sample rate %d out of range", sampleRate)
	}
	if d.sampleRate == sampleRate {
		return nil
	}
	d.sampleRate = sampleRate
	return d.reset()
}

func (e *Encoder) Reset() error {
	if e == nil {
		return nil
	}
	return e.reset()
}

func (d *Decoder) Reset() error {
	if d == nil {
		return nil
	}
	return d.reset()
}

func (e *Encoder) reset() error {
	if e.state == nil {
		return fmt.Errorf("encoder state is nil")
	}
	var encStatus C.SKP_SILK_SDK_EncControlStruct
	C.SKP_Silk_SDK_InitEncoder(e.state, &encStatus)
	return nil
}

func (d *Decoder) reset() error {
	if d.state == nil {
		return fmt.Errorf("decoder state is nil")
	}
	C.SKP_Silk_SDK_InitDecoder(d.state)
	return nil
}

func (e *Encoder) EncodePCM16LE(raw []byte) ([]byte, error) {
	if e == nil || e.state == nil {
		return nil, fmt.Errorf("encoder state is nil")
	}
	frameBytes := frameLengthMillis * e.sampleRate / 1000 * 2
	if len(raw) != frameBytes {
		return nil, fmt.Errorf("pcm bytes=%d want %d", len(raw), frameBytes)
	}
	encControl := buildEncControl(e.sampleRate, e.bitrate)
	nBytes := C.SKP_int16(maxBytesPerFrame * maxInputFrames)
	payload := make([]byte, int(nBytes))
	ret := C.SKP_Silk_SDK_Encode(
		e.state,
		encControl,
		(*C.SKP_int16)(unsafe.Pointer(&raw[0])),
		C.SKP_int(len(raw)/2),
		(*C.SKP_uint8)(unsafe.Pointer(&payload[0])),
		(*C.SKP_int16)(unsafe.Pointer(&nBytes)),
	)
	if ret != 0 {
		return nil, fmt.Errorf("silk encode failed ret=%d", int(ret))
	}
	return append([]byte(nil), payload[:int(nBytes)]...), nil
}

func (d *Decoder) DecodePCM16LE(packet []byte) ([]byte, error) {
	if d == nil || d.state == nil {
		return nil, fmt.Errorf("decoder state is nil")
	}
	if len(packet) == 0 {
		return nil, fmt.Errorf("packet is empty")
	}
	decControl := C.SKP_SILK_SDK_DecControlStruct{}
	decControl.API_sampleRate = C.SKP_int32(d.sampleRate)
	decControl.framesPerPacket = C.SKP_int(1)
	buf := make([]byte, maxDecoderFrameBytes)
	nSamples := C.SKP_int16(len(buf) / 2)
	C.SKP_Silk_SDK_Decode(
		d.state,
		&decControl,
		0,
		(*C.SKP_uint8)(unsafe.Pointer(&packet[0])),
		C.SKP_int(len(packet)),
		(*C.SKP_int16)(unsafe.Pointer(&buf[0])),
		&nSamples,
	)
	if nSamples <= 0 {
		return nil, fmt.Errorf("silk decode produced no samples")
	}
	return append([]byte(nil), buf[:int(nSamples)*2]...), nil
}

func buildEncControl(sampleRate, bitrate int) *C.SKP_SILK_SDK_EncControlStruct {
	encControl := &C.SKP_SILK_SDK_EncControlStruct{}
	encControl.API_sampleRate = C.SKP_int32(sampleRate)
	encControl.maxInternalSampleRate = C.SKP_int32(defaultSampleRate)
	if sampleRate < defaultSampleRate {
		encControl.maxInternalSampleRate = C.SKP_int32(sampleRate)
	}
	encControl.packetSize = C.SKP_int(frameLengthMillis * sampleRate / 1000)
	encControl.packetLossPercentage = 0
	encControl.useInBandFEC = 0
	encControl.useDTX = 0
	encControl.complexity = C.SKP_int(defaultComplexity)
	encControl.bitRate = C.SKP_int32(bitrate)
	return encControl
}

func getEncoderSize() int32 {
	var size int32
	C.SKP_Silk_SDK_Get_Encoder_Size((*C.SKP_int32)(unsafe.Pointer(&size)))
	return size
}

func getDecoderSize() int32 {
	var size int32
	C.SKP_Silk_SDK_Get_Decoder_Size((*C.SKP_int32)(unsafe.Pointer(&size)))
	return size
}

func malloc(size int32) (unsafe.Pointer, func()) {
	p := C.malloc(C.ulong(size))
	return p, func() { C.free(p) }
}
