package music

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestParseGENMIDIOP2PatchBank(t *testing.T) {
	data := make([]byte, genmidiDataOffset+genmidiTotalInstrs*genmidiInstrSize)
	copy(data[:genmidiDataOffset], []byte(genmidiHeader))

	// Program 0, first voice.
	base0 := genmidiDataOffset
	data[base0+0] = 0x05 // fixed + 2voice flags
	data[base0+2] = 70   // fine_tuning
	data[base0+3] = 48   // fixed note
	voice0 := base0 + 4
	data[voice0+0] = 0x21 // mod 0x20
	data[voice0+1] = 0x42 // mod 0x60
	data[voice0+2] = 0x63 // mod 0x80
	data[voice0+3] = 0x01 // mod 0xE0
	data[voice0+4] = 0x80 // mod scale bits
	data[voice0+5] = 0x12 // mod level bits
	data[voice0+6] = 0x07 // C0 lower bits
	data[voice0+7] = 0x31 // car 0x20
	data[voice0+8] = 0x52 // car 0x60
	data[voice0+9] = 0x73 // car 0x80
	data[voice0+10] = 0x02
	data[voice0+11] = 0x40 // car scale bits
	data[voice0+12] = 0x0a // car level bits
	data[voice0+14] = 0xFF // base note offset = -1
	data[voice0+15] = 0xFF

	voice1 := base0 + 20
	data[voice1+0] = 0x41
	data[voice1+7] = 0x51
	data[voice1+14] = 0x02 // base note offset = +2
	data[voice1+15] = 0x00

	// Percussion note 35 maps to bank index 128.
	basePerc := genmidiDataOffset + genmidiInstrCount*genmidiInstrSize
	voicePerc := basePerc + 4
	data[voicePerc+0] = 0x11
	data[voicePerc+7] = 0x22

	bank, err := ParseGENMIDIOP2PatchBank(data)
	if err != nil {
		t.Fatalf("ParseGENMIDIOP2PatchBank() error: %v", err)
	}
	p := bank.Patch(0, false, 60)
	if p.Mod20 != 0x21 || p.Mod60 != 0x42 || p.Mod80 != 0x63 || p.ModE0 != 0x01 {
		t.Fatalf("program patch mod fields=%+v", p)
	}
	if p.Mod40 != 0x92 {
		t.Fatalf("program patch Mod40=%#02x want=0x92", p.Mod40)
	}
	if p.Car20 != 0x31 || p.Car60 != 0x52 || p.Car80 != 0x73 || p.CarE0 != 0x02 {
		t.Fatalf("program patch car fields=%+v", p)
	}
	if p.Car40 != 0x4a {
		t.Fatalf("program patch Car40=%#02x want=0x4a", p.Car40)
	}
	if p.C0 != 0x07 {
		t.Fatalf("program patch C0=%#02x want=0x07", p.C0)
	}

	pp := bank.Patch(0, true, 35)
	if pp.Mod20 != 0x11 || pp.Car20 != 0x22 {
		t.Fatalf("percussion patch=%+v", pp)
	}

	vb, ok := bank.(VoicePatchBank)
	if !ok {
		t.Fatalf("bank does not implement VoicePatchBank")
	}
	voices := vb.PatchVoices(0, false, 60)
	if len(voices) != 2 {
		t.Fatalf("voice count=%d want=2", len(voices))
	}
	if !voices[0].Fixed || voices[0].FixedNote != 48 || voices[0].BaseNoteOffset != -1 {
		t.Fatalf("voice0 metadata=%+v", voices[0])
	}
	if voices[1].Patch.Mod20 != 0x41 || voices[1].Patch.Car20 != 0x51 || voices[1].BaseNoteOffset != 2 {
		t.Fatalf("voice1=%+v", voices[1])
	}
	if voices[1].FineTune != -29 {
		t.Fatalf("voice1 fine_tune=%d want=-29", voices[1].FineTune)
	}
}

func TestParseGENMIDIOP2PatchBankErrors(t *testing.T) {
	if _, err := ParseGENMIDIOP2PatchBank([]byte("short")); err == nil {
		t.Fatal("expected short data error")
	}
	data := make([]byte, genmidiDataOffset+genmidiTotalInstrs*genmidiInstrSize)
	copy(data[:genmidiDataOffset], []byte("BADHEAD!"))
	if _, err := ParseGENMIDIOP2PatchBank(data); err == nil {
		t.Fatal("expected bad header error")
	}
}

func TestParseGENMIDIOP2PatchBankFile(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "bank.op2")
	data := make([]byte, genmidiDataOffset+genmidiTotalInstrs*genmidiInstrSize)
	copy(data[:genmidiDataOffset], []byte(genmidiHeader))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	bank, err := ParseGENMIDIOP2PatchBankFile(path)
	if err != nil {
		t.Fatalf("ParseGENMIDIOP2PatchBankFile() error: %v", err)
	}
	if bank == nil {
		t.Fatal("expected parsed patch bank")
	}
}

func TestParseRealWADGENMIDIMapsRawBytesDirectly(t *testing.T) {
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	lump, ok := wf.LumpByName("GENMIDI")
	if !ok {
		t.Fatalf("%s missing GENMIDI lump", wadPath)
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		t.Fatalf("read GENMIDI: %v", err)
	}
	parsedAny, err := ParseGENMIDIOP2PatchBank(data)
	if err != nil {
		t.Fatalf("ParseGENMIDIOP2PatchBank() error: %v", err)
	}
	parsed, ok := parsedAny.(*OP2PatchBank)
	if !ok {
		t.Fatalf("bank type=%T want *OP2PatchBank", parsedAny)
	}

	checkIndex := func(t *testing.T, idx int) {
		t.Helper()
		if idx < 0 || idx >= genmidiTotalInstrs {
			t.Fatalf("invalid instrument index %d", idx)
		}
		base := genmidiDataOffset + idx*genmidiInstrSize
		rawFlags := binary.LittleEndian.Uint16(data[base : base+2])
		rawFine := data[base+2]
		rawFixed := data[base+3]

		got := parsed.instruments[idx]
		if got.flags != rawFlags {
			t.Fatalf("idx=%d flags=%#04x want=%#04x", idx, got.flags, rawFlags)
		}
		if got.fineTuning != rawFine {
			t.Fatalf("idx=%d fine_tuning=%d want=%d", idx, got.fineTuning, rawFine)
		}
		if got.fixedNote != rawFixed {
			t.Fatalf("idx=%d fixed_note=%d want=%d", idx, got.fixedNote, rawFixed)
		}

		for voice := 0; voice < 2; voice++ {
			vbase := base + 4 + voice*16
			wantMod20 := data[vbase+0]
			wantMod40 := (data[vbase+4] & 0xC0) | (data[vbase+5] & 0x3F)
			wantMod60 := data[vbase+1]
			wantMod80 := data[vbase+2]
			wantModE0 := data[vbase+3]
			wantC0 := data[vbase+6] & 0x0F
			wantCar20 := data[vbase+7]
			wantCar40 := (data[vbase+11] & 0xC0) | (data[vbase+12] & 0x3F)
			wantCar60 := data[vbase+8]
			wantCar80 := data[vbase+9]
			wantCarE0 := data[vbase+10]
			wantBase := int16(binary.LittleEndian.Uint16(data[vbase+14 : vbase+16]))

			gotPatch := parsed.voiceToPatch(got.voices[voice])
			if gotPatch.Mod20 != wantMod20 || gotPatch.Mod40 != wantMod40 || gotPatch.Mod60 != wantMod60 || gotPatch.Mod80 != wantMod80 || gotPatch.ModE0 != wantModE0 {
				t.Fatalf("idx=%d voice=%d mod patch=%+v raw=(20=%#02x 40=%#02x 60=%#02x 80=%#02x e0=%#02x)", idx, voice, gotPatch, wantMod20, wantMod40, wantMod60, wantMod80, wantModE0)
			}
			if gotPatch.Car20 != wantCar20 || gotPatch.Car40 != wantCar40 || gotPatch.Car60 != wantCar60 || gotPatch.Car80 != wantCar80 || gotPatch.CarE0 != wantCarE0 {
				t.Fatalf("idx=%d voice=%d car patch=%+v raw=(20=%#02x 40=%#02x 60=%#02x 80=%#02x e0=%#02x)", idx, voice, gotPatch, wantCar20, wantCar40, wantCar60, wantCar80, wantCarE0)
			}
			if gotPatch.C0 != wantC0 {
				t.Fatalf("idx=%d voice=%d C0=%#02x want=%#02x", idx, voice, gotPatch.C0, wantC0)
			}
			if got.voices[voice].baseNoteOffset != wantBase {
				t.Fatalf("idx=%d voice=%d base_note_offset=%d want=%d", idx, voice, got.voices[voice].baseNoteOffset, wantBase)
			}
		}
	}

	for _, idx := range []int{0, 30, 44, 118, 128, 174} {
		t.Run(fmt.Sprintf("idx_%d", idx), func(t *testing.T) {
			checkIndex(t, idx)
		})
	}
}
