package doomtex

import (
	"testing"

	"gddoom/internal/wad"
)

func TestLoadFlatsIndexedSharesLumpView(t *testing.T) {
	const (
		headerLen   = 12
		dirEntryLen = 16
		flatName    = "FLAT1"
	)
	flat := make([]byte, doomFlatSize)
	flat[0] = 0x11
	startMarker := make([]byte, 0)
	endMarker := make([]byte, 0)
	lumps := []struct {
		name string
		data []byte
	}{
		{name: "F_START", data: startMarker},
		{name: flatName, data: flat},
		{name: "F_END", data: endMarker},
	}
	dataLen := headerLen
	for _, l := range lumps {
		dataLen += len(l.data)
	}
	dirPos := dataLen
	buf := make([]byte, dirPos+len(lumps)*dirEntryLen)
	copy(buf[0:4], []byte("IWAD"))
	putU32LE(buf[4:8], uint32(len(lumps)))
	putU32LE(buf[8:12], uint32(dirPos))
	writePos := headerLen
	for i, l := range lumps {
		copy(buf[writePos:writePos+len(l.data)], l.data)
		dir := buf[dirPos+i*dirEntryLen : dirPos+(i+1)*dirEntryLen]
		putU32LE(dir[0:4], uint32(writePos))
		putU32LE(dir[4:8], uint32(len(l.data)))
		copy(dir[8:16], []byte(l.name))
		writePos += len(l.data)
	}
	wf, err := wad.OpenData("mem.wad", buf)
	if err != nil {
		t.Fatalf("OpenData() error=%v", err)
	}

	flats, err := LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatalf("LoadFlatsIndexed() error=%v", err)
	}
	got := flats[flatName]
	if len(got) != doomFlatSize {
		t.Fatalf("flat len=%d want=%d", len(got), doomFlatSize)
	}
	got[0] = 0x42
	view, err := wf.LumpDataView(wf.Lumps[1])
	if err != nil {
		t.Fatalf("LumpDataView() error=%v", err)
	}
	if view[0] != 0x42 {
		t.Fatalf("flat view byte=%#x want shared payload %#x", view[0], byte(0x42))
	}
}

func putU32LE(dst []byte, v uint32) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
	dst[3] = byte(v >> 24)
}
