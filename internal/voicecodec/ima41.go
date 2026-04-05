package voicecodec

import "fmt"

var imaIndexTable = [16]int{
	-1, -1, -1, -1, 2, 4, 6, 8,
	-1, -1, -1, -1, 2, 4, 6, 8,
}

var imaStepTable = [89]int{
	7, 8, 9, 10, 11, 12, 13, 14, 16, 17,
	19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
	50, 55, 60, 66, 73, 80, 88, 97, 107, 118,
	130, 143, 157, 173, 190, 209, 230, 253, 279, 307,
	337, 371, 408, 449, 494, 544, 598, 658, 724, 796,
	876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066,
	2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358,
	5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
	15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767,
}

type IMA41Encoder struct {
	predictor int16
	stepIndex int
	needSeed  bool
	frames    int
}

type IMA41Decoder struct {
	predictor int16
	stepIndex int
}

const ima41SeedHeaderBytes = 4
const ima41ResyncIntervalFrames = 50

func NewIMA41Encoder() *IMA41Encoder {
	enc := &IMA41Encoder{}
	enc.Reset()
	return enc
}

func NewIMA41Decoder() *IMA41Decoder {
	return &IMA41Decoder{}
}

func (e *IMA41Encoder) Reset() {
	if e == nil {
		return
	}
	e.predictor = 0
	e.stepIndex = 0
	e.needSeed = true
	e.frames = 0
}

func (d *IMA41Decoder) Reset() {
	if d == nil {
		return
	}
	d.predictor = 0
	d.stepIndex = 0
}

func (e *IMA41Encoder) Encode(pcm []int16) ([]byte, error) {
	if len(pcm) < FrameSamples*Channels {
		return nil, fmt.Errorf("pcm samples=%d want at least %d", len(pcm), FrameSamples*Channels)
	}
	frame := pcm[:FrameSamples*Channels]
	if len(frame) == 0 {
		return nil, nil
	}
	seeded := e.needSeed || e.frames%ima41ResyncIntervalFrames == 0
	out := make([]byte, len(frame)/2)
	write := 0
	if seeded {
		out = make([]byte, ima41SeedHeaderBytes+len(frame)/2)
		putI16LE(out[0:2], e.predictor)
		out[2] = byte(e.stepIndex)
		out[3] = 0
		write = ima41SeedHeaderBytes
	}
	for i := 0; i < len(frame); i += 2 {
		lo := e.encodeNibble(frame[i])
		hi := byte(0)
		if i+1 < len(frame) {
			hi = e.encodeNibble(frame[i+1])
		}
		out[write] = lo | (hi << 4)
		write++
	}
	e.needSeed = false
	e.frames++
	return out, nil
}

func (d *IMA41Decoder) Decode(packet []byte) ([]int16, error) {
	payload := packet
	switch len(packet) {
	case IMA41PacketBytes:
	case ima41SeedHeaderBytes + IMA41PacketBytes:
		d.predictor = getI16LE(packet[0:2])
		d.stepIndex = int(packet[2])
		if d.stepIndex < 0 || d.stepIndex >= len(imaStepTable) {
			return nil, fmt.Errorf("ima 4:1 step index out of range: %d", d.stepIndex)
		}
		payload = packet[ima41SeedHeaderBytes:]
	default:
		return nil, fmt.Errorf("ima 4:1 packet len=%d want=%d or %d", len(packet), IMA41PacketBytes, ima41SeedHeaderBytes+IMA41PacketBytes)
	}
	out := make([]int16, FrameSamples*Channels)
	write := 0
	for _, b := range payload {
		if write < len(out) {
			out[write] = d.decodeNibble(b & 0x0f)
			write++
		}
		if write < len(out) {
			out[write] = d.decodeNibble((b >> 4) & 0x0f)
			write++
		}
	}
	for write < len(out) {
		out[write] = d.predictor
		write++
	}
	return out, nil
}

func (e *IMA41Encoder) encodeNibble(sample int16) byte {
	step := imaStepTable[e.stepIndex]
	diff := int(sample) - int(e.predictor)
	code := byte(0)
	if diff < 0 {
		code |= 0x08
		diff = -diff
	}
	delta := 0
	if diff >= step {
		code |= 0x04
		diff -= step
		delta += step
	}
	if diff >= step/2 {
		code |= 0x02
		diff -= step / 2
		delta += step / 2
	}
	if diff >= step/4 {
		code |= 0x01
		delta += step / 4
	}
	delta += step / 8
	e.applyDelta(code, delta)
	return code
}

func (d *IMA41Decoder) decodeNibble(code byte) int16 {
	step := imaStepTable[d.stepIndex]
	delta := step >> 3
	if code&0x04 != 0 {
		delta += step
	}
	if code&0x02 != 0 {
		delta += step >> 1
	}
	if code&0x01 != 0 {
		delta += step >> 2
	}
	d.applyDelta(code, delta)
	return d.predictor
}

func (e *IMA41Encoder) applyDelta(code byte, delta int) {
	predictor := int(e.predictor)
	if code&0x08 != 0 {
		predictor -= delta
	} else {
		predictor += delta
	}
	e.predictor = clamp16(predictor)
	e.stepIndex += imaIndexTable[code&0x0f]
	if e.stepIndex < 0 {
		e.stepIndex = 0
	}
	if e.stepIndex >= len(imaStepTable) {
		e.stepIndex = len(imaStepTable) - 1
	}
}

func (d *IMA41Decoder) applyDelta(code byte, delta int) {
	predictor := int(d.predictor)
	if code&0x08 != 0 {
		predictor -= delta
	} else {
		predictor += delta
	}
	d.predictor = clamp16(predictor)
	d.stepIndex += imaIndexTable[code&0x0f]
	if d.stepIndex < 0 {
		d.stepIndex = 0
	}
	if d.stepIndex >= len(imaStepTable) {
		d.stepIndex = len(imaStepTable) - 1
	}
}

func clamp16(v int) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

func putI16LE(dst []byte, v int16) {
	dst[0] = byte(v)
	dst[1] = byte(uint16(v) >> 8)
}

func getI16LE(src []byte) int16 {
	return int16(uint16(src[0]) | uint16(src[1])<<8)
}
