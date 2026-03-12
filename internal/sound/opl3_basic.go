package sound

import "math"

const (
	opl3DefaultSampleRate = 49716
	opl3ChannelCount      = 18
	opl3OperatorCount     = 2

	oplEnvOff oplEnvStage = iota
	oplEnvAttack
	oplEnvDecay
	oplEnvSustain
	oplEnvRelease
)

const (
	operatorBaseGain = 0.7
	modulatorDepth   = 0.22
	channelMixGain   = 0.38
)

var (
	oplSlotToChannel = [22]int{
		0, 1, 2, 0, 1, 2, -1, -1,
		3, 4, 5, 3, 4, 5, -1, -1,
		6, 7, 8, 6, 7, 8,
	}
	oplSlotToOperator = [22]int{
		0, 0, 0, 1, 1, 1, -1, -1,
		0, 0, 0, 1, 1, 1, -1, -1,
		0, 0, 0, 1, 1, 1,
	}
	oplMultiples = [16]float64{
		0.5, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 10, 12, 12, 15, 15,
	}
	oplFeedbackScale = [8]float64{
		0.0, 0.03125, 0.0625, 0.125,
		0.25, 0.5, 1.0, 2.0,
	}
)

type oplEnvStage uint8

type basicOperatorState struct {
	phase        float64
	env          float64
	stage        oplEnvStage
	multiple     float64
	level        float64
	attackCoef   float64
	decayCoef    float64
	sustainLevel float64
	releaseCoef  float64
	waveform     int
	vibrato      bool
	tremolo      bool
	sustain      bool
}

type basicChannelState struct {
	keyOn    bool
	freq     float64
	additive bool
	panL     float64
	panR     float64
	feedback float64
	fbPrev   [2]float64
	ops      [opl3OperatorCount]basicOperatorState
}

// BasicOPL3 is a pure-Go OPL3-inspired synth for the subset of the chip this
// codebase drives: 2-op voices, operator envelopes, feedback, waveforms, pan,
// and DMX-style register writes.
type BasicOPL3 struct {
	sampleRate       int
	regs             [0x200]uint8
	ch               [opl3ChannelCount]basicChannelState
	waveformSelectOn bool
	vibPhase         float64
	tremPhase        float64
	stereoBuf        []int16
	monoBuf          []byte
}

// NewBasicOPL3 creates a pure-Go OPL3 fallback at the provided sample rate.
func NewBasicOPL3(sampleRate int) *BasicOPL3 {
	if sampleRate <= 0 {
		sampleRate = opl3DefaultSampleRate
	}
	o := &BasicOPL3{sampleRate: sampleRate}
	o.Reset()
	return o
}

// Reset clears all registers and runtime state.
func (o *BasicOPL3) Reset() {
	if o == nil {
		return
	}
	o.regs = [0x200]uint8{}
	o.ch = [opl3ChannelCount]basicChannelState{}
	o.waveformSelectOn = false
	o.vibPhase = 0
	o.tremPhase = 0
	for i := range o.ch {
		o.ch[i].panL = 1
		o.ch[i].panR = 1
		for op := range o.ch[i].ops {
			o.ch[i].ops[op] = basicOperatorState{
				multiple:     1,
				level:        0,
				attackCoef:   rateCoef(o.sampleRate, attackSeconds(15)),
				decayCoef:    rateCoef(o.sampleRate, decayReleaseSeconds(15)),
				sustainLevel: 0,
				releaseCoef:  rateCoef(o.sampleRate, decayReleaseSeconds(15)),
			}
		}
	}
}

// WriteReg applies a subset of OPL3 register writes.
func (o *BasicOPL3) WriteReg(addr uint16, value uint8) {
	if o == nil {
		return
	}
	a := int(addr & 0x1FF)
	o.regs[a] = value
	switch a {
	case 0x01:
		o.waveformSelectOn = (value & 0x20) != 0
		for ch := range o.ch {
			for op := 0; op < opl3OperatorCount; op++ {
				o.refreshOperator(ch, op)
			}
		}
		return
	}

	bank := 0
	off := a
	if a >= 0x100 {
		bank = 1
		off = a - 0x100
	}

	switch {
	case off >= 0x20 && off < 0x20+len(oplSlotToChannel):
		if ch, op, ok := decodeOperatorSlot(bank, off-0x20); ok {
			o.refreshOperator(ch, op)
		}
	case off >= 0x40 && off < 0x40+len(oplSlotToChannel):
		if ch, op, ok := decodeOperatorSlot(bank, off-0x40); ok {
			o.refreshOperator(ch, op)
		}
	case off >= 0x60 && off < 0x60+len(oplSlotToChannel):
		if ch, op, ok := decodeOperatorSlot(bank, off-0x60); ok {
			o.refreshOperator(ch, op)
		}
	case off >= 0x80 && off < 0x80+len(oplSlotToChannel):
		if ch, op, ok := decodeOperatorSlot(bank, off-0x80); ok {
			o.refreshOperator(ch, op)
		}
	case off >= 0xE0 && off < 0xE0+len(oplSlotToChannel):
		if ch, op, ok := decodeOperatorSlot(bank, off-0xE0); ok {
			o.refreshOperator(ch, op)
		}
	case off >= 0xA0 && off <= 0xA8:
		o.refreshChannelFreq(bank*9 + off - 0xA0)
	case off >= 0xB0 && off <= 0xB8:
		ch := bank*9 + off - 0xB0
		o.refreshChannelFreq(ch)
		keyOn := (value & 0x20) != 0
		if keyOn != o.ch[ch].keyOn {
			o.ch[ch].keyOn = keyOn
			if keyOn {
				o.keyOnChannel(ch)
			} else {
				o.keyOffChannel(ch)
			}
		}
	case off >= 0xC0 && off <= 0xC8:
		o.refreshChannelControl(bank*9 + off - 0xC0)
	}
}

// GenerateStereoS16 produces interleaved stereo signed-16 PCM.
// The returned slice is backed by an internal reusable buffer and is only
// valid until the next GenerateStereoS16/GenerateMonoU8 call on this instance.
func (o *BasicOPL3) GenerateStereoS16(frames int) []int16 {
	if o == nil || frames <= 0 || o.sampleRate <= 0 {
		return nil
	}
	need := frames * 2
	if cap(o.stereoBuf) < need {
		o.stereoBuf = make([]int16, need)
	} else {
		o.stereoBuf = o.stereoBuf[:need]
	}
	out := o.stereoBuf
	invSampleRate := 1.0 / float64(o.sampleRate)
	for i := 0; i < frames; i++ {
		o.vibPhase += 6.1 * invSampleRate
		if o.vibPhase >= 1 {
			o.vibPhase -= math.Floor(o.vibPhase)
		}
		o.tremPhase += 3.7 * invSampleRate
		if o.tremPhase >= 1 {
			o.tremPhase -= math.Floor(o.tremPhase)
		}
		var l, r float64
		for ch := 0; ch < opl3ChannelCount; ch++ {
			sl, sr := o.renderChannel(ch, invSampleRate)
			l += sl
			r += sr
		}
		if l < -1 {
			l = -1
		} else if l > 1 {
			l = 1
		}
		if r < -1 {
			r = -1
		} else if r > 1 {
			r = 1
		}
		out[i*2] = int16(l * 32767)
		out[i*2+1] = int16(r * 32767)
	}
	return out
}

// GenerateMonoU8 produces unsigned 8-bit mono PCM from the mixed stereo output.
// The returned slice is backed by an internal reusable buffer and is only
// valid until the next GenerateStereoS16/GenerateMonoU8 call on this instance.
func (o *BasicOPL3) GenerateMonoU8(frames int) []byte {
	st := o.GenerateStereoS16(frames)
	if len(st) == 0 {
		return nil
	}
	if cap(o.monoBuf) < frames {
		o.monoBuf = make([]byte, frames)
	} else {
		o.monoBuf = o.monoBuf[:frames]
	}
	out := o.monoBuf
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

func (o *BasicOPL3) renderChannel(ch int, invSampleRate float64) (float64, float64) {
	if ch < 0 || ch >= len(o.ch) {
		return 0, 0
	}
	c := &o.ch[ch]
	if c.freq <= 0 {
		return 0, 0
	}

	mod := &c.ops[0]
	car := &c.ops[1]
	modEnv := o.advanceEnvelope(mod)
	carEnv := o.advanceEnvelope(car)
	if mod.stage == oplEnvOff && car.stage == oplEnvOff {
		c.fbPrev = [2]float64{}
		return 0, 0
	}

	modMul := mod.multiple
	if modMul <= 0 {
		modMul = 1
	}
	carMul := car.multiple
	if carMul <= 0 {
		carMul = 1
	}
	modFreq := c.freq * modMul
	carFreq := c.freq * carMul
	vib := math.Sin(2 * math.Pi * o.vibPhase)
	if mod.vibrato {
		modFreq *= 1 + vib*0.0025
	}
	if car.vibrato {
		carFreq *= 1 + vib*0.0025
	}
	modStep := (2 * math.Pi * modFreq) * invSampleRate
	carStep := (2 * math.Pi * carFreq) * invSampleRate

	modTrem := 1.0
	carTrem := 1.0
	if mod.tremolo {
		modTrem = 1 - ((math.Sin(2*math.Pi*o.tremPhase)+1)*0.5)*0.18
	}
	if car.tremolo {
		carTrem = 1 - ((math.Sin(2*math.Pi*o.tremPhase)+1)*0.5)*0.18
	}
	modAmp := modEnv * mod.level * operatorBaseGain * modTrem
	carAmp := carEnv * car.level * operatorBaseGain * carTrem
	modFeedback := (c.fbPrev[0] + c.fbPrev[1]) * c.feedback * 0.18
	mod.phase = wrapPhase(mod.phase + modStep)
	modRaw := oplWaveform(mod.waveform, mod.phase+modFeedback)
	modSample := modRaw * modAmp
	c.fbPrev[1] = c.fbPrev[0]
	c.fbPrev[0] = modSample

	car.phase = wrapPhase(car.phase + carStep)
	carInput := 0.0
	out := 0.0
	if c.additive {
		out += modSample * 0.42
	} else {
		// Keep FM coloration without letting the modulator pull the perceived note
		// center too far away from the programmed carrier pitch.
		carInput = modRaw * modAmp * (modulatorDepth + modMul*0.04 + carMul*0.01)
	}
	carSample := oplWaveform(car.waveform, car.phase+carInput) * carAmp
	out += carSample
	return out * c.panL * channelMixGain, out * c.panR * channelMixGain
}

func (o *BasicOPL3) advanceEnvelope(op *basicOperatorState) float64 {
	if op == nil {
		return 0
	}
	switch op.stage {
	case oplEnvOff:
		op.env = 0
	case oplEnvAttack:
		op.env += (1 - op.env) * op.attackCoef
		if op.env >= 0.999 {
			op.env = 1
			op.stage = oplEnvDecay
		}
	case oplEnvDecay:
		op.env += (op.sustainLevel - op.env) * op.decayCoef
		if math.Abs(op.env-op.sustainLevel) <= 0.0005 {
			op.env = op.sustainLevel
			op.stage = oplEnvSustain
		}
	case oplEnvSustain:
		if op.sustain {
			op.env = op.sustainLevel
		} else {
			op.env += (0 - op.env) * op.releaseCoef
			if op.env <= 0.0001 {
				op.env = 0
				op.stage = oplEnvOff
			}
		}
	case oplEnvRelease:
		op.env += (0 - op.env) * op.releaseCoef
		if op.env <= 0.0001 {
			op.env = 0
			op.stage = oplEnvOff
		}
	}
	return op.env
}

func (o *BasicOPL3) keyOnChannel(ch int) {
	if ch < 0 || ch >= len(o.ch) {
		return
	}
	c := &o.ch[ch]
	c.fbPrev = [2]float64{}
	for op := 0; op < opl3OperatorCount; op++ {
		c.ops[op].phase = 0
		c.ops[op].env = 0
		c.ops[op].stage = oplEnvAttack
	}
}

func (o *BasicOPL3) keyOffChannel(ch int) {
	if ch < 0 || ch >= len(o.ch) {
		return
	}
	for op := 0; op < opl3OperatorCount; op++ {
		if o.ch[ch].ops[op].stage != oplEnvOff {
			o.ch[ch].ops[op].stage = oplEnvRelease
		}
	}
}

func (o *BasicOPL3) refreshChannelFreq(ch int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 {
		return
	}
	a := o.regs[base+0xA0+ci]
	b := o.regs[base+0xB0+ci]
	fnum := int(a) | (int(b&0x03) << 8)
	block := int((b >> 2) & 0x07)
	if fnum == 0 {
		o.ch[ch].freq = 0
		return
	}
	o.ch[ch].freq = float64(fnum) * math.Ldexp(49716.0/1048576.0, block)
}

func (o *BasicOPL3) refreshChannelControl(ch int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 {
		return
	}
	c0 := o.regs[base+0xC0+ci]
	o.ch[ch].additive = (c0 & 0x01) != 0
	o.ch[ch].feedback = oplFeedbackScale[(c0>>1)&0x07]
	left := (c0 & 0x10) != 0
	right := (c0 & 0x20) != 0
	switch {
	case left && right:
		o.ch[ch].panL, o.ch[ch].panR = 1, 1
	case left:
		o.ch[ch].panL, o.ch[ch].panR = 1, 0
	case right:
		o.ch[ch].panL, o.ch[ch].panR = 0, 1
	default:
		o.ch[ch].panL, o.ch[ch].panR = 1, 1
	}
}

func (o *BasicOPL3) refreshOperator(ch int, op int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 || op < 0 || op >= opl3OperatorCount {
		return
	}
	slot := oplSlotForChannelOp(ci, op)
	if slot < 0 {
		return
	}
	s := &o.ch[ch].ops[op]
	reg20 := o.regs[base+0x20+slot]
	reg40 := o.regs[base+0x40+slot]
	reg60 := o.regs[base+0x60+slot]
	reg80 := o.regs[base+0x80+slot]
	regE0 := o.regs[base+0xE0+slot]

	s.multiple = oplMultiples[reg20&0x0F]
	s.level = totalLevelScale(reg40)
	s.attackCoef = rateCoef(o.sampleRate, attackSeconds(int((reg60>>4)&0x0F)))
	s.decayCoef = rateCoef(o.sampleRate, decayReleaseSeconds(int(reg60&0x0F)))
	s.sustainLevel = sustainScale(int((reg80 >> 4) & 0x0F))
	s.releaseCoef = rateCoef(o.sampleRate, decayReleaseSeconds(int(reg80&0x0F)))
	s.tremolo = (reg20 & 0x80) != 0
	s.vibrato = (reg20 & 0x40) != 0
	s.sustain = (reg20 & 0x20) != 0
	if o.waveformSelectOn {
		s.waveform = int(regE0 & 0x07)
	} else {
		s.waveform = 0
	}
}

func decodeOperatorSlot(bank int, slot int) (ch int, op int, ok bool) {
	if slot < 0 || slot >= len(oplSlotToChannel) {
		return 0, 0, false
	}
	localCh := oplSlotToChannel[slot]
	localOp := oplSlotToOperator[slot]
	if localCh < 0 || localOp < 0 {
		return 0, 0, false
	}
	return bank*9 + localCh, localOp, true
}

func oplBaseAndChannel(ch int) (base int, ci int) {
	if ch < 0 || ch >= opl3ChannelCount {
		return 0, -1
	}
	if ch < 9 {
		return 0x000, ch
	}
	return 0x100, ch - 9
}

func oplSlotForChannelOp(ch int, op int) int {
	modSlots := [9]int{0, 1, 2, 8, 9, 10, 16, 17, 18}
	carSlots := [9]int{3, 4, 5, 11, 12, 13, 19, 20, 21}
	if ch < 0 || ch >= 9 {
		return -1
	}
	if op == 0 {
		return modSlots[ch]
	}
	return carSlots[ch]
}

func wrapPhase(v float64) float64 {
	if v < 2*math.Pi {
		return v
	}
	return math.Mod(v, 2*math.Pi)
}

func totalLevelScale(reg40 uint8) float64 {
	tl := float64(reg40 & 0x3F)
	db := tl * 0.75
	return math.Pow(10, -db/20)
}

func sustainScale(level int) float64 {
	if level < 0 {
		level = 0
	}
	if level > 15 {
		level = 15
	}
	return math.Pow(10, -(float64(level)*3.0)/20.0)
}

func attackSeconds(rate int) float64 {
	return scaledRateSeconds(rate, 0.0025, 0.55)
}

func decayReleaseSeconds(rate int) float64 {
	return scaledRateSeconds(rate, 0.012, 0.5)
}

func scaledRateSeconds(rate int, base float64, exp float64) float64 {
	if rate < 0 {
		rate = 0
	}
	if rate > 15 {
		rate = 15
	}
	return base * math.Exp2(float64(15-rate)*exp)
}

func rateCoef(sampleRate int, seconds float64) float64 {
	if sampleRate <= 0 || seconds <= 0 {
		return 1
	}
	return 1 - math.Exp(-1/(seconds*float64(sampleRate)))
}

func oplWaveform(wave int, phase float64) float64 {
	p := wrapPhase(phase)
	s := math.Sin(p)
	switch wave & 0x07 {
	case 0:
		return s
	case 1:
		if s < 0 {
			return 0
		}
		return s
	case 2:
		return math.Abs(s)*2 - 1
	case 3:
		if s < 0 {
			return -1
		}
		return s*2 - 1
	case 4:
		return 1 - 2*(p/(2*math.Pi))
	case 5:
		if s < 0 {
			return -1
		}
		return 1
	case 6:
		x := p / (2 * math.Pi)
		if x < 0.25 {
			return x * 4
		}
		if x < 0.75 {
			return 2 - x*4
		}
		return x*4 - 4
	default:
		return (s * 0.7) + (math.Sin(2*p) * 0.3)
	}
}
