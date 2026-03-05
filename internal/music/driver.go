package music

import (
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

type NotePatch struct {
	Patch          Patch
	Fixed          bool
	FixedNote      uint8
	BaseNoteOffset int16
	FineTune       int16 // DMX-style 1/32 semitone units
}

type VoicePatchBank interface {
	PatchVoices(program uint8, percussion bool, note uint8) []NotePatch
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
	pitchBend  int16 // DMX-style -64..63 in 1/32 semitone units
}

type voiceState struct {
	active   bool
	ch       uint8
	note     uint8
	playNote uint8
	fineTune int16
	freqWord uint16
	id       uint64
	oplCh    int
}

type Driver struct {
	opl        sound.OPL3
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
		opl:        sound.NewOPL3(sampleRate),
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
		// DMX/Chocolate OPL path only uses MIDI pitch bend MSB.
		d.ch[ch].pitchBend = int16(ev.B) - 64
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
	prog := d.ch[ch&0x0F].program
	percussion := isPercussionChannel(ch)
	patches := d.voicePatches(prog, percussion, note)
	for _, np := range patches {
		vx := d.allocateVoice(ch, note)
		v := &d.voices[vx]
		v.active = true
		v.ch = ch
		v.note = note
		v.playNote = resolveVoiceNote(note, percussion, np)
		v.fineTune = np.FineTune
		d.nextVoice++
		v.id = d.nextVoice
		d.writePatch(v.oplCh, np.Patch)
		d.writeVolume(v.oplCh, ch, velocity, np.Patch)
		d.writePan(v.oplCh, ch, np.Patch.C0)
		v.freqWord = d.writeNote(v.oplCh, v.playNote, d.ch[ch&0x0F].pitchBend+v.fineTune, true)
	}
}

func isPercussionChannel(ch uint8) bool {
	// MUS parser maps percussion to MIDI channel 9.
	return (ch & 0x0F) == 9
}

func (d *Driver) noteOff(ch, note uint8) {
	found := false
	for i := range d.voices {
		v := &d.voices[i]
		if !v.active || v.ch != ch || v.note != note {
			continue
		}
		d.writeFreqWord(v.oplCh, v.freqWord, false)
		v.active = false
		found = true
	}
	if found {
		return
	}
}

func (d *Driver) refreshChannelPitch(ch uint8) {
	for i := range d.voices {
		v := &d.voices[i]
		if !v.active || v.ch != ch {
			continue
		}
		v.freqWord = d.writeNote(v.oplCh, v.playNote, d.ch[ch&0x0F].pitchBend+v.fineTune, true)
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
	d.writeFreqWord(d.voices[oldest].oplCh, d.voices[oldest].freqWord, false)
	d.voices[oldest].active = false
	d.voices[oldest].ch = ch
	d.voices[oldest].note = note
	return oldest
}

func (d *Driver) voicePatches(program uint8, percussion bool, note uint8) []NotePatch {
	if vb, ok := d.bank.(VoicePatchBank); ok {
		voices := vb.PatchVoices(program, percussion, note)
		if len(voices) > 0 {
			return voices
		}
	}
	return []NotePatch{{Patch: d.bank.Patch(program, percussion, note)}}
}

func resolveVoiceNote(inputNote uint8, percussion bool, p NotePatch) uint8 {
	n := int(inputNote)
	switch {
	case p.Fixed:
		n = int(p.FixedNote)
	case percussion:
		n = 60
	default:
		n += int(p.BaseNoteOffset)
	}
	// DMX/Chocolate path wraps to a 0..95 note range.
	for n < 0 {
		n += 12
	}
	for n > 95 {
		n -= 12
	}
	return uint8(n)
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

func (d *Driver) writeVolume(oplCh int, ch, velocity uint8, patch Patch) {
	base, ci := oplAddrBase(oplCh)
	modSlot, carSlot := oplSlots(ci)
	cv := clampMIDI7(int(d.ch[ch&0x0F].volume))
	vel := clampMIDI7(int(velocity))
	midiVolume := 2 * (volumeMappingTable[cv] + 1)
	fullVolume := (volumeMappingTable[vel] * midiVolume) >> 9
	carTL := 0x3f - fullVolume
	if carTL < 0 {
		carTL = 0
	}
	if carTL > 0x3f {
		carTL = 0x3f
	}
	d.opl.WriteReg(uint16(base+0x40+carSlot), (patch.Car40&0xC0)|uint8(carTL))

	// DMX behavior: in non-modulated feedback mode, modulator volume follows carrier.
	if (patch.C0&0x01) != 0 && (patch.Mod40&0x3f) != 0x3f {
		modTL := int(patch.Mod40 & 0x3f)
		if modTL < carTL {
			modTL = carTL
		}
		d.opl.WriteReg(uint16(base+0x40+modSlot), (patch.Mod40&0xC0)|uint8(modTL))
	}
}

func (d *Driver) writePan(oplCh int, ch, c0 uint8) {
	base, ci := oplAddrBase(oplCh)
	pan := d.ch[ch&0x0F].pan
	var lr uint8 = 0x30
	switch {
	case pan >= 96:
		lr = 0x10
	case pan <= 48:
		lr = 0x20
	}
	d.opl.WriteReg(uint16(base+0xC0+ci), (c0&0x0F)|lr)
}

func (d *Driver) writeNote(oplCh int, note uint8, bend int16, keyOn bool) uint16 {
	freqWord := dmxFrequencyWord(int(note), bend)
	d.writeFreqWord(oplCh, freqWord, keyOn)
	return freqWord
}

func (d *Driver) writeFreqWord(oplCh int, freqWord uint16, keyOn bool) {
	base, ci := oplAddrBase(oplCh)
	a := uint8(freqWord & 0x00FF)
	b := uint8((freqWord >> 8) & 0x1F)
	if keyOn {
		b |= 0x20
	}
	d.opl.WriteReg(uint16(base+0xA0+ci), a)
	d.opl.WriteReg(uint16(base+0xB0+ci), b)
}

func dmxFrequencyWord(note int, bend int16) uint16 {
	freqIndex := 64 + 32*note + int(bend)
	if freqIndex < 0 {
		freqIndex = 0
	}
	if freqIndex < 284 {
		return dmxFrequencyCurve[freqIndex]
	}
	subIndex := (freqIndex - 284) % (12 * 32)
	octave := (freqIndex - 284) / (12 * 32)
	if octave >= 7 {
		octave = 7
	}
	return dmxFrequencyCurve[subIndex+284] | uint16(octave<<10)
}

func clampMIDI7(v int) int {
	if v < 0 {
		return 0
	}
	if v > 127 {
		return 127
	}
	return v
}

// From Chocolate Doom i_oplmusic.c volume_mapping_table.
var volumeMappingTable = [128]int{
	0, 1, 3, 5, 6, 8, 10, 11,
	13, 14, 16, 17, 19, 20, 22, 23,
	25, 26, 27, 29, 30, 32, 33, 34,
	36, 37, 39, 41, 43, 45, 47, 49,
	50, 52, 54, 55, 57, 59, 60, 61,
	63, 64, 66, 67, 68, 69, 71, 72,
	73, 74, 75, 76, 77, 79, 80, 81,
	82, 83, 84, 84, 85, 86, 87, 88,
	89, 90, 91, 92, 92, 93, 94, 95,
	96, 96, 97, 98, 99, 99, 100, 101,
	101, 102, 103, 103, 104, 105, 105, 106,
	107, 107, 108, 109, 109, 110, 110, 111,
	112, 112, 113, 113, 114, 114, 115, 115,
	116, 117, 117, 118, 118, 119, 119, 120,
	120, 121, 121, 122, 122, 123, 123, 123,
	124, 124, 125, 125, 126, 126, 127, 127,
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
