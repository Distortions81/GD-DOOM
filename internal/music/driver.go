package music

import (
	"fmt"
	"math"

	"gddoom/internal/sound"
)

const (
	OutputSampleRate            = 44100
	defaultTicRate              = 140
	DefaultMUSVolumeCompression = 3.0
	// The music path uses 18 simultaneous 2-op voices.
	defaultVoices          = 18
	DefaultOutputGain      = 1.0
	MaxOutputGain          = 5.0
	outputSoftKneeStart    = 0.85
	controllerPan          = 10
	controllerVol          = 7
	controllerExpr         = 11
	controllerResetAll     = 121
	controllerAllSoundsOff = 120
	controllerAllNotesOff  = 123
	defaultChanVol         = 100
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
	synthCh    int
}

type Driver struct {
	synth        sound.Synth
	sampleRate   int
	ticRate      int
	musPanMax    float64
	outputGain   float64
	preEmphasis  bool
	preEmphPrev  [2]float64
	bank         PatchBank
	ch           [16]channelState
	voices       []voiceState
	freeList     []int
	allocList    []int
	allocScratch []int
}

func (d *Driver) ApplyEvent(ev Event) {
	d.applyEvent(ev)
}

func (d *Driver) GenerateStereoS16(frames int) []int16 {
	return d.generateStereoS16(frames)
}

func (d *Driver) SampleRate() int {
	if d == nil || d.sampleRate <= 0 {
		return OutputSampleRate
	}
	return d.sampleRate
}

func (d *Driver) TicRate() int {
	if d == nil || d.ticRate <= 0 {
		return defaultTicRate
	}
	return d.ticRate
}

func NewDriver(sampleRate int, bank PatchBank) *Driver {
	d, err := NewDriverWithBackend(sampleRate, bank, BackendAuto)
	if err != nil {
		return nil
	}
	return d
}

func NewDriverWithBackend(sampleRate int, bank PatchBank, backend Backend) (*Driver, error) {
	if sampleRate <= 0 {
		sampleRate = OutputSampleRate
	}
	if bank == nil {
		bank = DefaultPatchBank{}
	}
	if err := ValidateBackend(backend); err != nil {
		return nil, err
	}
	if ResolveBackend(backend) != BackendImpSynth {
		return nil, fmt.Errorf("music: backend %q does not use the OPL driver", backend)
	}
	synth, err := sound.NewSynthWithBackend(sampleRate, sound.BackendImpSynth)
	if err != nil {
		return nil, err
	}
	d := &Driver{
		synth:        synth,
		sampleRate:   sampleRate,
		ticRate:      defaultTicRate,
		musPanMax:    defaultMUSPanMax,
		outputGain:   DefaultOutputGain,
		bank:         bank,
		voices:       make([]voiceState, defaultVoices),
		freeList:     make([]int, 0, defaultVoices),
		allocList:    make([]int, 0, defaultVoices),
		allocScratch: make([]int, 0, defaultVoices),
	}
	for i := range d.voices {
		d.voices[i].synthCh = i
		d.freeList = append(d.freeList, i)
	}
	for i := range d.ch {
		d.ch[i] = channelState{
			volume:     defaultChanVol,
			expression: defaultChanExpr,
			pan:        defaultChanPan,
		}
	}
	return d, nil
}

func NewOutputDriver(bank PatchBank) *Driver {
	d, err := NewDriverWithBackend(OutputSampleRate, bank, BackendAuto)
	if err != nil {
		return nil
	}
	return d
}

func NewOutputDriverWithBackend(bank PatchBank, backend Backend) (*Driver, error) {
	return NewDriverWithBackend(OutputSampleRate, bank, backend)
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

func (d *Driver) SetPreEmphasis(enabled bool) {
	if d == nil {
		return
	}
	d.preEmphasis = enabled
	d.preEmphPrev = [2]float64{}
}

func (d *Driver) Reset() {
	if d == nil {
		return
	}
	d.synth.Reset()
	// Initialize the low register set expected by the music path.
	d.synth.WriteReg(0x04, 0x60) // reset timers
	d.synth.WriteReg(0x04, 0x80) // enable interrupts
	d.synth.WriteReg(0x01, 0x20) // waveform control enable
	// Enable extended register mode.
	d.synth.WriteReg(0x105, 0x01)
	d.synth.WriteReg(0x08, 0x40) // FM mode / keyboard split
	for i := range d.voices {
		d.voices[i] = voiceState{synthCh: i}
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
	d.preEmphPrev = [2]float64{}
}

// Render processes events and returns signed 16-bit stereo interleaved PCM.
func (d *Driver) Render(events []Event) []int16 {
	if d == nil || d.synth == nil || d.sampleRate <= 0 {
		return nil
	}
	if d.ticRate <= 0 {
		d.ticRate = defaultTicRate
	}
	if len(d.voices) == 0 {
		d.voices = make([]voiceState, defaultVoices)
		for i := range d.voices {
			d.voices[i].synthCh = i
		}
	}
	d.ensureVoiceLists()
	pcm := make([]int16, estimatedPCMBytesForEvents(events, d.sampleRate, d.ticRate)/2)
	n := 0
	for _, ev := range events {
		if ev.DeltaTics > 0 {
			frames := int((uint64(ev.DeltaTics) * uint64(d.sampleRate)) / uint64(d.ticRate))
			if frames > 0 {
				n += copy(pcm[n:], d.generateStereoS16(frames))
			}
		}
		d.applyEvent(ev)
		if ev.Type == EventEnd {
			break
		}
	}
	return pcm[:n]
}

// RenderMUS parses a MUS stream and renders it to signed 16-bit stereo PCM.
func (d *Driver) RenderMUS(musData []byte) ([]int16, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, err
	}
	return d.RenderParsedMUS(parsed)
}

func (d *Driver) RenderParsedMUS(parsed *ParsedMUS) ([]int16, error) {
	if d == nil || d.synth == nil || d.sampleRate <= 0 {
		return nil, nil
	}
	if parsed == nil {
		return nil, nil
	}
	if d.ticRate <= 0 {
		d.ticRate = defaultTicRate
	}
	if len(d.voices) == 0 {
		d.voices = make([]voiceState, defaultVoices)
		for i := range d.voices {
			d.voices[i].synthCh = i
		}
	}
	d.ensureVoiceLists()
	pcm := make([]int16, estimatedPCMBytesForEvents(parsed.events, d.sampleRate, d.ticRate)/2)
	n := 0
	for _, ev := range parsed.events {
		if ev.DeltaTics > 0 {
			frames := int((uint64(ev.DeltaTics) * uint64(d.sampleRate)) / uint64(d.ticRate))
			if frames > 0 {
				n += copy(pcm[n:], d.generateStereoS16(frames))
			}
		}
		d.applyEvent(ev)
	}
	return pcm[:n], nil
}

// RenderMUSS16LE parses MUS and returns little-endian signed 16-bit stereo PCM bytes.
func (d *Driver) RenderMUSS16LE(musData []byte) ([]byte, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, err
	}
	return d.RenderParsedMUSS16LE(parsed)
}

func (d *Driver) RenderParsedMUSS16LE(parsed *ParsedMUS) ([]byte, error) {
	return renderParsedMUSS16LE(d, parsed)
}

func PCMInt16ToBytesLE(samples []int16) []byte {
	if len(samples) == 0 {
		return nil
	}
	out := make([]byte, len(samples)*2)
	return PCMInt16ToBytesLEInto(out, samples)
}

func PCMInt16ToBytesLEInto(dst []byte, samples []int16) []byte {
	if len(samples) == 0 {
		return nil
	}
	need := len(samples) * 2
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}
	if nativeLittleEndian() {
		copy(dst, pcmInt16ViewAsBytesLE(samples))
		return dst
	}
	oi := 0
	for _, s := range samples {
		dst[oi] = byte(s)
		dst[oi+1] = byte(s >> 8)
		oi += 2
	}
	return dst
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
			d.refreshChannelVolume(uint8(ch))
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
		// The music path only uses the MIDI pitch bend MSB.
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
		d.writePatch(v.synthCh, np.Patch)
		d.writeVolume(v.synthCh, ch, velocity, np.Patch)
		d.writePan(v.synthCh, ch, np.Patch.C0)
		v.freqWord = d.writeNote(v.synthCh, v.playNote, d.ch[ch&0x0F].pitchBend+v.fineTune, true)
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
	reordered := d.allocScratch[:0]
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			reordered = append(reordered, vi)
		}
	}
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		v.freqWord = d.writeNote(v.synthCh, v.playNote, d.ch[ch&0x0F].pitchBend+v.fineTune, true)
		reordered = append(reordered, vi)
	}
	d.allocScratch = d.allocList[:0]
	d.allocList = reordered
}

func (d *Driver) refreshChannelVolume(ch uint8) {
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		d.writeVolume(v.synthCh, ch, v.velocity, v.patch)
	}
}

func (d *Driver) refreshChannelPan(ch uint8) {
	for _, vi := range d.allocList {
		v := &d.voices[vi]
		if !v.active || v.ch != ch {
			continue
		}
		d.writePan(v.synthCh, ch, v.patch.C0)
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
	d.writeFreqWord(v.synthCh, v.freqWord, false)
	synthCh := v.synthCh
	*v = voiceState{synthCh: synthCh}
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

func (d *Driver) writePatch(synthCh int, p Patch) {
	base, ch := synthAddrBase(synthCh)
	modSlot, carSlot := oplSlots(ch)

	// Apply voice/operator state in the expected register order:
	// load carrier first at minimum volume, then modulator. In additive mode
	// the modulator also starts at minimum volume until SetVoiceVolume runs.
	carInitTL := (p.Car40 & 0xC0) | 0x3F
	modInitTL := p.Mod40
	if (p.C0 & 0x01) != 0 {
		modInitTL = (p.Mod40 & 0xC0) | 0x3F
	}

	d.synth.WriteReg(uint16(base+0x40+carSlot), carInitTL)
	d.synth.WriteReg(uint16(base+0x20+carSlot), p.Car20)
	d.synth.WriteReg(uint16(base+0x60+carSlot), p.Car60)
	d.synth.WriteReg(uint16(base+0x80+carSlot), p.Car80)
	d.synth.WriteReg(uint16(base+0xE0+carSlot), p.CarE0)

	d.synth.WriteReg(uint16(base+0x40+modSlot), modInitTL)
	d.synth.WriteReg(uint16(base+0x20+modSlot), p.Mod20)
	d.synth.WriteReg(uint16(base+0x60+modSlot), p.Mod60)
	d.synth.WriteReg(uint16(base+0x80+modSlot), p.Mod80)
	d.synth.WriteReg(uint16(base+0xE0+modSlot), p.ModE0)
}

func (d *Driver) writeVolume(synthCh int, ch, velocity uint8, patch Patch) {
	base, ci := synthAddrBase(synthCh)
	modSlot, carSlot := oplSlots(ci)
	cv := clampMIDI7(int(d.ch[ch&0x0F].volume))
	ce := clampMIDI7(int(d.ch[ch&0x0F].expression))
	vel := clampMIDI7(int(velocity))
	midiVolume := 2 * (volumeMappingTable[cv] + 1)
	fullVolume := (volumeMappingTable[vel] * midiVolume * (volumeMappingTable[ce] + 1)) >> 16
	carTL := 0x3f - fullVolume
	if carTL < 0 {
		carTL = 0
	}
	if carTL > 0x3f {
		carTL = 0x3f
	}
	d.synth.WriteReg(uint16(base+0x40+carSlot), (patch.Car40&0xC0)|uint8(carTL))

	// DMX behavior: in non-modulated feedback mode, modulator volume follows carrier.
	if (patch.C0&0x01) != 0 && (patch.Mod40&0x3f) != 0x3f {
		modTL := int(patch.Mod40 & 0x3f)
		if modTL < carTL {
			modTL = carTL
		}
		d.synth.WriteReg(uint16(base+0x40+modSlot), (patch.Mod40&0xC0)|uint8(modTL))
	}
}

func (d *Driver) writePan(synthCh int, ch, c0 uint8) {
	base, ci := synthAddrBase(synthCh)
	pan := scaleMUSPan(d.ch[ch&0x0F].pan, d.musPanMax)
	var lr uint8 = 0x30
	switch {
	case pan >= 96:
		lr = 0x10
	case pan <= 48:
		lr = 0x20
	}
	d.synth.WriteReg(uint16(base+0xC0+ci), (c0&0x0F)|lr)
}

func (d *Driver) writeNote(synthCh int, note uint8, bend int16, keyOn bool) uint16 {
	freqWord := dmxFrequencyWord(int(note), bend)
	d.writeFreqWord(synthCh, freqWord, keyOn)
	return freqWord
}

func (d *Driver) writeFreqWord(synthCh int, freqWord uint16, keyOn bool) {
	base, ci := synthAddrBase(synthCh)
	a := uint8(freqWord & 0x00FF)
	b := uint8((freqWord >> 8) & 0x1F)
	if keyOn {
		b |= 0x20
	}
	d.synth.WriteReg(uint16(base+0xA0+ci), a)
	d.synth.WriteReg(uint16(base+0xB0+ci), b)
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

func clampMUSVolumeCompression(v float64) float64 {
	if math.IsNaN(v) || v < 1 {
		return 1
	}
	if v > 8 {
		return 8
	}
	return v
}

func compressMUSLevel(v uint8, ratio float64) uint8 {
	ratio = clampMUSVolumeCompression(ratio)
	if ratio <= 1 {
		return v
	}
	x := float64(clampMIDI7(int(v))) / 127.0
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 127
	}
	y := math.Pow(x, 1.0/ratio)
	return uint8(clampMIDI7(int(math.Round(y * 127.0))))
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
	if d == nil || d.synth == nil || frames <= 0 {
		return nil
	}
	out := d.synth.GenerateStereoS16(frames)
	if len(out) == 0 {
		return out
	}
	applyOutputGainSoftKnee(out, d.outputGain)
	if d.preEmphasis {
		applyPreEmphasis(out, &d.preEmphPrev)
	}
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

func applyPreEmphasis(samples []int16, prev *[2]float64) {
	if len(samples) == 0 || prev == nil {
		return
	}
	const coeff = 0.85
	for i := 0; i+1 < len(samples); i += 2 {
		for ch := 0; ch < 2; ch++ {
			in := float64(samples[i+ch])
			out := in - coeff*prev[ch]
			prev[ch] = in
			if out > 32767 {
				out = 32767
			} else if out < -32768 {
				out = -32768
			}
			samples[i+ch] = int16(out)
		}
	}
}

// Volume mapping table used by the music driver.
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

func synthAddrBase(synthCh int) (base int, ch int) {
	if synthCh < 9 {
		return 0x000, synthCh
	}
	return 0x100, synthCh - 9
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
