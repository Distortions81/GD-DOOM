package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestShouldMarkMappedNearLine(t *testing.T) {
	verts := []mapdata.Vertex{{X: 0, Y: 0}, {X: 128, Y: 0}}
	ld := mapdata.Linedef{V1: 0, V2: 1}
	if !shouldMarkMapped(ld, verts, 32, 16, 64) {
		t.Fatalf("expected nearby line to be marked mapped")
	}
}

func TestShouldMarkMappedFarLine(t *testing.T) {
	verts := []mapdata.Vertex{{X: 0, Y: 0}, {X: 128, Y: 0}}
	ld := mapdata.Linedef{V1: 0, V2: 1}
	if shouldMarkMapped(ld, verts, 500, 500, 64) {
		t.Fatalf("expected far line to remain unmapped")
	}
}

func TestShouldMarkMappedNeverSeeFalse(t *testing.T) {
	verts := []mapdata.Vertex{{X: 0, Y: 0}, {X: 128, Y: 0}}
	ld := mapdata.Linedef{V1: 0, V2: 1, Flags: lineNeverSee}
	if shouldMarkMapped(ld, verts, 8, 0, 64) {
		t.Fatalf("expected never-see line not to be mapped")
	}
}
