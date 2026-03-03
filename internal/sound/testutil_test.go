package sound

import (
	"encoding/binary"
	"testing"
)

type lumpSpec struct {
	name string
	data []byte
}

func buildWADForSoundTests(t *testing.T, lumps []lumpSpec) []byte {
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
