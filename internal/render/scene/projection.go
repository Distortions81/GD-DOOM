package scene

import "math"

type WallProjection struct {
	SX1         float64
	SX2         float64
	MinX        int
	MaxX        int
	InvDepth1   float64
	InvDepth2   float64
	UOverDepth1 float64
	UOverDepth2 float64
}

type WallProjectionStatus uint8

const (
	WallProjectionOK WallProjectionStatus = iota
	WallProjectionFlipped
	WallProjectionOffscreen
)

func ClipSegmentToNear(f1, s1, f2, s2, near float64) (float64, float64, float64, float64, bool) {
	const eps = 0.125
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, false
	}
	of1, os1 := f1, s1
	of2, os2 := f2, s2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, false
	}
	return f1, s1, f2, s2, true
}

func ClipSegmentToNearWithAttr(f1, s1, a1, f2, s2, a2, near float64) (float64, float64, float64, float64, float64, float64, bool) {
	const eps = 0.125
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, 0, 0, false
	}
	of1, os1, oa1 := f1, s1, a1
	of2, os2, oa2 := f2, s2, a2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
		a1 = oa1 + (oa2-oa1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
		a2 = oa2 + (oa1-oa2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, 0, 0, false
	}
	return f1, s1, a1, f2, s2, a2, true
}

func ProjectWallSegment(f1, s1, u1, f2, s2, u2 float64, viewW int, focal float64) (WallProjection, WallProjectionStatus) {
	if viewW <= 0 || focal <= 0 {
		return WallProjection{}, WallProjectionOffscreen
	}
	sx1 := float64(viewW)/2 - (s1/f1)*focal
	sx2 := float64(viewW)/2 - (s2/f2)*focal
	if !isFinite(sx1) || !isFinite(sx2) {
		return WallProjection{}, WallProjectionFlipped
	}
	minX := int(math.Floor(math.Min(sx1, sx2)))
	maxX := int(math.Ceil(math.Max(sx1, sx2)))
	if minX < 0 {
		minX = 0
	}
	if maxX >= viewW {
		maxX = viewW - 1
	}
	if minX > maxX {
		return WallProjection{}, WallProjectionOffscreen
	}
	return WallProjection{
		SX1:         sx1,
		SX2:         sx2,
		MinX:        minX,
		MaxX:        maxX,
		InvDepth1:   1.0 / f1,
		InvDepth2:   1.0 / f2,
		UOverDepth1: u1 / f1,
		UOverDepth2: u2 / f2,
	}, WallProjectionOK
}

func ProjectedWallDepthAtX(proj WallProjection, x int) (float64, bool) {
	depth, _, ok := ProjectedWallSampleAtX(proj, x)
	return depth, ok
}

func ProjectedWallTexUAtX(proj WallProjection, x int) (float64, bool) {
	_, texU, ok := ProjectedWallSampleAtX(proj, x)
	return texU, ok
}

func ProjectedWallSampleAtX(proj WallProjection, x int) (float64, float64, bool) {
	if proj.SX2 == proj.SX1 {
		return 0, 0, false
	}
	t := (float64(x) - proj.SX1) / (proj.SX2 - proj.SX1)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	invDepth := proj.InvDepth1 + (proj.InvDepth2-proj.InvDepth1)*t
	if invDepth <= 0 {
		return 0, 0, false
	}
	depth := 1.0 / invDepth
	if depth <= 0 {
		return 0, 0, false
	}
	uOverDepth := proj.UOverDepth1 + (proj.UOverDepth2-proj.UOverDepth1)*t
	return depth, uOverDepth * depth, true
}

func ProjectedWallYDepthAtX(proj WallProjection, x, viewH int, z, focal float64) (float64, float64, bool) {
	depth, _, ok := ProjectedWallSampleAtX(proj, x)
	if !ok {
		return 0, 0, false
	}
	y := float64(viewH)/2 - (z/depth)*focal
	return y, depth, true
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
