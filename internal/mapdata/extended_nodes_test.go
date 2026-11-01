package mapdata

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"reflect"
	"testing"
)

func extendedFixture(t *testing.T, wide bool) []byte {
	t.Helper()
	var b bytes.Buffer
	put := func(v any) {
		t.Helper()
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	put(uint32(2))
	extra := uint32(1)
	if wide {
		extra = 65536
	}
	put(extra)
	for i := uint32(0); i < extra; i++ {
		put([2]int32{32768, -16384})
	}
	put(uint32(1))
	put(uint32(1))
	put(uint32(1))
	vertex := uint32(2)
	if wide {
		vertex = 65537
	}
	put(vertex)
	put(uint32(1))
	put(uint16(0))
	put(uint8(0))
	put(uint32(1))
	put(Node{ChildID: [2]uint32{SubsectorFlag, SubsectorFlag}})
	return append([]byte("XNOD"), b.Bytes()...)
}
func extendedMap() *Map {
	return &Map{Vertexes: []Vertex{{0, 0}, {1, 0}}, Linedefs: []Linedef{{V2: 1, SideNum: [2]int32{0, -1}}}, Sidedefs: []Sidedef{{}}, Sectors: []Sector{{}}}
}
func TestExtendedNodes(t *testing.T) {
	for _, wide := range []bool{false, true} {
		raw := extendedFixture(t, wide)
		var compressed bytes.Buffer
		compressed.WriteString("ZNOD")
		z := zlib.NewWriter(&compressed)
		if _, err := z.Write(raw[4:]); err != nil {
			t.Fatal(err)
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		a, b := extendedMap(), extendedMap()
		for i, item := range []struct {
			m    *Map
			data []byte
		}{{a, raw}, {b, compressed.Bytes()}} {
			if err := decodeExtendedNodes(item.m, item.data); err != nil {
				t.Fatalf("format %d: %v", i, err)
			}
			if err := Validate(item.m); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatal("compressed and uncompressed geometry differ")
		}
		if got := a.Vertexes[a.Segs[0].StartVertex]; got != (Vertex{0.5, -0.25}) {
			t.Fatalf("split vertex lost precision: %v", got)
		}
		if wide && a.Segs[0].StartVertex <= 65535 {
			t.Fatal("wide index truncated")
		}
	}
}
func TestExtendedNodesMalformed(t *testing.T) {
	raw := extendedFixture(t, false)
	for n := 4; n < len(raw); n++ {
		if err := decodeExtendedNodes(extendedMap(), raw[:n]); err == nil {
			t.Fatalf("accepted truncated payload %d", n)
		}
	}
	bad := append([]byte(nil), raw...)
	binary.LittleEndian.PutUint32(bad[8:], 0xffffffff)
	if err := decodeExtendedNodes(extendedMap(), bad); err == nil {
		t.Fatal("accepted oversized count")
	}
	if err := decodeExtendedNodes(extendedMap(), append(raw, 0)); err == nil {
		t.Fatal("accepted trailing bytes")
	}
	if err := decodeExtendedNodes(extendedMap(), []byte("ZNODbroken")); err == nil {
		t.Fatal("accepted invalid compression")
	}
	m := extendedMap()
	if err := decodeExtendedNodes(m, raw); err != nil {
		t.Fatal(err)
	}
	m.Nodes[0].ChildID[0] = 0
	if err := Validate(m); err == nil {
		t.Fatal("accepted cyclic BSP")
	}
	m.Nodes[0].ChildID[0] = SubsectorFlag | 65536
	if err := Validate(m); err == nil {
		t.Fatal("accepted out of range wide leaf")
	}
}
