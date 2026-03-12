//go:build cgo

package doomruntime

import "testing"

func TestCDTSegmentsValid_RejectsCrossingSegments(t *testing.T) {
	pts := [][2]float64{
		{0, 0},
		{2, 2},
		{0, 2},
		{2, 0},
	}
	segs := [][2]int32{
		{0, 1},
		{2, 3},
		{0, 2},
	}
	if cdtSegmentsValid(pts, segs) {
		t.Fatal("expected crossing PSLG segments to be rejected")
	}
}

func TestCDTSegmentsValid_RejectsDegenerateSegment(t *testing.T) {
	pts := [][2]float64{
		{0, 0},
		{1, 0},
		{0, 1},
	}
	segs := [][2]int32{
		{0, 1},
		{1, 1},
		{1, 2},
	}
	if cdtSegmentsValid(pts, segs) {
		t.Fatal("expected degenerate segment to be rejected")
	}
}

func TestPointOnAnyRingEdge(t *testing.T) {
	rings := [][]worldPt{{
		{x: 0, y: 0},
		{x: 4, y: 0},
		{x: 4, y: 4},
		{x: 0, y: 4},
	}}
	if !pointOnAnyRingEdge(worldPt{x: 2, y: 0}, rings, 1e-6) {
		t.Fatal("point on edge should be detected")
	}
	if pointOnAnyRingEdge(worldPt{x: 2, y: 2}, rings, 1e-6) {
		t.Fatal("interior point should not be treated as edge point")
	}
}
