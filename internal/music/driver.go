package music

import (
	"gddoom/internal/sound"
	"math"
)

const (
	OutputSampleRate = 44100
	defaultTicRate   = 140
	// Doom DMX OPL path uses 9 simultaneous 2-op channels.
	defaultVoices          = 9
	DefaultOutputGain      = 1.0
	MaxOutputGain          = 5.0
	outputSoftKneeStart    = 0.85
	controllerPan          = 10
	controllerVol          = 7
	controllerExpr         = 11
	controllerResetAll     = 121
	controllerAllSoundsOff = 120
	controllerAllNotesOff  = 123
	defaultChanVol         = 127
	defaultChanExpr        = 127
	defaultChanPan         = 64
	defaultMUSPanMax       = 1.0
	percussionNoteMin      = 35
	percussionNoteMax      = 81
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
	active     bool
	ch         uint8
	note       uint8
	velocity   uint8
	playNote   uint8
	fineTune   int16
	freqWord   uint16
	instrVoice uint8
	patch      Patch
	oplCh      int
}

type Driver struct {
	opl        sound.OPL3
	sampleRate int
	ticRate    int
	musPanMax  float64
	outputGain float64
	bank       PatchBank
	ch         [16]channelState
	voices     []voiceState
	freeList   []int
	allocList  []int
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
		musPanMax:  defaultMUSPanMax,
		outputGain: DefaultOutputGain,
		bank:       bank,
		voices:     make([]voiceState, defaultVoices),
		freeList:   make([]int, 0, defaultVoices),
		allocList:  make([]int, 0, defaultVoices),
	}
	for i := range d.voices {
		d.voices[i].oplCh = i
		d.freeList = append(d.freeList, i)
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

// SetMUSPanMax sets the MUS pan scaling factor in the 0..1 range.
// 1.0 preserves full MUS pan range; lower values pull pan toward center.
func (d *Driver) SetMUSPanMax(max float64) {
	if d == nil {
		return
	}
	d.musPanMax = clampUnit(max)
}

// SetOutputGain sets final PCM output gain.
// Value is clamped to [0, MaxOutputGain].
func (d *Driver) SetOutputGain(gain float64) {
	if d == nil {
		return
	}
	d.outputGain = clampOutputGain(gain)
}

func (d *Driver) Reset() {
	if d == nil {
		return
	}
	d.opl.Reset()
	// Mirror Chocolate Doom OPL init low-register setup.
	d.opl.WriteReg(0x04, 0x60) // reset timers
	d.opl.WriteReg(0x04, 0x80) // enable interrupts
	d.opl.WriteReg(0x01, 0x20) // waveform control enable
	// Use OPL3 NEW mode so upper-bank channel registers (0x1xx) are active.
	d.opl.WriteReg(0x105, 0x01)
	d.opl.WriteReg(0x08, 0x40) // FM mode / keyboard split
	for i := range d.voices {
		d.voices[i] = voiceState{oplCh: i}
	}
	d.freeList = d.freeList[:0]
	d.allocList = d.allocList[:0]
	for i := range d.voices {
		d.freeList = append(d.freeList, i)
	}
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
	d.ensureVoiceLists()
	var pcm []int16
	for _, ev := range events {
		if ev.DeltaTics > 0 {
			frames := int((uint64(ev.DeltaTics) * uint64(d.sampleRate)) / uint64(d.ticRate))
			if frames > 0 {
				pcm = append(pcm, d.generateStereoS16(frames)...)
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
			d.refreshChannelVolume(uint8(ch))
		case controllerExpr:
			d.ch[ch].expression = ev.B
		case controllerPan:
			d.ch[ch].pan = ev.B
			d.refreshChannelPan(uint8(ch))
		case controllerResetAll:
			d.ch[ch].volume = defaultChanVol
			d.ch[ch].expression = defaultChanExpr
			d.ch[ch].pan = defaultChanPan
			d.ch[ch].pitchBend = 0
			d.refreshChannelVolume(uint8(ch))
			d.refreshChannelPan(uint8(ch))
			d.refreshChannelPitch(uint8(ch))
		case controllerAllSoundsOff, controllerAllNotesOff:
			d.allNotesOff(ev.Channel)
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
	if percussion && (note < percussionNoteMin || note > percussionNoteMax) {
		return
	}
	patches := d.voicePatches(prog, percussion, note)
	for i, np := range patches {
		vx := d.allocateVoice()
		v := &d.voices[vx]
		v.active = true
		v.ch = ch
		v.note = note
		v.velocity = velocity
		v.playNote = resolveVoiceNote(note, percussion, np)
		v.fineTune = np.FineTune
		v.instrVoice = uint8(i)
		v.patch = np.Patch
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
	for i := 0; i < len(d.allocList); i++ {
		vi := d.allocList[i]
		v := &d.voices[vi]
		if !v.active || v.ch != ch || v.note != note {
			continue
		}
		d.releaseVoiceAt(i)
		i--
	}
}

func (d *Driver) refreshChannelPitch(ch uint8) {
	updated := make([]int, 0, len(d.allocList))
	unchanged := make([]int, 0, len(d.allocList))
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			unchanged = append(unchanged, vi)
			continue
		}
		v.freqWord = d.writeNote(v.oplCh, v.playNote, d.ch[ch&0x0F].pitchBend+v.fineTune, true)
		updated = append(updated, vi)
	}
	d.allocList = append(unchanged, updated...)
}

func (d *Driver) refreshChannelVolume(ch uint8) {
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		d.writeVolume(v.oplCh, ch, v.velocity, v.patch)
	}
}

func (d *Driver) refreshChannelPan(ch uint8) {
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		d.writePan(v.oplCh, ch, v.patch.C0)
	}
}

func (d *Driver) allocateVoice() int {
	if len(d.freeList) == 0 {
		d.replaceExistingVoice()
	}
	if len(d.freeList) == 0 {
		return 0
	}
	vi := d.freeList[0]
	d.freeList = d.freeList[1:]
	d.allocList = append(d.allocList, vi)
	return vi
}

func (d *Driver) allNotesOff(ch uint8) {
	for i := 0; i < len(d.allocList); i++ {
		vi := d.allocList[i]
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		d.releaseVoiceAt(i)
		i--
	}
}

// Doom 1.9 DMX-style replacement:
// prefer stealing 2nd voice of a double-voice instrument, else the
// latest eligible same-or-higher channel entry in allocation order.
func (d *Driver) replaceExistingVoice() {
	if len(d.allocList) == 0 {
		return
	}
	result := 0
	for i := 0; i < len(d.allocList); i++ {
		vi := d.allocList[i]
		rv := d.allocList[result]
		if d.voices[vi].instrVoice != 0 || d.voices[vi].ch >= d.voices[rv].ch {
			result = i
		}
	}
	d.releaseVoiceAt(result)
}

func (d *Driver) releaseVoiceAt(allocIdx int) {
	if allocIdx < 0 || allocIdx >= len(d.allocList) {
		return
	}
	vi := d.allocList[allocIdx]
	v := &d.voices[vi]
	d.writeFreqWord(v.oplCh, v.freqWord, false)
	oplCh := v.oplCh
	*v = voiceState{oplCh: oplCh}
	copy(d.allocList[allocIdx:], d.allocList[allocIdx+1:])
	d.allocList = d.allocList[:len(d.allocList)-1]
	d.freeList = append(d.freeList, vi)
}

func (d *Driver) ensureVoiceLists() {
	if d == nil {
		return
	}
	if len(d.freeList)+len(d.allocList) == len(d.voices) {
		return
	}
	d.freeList = d.freeList[:0]
	d.allocList = d.allocList[:0]
	for i := range d.voices {
		if d.voices[i].active {
			d.allocList = append(d.allocList, i)
		} else {
			d.freeList = append(d.freeList, i)
		}
	}
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
	pan := scaleMUSPan(d.ch[ch&0x0F].pan, d.musPanMax)
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

func clampUnit(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func scaleMUSPan(pan uint8, max float64) uint8 {
	m := clampUnit(max)
	if m >= 1 {
		return pan
	}
	scaled := int(math.Round(64 + (float64(int(pan)-64) * m)))
	return uint8(clampMIDI7(scaled))
}

func (d *Driver) generateStereoS16(frames int) []int16 {
	if d == nil || d.opl == nil || frames <= 0 {
		return nil
	}
	out := d.opl.GenerateStereoS16(frames)
	if len(out) == 0 {
		return out
	}
	applyOutputGainSoftKnee(out, d.outputGain)
	return out
}

func clampOutputGain(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > MaxOutputGain {
		return MaxOutputGain
	}
	return v
}

func applyOutputGainSoftKnee(samples []int16, gain float64) {
	if len(samples) == 0 {
		return
	}
	g := clampOutputGain(gain)
	if g == 1 {
		return
	}
	const fullScale = 32767.0
	const negFullScale = 32768.0
	kneeStart := outputSoftKneeStart
	if kneeStart <= 0 || kneeStart >= 1 {
		kneeStart = 0.85
	}
	for i := range samples {
		x := float64(samples[i]) / fullScale
		if x > 1 {
			x = 1
		}
		if x < -1 {
			x = -1
		}
		y := x * g
		ay := math.Abs(y)
		if ay > kneeStart {
			// Soft-knee saturation above threshold with asymptotic limit at +/-1.
			over := (ay - kneeStart) / (1 - kneeStart)
			soft := kneeStart + (1-kneeStart)*(over/(1+over))
			if y < 0 {
				y = -soft
			} else {
				y = soft
			}
		}
		if y > 1 {
			y = 1
		}
		if y < -1 {
			y = -1
		}
		if y < 0 {
			samples[i] = int16(math.Round(y * negFullScale))
		} else {
			samples[i] = int16(math.Round(y * fullScale))
		}
	}
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
