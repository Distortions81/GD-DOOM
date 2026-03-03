package automap

import (
	"math"
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

func TestSubsectorVertexLoopFromSegOrder_ReorientsAndUsesAllEdges(t *testing.T) {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{
			{X: 0, Y: 0},   // 0
			{X: 64, Y: 0},  // 1
			{X: 64, Y: 64}, // 2
			{X: 0, Y: 64},  // 3
		},
		// Same square, but with mixed direction/order after the first edge.
		Segs: []mapdata.Seg{
			{StartVertex: 0, EndVertex: 1},
			{StartVertex: 2, EndVertex: 1}, // reversed
			{StartVertex: 3, EndVertex: 2}, // reversed
			{StartVertex: 0, EndVertex: 3}, // reversed
		},
	}
	sub := mapdata.SubSector{FirstSeg: 0, SegCount: 4}
	chain, ok := subsectorVertexLoopFromSegOrder(m, sub)
	if !ok {
		t.Fatal("expected loop reconstruction from mixed edge directions")
	}
	if got, want := len(chain), 4; got != want {
		t.Fatalf("chain len=%d want=%d", got, want)
	}
}

func TestSubsectorWorldVertices_FallbackBeatsStrictSegStartOrder(t *testing.T) {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{
			{X: 0, Y: 0},   // 0
			{X: 64, Y: 0},  // 1
			{X: 64, Y: 64}, // 2
			{X: 0, Y: 64},  // 3
		},
		// Valid square edges, but mixed directions produce a degenerate
		// sequence if you only read StartVertex in seg order.
		Segs: []mapdata.Seg{
			{StartVertex: 0, EndVertex: 1},
			{StartVertex: 2, EndVertex: 1},
			{StartVertex: 2, EndVertex: 3},
			{StartVertex: 0, EndVertex: 3},
		},
		SubSectors: []mapdata.SubSector{
			{FirstSeg: 0, SegCount: 4},
		},
	}
	g := &game{m: m}

	if _, _, _, ok := g.subSectorVerticesFromSegList(0); ok {
		t.Fatal("expected strict seg-start path to reject this mixed-direction loop")
	}
	verts, _, _, ok := g.subSectorWorldVertices(0)
	if !ok {
		t.Fatal("expected robust subsector world-vertex reconstruction to succeed")
	}
	if got, want := len(verts), 4; got != want {
		t.Fatalf("vertex count=%d want=%d", got, want)
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

func TestClipSubSectorPolyBySegBounds_ClampsToSegEnvelope(t *testing.T) {
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
		SubSectors: []mapdata.SubSector{
			{FirstSeg: 0, SegCount: 4},
		},
	}
	g := &game{m: m}
	in := []worldPt{
		{x: -256, y: -256},
		{x: 256, y: -256},
		{x: 256, y: 256},
		{x: -256, y: 256},
	}

	out := g.clipSubSectorPolyBySegBounds(0, in)
	if len(out) < 3 {
		t.Fatal("expected clipped subsector polygon")
	}
	b := worldPolyBBox(out)
	const eps = 0.05
	if b.minX < -eps || b.minY < -eps || b.maxX > 64+eps || b.maxY > 64+eps {
		t.Fatalf("bbox=(%.3f,%.3f)-(%.3f,%.3f) outside expected 0..64", b.minX, b.minY, b.maxX, b.maxY)
	}
	area := math.Abs(polygonArea2(out)) * 0.5
	if area < 4000 || area > 4200 {
		t.Fatalf("area=%.3f want near 4096", area)
	}
}
