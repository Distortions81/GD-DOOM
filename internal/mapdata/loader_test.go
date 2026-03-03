package mapdata

import (
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
