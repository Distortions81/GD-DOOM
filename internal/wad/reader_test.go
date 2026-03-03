package wad

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenValidMinimalIWAD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "minimal.wad")
	data := minimalWAD(t, "IWAD", "TEST", []byte{1, 2, 3, 4})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, want := f.Header.Identification, "IWAD"; got != want {
		t.Fatalf("identification = %q, want %q", got, want)
	}
	if len(f.Lumps) != 1 {
		t.Fatalf("len(lumps) = %d, want 1", len(f.Lumps))
	}
	if f.Lumps[0].Name != "TEST" {
		t.Fatalf("lump name = %q", f.Lumps[0].Name)
	}

	bytes, err := f.LumpData(f.Lumps[0])
	if err != nil {
		t.Fatalf("LumpData() error = %v", err)
	}
	if len(bytes) != 4 || bytes[2] != 3 {
		t.Fatalf("unexpected lump bytes: %#v", bytes)
	}
}

func TestOpenRejectsNonIWAD(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "badid.wad")
	data := minimalWAD(t, "PWAD", "TEST", []byte{1, 2})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("Open() expected error")
	}
}

func TestOpenTruncatedDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "trunc.wad")
	data := make([]byte, 12)
	copy(data[0:4], []byte("IWAD"))
	binary.LittleEndian.PutUint32(data[4:8], 1)
	binary.LittleEndian.PutUint32(data[8:12], 100)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("Open() expected truncated directory error")
	}
}

func minimalWAD(t *testing.T, ident, lumpName string, lumpData []byte) []byte {
	t.Helper()
	if len(lumpName) > 8 {
		t.Fatalf("lumpName too long: %q", lumpName)
	}
	const headerLen = 12
	const dirEntryLen = 16
	filePos := int32(headerLen)
	dirPos := int32(headerLen + len(lumpData))

	buf := make([]byte, headerLen+len(lumpData)+dirEntryLen)
	copy(buf[0:4], []byte(ident))
	binary.LittleEndian.PutUint32(buf[4:8], 1)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dirPos))
	copy(buf[filePos:], lumpData)

	dir := buf[dirPos : dirPos+dirEntryLen]
	binary.LittleEndian.PutUint32(dir[0:4], uint32(filePos))
	binary.LittleEndian.PutUint32(dir[4:8], uint32(len(lumpData)))
	copy(dir[8:16], []byte(lumpName))
	return buf
}
