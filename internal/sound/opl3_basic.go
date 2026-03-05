package sound

import "math"

const (
	opl3DefaultSampleRate = 49716
	opl3ChannelCount      = 18
)

// BasicOPL3 is a lightweight OPL3-like oscillator bank with OPL register writes.
// It is intentionally minimal and non-cycle-accurate; it exists as a basic
// emulator scaffold until a full Nuked-OPL3 bridge is wired.
type BasicOPL3 struct {
	sampleRate int
	regs       [0x200]uint8
	phase      [opl3ChannelCount]float64
	keyOn      [opl3ChannelCount]bool
}

// NewBasicOPL3 creates a basic OPL3 emulator at the provided sample rate.
func NewBasicOPL3(sampleRate int) *BasicOPL3 {
	if sampleRate <= 0 {
		sampleRate = opl3DefaultSampleRate
	}
	return &BasicOPL3{sampleRate: sampleRate}
}

// Reset clears all registers and channel phases.
func (o *BasicOPL3) Reset() {
	if o == nil {
		return
	}
	o.regs = [0x200]uint8{}
	o.phase = [opl3ChannelCount]float64{}
	o.keyOn = [opl3ChannelCount]bool{}
}

// WriteReg applies a register write.
func (o *BasicOPL3) WriteReg(addr uint16, value uint8) {
	if o == nil {
		return
	}
	a := int(addr & 0x1FF)
	o.regs[a] = value
	bank := 0
	off := a
	if a >= 0x100 {
		bank = 1
		off = a - 0x100
	}
	if off >= 0xB0 && off <= 0xB8 {
		ch := bank*9 + (off - 0xB0)
		if ch >= 0 && ch < opl3ChannelCount {
			o.keyOn[ch] = (value & 0x20) != 0
		}
	}
}

// GenerateStereoS16 produces interleaved stereo signed-16 PCM.
func (o *BasicOPL3) GenerateStereoS16(frames int) []int16 {
	if o == nil || frames <= 0 || o.sampleRate <= 0 {
		return nil
	}
	out := make([]int16, frames*2)
	if len(out) == 0 {
		return out
	}
	invSampleRate := 1.0 / float64(o.sampleRate)
	for i := 0; i < frames; i++ {
		var l, r float64
		for ch := 0; ch < opl3ChannelCount; ch++ {
			if !o.keyOn[ch] {
				continue
			}
			freq := o.channelFreq(ch)
			if freq <= 0 {
				continue
			}
			amp := o.channelAmplitude(ch)
			if amp <= 0 {
				continue
			}
			panL, panR := o.channelPan(ch)
			o.phase[ch] += 2 * math.Pi * freq * invSampleRate
			if o.phase[ch] > 2*math.Pi {
				o.phase[ch] -= 2 * math.Pi
			}
			s := math.Sin(o.phase[ch]) * amp
			l += s * panL
			r += s * panR
		}
		// Conservative soft clip.
		l = math.Tanh(l * 1.4)
		r = math.Tanh(r * 1.4)
		out[i*2] = int16(l * 32767)
		out[i*2+1] = int16(r * 32767)
	}
	return out
}

// GenerateMonoU8 produces unsigned 8-bit mono PCM from the mixed stereo output.
func (o *BasicOPL3) GenerateMonoU8(frames int) []byte {
	st := o.GenerateStereoS16(frames)
	if len(st) == 0 {
		return nil
	}
	out := make([]byte, frames)
	for i := 0; i < frames; i++ {
		l := int(st[i*2])
		r := int(st[i*2+1])
		m := (l + r) / 2
		u := (m >> 8) + 128
		if u < 0 {
			u = 0
		} else if u > 255 {
			u = 255
		}
		out[i] = byte(u)
	}
	return out
}

func (o *BasicOPL3) channelFreq(ch int) float64 {
	base := 0
	if ch >= 9 {
		base = 0x100
		ch -= 9
	}
	if ch < 0 || ch > 8 {
		return 0
	}
	a := o.regs[base+0xA0+ch]
	b := o.regs[base+0xB0+ch]
	fnum := int(a) | (int(b&0x03) << 8)
	block := int((b >> 2) & 0x07)
	if fnum == 0 {
		return 0
	}
	// OPL family frequency estimate for FNUM/BLOCK.
	f := float64(fnum) * math.Ldexp(1.0, block) * 49716.0 / 1048576.0
	return f
}

func (o *BasicOPL3) channelAmplitude(ch int) float64 {
	// 2-op carrier slots for channels 0..8.
	carrierSlot := [9]int{3, 4, 5, 11, 12, 13, 19, 20, 21}
	base := 0
	if ch >= 9 {
		base = 0x100
		ch -= 9
	}
	if ch < 0 || ch > 8 {
		return 0
	}
	tlReg := base + 0x40 + carrierSlot[ch]
	tl := float64(o.regs[tlReg] & 0x3F)
	// Approximate 0.75 dB per TL step.
	db := tl * 0.75
	amp := math.Pow(10, -db/20)
	return amp * 0.18
}

func (o *BasicOPL3) channelPan(ch int) (float64, float64) {
	base := 0
	if ch >= 9 {
		base = 0x100
		ch -= 9
	}
	if ch < 0 || ch > 8 {
		return 1, 1
	}
	c0 := o.regs[base+0xC0+ch]
	left := (c0 & 0x10) != 0
	right := (c0 & 0x20) != 0
	switch {
	case left && right:
		return 1, 1
	case left:
		return 1, 0
	case right:
		return 0, 1
	default:
		return 1, 1
	}
}
