package sound

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestParseDigitalLumpValid(t *testing.T) {
	data := make([]byte, 8+4)
	binary.LittleEndian.PutUint16(data[0:2], 3)
	binary.LittleEndian.PutUint16(data[2:4], 11025)
	binary.LittleEndian.PutUint32(data[4:8], 4)
	data[8] = 0x80
	data[9] = 0x81
	data[10] = 0x82
	data[11] = 0x83

	s, err := ParseDigitalLump("DSSAMP", data)
	if err != nil {
		t.Fatalf("ParseDigitalLump() error=%v", err)
	}
	if s.SampleRate != 11025 || len(s.Samples) != 4 {
		t.Fatalf("decoded %+v", s)
	}
}

func TestParseDigitalLumpSharesPayloadSlice(t *testing.T) {
	data := make([]byte, 8+2)
	binary.LittleEndian.PutUint16(data[0:2], 3)
	binary.LittleEndian.PutUint16(data[2:4], 11025)
	binary.LittleEndian.PutUint32(data[4:8], 2)
	data[8] = 0x80
	data[9] = 0x81

	s, err := ParseDigitalLump("DSSAMP", data)
	if err != nil {
		t.Fatalf("ParseDigitalLump() error=%v", err)
	}
	data[8] = 0x42
	if s.Samples[0] != 0x42 {
		t.Fatalf("Samples[0]=%#x want shared payload %#x", s.Samples[0], byte(0x42))
	}
}

func TestParseDigitalLumpBadFormat(t *testing.T) {
	data := []byte{2, 0, 0x11, 0x2b, 1, 0, 0, 0, 0x80}
	if _, err := ParseDigitalLump("DSSAMP", data); err == nil {
		t.Fatal("expected format error")
	}
}

func TestParseDigitalLumpSizeMismatch(t *testing.T) {
	data := []byte{3, 0, 0x11, 0x2b, 2, 0, 0, 0, 0x80}
	if _, err := ParseDigitalLump("DSSAMP", data); err == nil {
		t.Fatal("expected size mismatch error")
	}
}

func TestImportDigitalSounds(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "digital.wad")
	data := buildWADForSoundTests(t, []lumpSpec{
		{name: "DSGOOD", data: []byte{3, 0, 0x11, 0x2b, 1, 0, 0, 0, 0x80}},
		{name: "DSBAD", data: []byte{2, 0, 0x11, 0x2b, 0, 0, 0, 0}},
		{name: "NOTSND", data: []byte{0xff}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	r := ImportDigitalSounds(f)
	if r.Found != 2 || r.Decoded != 1 || r.Failed != 1 {
		t.Fatalf("report=%+v", r)
	}
}

func TestImportDigitalSoundsSharesWADPayloadView(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "digital.wad")
	data := buildWADForSoundTests(t, []lumpSpec{
		{name: "DSGOOD", data: []byte{3, 0, 0x11, 0x2b, 2, 0, 0, 0, 0x80, 0x81}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	r := ImportDigitalSounds(f)
	if len(r.Sounds) != 1 {
		t.Fatalf("decoded=%d want=1", len(r.Sounds))
	}
	view, err := f.LumpDataView(f.Lumps[0])
	if err != nil {
		t.Fatalf("LumpDataView() error: %v", err)
	}
	view[8] = 0x42
	if r.Sounds[0].Samples[0] != 0x42 {
		t.Fatalf("Samples[0]=%#x want shared payload %#x", r.Sounds[0].Samples[0], byte(0x42))
	}
}
