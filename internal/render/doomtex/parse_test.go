package doomtex

import (
	"encoding/binary"
	"testing"
)

func TestParsePNames(t *testing.T) {
	b := make([]byte, 4+2*8)
	binary.LittleEndian.PutUint32(b[0:4], 2)
	copy(b[4:12], []byte{'P', 'A', 'T', 'C', 'H', 'A'})
	copy(b[12:20], []byte{'P', 'A', 'T', 'C', 'H', 'B'})
	got, err := parsePNames(b)
	if err != nil {
		t.Fatalf("parsePNames: %v", err)
	}
	if len(got) != 2 || got[0] != "PATCHA" || got[1] != "PATCHB" {
		t.Fatalf("unexpected pnames: %#v", got)
	}
}

func TestParseTextureLumpSingle(t *testing.T) {
	patchNames := []string{"PATCHA"}
	// count=1, offsets[0]=8; texture entry starts at byte 8
	b := make([]byte, 8+22+10)
	binary.LittleEndian.PutUint32(b[0:4], 1)
	binary.LittleEndian.PutUint32(b[4:8], 8)
	o := 8
	copy(b[o:o+8], []byte{'T', 'E', 'X', 'A'})
	binary.LittleEndian.PutUint16(b[o+12:o+14], 64)
	binary.LittleEndian.PutUint16(b[o+14:o+16], 32)
	binary.LittleEndian.PutUint16(b[o+20:o+22], 1)
	po := o + 22
	binary.LittleEndian.PutUint16(b[po:po+2], 4)
	binary.LittleEndian.PutUint16(b[po+2:po+4], 8)
	binary.LittleEndian.PutUint16(b[po+4:po+6], 0)

	got, err := parseTextureLump(b, patchNames)
	if err != nil {
		t.Fatalf("parseTextureLump: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want=1", len(got))
	}
	if got[0].Name != "TEXA" || got[0].Width != 64 || got[0].Height != 32 {
		t.Fatalf("bad texture header: %#v", got[0])
	}
	if len(got[0].Patches) != 1 || got[0].Patches[0].PatchName != "PATCHA" {
		t.Fatalf("bad patches: %#v", got[0].Patches)
	}
}
