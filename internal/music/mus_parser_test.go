package music

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseMUSBasicSequence(t *testing.T) {
	score := []byte{
		0x90,      // type=1 note-on ch0 with last flag
		0xBC, 100, // note=60 with velocity=100
		0x0A,       // delta=10
		0x00, 0x3C, // type=0 note-off ch0 note=60
		0x60, // type=6 end
	}
	data := buildMUSTestLump(score)
	evs, err := ParseMUS(data)
	if err != nil {
		t.Fatalf("ParseMUS() error: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("events len=%d want=3", len(evs))
	}
	if evs[0].Type != EventNoteOn || evs[0].Channel != 0 || evs[0].A != 60 || evs[0].B != 100 {
		t.Fatalf("event0=%+v", evs[0])
	}
	if evs[1].DeltaTics != 10 {
		t.Fatalf("event1 delta=%d want=10", evs[1].DeltaTics)
	}
	if evs[1].Type != EventNoteOff || evs[1].A != 60 {
		t.Fatalf("event1=%+v", evs[1])
	}
	if evs[2].Type != EventEnd {
		t.Fatalf("event2=%+v want end", evs[2])
	}
}

func TestParseMUSPercussionChannelMapping(t *testing.T) {
	score := []byte{
		0x1F, 35, // note-on ch15 (percussion), default vel
		0x60, // end
	}
	evs, err := ParseMUS(buildMUSTestLump(score))
	if err != nil {
		t.Fatalf("ParseMUS() error: %v", err)
	}
	if len(evs) < 1 {
		t.Fatal("expected at least one event")
	}
	if evs[0].Channel != 9 {
		t.Fatalf("percussion channel mapped=%d want=9", evs[0].Channel)
	}
}

func TestParseMUSControllerAndProgram(t *testing.T) {
	score := []byte{
		0x40, 0, 23, // controller ch0, ctrl=0 => program 23
		0x40, 3, 100, // controller ch0, ctrl=3 => MIDI volume
		0x60,
	}
	evs, err := ParseMUS(buildMUSTestLump(score))
	if err != nil {
		t.Fatalf("ParseMUS() error: %v", err)
	}
	if len(evs) < 3 {
		t.Fatalf("events len=%d want>=3", len(evs))
	}
	if evs[0].Type != EventProgramChange || evs[0].A != 23 {
		t.Fatalf("event0=%+v", evs[0])
	}
	if evs[1].Type != EventControlChange || evs[1].A != 7 || evs[1].B != 100 {
		t.Fatalf("event1=%+v", evs[1])
	}
}

func TestParseMUSControllerBankSelectMatchesChocolate(t *testing.T) {
	score := []byte{
		0x40, 1, 9, // controller ch0, ctrl=1 => MIDI CC 32 in mus2mid
		0x60,
	}
	evs, err := ParseMUS(buildMUSTestLump(score))
	if err != nil {
		t.Fatalf("ParseMUS() error: %v", err)
	}
	if len(evs) < 1 {
		t.Fatal("expected controller event")
	}
	if evs[0].Type != EventControlChange || evs[0].A != 32 || evs[0].B != 9 {
		t.Fatalf("event0=%+v want control=32 value=9", evs[0])
	}
}

func TestParseMUSPitchWheelUsesRawByte(t *testing.T) {
	score := []byte{
		0x20, 0x80, // pitch wheel ch0 raw 0x80
		0x60,
	}
	evs, err := ParseMUS(buildMUSTestLump(score))
	if err != nil {
		t.Fatalf("ParseMUS() error: %v", err)
	}
	if len(evs) < 1 {
		t.Fatal("expected pitch event")
	}
	if evs[0].Type != EventPitchBend {
		t.Fatalf("event0=%+v", evs[0])
	}
	if evs[0].A != 0x00 || evs[0].B != 0x40 {
		t.Fatalf("pitch bytes A=%d B=%d want 0 64", evs[0].A, evs[0].B)
	}
}

func buildMUSTestLump(score []byte) []byte {
	var b bytes.Buffer
	b.WriteString("MUS\x1a")
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(score))) // score len
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))         // score start
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))          // primary channels
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))          // secondary channels
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))          // instrument count
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))          // reserved
	b.Write(score)
	return b.Bytes()
}
