package music

import (
	"encoding/binary"
	"errors"
)

const (
	genmidiHeader      = "#OPL_II#"
	genmidiInstrCount  = 128
	genmidiPercStart   = 35
	genmidiPercCount   = 47
	genmidiTotalInstrs = genmidiInstrCount + genmidiPercCount
	genmidiInstrSize   = 36
	genmidiDataOffset  = 8

	genmidiFlagFixed  = 0x0001
	genmidiFlag2Voice = 0x0004
)

var (
	errGENMIDITooShort    = errors.New("music: GENMIDI/OP2 data too short")
	errGENMIDIInvalidHead = errors.New("music: invalid GENMIDI/OP2 header")
)

type op2Operator struct {
	tremolo uint8
	attack  uint8
	sustain uint8
	wave    uint8
	scale   uint8
	level   uint8
}

type op2Voice struct {
	mod            op2Operator
	feedback       uint8
	car            op2Operator
	_              uint8
	baseNoteOffset int16
}

type op2Instrument struct {
	flags      uint16
	fineTuning uint8
	fixedNote  uint8
	voices     [2]op2Voice
}

// OP2PatchBank maps GENMIDI/OP2 instruments to the driver's patch format.
type OP2PatchBank struct {
	instruments [genmidiTotalInstrs]op2Instrument
}

// ParseGENMIDIOP2 parses a Doom-compatible GENMIDI/OP2 bank.
func ParseGENMIDIOP2(data []byte) (*OP2PatchBank, error) {
	minLen := genmidiDataOffset + genmidiTotalInstrs*genmidiInstrSize
	if len(data) < minLen {
		return nil, errGENMIDITooShort
	}
	if string(data[:genmidiDataOffset]) != genmidiHeader {
		return nil, errGENMIDIInvalidHead
	}
	b := &OP2PatchBank{}
	for i := 0; i < genmidiTotalInstrs; i++ {
		base := genmidiDataOffset + i*genmidiInstrSize
		in := &b.instruments[i]
		in.flags = binary.LittleEndian.Uint16(data[base : base+2])
		in.fineTuning = data[base+2]
		in.fixedNote = data[base+3]
		in.voices[0] = parseOP2Voice(data[base+4 : base+20])
		in.voices[1] = parseOP2Voice(data[base+20 : base+36])
	}
	return b, nil
}

func parseOP2Voice(data []byte) op2Voice {
	if len(data) < 16 {
		return op2Voice{}
	}
	return op2Voice{
		mod: op2Operator{
			tremolo: data[0],
			attack:  data[1],
			sustain: data[2],
			wave:    data[3],
			scale:   data[4],
			level:   data[5],
		},
		feedback: data[6],
		car: op2Operator{
			tremolo: data[7],
			attack:  data[8],
			sustain: data[9],
			wave:    data[10],
			scale:   data[11],
			level:   data[12],
		},
		baseNoteOffset: int16(binary.LittleEndian.Uint16(data[14:16])),
	}
}

func (b *OP2PatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	if b == nil {
		return DefaultPatchBank{}.Patch(program, percussion, note)
	}
	idx := b.instrumentIndex(program, percussion, note)
	if idx < 0 || idx >= len(b.instruments) {
		idx = 0
	}
	v := b.instruments[idx].voices[0]
	return Patch{
		Mod20: v.mod.tremolo,
		Mod40: (v.mod.scale & 0xC0) | (v.mod.level & 0x3F),
		Mod60: v.mod.attack,
		Mod80: v.mod.sustain,
		ModE0: v.mod.wave,
		Car20: v.car.tremolo,
		Car40: (v.car.scale & 0xC0) | (v.car.level & 0x3F),
		Car60: v.car.attack,
		Car80: v.car.sustain,
		CarE0: v.car.wave,
		C0:    v.feedback & 0x0F,
	}
}

// ParseGENMIDIOP2PatchBank parses bytes and returns a PatchBank interface.
func ParseGENMIDIOP2PatchBank(data []byte) (PatchBank, error) {
	return ParseGENMIDIOP2(data)
}

func (b *OP2PatchBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	if b == nil {
		return nil
	}
	idx := b.instrumentIndex(program, percussion, note)
	if idx < 0 || idx >= len(b.instruments) {
		return nil
	}
	in := b.instruments[idx]
	voices := []NotePatch{{
		Patch:          b.voiceToPatch(in.voices[0]),
		Fixed:          (in.flags & genmidiFlagFixed) != 0,
		FixedNote:      in.fixedNote,
		BaseNoteOffset: in.voices[0].baseNoteOffset,
	}}
	if (in.flags & genmidiFlag2Voice) != 0 {
		voices = append(voices, NotePatch{
			Patch:          b.voiceToPatch(in.voices[1]),
			Fixed:          (in.flags & genmidiFlagFixed) != 0,
			FixedNote:      in.fixedNote,
			BaseNoteOffset: in.voices[1].baseNoteOffset,
			FineTune:       int16(in.fineTuning/2) - 64,
		})
	}
	return voices
}

func (b *OP2PatchBank) voiceToPatch(v op2Voice) Patch {
	return Patch{
		Mod20: v.mod.tremolo,
		Mod40: (v.mod.scale & 0xC0) | (v.mod.level & 0x3F),
		Mod60: v.mod.attack,
		Mod80: v.mod.sustain,
		ModE0: v.mod.wave,
		Car20: v.car.tremolo,
		Car40: (v.car.scale & 0xC0) | (v.car.level & 0x3F),
		Car60: v.car.attack,
		Car80: v.car.sustain,
		CarE0: v.car.wave,
		C0:    v.feedback & 0x0F,
	}
}

func (b *OP2PatchBank) instrumentIndex(program uint8, percussion bool, note uint8) int {
	idx := int(program)
	if percussion {
		if note < genmidiPercStart || note >= genmidiPercStart+genmidiPercCount {
			idx = genmidiInstrCount
		} else {
			idx = genmidiInstrCount + int(note-genmidiPercStart)
		}
	}
	return idx
}
