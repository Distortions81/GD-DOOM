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

type WallProjectionStepper struct {
	proj  WallProjection
	t     float64
	tStep float64
	ok    bool
}

type WallPrepass struct {
	Projection WallProjection
	LogReason  string
	LogZ1      float64
	LogZ2      float64
	LogX1      float64
	LogX2      float64
	OK         bool
}

type WallPrepassWorldInput struct {
	X1W float64
	Y1W float64
	U1  float64
	X2W float64
	Y2W float64
	U2  float64
}

type MaskedMidSeg struct {
	Dist       float64
	X0         int
	X1         int
	Projection WallProjection
	WorldHigh  float64
	WorldLow   float64
	TexUOff    float64
	TexMid     float64
}

func NewWallPrepassWorldInput(x1w, y1w, x2w, y2w, uBase, segLen float64, frontSide int) WallPrepassWorldInput {
	u2 := uBase + segLen
	if frontSide == 1 {
		u2 = uBase - segLen
	}
	return WallPrepassWorldInput{
		X1W: x1w, Y1W: y1w, U1: uBase,
		X2W: x2w, Y2W: y2w, U2: u2,
	}
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

func BuildWallPrepass(f1, s1, u1, f2, s2, u2 float64, viewW int, focal, near float64) WallPrepass {
	origF1, origS1, origF2, origS2 := f1, s1, f2, s2
	preSX1 := float64(viewW) / 2
	preSX2 := float64(viewW) / 2
	if math.Abs(origF1) > 1e-9 {
		preSX1 -= (origS1 / origF1) * focal
	}
	if math.Abs(origF2) > 1e-9 {
		preSX2 -= (origS2 / origF2) * focal
	}

	var ok bool
	f1, s1, u1, f2, s2, u2, ok = ClipSegmentToNearWithAttr(f1, s1, u1, f2, s2, u2, near)
	if !ok {
		return WallPrepass{
			LogReason: "BEHIND",
			LogZ1:     origF1,
			LogZ2:     origF2,
			LogX1:     preSX1,
			LogX2:     preSX2,
		}
	}
	if f1*s2-s1*f2 >= 0 {
		return WallPrepass{
			LogReason: "BACKFACE",
			LogZ1:     f1,
			LogZ2:     f2,
			LogX1:     s1,
			LogX2:     s2,
		}
	}

	proj, status := ProjectWallSegment(f1, s1, u1, f2, s2, u2, viewW, focal)
	sx1 := float64(viewW)/2 - (s1/f1)*focal
	sx2 := float64(viewW)/2 - (s2/f2)*focal
	switch status {
	case WallProjectionFlipped:
		return WallPrepass{
			LogReason: "FLIPPED",
			LogZ1:     f1,
			LogZ2:     f2,
			LogX1:     sx1,
			LogX2:     sx2,
		}
	case WallProjectionOffscreen:
		return WallPrepass{
			LogReason: "OFFSCREEN",
			LogZ1:     f1,
			LogZ2:     f2,
			LogX1:     sx1,
			LogX2:     sx2,
		}
	default:
		return WallPrepass{
			Projection: proj,
			LogZ1:      f1,
			LogZ2:      f2,
			LogX1:      sx1,
			LogX2:      sx2,
			OK:         true,
		}
	}
}

func BuildWallPrepassFromWorld(input WallPrepassWorldInput, camX, camY, ca, sa float64, viewW int, focal, near float64) WallPrepass {
	x1 := input.X1W - camX
	y1 := input.Y1W - camY
	x2 := input.X2W - camX
	y2 := input.Y2W - camY
	f1 := x1*ca + y1*sa
	s1 := -x1*sa + y1*ca
	f2 := x2*ca + y2*sa
	s2 := -x2*sa + y2*ca
	return BuildWallPrepass(f1, s1, input.U1, f2, s2, input.U2, viewW, focal, near)
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

func NewWallProjectionStepper(proj WallProjection, x int) WallProjectionStepper {
	if proj.SX2 == proj.SX1 {
		return WallProjectionStepper{}
	}
	return WallProjectionStepper{
		proj:  proj,
		t:     (float64(x) - proj.SX1) / (proj.SX2 - proj.SX1),
		tStep: 1.0 / (proj.SX2 - proj.SX1),
		ok:    true,
	}
}

func (s WallProjectionStepper) Sample() (float64, float64, bool) {
	if !s.ok {
		return 0, 0, false
	}
	t := s.t
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	invDepth := s.proj.InvDepth1 + (s.proj.InvDepth2-s.proj.InvDepth1)*t
	if invDepth <= 0 {
		return 0, 0, false
	}
	depth := 1.0 / invDepth
	if depth <= 0 {
		return 0, 0, false
	}
	uOverDepth := s.proj.UOverDepth1 + (s.proj.UOverDepth2-s.proj.UOverDepth1)*t
	return depth, uOverDepth * depth, true
}

func (s *WallProjectionStepper) Next() {
	if s == nil || !s.ok {
		return
	}
	s.t += s.tStep
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
