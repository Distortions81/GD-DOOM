package scene

import (
	"math"

	"gddoom/internal/mapdata"
)

func NodeChildBBoxMaybeVisible(n mapdata.Node, childSide int, pxw, pyw, ca, sa, near, tanHalfFOV float64) bool {
	bb := n.BBoxR
	if childSide != 0 {
		bb = n.BBoxL
	}
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	minX := math.Min(left, right)
	maxX := math.Max(left, right)
	minY := math.Min(bottom, top)
	maxY := math.Max(bottom, top)
	if minX > maxX || minY > maxY {
		return true
	}
	if pxw >= minX && pxw <= maxX && pyw >= minY && pyw <= maxY {
		return true
	}
	corners := [4][2]float64{
		{minX, minY},
		{maxX, minY},
		{maxX, maxY},
		{minX, maxY},
	}
	outNear := 0
	outLeft := 0
	outRight := 0
	for _, c := range corners {
		dx := c[0] - pxw
		dy := c[1] - pyw
		f := dx*ca + dy*sa
		s := -dx*sa + dy*ca
		if f < near {
			outNear++
		}
		if s+tanHalfFOV*f < 0 {
			outLeft++
		}
		if -s+tanHalfFOV*f < 0 {
			outRight++
		}
	}
	return outNear != len(corners) && outLeft != len(corners) && outRight != len(corners)
}

func NodeChildScreenRange(n mapdata.Node, childSide int, pxw, pyw, ca, sa, near, focal float64, viewW int) (int, int, bool) {
	bb := n.BBoxR
	if childSide != 0 {
		bb = n.BBoxL
	}
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	minX := math.Min(left, right)
	maxX := math.Max(left, right)
	minY := math.Min(bottom, top)
	maxY := math.Max(bottom, top)
	if minX > maxX || minY > maxY || viewW <= 0 {
		return 0, 0, false
	}
	if pxw >= minX && pxw <= maxX && pyw >= minY && pyw <= maxY {
		return 0, viewW - 1, true
	}
	corners := [4][2]float64{
		{minX, minY},
		{maxX, minY},
		{maxX, maxY},
		{minX, maxY},
	}
	minSX := float64(viewW - 1)
	maxSX := 0.0
	any := false
	for _, c := range corners {
		dx := c[0] - pxw
		dy := c[1] - pyw
		f := dx*ca + dy*sa
		s := -dx*sa + dy*ca
		if f < near {
			f = near
		}
		if f <= 0 {
			continue
		}
		sx := float64(viewW)/2 - (s/f)*focal
		if sx < minSX {
			minSX = sx
		}
		if sx > maxSX {
			maxSX = sx
		}
		any = true
	}
	if !any {
		return 0, 0, false
	}
	const childCullPad = 1
	l := int(math.Floor(minSX)) - childCullPad
	r := int(math.Ceil(maxSX)) + childCullPad
	if l < 0 {
		l = 0
	}
	if r >= viewW {
		r = viewW - 1
	}
	if l > r {
		return 0, 0, false
	}
	return l, r, true
}

func SegScreenRangeFromWorld(x1w, y1w, x2w, y2w, pxw, pyw, ca, sa, near, focal float64, viewW int) (int, int, bool) {
	if viewW <= 0 {
		return 0, 0, false
	}
	x1 := x1w - pxw
	y1 := y1w - pyw
	x2 := x2w - pxw
	y2 := y2w - pyw
	f1 := x1*ca + y1*sa
	s1 := -x1*sa + y1*ca
	f2 := x2*ca + y2*sa
	s2 := -x2*sa + y2*ca
	f1, s1, f2, s2, ok := ClipSegmentToNear(f1, s1, f2, s2, near)
	if !ok {
		return 0, 0, false
	}
	if f1*s2-s1*f2 >= 0 {
		return 0, 0, false
	}
	sx1 := float64(viewW)/2 - (s1/f1)*focal
	sx2 := float64(viewW)/2 - (s2/f2)*focal
	if !isFinite(sx1) || !isFinite(sx2) {
		return 0, 0, false
	}
	l := int(math.Floor(math.Min(sx1, sx2)))
	r := int(math.Ceil(math.Max(sx1, sx2)))
	if l < 0 {
		l = 0
	}
	if r >= viewW {
		r = viewW - 1
	}
	if l > r {
		return 0, 0, false
	}
	return l, r, true
}
