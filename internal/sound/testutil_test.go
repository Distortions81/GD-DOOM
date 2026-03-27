package sound

import (
	"encoding/binary"
	"testing"

	"gddoom/internal/wad"
)

type lumpSpec struct {
	name string
	data []byte
}

func buildWADForSoundTests(t *testing.T, lumps []lumpSpec) []byte {
	t.Helper()
	fileDataLen := 0
	for _, l := range lumps {
		if len(l.name) > 8 {
			t.Fatalf("lump name too long: %q", l.name)
		}
		fileDataLen += len(l.data)
	}
	dirPos := wad.HeaderSize + fileDataLen
	buf := make([]byte, wad.HeaderSize+fileDataLen+len(lumps)*wad.DirectorySize)
	copy(buf[0:4], []byte("IWAD"))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(lumps)))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dirPos))

	writePos := wad.HeaderSize
	for i, l := range lumps {
		copy(buf[writePos:], l.data)
		entry := buf[dirPos+i*wad.DirectorySize : dirPos+(i+1)*wad.DirectorySize]
		binary.LittleEndian.PutUint32(entry[0:4], uint32(writePos))
		binary.LittleEndian.PutUint32(entry[4:8], uint32(len(l.data)))
		copy(entry[8:16], []byte(l.name))
		writePos += len(l.data)
	}
	return buf
}
