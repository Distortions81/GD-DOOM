package sound

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestParsePCSpeakerLumpValid(t *testing.T) {
	data := make([]byte, 4+3)
	binary.LittleEndian.PutUint16(data[0:2], 0)
	binary.LittleEndian.PutUint16(data[2:4], 3)
	data[4] = 0x20
	data[5] = 0x30
	data[6] = 0x40

	s, err := ParsePCSpeakerLump("DPTONE", data)
	if err != nil {
		t.Fatalf("ParsePCSpeakerLump() error=%v", err)
	}
	if got, want := len(s.Tones), 3; got != want {
		t.Fatalf("tones len=%d want=%d", got, want)
	}
	if s.Tones[1] != 0x30 {
		t.Fatalf("tone[1]=%d want=48", s.Tones[1])
	}
}

func TestParsePCSpeakerLumpInvalidHeader(t *testing.T) {
	data := []byte{1, 0, 1, 0, 0x20}
	if _, err := ParsePCSpeakerLump("DPTONE", data); err == nil {
		t.Fatal("expected header error")
	}
}

func TestParsePCSpeakerLumpInvalidSize(t *testing.T) {
	data := []byte{0, 0, 2, 0, 0x20}
	if _, err := ParsePCSpeakerLump("DPTONE", data); err == nil {
		t.Fatal("expected size mismatch error")
	}
}

func TestImportPCSpeakerSounds(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sound.wad")
	data := buildWAD(t, []lumpSpec{
		{name: "DPGOOD", data: []byte{0, 0, 1, 0, 0x33}},
		{name: "DPBAD", data: []byte{1, 0, 0, 0}},
		{name: "NOTSND", data: []byte{0xff}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	r := ImportPCSpeakerSounds(f)
	if r.Found != 2 || r.Decoded != 1 || r.Failed != 1 {
		t.Fatalf("report=%+v", r)
	}
}

type lumpSpec struct {
	name string
	data []byte
}

func buildWAD(t *testing.T, lumps []lumpSpec) []byte {
	t.Helper()
	const (
		headerLen = 12
		dirLen    = 16
	)
	fileDataLen := 0
	for _, l := range lumps {
		if len(l.name) > 8 {
			t.Fatalf("lump name too long: %q", l.name)
		}
		fileDataLen += len(l.data)
	}
	dirPos := headerLen + fileDataLen
	buf := make([]byte, headerLen+fileDataLen+len(lumps)*dirLen)
	copy(buf[0:4], []byte("IWAD"))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(lumps)))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dirPos))

	writePos := headerLen
	for i, l := range lumps {
		copy(buf[writePos:], l.data)
		entry := buf[dirPos+i*dirLen : dirPos+(i+1)*dirLen]
		binary.LittleEndian.PutUint32(entry[0:4], uint32(writePos))
		binary.LittleEndian.PutUint32(entry[4:8], uint32(len(l.data)))
		copy(entry[8:16], []byte(l.name))
		writePos += len(l.data)
	}
	return buf
}
