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

func TestTriangulateWorldPolygon_Concave(t *testing.T) {
	// Simple concave "arrow" polygon, CCW.
	verts := []worldPt{
		{x: 0, y: 0},
		{x: 4, y: 0},
		{x: 4, y: 1},
		{x: 2, y: 0.4},
		{x: 0, y: 1},
	}
	tris, ok := triangulateWorldPolygon(verts)
	if !ok {
		t.Fatal("expected concave polygon to triangulate")
	}
	if got, want := len(tris), len(verts)-2; got != want {
		t.Fatalf("triangle count=%d want=%d", got, want)
	}
}

func TestTriangulateWorldPolygon_SelfIntersectReject(t *testing.T) {
	// Bowtie/self-intersecting polygon.
	verts := []worldPt{
		{x: 0, y: 0},
		{x: 2, y: 2},
		{x: 0, y: 2},
		{x: 2, y: 0},
	}
	_, ok := triangulateWorldPolygon(verts)
	if ok {
		t.Fatal("expected self-intersecting polygon to be rejected")
	}
}
