package mapdata

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/wad"
)

func TestFirstMapNameSkipsInvalidMarkerBundle(t *testing.T) {
	f := &wad.File{Lumps: []wad.Lump{
		{Name: "E1M1"},
		{Name: "THINGS"},
		{Name: "LINEDEFS"},
		{Name: "BROKEN"},
		{Name: "E1M2"},
		{Name: "THINGS"},
		{Name: "LINEDEFS"},
		{Name: "SIDEDEFS"},
		{Name: "VERTEXES"},
		{Name: "SEGS"},
		{Name: "SSECTORS"},
		{Name: "NODES"},
		{Name: "SECTORS"},
		{Name: "REJECT"},
		{Name: "BLOCKMAP"},
	}}

	got, err := FirstMapName(f)
	if err != nil {
		t.Fatalf("FirstMapName() error = %v", err)
	}
	if got != "E1M2" {
		t.Fatalf("FirstMapName() = %q, want E1M2", got)
	}
}

func TestDecodeSegsAndSSECTORS_LittleEndianAndStride(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "decode.wad")

	segLump := make([]byte, 24) // 2 entries * 12 bytes
	// seg[0]
	binary.LittleEndian.PutUint16(segLump[0:2], 10)   // start vertex
	binary.LittleEndian.PutUint16(segLump[2:4], 20)   // end vertex
	binary.LittleEndian.PutUint16(segLump[4:6], 30)   // angle
	binary.LittleEndian.PutUint16(segLump[6:8], 40)   // linedef
	binary.LittleEndian.PutUint16(segLump[8:10], 50)  // direction
	binary.LittleEndian.PutUint16(segLump[10:12], 60) // offset
	// seg[1]
	binary.LittleEndian.PutUint16(segLump[12:14], 11)
	binary.LittleEndian.PutUint16(segLump[14:16], 21)
	binary.LittleEndian.PutUint16(segLump[16:18], 31)
	binary.LittleEndian.PutUint16(segLump[18:20], 41)
	binary.LittleEndian.PutUint16(segLump[20:22], 51)
	binary.LittleEndian.PutUint16(segLump[22:24], 61)

	ssLump := make([]byte, 8) // 2 entries * 4 bytes
	// ss[0]
	binary.LittleEndian.PutUint16(ssLump[0:2], 7) // numsegs
	binary.LittleEndian.PutUint16(ssLump[2:4], 3) // firstseg
	// ss[1]
	binary.LittleEndian.PutUint16(ssLump[4:6], 9)
	binary.LittleEndian.PutUint16(ssLump[6:8], 5)

	data := buildTestWAD(t, []wadLumpData{
		{name: "SEGS", data: segLump},
		{name: "SSECTORS", data: ssLump},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open test wad: %v", err)
	}

	segEntry, ok := f.LumpByName("SEGS")
	if !ok {
		t.Fatal("missing SEGS lump")
	}
	ssEntry, ok := f.LumpByName("SSECTORS")
	if !ok {
		t.Fatal("missing SSECTORS lump")
	}

	segs, err := decodeSegs(f, segEntry)
	if err != nil {
		t.Fatalf("decodeSegs: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("decodeSegs len=%d want=2", len(segs))
	}
	if segs[0].StartVertex != 10 || segs[0].EndVertex != 20 || segs[0].Angle != 30 || segs[0].Linedef != 40 || segs[0].Direction != 50 || segs[0].Offset != 60 {
		t.Fatalf("decodeSegs[0] mismatch: %+v", segs[0])
	}
	if segs[1].StartVertex != 11 || segs[1].EndVertex != 21 || segs[1].Angle != 31 || segs[1].Linedef != 41 || segs[1].Direction != 51 || segs[1].Offset != 61 {
		t.Fatalf("decodeSegs[1] mismatch: %+v", segs[1])
	}

	subs, err := decodeSubSectors(f, ssEntry)
	if err != nil {
		t.Fatalf("decodeSubSectors: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("decodeSubSectors len=%d want=2", len(subs))
	}
	if subs[0].SegCount != 7 || subs[0].FirstSeg != 3 {
		t.Fatalf("decodeSubSectors[0] mismatch: %+v", subs[0])
	}
	if subs[1].SegCount != 9 || subs[1].FirstSeg != 5 {
		t.Fatalf("decodeSubSectors[1] mismatch: %+v", subs[1])
	}
}

func TestDecodeSegsAndSSECTORS_StrideErrors(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "badstride.wad")

	data := buildTestWAD(t, []wadLumpData{
		{name: "SEGS", data: []byte{1, 2, 3}},     // not divisible by 12
		{name: "SSECTORS", data: []byte{1, 2, 3}}, // not divisible by 4
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open test wad: %v", err)
	}

	segEntry, _ := f.LumpByName("SEGS")
	ssEntry, _ := f.LumpByName("SSECTORS")
	if _, err := decodeSegs(f, segEntry); err == nil {
		t.Fatal("decodeSegs expected stride error")
	}
	if _, err := decodeSubSectors(f, ssEntry); err == nil {
		t.Fatal("decodeSubSectors expected stride error")
	}
}

func TestLoadMapReportsUnsupportedXNODBSPWhenNodesAreUnusable(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "xnod_map.wad")

	data := buildTestWAD(t, []wadLumpData{
		{name: "MAP01", data: nil},
		{name: "THINGS", data: nil},
		{name: "LINEDEFS", data: nil},
		{name: "SIDEDEFS", data: nil},
		{name: "VERTEXES", data: nil},
		{name: "SEGS", data: nil},
		{name: "SSECTORS", data: nil},
		{name: "NODES", data: []byte{1}}, // invalid vanilla nodes
		{name: "SECTORS", data: nil},
		{name: "REJECT", data: nil},
		{name: "BLOCKMAP", data: []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{name: "XNOD", data: []byte{1, 2, 3, 4}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open test wad: %v", err)
	}

	_, err = LoadMap(f, "MAP01")
	if err == nil {
		t.Fatal("LoadMap expected unsupported XNOD error")
	}
	if !strings.Contains(err.Error(), "includes XNOD data") {
		t.Fatalf("LoadMap error %q does not mention XNOD", err)
	}
}

type wadLumpData struct {
	name string
	data []byte
}

func buildTestWAD(t *testing.T, lumps []wadLumpData) []byte {
	t.Helper()
	if len(lumps) == 0 {
		t.Fatal("buildTestWAD requires at least one lump")
	}
	payloadSize := 0
	for _, l := range lumps {
		if len(l.name) > 8 {
			t.Fatalf("lump name too long: %q", l.name)
		}
		payloadSize += len(l.data)
	}
	dirPos := wad.HeaderSize + payloadSize
	total := wad.HeaderSize + payloadSize + len(lumps)*wad.DirectorySize
	buf := make([]byte, total)
	copy(buf[0:4], []byte("IWAD"))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(lumps)))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dirPos))

	writePos := wad.HeaderSize
	for i, l := range lumps {
		copy(buf[writePos:writePos+len(l.data)], l.data)
		dir := buf[dirPos+i*wad.DirectorySize : dirPos+(i+1)*wad.DirectorySize]
		binary.LittleEndian.PutUint32(dir[0:4], uint32(writePos))
		binary.LittleEndian.PutUint32(dir[4:8], uint32(len(l.data)))
		copy(dir[8:16], []byte(l.name))
		writePos += len(l.data)
	}
	return buf
}
