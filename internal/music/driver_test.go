package music

import (
	"math"
	"testing"
)

func TestDriverRenderSimpleNote(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	events := []Event{
		{Type: EventProgramChange, Channel: 0, A: 0},
		{Type: EventNoteOn, Channel: 0, A: 60, B: 120},
		{Type: EventNoteOff, Channel: 0, A: 60, DeltaTics: 35},
		{Type: EventEnd},
	}
	pcm := d.Render(events)
	if len(pcm) == 0 {
		t.Fatal("expected non-empty PCM")
	}
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-zero PCM samples")
	}
}

func TestDMXFrequencyWordRange(t *testing.T) {
	for note := 0; note <= 127; note++ {
		freq := dmxFrequencyWord(note, 0)
		fnum := int(freq & 0x03FF)
		block := int((freq >> 10) & 0x07)
		if fnum < 1 || fnum > 1023 {
			t.Fatalf("note=%d fnum=%d out of range", note, fnum)
		}
		if block < 0 || block > 7 {
			t.Fatalf("note=%d block=%d out of range", note, block)
		}
	}
}

func TestVoiceStealKeepsRendering(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	var evs []Event
	// Force more notes than voices.
	for n := 24; n < 24+24; n++ {
		evs = append(evs, Event{Type: EventNoteOn, Channel: 0, A: uint8(n), B: 100, DeltaTics: 1})
	}
	evs = append(evs, Event{Type: EventEnd})
	pcm := d.Render(evs)
	if len(pcm) == 0 {
		t.Fatal("expected PCM after voice stealing")
	}
}

func TestDriverRenderMUS(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	score := []byte{
		0x40, 0, 0, // program 0 on channel 0
		0x90,      // note on, last
		0xBC, 110, // note 60 with velocity 110
		0x14,       // delta 20 tics
		0x00, 0x3C, // note off
		0x60, // end
	}
	mus := buildMUSTestLump(score)
	pcm, err := d.RenderMUS(mus)
	if err != nil {
		t.Fatalf("RenderMUS() error: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("expected non-empty PCM")
	}
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-zero PCM from MUS render")
	}
}

func TestDriverRenderMUSS16LE(t *testing.T) {
	d := NewOutputDriver(nil)
	d.Reset()
	score := []byte{
		0x10, 60, // note on ch0 default velocity
		0x80, 60, // note off ch0 with delay flag
		0x08, // delay 8 tics
		0x60, // end
	}
	mus := buildMUSTestLump(score)
	pcm, err := d.RenderMUSS16LE(mus)
	if err != nil {
		t.Fatalf("RenderMUSS16LE() error: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("expected non-empty byte PCM")
	}
	if len(pcm)%2 != 0 {
		t.Fatalf("pcm byte len=%d should be even", len(pcm))
	}
}

type capturePercussionPatchBank struct {
	percussionFlags []bool
}

func (b *capturePercussionPatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	b.percussionFlags = append(b.percussionFlags, percussion)
	return DefaultPatchBank{}.Patch(program, percussion, note)
}

func TestDriverPercussionChannelUsesPercussionPatch(t *testing.T) {
	bank := &capturePercussionPatchBank{}
	d := NewOutputDriver(bank)
	d.Reset()
	_ = d.Render([]Event{
		{Type: EventNoteOn, Channel: 9, A: 35, B: 100},
		{Type: EventEnd},
	})
	if len(bank.percussionFlags) == 0 {
		t.Fatal("expected at least one patch lookup")
	}
	if !bank.percussionFlags[0] {
		t.Fatalf("percussion flag=%v want=true", bank.percussionFlags[0])
	}
}

type doubleVoiceBank struct{}

func (doubleVoiceBank) Patch(program uint8, percussion bool, note uint8) Patch {
	return DefaultPatchBank{}.Patch(program, percussion, note)
}

func (doubleVoiceBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	return []NotePatch{
		{Patch: DefaultPatchBank{}.Patch(program, percussion, note)},
		{Patch: DefaultPatchBank{}.Patch(program, percussion, note), BaseNoteOffset: 12},
	}
}

type singleVoicePatchBank struct {
	patch Patch
}

func (b singleVoicePatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	return b.patch
}

func (b singleVoicePatchBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	return []NotePatch{{Patch: b.patch}}
}

func TestDriverNoteOffStopsAllVoicesForNote(t *testing.T) {
	d := NewDriver(49716, doubleVoiceBank{})
	d.Reset()
	_ = d.Render([]Event{
		{Type: EventNoteOn, Channel: 0, A: 60, B: 100},
		{Type: EventNoteOff, Channel: 0, A: 60},
		{Type: EventEnd},
	})
	for i, v := range d.voices {
		if v.active {
			t.Fatalf("voice %d still active after note off", i)
		}
	}
}

func TestResolveVoiceNote(t *testing.T) {
	if got := resolveVoiceNote(64, false, NotePatch{BaseNoteOffset: -12}); got != 52 {
		t.Fatalf("offset note=%d want=52", got)
	}
	if got := resolveVoiceNote(64, true, NotePatch{}); got != 60 {
		t.Fatalf("percussion note=%d want=60", got)
	}
	if got := resolveVoiceNote(64, false, NotePatch{Fixed: true, FixedNote: 40}); got != 40 {
		t.Fatalf("fixed note=%d want=40", got)
	}
	if got := resolveVoiceNote(120, false, NotePatch{}); got != 84 {
		t.Fatalf("wrapped note=%d want=84", got)
	}
}

func TestDriverPitchBendUsesMSB(t *testing.T) {
	d := NewDriver(49716, nil)
	d.Reset()
	// LSB differs; MSB same => same bend in DMX semantics.
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x00, B: 0x40})
	b0 := d.ch[0].pitchBend
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x7F, B: 0x40})
	b1 := d.ch[0].pitchBend
	if b0 != 0 || b1 != 0 {
		t.Fatalf("center bend b0=%d b1=%d want=0", b0, b1)
	}
	d.applyEvent(Event{Type: EventPitchBend, Channel: 0, A: 0x00, B: 0x41})
	if got := d.ch[0].pitchBend; got != 1 {
		t.Fatalf("bend=%d want=1", got)
	}
}

func TestDriverResetMatchesChocolateChannelDefaults(t *testing.T) {
	d := NewOutputDriver(nil)
	synth := &captureSynth{}
	d.synth = synth
	d.Reset()
	if got := len(d.voices); got != 18 {
		t.Fatalf("voice count=%d want=18", got)
	}
	if got, ok := synth.lastWrite(0x105); !ok || got != 0x01 {
		t.Fatalf("reset synth extended-mode write=%#02x ok=%v want 0x01,true", got, ok)
	}
	for i, ch := range d.ch {
		if ch.volume != 100 {
			t.Fatalf("channel %d volume=%d want=100", i, ch.volume)
		}
		if ch.expression != 127 {
			t.Fatalf("channel %d expression=%d want=127", i, ch.expression)
		}
		if ch.pan != 64 {
			t.Fatalf("channel %d pan=%d want=64", i, ch.pan)
		}
	}
}

func TestClampMIDI7(t *testing.T) {
	if got := clampMIDI7(-5); got != 0 {
		t.Fatalf("clamp -5=%d want=0", got)
	}
	if got := clampMIDI7(200); got != 127 {
		t.Fatalf("clamp 200=%d want=127", got)
	}
	if got := clampMIDI7(64); got != 64 {
		t.Fatalf("clamp 64=%d want=64", got)
	}
}

type synthRegWrite struct {
	addr uint16
	val  uint8
}

type captureSynth struct {
	writes []synthRegWrite
	pcm    []int16
}

func (o *captureSynth) Reset() {}

func (o *captureSynth) WriteReg(addr uint16, value uint8) {
	o.writes = append(o.writes, synthRegWrite{addr: addr, val: value})
}

func (o *captureSynth) GenerateStereoS16(frames int) []int16 {
	if frames <= 0 {
		return nil
	}
	if len(o.pcm) == 0 {
		return make([]int16, frames*2)
	}
	out := make([]int16, frames*2)
	for i := range out {
		out[i] = o.pcm[i%len(o.pcm)]
	}
	return out
}

func (o *captureSynth) GenerateMonoU8(frames int) []byte { return nil }

func (o *captureSynth) wroteAddr(addr uint16) bool {
	for _, w := range o.writes {
		if w.addr == addr {
			return true
		}
	}
	return false
}

func (o *captureSynth) lastWrite(addr uint16) (uint8, bool) {
	for i := len(o.writes) - 1; i >= 0; i-- {
		if o.writes[i].addr == addr {
			return o.writes[i].val, true
		}
	}
	return 0, false
}

func TestDriverControlChangeRefreshesActiveVoiceRegisters(t *testing.T) {
	d := NewOutputDriver(nil)
	synth := &captureSynth{}
	d.synth = synth
	d.Reset()
	d.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})
	if !d.voices[0].active {
		t.Fatal("expected voice 0 active after note-on")
	}

	synth.writes = nil
	d.applyEvent(Event{Type: EventControlChange, Channel: 0, A: controllerVol, B: 90})
	if !synth.wroteAddr(0x43) {
		t.Fatal("expected carrier volume register write on controller volume change")
	}

	synth.writes = nil
	d.applyEvent(Event{Type: EventControlChange, Channel: 0, A: controllerPan, B: 127})
	if !synth.wroteAddr(0xC0) {
		t.Fatal("expected pan/feedback register write on controller pan change")
	}
}

func TestDriverNoteOnProgramsOperatorsInChocolateOrder(t *testing.T) {
	d := NewOutputDriver(nil)
	synth := &captureSynth{}
	d.synth = synth
	d.Reset()

	patch := Patch{
		Mod20: 0x21, Mod40: 0x12, Mod60: 0xF3, Mod80: 0x24, ModE0: 0x02,
		Car20: 0x01, Car40: 0x05, Car60: 0xF4, Car80: 0x25, CarE0: 0x03,
		C0: 0x01,
	}
	d.bank = singleVoicePatchBank{patch: patch}
	synth.writes = nil

	d.noteOn(0, 60, 100)

	want := []synthRegWrite{
		{addr: 0x43, val: 0x3F},
		{addr: 0x23, val: patch.Car20},
		{addr: 0x63, val: patch.Car60},
		{addr: 0x83, val: patch.Car80},
		{addr: 0xE3, val: patch.CarE0},
		{addr: 0x40, val: 0x3F},
		{addr: 0x20, val: patch.Mod20},
		{addr: 0x60, val: patch.Mod60},
		{addr: 0x80, val: patch.Mod80},
		{addr: 0xE0, val: patch.ModE0},
	}

	if len(synth.writes) < len(want) {
		t.Fatalf("writes=%d want at least %d", len(synth.writes), len(want))
	}
	for i, w := range want {
		if synth.writes[i] != w {
			t.Fatalf("write[%d]=%+v want=%+v", i, synth.writes[i], w)
		}
	}
}

func TestDriverPitchBendReordersUpdatedVoicesToEnd(t *testing.T) {
	d := NewOutputDriver(nil)
	d.synth = &captureSynth{}
	d.Reset()
	d.allocList = []int{0, 1, 2}
	d.freeList = d.freeList[:0]
	d.voices[0] = voiceState{active: true, ch: 0, playNote: 60, synthCh: 0}
	d.voices[1] = voiceState{active: true, ch: 1, playNote: 62, synthCh: 1}
	d.voices[2] = voiceState{active: true, ch: 0, playNote: 64, synthCh: 2}

	d.refreshChannelPitch(0)
	got := d.allocList
	want := []int{1, 0, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("allocList=%v want=%v", got, want)
		}
	}
}

func TestDriverPercussionOutOfRangeIgnored(t *testing.T) {
	bank := &capturePercussionPatchBank{}
	d := NewOutputDriver(bank)
	d.Reset()
	d.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 34, B: 100})
	d.applyEvent(Event{Type: EventNoteOn, Channel: 9, A: 82, B: 100})
	if len(bank.percussionFlags) != 0 {
		t.Fatalf("unexpected patch lookups for out-of-range percussion notes: %d", len(bank.percussionFlags))
	}
	for i, v := range d.voices {
		if v.active {
			t.Fatalf("voice %d active for ignored percussion note", i)
		}
	}
}

func TestScaleMUSPan(t *testing.T) {
	if got := scaleMUSPan(0, 0.8); got != 13 {
		t.Fatalf("scaleMUSPan(0, 0.8)=%d want=13", got)
	}
	if got := scaleMUSPan(127, 0.8); got != 114 {
		t.Fatalf("scaleMUSPan(127, 0.8)=%d want=114", got)
	}
	if got := scaleMUSPan(100, 0.8); got != 93 {
		t.Fatalf("scaleMUSPan(100, 0.8)=%d want=93", got)
	}
	if got := scaleMUSPan(100, 2.0); got != 100 {
		t.Fatalf("scaleMUSPan(100, 2.0)=%d want=100", got)
	}
}

func TestDriverPanScalingAffectsSynthPanBucket(t *testing.T) {
	d := NewOutputDriver(nil)
	synth := &captureSynth{}
	d.synth = synth
	d.Reset()
	d.applyEvent(Event{Type: EventNoteOn, Channel: 0, A: 60, B: 100})
	if !d.voices[0].active {
		t.Fatal("expected active voice after note-on")
	}
	synth.writes = nil

	d.SetMUSPanMax(0.8)
	d.applyEvent(Event{Type: EventControlChange, Channel: 0, A: controllerPan, B: 100})
	val, ok := synth.lastWrite(0xC0)
	if !ok {
		t.Fatal("expected C0 pan register write")
	}
	if got := val & 0x30; got != 0x30 {
		t.Fatalf("scaled pan lr bits=0x%02X want=0x30 (center)", got)
	}
	if synth.wroteAddr(0xD0) {
		t.Fatal("unexpected D0 stereo-extension pan register write")
	}

	synth.writes = nil
	d.SetMUSPanMax(1.0)
	d.applyEvent(Event{Type: EventControlChange, Channel: 0, A: controllerPan, B: 100})
	val, ok = synth.lastWrite(0xC0)
	if !ok {
		t.Fatal("expected C0 pan register write")
	}
	if got := val & 0x30; got != 0x10 {
		t.Fatalf("full pan lr bits=0x%02X want=0x10 (right)", got)
	}
	if synth.wroteAddr(0xD0) {
		t.Fatal("unexpected D0 stereo-extension pan register write")
	}
}

func TestSetOutputGainClampsRange(t *testing.T) {
	d := NewOutputDriver(nil)
	d.SetOutputGain(-1)
	if d.outputGain != 0 {
		t.Fatalf("outputGain=%v want 0", d.outputGain)
	}
	d.SetOutputGain(math.NaN())
	if d.outputGain != 0 {
		t.Fatalf("outputGain after NaN=%v want 0", d.outputGain)
	}
	d.SetOutputGain(99)
	if d.outputGain != MaxOutputGain {
		t.Fatalf("outputGain=%v want %v", d.outputGain, MaxOutputGain)
	}
}

func TestGenerateStereoS16AppliesOutputGainSoftKnee(t *testing.T) {
	d := NewOutputDriver(nil)
	d.synth = &captureSynth{pcm: []int16{10000, -10000}}
	d.SetOutputGain(2.0)
	out := d.generateStereoS16(1)
	if len(out) != 2 {
		t.Fatalf("len(out)=%d want 2", len(out))
	}
	if out[0] <= 10000 {
		t.Fatalf("left sample=%d want >10000", out[0])
	}
	if out[1] >= -10000 {
		t.Fatalf("right sample=%d want <-10000", out[1])
	}
}

func TestApplyOutputGainSoftKneeLimitsClipping(t *testing.T) {
	samples := []int16{30000, -30000}
	applyOutputGainSoftKnee(samples, 4.0)
	if samples[0] >= 32767 || samples[1] <= -32768 {
		t.Fatalf("expected soft-knee headroom before hard clip, got %v", samples)
	}
}

func TestSetPreEmphasisTogglesAndResetsState(t *testing.T) {
	d := NewOutputDriver(nil)
	d.preEmphPrev = [2]float64{12, -8}
	d.SetPreEmphasis(true)
	if !d.preEmphasis {
		t.Fatal("preEmphasis=false want true")
	}
	if d.preEmphPrev != [2]float64{} {
		t.Fatalf("preEmphPrev=%v want zeroed", d.preEmphPrev)
	}
	d.preEmphPrev = [2]float64{1, 2}
	d.SetPreEmphasis(false)
	if d.preEmphasis {
		t.Fatal("preEmphasis=true want false")
	}
	if d.preEmphPrev != [2]float64{} {
		t.Fatalf("preEmphPrev=%v want zeroed after disable", d.preEmphPrev)
	}
}

func TestGenerateStereoS16AppliesPreEmphasisWhenEnabled(t *testing.T) {
	d := NewOutputDriver(nil)
	d.synth = &captureSynth{pcm: []int16{1000, -1000, 500, -500}}
	d.SetOutputGain(1.0)
	d.SetPreEmphasis(true)
	out := d.generateStereoS16(2)
	if len(out) != 4 {
		t.Fatalf("len(out)=%d want 4", len(out))
	}
	if out[0] != 1000 || out[1] != -1000 {
		t.Fatalf("first frame=%v want unchanged first sample pair", out[:2])
	}
	if out[2] == 500 || out[3] == -500 {
		t.Fatalf("second frame=%v want pre-emphasized samples", out[2:4])
	}
}
