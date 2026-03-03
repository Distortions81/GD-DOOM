package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSubsectorVertexLoopFromSegOrder(t *testing.T) {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{
			{X: 0, Y: 0},
			{X: 64, Y: 0},
			{X: 64, Y: 64},
			{X: 0, Y: 64},
		},
		Segs: []mapdata.Seg{
			{StartVertex: 0, EndVertex: 1},
			{StartVertex: 1, EndVertex: 2},
			{StartVertex: 2, EndVertex: 3},
			{StartVertex: 3, EndVertex: 0},
		},
	}
	sub := mapdata.SubSector{FirstSeg: 0, SegCount: 4}

	chain, ok := subsectorVertexLoopFromSegOrder(m, sub)
	if !ok {
		t.Fatal("expected seg-order loop to close")
	}
	if len(chain) != 4 {
		t.Fatalf("expected 4 vertices, got %d", len(chain))
	}
	want := []uint16{0, 1, 2, 3}
	for i := range want {
		if chain[i] != want[i] {
			t.Fatalf("chain[%d]=%d want %d", i, chain[i], want[i])
		}
	}
}
