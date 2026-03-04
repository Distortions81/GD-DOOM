package music

import (
	"math"

	"gddoom/internal/sound"
)

const (
	OutputSampleRate = 44100
	defaultTicRate   = 140
	defaultVoices    = 18
	controllerPan    = 10
	controllerVol    = 7
	controllerExpr   = 11
	defaultChanVol   = 127
	defaultChanExpr  = 127
	defaultChanPan   = 64
)

type EventType uint8

const (
	EventNoteOn EventType = iota
	EventNoteOff
	EventProgramChange
	EventControlChange
	EventPitchBend
	EventEnd
)

type Event struct {
	DeltaTics uint32
	Type      EventType
	Channel   uint8
	A         uint8
	B         uint8
}

type Patch struct {
	Mod20 uint8
	Mod40 uint8
	Mod60 uint8
	Mod80 uint8
	ModE0 uint8
	Car20 uint8
	Car40 uint8
	Car60 uint8
	Car80 uint8
	CarE0 uint8
	C0    uint8
}

type PatchBank interface {
	Patch(program uint8, percussion bool, note uint8) Patch
}

type DefaultPatchBank struct{}

func (DefaultPatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	// Minimal audible default. DMX/GENMIDI patch application plugs in here.
	return Patch{
		Mod20: 0x21, Mod40: 0x3F, Mod60: 0xF0, Mod80: 0x77, ModE0: 0x00,
		Car20: 0x21, Car40: 0x00, Car60: 0xF0, Car80: 0x77, CarE0: 0x00,
		C0: 0x01,
	}
}

type channelState struct {
	program    uint8
	volume     uint8
	expression uint8
	pan        uint8
	pitchBend  int16 // -8192..8191
}

type voiceState struct {
	active bool
	ch     uint8
	note   uint8
	id     uint64
	oplCh  int
}

type Driver struct {
	opl        *sound.BasicOPL3
	sampleRate int
	ticRate    int
	bank       PatchBank
	ch         [16]channelState
	voices     []voiceState
	nextVoice  uint64
}

func NewDriver(sampleRate int, bank PatchBank) *Driver {
	if sampleRate <= 0 {
		sampleRate = OutputSampleRate
	}
	if bank == nil {
		bank = DefaultPatchBank{}
	}
	d := &Driver{
		opl:        sound.NewBasicOPL3(sampleRate),
		sampleRate: sampleRate,
		ticRate:    defaultTicRate,
		bank:       bank,
		voices:     make([]voiceState, defaultVoices),
	}
	for i := range d.ch {
		d.ch[i] = channelState{
			volume:     defaultChanVol,
			expression: defaultChanExpr,
			pan:        defaultChanPan,
		}
	}
	return d
}

func NewOutputDriver(bank PatchBank) *Driver {
	return NewDriver(OutputSampleRate, bank)
}

func (d *Driver) Reset() {
	if d == nil {
		return
	}
	d.opl.Reset()
	for i := range d.voices {
		d.voices[i] = voiceState{oplCh: i}
	}
	d.nextVoice = 0
	for i := range d.ch {
		d.ch[i] = channelState{
			volume:     defaultChanVol,
			expression: defaultChanExpr,
			pan:        defaultChanPan,
		}
	}
}

// Render processes events and returns signed 16-bit stereo interleaved PCM.
func (d *Driver) Render(events []Event) []int16 {
	if d == nil || d.opl == nil || d.sampleRate <= 0 {
		return nil
	}
	if d.ticRate <= 0 {
		d.ticRate = defaultTicRate
	}
	if len(d.voices) == 0 {
		d.voices = make([]voiceState, defaultVoices)
		for i := range d.voices {
			d.voices[i].oplCh = i
		}
	}
	var pcm []int16
	for _, ev := range events {
		if ev.DeltaTics > 0 {
			frames := int((uint64(ev.DeltaTics) * uint64(d.sampleRate)) / uint64(d.ticRate))
			if frames > 0 {
				pcm = append(pcm, d.opl.GenerateStereoS16(frames)...)
			}
		}
		d.applyEvent(ev)
		if ev.Type == EventEnd {
			break
		}
	}
	return pcm
}

// RenderMUS parses a MUS stream and renders it to signed 16-bit stereo PCM.
func (d *Driver) RenderMUS(musData []byte) ([]int16, error) {
	events, err := ParseMUS(musData)
	if err != nil {
		return nil, err
	}
	return d.Render(events), nil
}

// RenderMUSS16LE parses MUS and returns little-endian signed 16-bit stereo PCM bytes.
func (d *Driver) RenderMUSS16LE(musData []byte) ([]byte, error) {
	s, err := d.RenderMUS(musData)
	if err != nil {
		return nil, err
	}
	return PCMInt16ToBytesLE(s), nil
}

func PCMInt16ToBytesLE(samples []int16) []byte {
	if len(samples) == 0 {
		return nil
	}
	out := make([]byte, len(samples)*2)
	oi := 0
	for _, s := range samples {
		out[oi] = byte(s)
		out[oi+1] = byte(s >> 8)
		oi += 2
	}
	return out
}

func (d *Driver) applyEvent(ev Event) {
	ch := int(ev.Channel & 0x0F)
	switch ev.Type {
	case EventProgramChange:
		d.ch[ch].program = ev.A
	case EventControlChange:
		switch ev.A {
		case controllerVol:
			d.ch[ch].volume = ev.B
		case controllerExpr:
			d.ch[ch].expression = ev.B
		case controllerPan:
			d.ch[ch].pan = ev.B
		}
	case EventPitchBend:
		// MIDI-style 14-bit: lsb=A, msb=B.
		v := int16((int(ev.B)<<7 | int(ev.A)) - 8192)
		d.ch[ch].pitchBend = v
		d.refreshChannelPitch(uint8(ch))
	case EventNoteOn:
		if ev.B == 0 {
			d.noteOff(ev.Channel, ev.A)
			return
		}
		d.noteOn(ev.Channel, ev.A, ev.B)
	case EventNoteOff:
		d.noteOff(ev.Channel, ev.A)
	}
}

func (d *Driver) noteOn(ch, note, velocity uint8) {
	vx := d.allocateVoice(ch, note)
	v := &d.voices[vx]
	v.active = true
	v.ch = ch
	v.note = note
	d.nextVoice++
	v.id = d.nextVoice
	prog := d.ch[ch&0x0F].program
	patch := d.bank.Patch(prog, ch == 15, note)
	d.writePatch(v.oplCh, patch)
	d.writeVolume(v.oplCh, ch, velocity, patch.Car40)
	d.writePan(v.oplCh, ch, patch.C0)
	d.writeNote(v.oplCh, note, d.ch[ch&0x0F].pitchBend, true)
}

func (d *Driver) noteOff(ch, note uint8) {
	for i := range d.voices {
		v := &d.voices[i]
		if !v.active || v.ch != ch || v.note != note {
			continue
		}
		d.writeNote(v.oplCh, v.note, 0, false)
		v.active = false
		return
	}
}

func (d *Driver) refreshChannelPitch(ch uint8) {
	for i := range d.voices {
		v := &d.voices[i]
		if !v.active || v.ch != ch {
			continue
		}
		d.writeNote(v.oplCh, v.note, d.ch[ch&0x0F].pitchBend, true)
	}
}

func (d *Driver) allocateVoice(ch, note uint8) int {
	for i := range d.voices {
		if !d.voices[i].active {
			return i
		}
	}
	oldest := 0
	for i := 1; i < len(d.voices); i++ {
		if d.voices[i].id < d.voices[oldest].id {
			oldest = i
		}
	}
	d.writeNote(d.voices[oldest].oplCh, d.voices[oldest].note, 0, false)
	d.voices[oldest].active = false
	d.voices[oldest].ch = ch
	d.voices[oldest].note = note
	return oldest
}

func (d *Driver) writePatch(oplCh int, p Patch) {
	base, ch := oplAddrBase(oplCh)
	modSlot, carSlot := oplSlots(ch)
	d.opl.WriteReg(uint16(base+0x20+modSlot), p.Mod20)
	d.opl.WriteReg(uint16(base+0x40+modSlot), p.Mod40)
	d.opl.WriteReg(uint16(base+0x60+modSlot), p.Mod60)
	d.opl.WriteReg(uint16(base+0x80+modSlot), p.Mod80)
	d.opl.WriteReg(uint16(base+0xE0+modSlot), p.ModE0)
	d.opl.WriteReg(uint16(base+0x20+carSlot), p.Car20)
	d.opl.WriteReg(uint16(base+0x40+carSlot), p.Car40)
	d.opl.WriteReg(uint16(base+0x60+carSlot), p.Car60)
	d.opl.WriteReg(uint16(base+0x80+carSlot), p.Car80)
	d.opl.WriteReg(uint16(base+0xE0+carSlot), p.CarE0)
}

func (d *Driver) writeVolume(oplCh int, ch, velocity, patchCar40 uint8) {
	base, ci := oplAddrBase(oplCh)
	_, carSlot := oplSlots(ci)
	cv := int(d.ch[ch&0x0F].volume)
	expr := int(d.ch[ch&0x0F].expression)
	vel := int(velocity)
	level := (cv * expr * vel) / (127 * 127)
	if level < 0 {
		level = 0
	}
	if level > 127 {
		level = 127
	}
	// OPL TL: 0 loud .. 63 quiet.
	tl := 63 - (level*63)/127
	if tl < 0 {
		tl = 0
	}
	if tl > 63 {
		tl = 63
	}
	v := (patchCar40 & 0xC0) | uint8(tl)
	d.opl.WriteReg(uint16(base+0x40+carSlot), v)
}

func (d *Driver) writePan(oplCh int, ch, c0 uint8) {
	base, ci := oplAddrBase(oplCh)
	pan := d.ch[ch&0x0F].pan
	var lr uint8 = 0x30
	switch {
	case pan < 42:
		lr = 0x10
	case pan > 85:
		lr = 0x20
	}
	d.opl.WriteReg(uint16(base+0xC0+ci), (c0&0x0F)|lr)
}

func (d *Driver) writeNote(oplCh int, note uint8, bend int16, keyOn bool) {
	base, ci := oplAddrBase(oplCh)
	fnum, block := noteToFnumBlock(int(note), bend)
	a := uint8(fnum & 0xFF)
	b := uint8((fnum>>8)&0x03) | uint8((block&0x07)<<2)
	if keyOn {
		b |= 0x20
	}
	d.opl.WriteReg(uint16(base+0xA0+ci), a)
	d.opl.WriteReg(uint16(base+0xB0+ci), b)
}

func noteToFnumBlock(note int, bend int16) (int, int) {
	semi := float64(note-69) + (float64(bend)/8192.0)*2.0
	freq := 440.0 * math.Pow(2, semi/12.0)
	bestF := 0
	bestB := 0
	bestErr := math.MaxFloat64
	for block := 0; block < 8; block++ {
		scale := math.Ldexp(1.0, 20-block)
		f := int(math.Round(freq * scale / 49716.0))
		if f < 1 || f > 1023 {
			continue
		}
		est := float64(f) * 49716.0 / scale
		err := math.Abs(est - freq)
		if err < bestErr {
			bestErr = err
			bestF = f
			bestB = block
		}
	}
	if bestF == 0 {
		if freq < 1 {
			return 1, 0
		}
		return 1023, 7
	}
	return bestF, bestB
}

func oplAddrBase(oplCh int) (base int, ch int) {
	if oplCh < 9 {
		return 0x000, oplCh
	}
	return 0x100, oplCh - 9
}

func oplSlots(ch int) (mod int, car int) {
	// Standard 2-op channel slot order.
	modSlots := [9]int{0, 1, 2, 8, 9, 10, 16, 17, 18}
	carSlots := [9]int{3, 4, 5, 11, 12, 13, 19, 20, 21}
	if ch < 0 || ch > 8 {
		return 0, 3
	}
	return modSlots[ch], carSlots[ch]
}
