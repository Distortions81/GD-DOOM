package sound

import "math"

const (
	opl3DefaultSampleRate = 49716
	opl3ChannelCount      = 18
	opl3OperatorCount     = 2

	oplWaveTableSize  = 1024
	oplWaveTableMask  = oplWaveTableSize - 1
	oplPhaseFracBits  = 9
	oplEnvelopeSilent = 0x1ff
	oplAttenTableSize = 2048

	oplChannelMixGain                      = 0.125
	oplPhaseModScale                       = 2032.0
	oplFeedbackPhaseScaleRatio             = 4.0
	oplEnvOff                  oplEnvStage = iota
	oplEnvAttack
	oplEnvDecay
	oplEnvSustain
	oplEnvRelease
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
	// Nuked's OPL frequency multiplier table is stored doubled.
	oplFrequencyMultiples = [16]uint32{
		1, 2, 4, 6, 8, 10, 12, 14,
		16, 18, 20, 20, 24, 24, 30, 30,
	}
	oplKSLROM = [16]uint8{
		0, 32, 40, 45, 48, 51, 53, 55,
		56, 58, 59, 60, 61, 62, 63, 64,
	}
	oplKSLShift  = [4]uint8{8, 1, 2, 0}
	oplEGIncStep = [4][4]uint8{
		{0, 0, 0, 0},
		{1, 0, 0, 0},
		{1, 0, 1, 0},
		{1, 1, 1, 0},
	}
	oplFeedbackPhaseScale = [8]float64{
		0,
		1 * oplFeedbackPhaseScaleRatio,
		2 * oplFeedbackPhaseScaleRatio,
		4 * oplFeedbackPhaseScaleRatio,
		8 * oplFeedbackPhaseScaleRatio,
		16 * oplFeedbackPhaseScaleRatio,
		32 * oplFeedbackPhaseScaleRatio,
		64 * oplFeedbackPhaseScaleRatio,
	}
	oplWaveTable  [8][oplWaveTableSize]float64
	oplAttenTable [oplAttenTableSize]float64
)

type oplEnvStage uint8

type basicOperatorState struct {
	pgPhase    uint32
	phaseReset bool
	egRout     uint16
	egOut      uint16
	stage      oplEnvStage
	regVib     bool
	regTrem    bool
	regType    bool
	regKSR     bool
	regMult    uint8
	regKSL     uint8
	regTL      uint8
	regAR      uint8
	regDR      uint8
	regSL      uint8
	regRR      uint8
	regWave    uint8
	egKSL      uint8
	out        float64
}

type basicChannelState struct {
	keyOn    bool
	fnum     uint16
	block    uint8
	ksv      uint8
	additive bool
	panL     float64
	panR     float64
	feedback uint8
	fbPrev   [2]int
	ops      [opl3OperatorCount]basicOperatorState
}

// DMXLikeOPL3 is a pure-Go OPL3-inspired synth for the subset of the chip this
// codebase drives: 2-op voices, operator envelopes, feedback, waveforms, pan,
// and DMX-style register writes.
type DMXLikeOPL3 struct {
	sampleRate       int
	regs             [0x200]uint8
	ch               [opl3ChannelCount]basicChannelState
	waveformSelectOn bool
	stereoExt        bool
	noteSelect       uint8
	tremoloShift     uint8
	vibShift         uint8
	tremoloPos       uint8
	tremolo          uint8
	vibPos           uint8
	timer            uint64
	egTimer          uint64
	egState          uint8
	egAdd            uint8
	egTimerLo        uint8
	stereoBuf        []int16
	monoBuf          []byte
}

func init() {
	buildOPLWaveTables()
	buildOPLAttenuationTable()
}

// NewDMXLikeOPL3 creates a pure-Go OPL3 fallback at the provided sample rate.
func NewDMXLikeOPL3(sampleRate int) *DMXLikeOPL3 {
	if sampleRate <= 0 {
		sampleRate = opl3DefaultSampleRate
	}
	o := &DMXLikeOPL3{sampleRate: sampleRate}
	o.Reset()
	return o
}

// BasicOPL3 is kept as a compatibility alias for the older synth name.
type BasicOPL3 = DMXLikeOPL3

// NewBasicOPL3 is kept as a compatibility wrapper for the older constructor name.
func NewBasicOPL3(sampleRate int) *DMXLikeOPL3 {
	return NewDMXLikeOPL3(sampleRate)
}

// Reset clears all registers and runtime state.
func (o *DMXLikeOPL3) Reset() {
	if o == nil {
		return
	}
	o.regs = [0x200]uint8{}
	o.ch = [opl3ChannelCount]basicChannelState{}
	o.waveformSelectOn = false
	o.stereoExt = false
	o.noteSelect = 0
	o.tremoloShift = 4
	o.vibShift = 1
	o.tremoloPos = 0
	o.tremolo = 0
	o.vibPos = 0
	o.timer = 0
	o.egTimer = 0
	o.egState = 0
	o.egAdd = 0
	o.egTimerLo = 0
	for i := range o.ch {
		o.ch[i].panL = 1
		o.ch[i].panR = 1
		for op := range o.ch[i].ops {
			o.ch[i].ops[op] = basicOperatorState{
				egRout: oplEnvelopeSilent,
				egOut:  oplEnvelopeSilent,
				stage:  oplEnvRelease,
			}
		}
	}
}

// WriteReg applies a subset of OPL3 register writes.
func (o *DMXLikeOPL3) WriteReg(addr uint16, value uint8) {
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
	case 0x105:
		o.stereoExt = (value & 0x02) != 0
		if !o.stereoExt {
			for ch := range o.ch {
				o.refreshChannelControl(ch)
			}
		}
		return
	case 0x08:
		o.noteSelect = (value >> 6) & 0x01
		for ch := range o.ch {
			o.refreshChannelFreq(ch)
		}
		return
	case 0xBD:
		o.tremoloShift = (((value >> 7) ^ 1) << 1) + 2
		o.vibShift = ((value >> 6) & 0x01) ^ 1
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
	case off >= 0xD0 && off <= 0xD8:
		if o.stereoExt {
			o.refreshChannelStereoPan(bank*9 + off - 0xD0)
		}
	}
}

// GenerateStereoS16 produces interleaved stereo signed-16 PCM.
// The returned slice is backed by an internal reusable buffer and is only
// valid until the next GenerateStereoS16/GenerateMonoU8 call on this instance.
func (o *DMXLikeOPL3) GenerateStereoS16(frames int) []int16 {
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
	for i := 0; i < frames; i++ {
		var l, r float64
		for ch := 0; ch < opl3ChannelCount; ch++ {
			sl, sr := o.renderChannel(ch)
			l += sl
			r += sr
		}
		l = clampSample(l)
		r = clampSample(r)
		out[i*2] = int16(l * 32767)
		out[i*2+1] = int16(r * 32767)
		o.advanceChipState()
	}
	return out
}

// GenerateMonoU8 produces unsigned 8-bit mono PCM from the mixed stereo output.
// The returned slice is backed by an internal reusable buffer and is only
// valid until the next GenerateStereoS16/GenerateMonoU8 call on this instance.
func (o *DMXLikeOPL3) GenerateMonoU8(frames int) []byte {
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

func (o *DMXLikeOPL3) renderChannel(ch int) (float64, float64) {
	if ch < 0 || ch >= len(o.ch) {
		return 0, 0
	}
	c := &o.ch[ch]
	if c.fnum == 0 && !c.keyOn && c.ops[0].egRout >= oplEnvelopeSilent && c.ops[1].egRout >= oplEnvelopeSilent {
		return 0, 0
	}

	mod := &c.ops[0]
	car := &c.ops[1]

	o.advanceEnvelope(c, mod)
	modPhase := o.advanceOperatorPhase(c, mod)
	modFB := 0.0
	if c.feedback != 0 {
		modFB = float64(oplFeedbackPhaseOffset(c.fbPrev[0], c.fbPrev[1], c.feedback))
	}
	modSample := o.sampleOperator(mod, modPhase, modFB)
	modPhaseOut := phaseModFromSample(modSample)
	mod.out = modSample
	c.fbPrev[1] = c.fbPrev[0]
	c.fbPrev[0] = modPhaseOut

	o.advanceEnvelope(c, car)
	carPhase := o.advanceOperatorPhase(c, car)
	carMod := 0.0
	if !c.additive {
		carMod = float64(modPhaseOut)
	}
	carSample := o.sampleOperator(car, carPhase, carMod)

	out := carSample
	if c.additive {
		out += modSample
	}
	return out * c.panL * oplChannelMixGain, out * c.panR * oplChannelMixGain
}

func (o *DMXLikeOPL3) sampleOperator(op *basicOperatorState, phase int, phaseMod float64) float64 {
	if op == nil {
		return 0
	}
	idx := (phase + int(phaseMod)) & oplWaveTableMask
	atten := int(op.egOut)
	if atten < 0 {
		atten = 0
	} else if atten >= len(oplAttenTable) {
		atten = len(oplAttenTable) - 1
	}
	return oplWaveTable[op.regWave&0x07][idx] * oplAttenTable[atten]
}

func (o *DMXLikeOPL3) advanceEnvelope(c *basicChannelState, op *basicOperatorState) {
	if c == nil || op == nil {
		return
	}
	trem := 0
	if op.regTrem {
		trem = int(o.tremolo)
	}

	reset := c.keyOn && op.stage == oplEnvRelease
	regRate := uint8(0)
	if reset {
		regRate = op.regAR
	} else {
		switch op.stage {
		case oplEnvAttack:
			regRate = op.regAR
		case oplEnvDecay:
			regRate = op.regDR
		case oplEnvSustain:
			if !op.regType {
				regRate = op.regRR
			}
		case oplEnvRelease:
			regRate = op.regRR
		}
	}
	op.phaseReset = reset

	ks := int(c.ksv)
	if !op.regKSR {
		ks >>= 2
	}
	nonZero := regRate != 0
	rate := ks + int(regRate<<2)
	if rate > 0x3f {
		rate = 0x3f
	}
	rateHi := rate >> 2
	rateLo := rate & 0x03
	egShift := rateHi + int(o.egAdd)
	shift := 0
	if nonZero {
		if rateHi < 12 {
			if o.egState != 0 {
				switch egShift {
				case 12:
					shift = 1
				case 13:
					shift = (rateLo >> 1) & 0x01
				case 14:
					shift = rateLo & 0x01
				}
			}
		} else {
			shift = (rateHi & 0x03) + int(oplEGIncStep[rateLo][o.egTimerLo])
			if (shift & 0x04) != 0 {
				shift = 0x03
			}
			if shift == 0 {
				shift = int(o.egState)
			}
		}
	}
	egRout := int(op.egRout)
	if reset && rateHi == 0x0f {
		egRout = 0
	}
	egOff := (op.egRout & 0x1f8) == 0x1f8
	if op.stage != oplEnvAttack && !reset && egOff {
		egRout = oplEnvelopeSilent
	}

	egInc := 0
	switch op.stage {
	case oplEnvAttack:
		if op.egRout == 0 {
			op.stage = oplEnvDecay
		} else if c.keyOn && shift > 0 && rateHi != 0x0f {
			// Match the chip's 9-bit attack wraparound instead of masking the
			// complement first. Masking first leaves a fully silent operator
			// stuck at 0x1ff for medium attack rates.
			egInc = int(^op.egRout) >> uint(4-shift)
		}
	case oplEnvDecay:
		if int(op.egRout>>4) >= int(op.regSL) {
			op.stage = oplEnvSustain
		} else if !egOff && !reset && shift > 0 {
			egInc = 1 << (shift - 1)
		}
	case oplEnvSustain, oplEnvRelease:
		if !egOff && !reset && shift > 0 {
			egInc = 1 << (shift - 1)
		}
	}

	op.egRout = uint16((egRout + egInc) & oplEnvelopeSilent)
	if reset {
		op.stage = oplEnvAttack
	}
	if !c.keyOn {
		op.stage = oplEnvRelease
	}

	baseAtten := int(op.egRout) + int(op.regTL<<2) + int(op.egKSL>>oplKSLShift[op.regKSL]) + trem
	op.egOut = uint16(clampAtten(baseAtten))
}

func (o *DMXLikeOPL3) advanceOperatorPhase(c *basicChannelState, op *basicOperatorState) int {
	if c == nil || op == nil {
		return 0
	}
	phase := int(uint16(op.pgPhase >> oplPhaseFracBits))
	if op.phaseReset {
		op.pgPhase = 0
		phase = 0
		op.phaseReset = false
	}

	fnum := int(c.fnum)
	if op.regVib {
		rangeVal := (fnum >> 7) & 0x07
		vibPos := int(o.vibPos)
		if (vibPos & 0x03) == 0 {
			rangeVal = 0
		} else if (vibPos & 0x01) != 0 {
			rangeVal >>= 1
		}
		rangeVal >>= o.vibShift
		if (vibPos & 0x04) != 0 {
			rangeVal = -rangeVal
		}
		fnum += rangeVal
	}

	baseFreq := (fnum << c.block) >> 1
	op.pgPhase += uint32((baseFreq * int(oplFrequencyMultiples[op.regMult])) >> 1)
	return phase & oplWaveTableMask
}

func (o *DMXLikeOPL3) keyOnChannel(ch int) {
	if ch < 0 || ch >= len(o.ch) {
		return
	}
	o.ch[ch].fbPrev = [2]int{}
}

func (o *DMXLikeOPL3) keyOffChannel(ch int) {
	if ch < 0 || ch >= len(o.ch) {
		return
	}
	o.ch[ch].fbPrev = [2]int{}
	for op := range o.ch[ch].ops {
		o.ch[ch].ops[op].stage = oplEnvRelease
	}
}

func (o *DMXLikeOPL3) refreshChannelFreq(ch int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 {
		return
	}
	a := o.regs[base+0xA0+ci]
	b := o.regs[base+0xB0+ci]
	o.ch[ch].fnum = uint16(a) | (uint16(b&0x03) << 8)
	o.ch[ch].block = (b >> 2) & 0x07
	o.ch[ch].ksv = (o.ch[ch].block << 1) | uint8((o.ch[ch].fnum>>(0x09-o.noteSelect))&0x01)
	for op := 0; op < opl3OperatorCount; op++ {
		o.updateOperatorKSL(ch, op)
	}
}

func (o *DMXLikeOPL3) refreshChannelControl(ch int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 {
		return
	}
	c0 := o.regs[base+0xC0+ci]
	o.ch[ch].additive = (c0 & 0x01) != 0
	o.ch[ch].feedback = (c0 >> 1) & 0x07
	left := (c0 & 0x10) != 0
	right := (c0 & 0x20) != 0
	if o.stereoExt {
		return
	}
	switch {
	case left && right:
		o.ch[ch].panL, o.ch[ch].panR = 1, 1
	case left:
		o.ch[ch].panL, o.ch[ch].panR = 0, 1
	case right:
		o.ch[ch].panL, o.ch[ch].panR = 1, 0
	default:
		o.ch[ch].panL, o.ch[ch].panR = 1, 1
	}
}

func (o *DMXLikeOPL3) refreshChannelStereoPan(ch int) {
	base, ci := oplBaseAndChannel(ch)
	if ci < 0 {
		return
	}
	pan := o.regs[base+0xD0+ci]
	o.ch[ch].panL, o.ch[ch].panR = oplStereoPanGains(pan)
}

func (o *DMXLikeOPL3) refreshOperator(ch int, op int) {
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

	s.regTrem = (reg20 & 0x80) != 0
	s.regVib = (reg20 & 0x40) != 0
	s.regType = (reg20 & 0x20) != 0
	s.regKSR = (reg20 & 0x10) != 0
	s.regMult = reg20 & 0x0F
	s.regKSL = (reg40 >> 6) & 0x03
	s.regTL = reg40 & 0x3F
	s.regAR = (reg60 >> 4) & 0x0F
	s.regDR = reg60 & 0x0F
	s.regSL = (reg80 >> 4) & 0x0F
	if s.regSL == 0x0F {
		s.regSL = 0x1F
	}
	s.regRR = reg80 & 0x0F
	s.regWave = regE0 & 0x07
	if !o.waveformSelectOn {
		s.regWave &= 0x03
	}
	o.updateOperatorKSL(ch, op)
}

func (o *DMXLikeOPL3) updateOperatorKSL(ch int, op int) {
	if ch < 0 || ch >= len(o.ch) || op < 0 || op >= opl3OperatorCount {
		return
	}
	fnumIndex := int(o.ch[ch].fnum >> 6)
	if fnumIndex < 0 {
		fnumIndex = 0
	} else if fnumIndex >= len(oplKSLROM) {
		fnumIndex = len(oplKSLROM) - 1
	}
	ksl := (int(oplKSLROM[fnumIndex]) << 2) - ((0x08 - int(o.ch[ch].block)) << 5)
	if ksl < 0 {
		ksl = 0
	}
	o.ch[ch].ops[op].egKSL = uint8(ksl)
}

func (o *DMXLikeOPL3) advanceChipState() {
	if (o.timer & 0x3F) == 0x3F {
		o.tremoloPos = (o.tremoloPos + 1) % 210
	}
	if o.tremoloPos < 105 {
		o.tremolo = o.tremoloPos >> o.tremoloShift
	} else {
		o.tremolo = (210 - o.tremoloPos) >> o.tremoloShift
	}

	if (o.timer & 0x3FF) == 0x3FF {
		o.vibPos = (o.vibPos + 1) & 7
	}
	o.timer++

	if o.egState != 0 {
		shift := uint8(0)
		for shift < 13 && ((o.egTimer>>shift)&0x01) == 0 {
			shift++
		}
		if shift > 12 {
			o.egAdd = 0
		} else {
			o.egAdd = shift + 1
		}
		o.egTimerLo = uint8(o.egTimer & 0x03)
		o.egTimer++
	}
	o.egState ^= 1
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

func buildOPLWaveTables() {
	for wave := 0; wave < len(oplWaveTable); wave++ {
		for i := 0; i < oplWaveTableSize; i++ {
			oplWaveTable[wave][i] = oplWaveSample(wave, i)
		}
	}
}

func oplWaveSample(wave int, idx int) float64 {
	idx &= oplWaveTableMask
	phase := float64(idx) / float64(oplWaveTableSize)
	switch wave & 0x07 {
	case 0:
		return math.Sin(phase * 2 * math.Pi)
	case 1:
		if idx >= 512 {
			return 0
		}
		return math.Sin((float64(idx) / 512.0) * math.Pi)
	case 2:
		return math.Abs(math.Sin(phase * 2 * math.Pi))
	case 3:
		if (idx & 0x100) != 0 {
			return 0
		}
		return math.Sin((float64(idx&0x0FF) / 256.0) * (math.Pi / 2))
	case 4:
		if idx >= 512 {
			return 0
		}
		if idx < 256 {
			return math.Sin((float64(idx) / 256.0) * math.Pi)
		}
		return -math.Sin((float64(idx-256) / 256.0) * math.Pi)
	case 5:
		if idx >= 512 {
			return 0
		}
		if idx < 256 {
			return math.Sin((float64(idx) / 256.0) * math.Pi)
		}
		return math.Sin((float64(idx-256) / 256.0) * math.Pi)
	case 6:
		if idx < 512 {
			return 1
		}
		return -1
	default:
		if idx < 512 {
			return 1 - float64(idx)/256.0
		}
		return float64(idx-512)/256.0 - 1
	}
}

func oplStereoPanGains(pan uint8) (float64, float64) {
	left := math.Sin((float64(255-pan) * math.Pi) / 512.0)
	right := math.Sin((float64(pan) * math.Pi) / 512.0)
	return left, right
}

func buildOPLAttenuationTable() {
	for i := 0; i < len(oplAttenTable); i++ {
		oplAttenTable[i] = math.Exp2(-float64(i) / 32.0)
	}
}

func clampEnvelope(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > oplEnvelopeSilent {
		return oplEnvelopeSilent
	}
	return uint16(v)
}

func clampAtten(v int) int {
	if v < 0 {
		return 0
	}
	if v >= oplAttenTableSize {
		return oplAttenTableSize - 1
	}
	return v
}

func clampSample(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

func phaseModFromSample(sample float64) int {
	return int(math.Round(sample * oplPhaseModScale))
}

func oplFeedbackPhaseOffset(prev0, prev1 int, feedback uint8) int {
	if feedback == 0 {
		return 0
	}
	shift := 9 - int(feedback)
	if shift <= 0 {
		return prev0 + prev1
	}
	return (prev0 + prev1) / (1 << shift)
}
