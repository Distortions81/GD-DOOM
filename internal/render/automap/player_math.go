package automap

import (
	"math"
	"math/bits"
)

const (
	doomSlopeRange = 2048
	doomAng90      = 0x40000000
	doomAng180     = 0x80000000
	doomAng270     = 0xc0000000
)

var doomTanToAngle = func() [doomSlopeRange + 1]uint32 {
	var table [doomSlopeRange + 1]uint32
	for i := 0; i <= doomSlopeRange; i++ {
		f := math.Atan(float64(i)/doomSlopeRange) / (2 * math.Pi)
		table[i] = uint32(float64(^uint32(0)) * f)
	}
	return table
}()

type divline struct {
	x  int64
	y  int64
	dx int64
	dy int64
}

func doomPointOnDivlineSide(x, y int64, line divline) int {
	if line.dx == 0 {
		if x <= line.x {
			return b2i(line.dy > 0)
		}
		return b2i(line.dy < 0)
	}
	if line.dy == 0 {
		if y <= line.y {
			return b2i(line.dx < 0)
		}
		return b2i(line.dx > 0)
	}
	dx := x - line.x
	dy := y - line.y
	if (line.dy^line.dx^dx^dy)&0x80000000 != 0 {
		if (line.dy^dx)&0x80000000 != 0 {
			return 1
		}
		return 0
	}
	left := fixedMul(line.dy>>8, dx>>8)
	right := fixedMul(dy>>8, line.dx>>8)
	if right < left {
		return 0
	}
	return 1
}

func pointOnDivlineSide(x, y int64, line divline) int {
	if line.dx == 0 {
		if x <= line.x {
			if line.dy > 0 {
				return 1
			}
			return 0
		}
		if line.dy < 0 {
			return 1
		}
		return 0
	}
	if line.dy == 0 {
		if y <= line.y {
			if line.dx < 0 {
				return 1
			}
			return 0
		}
		if line.dx > 0 {
			return 1
		}
		return 0
	}
	dx := x - line.x
	dy := y - line.y
	// Keep full fixed-point precision here. Losing bits near node planes can
	// flip side classification and produce angle-dependent BSP ordering artifacts.
	left := line.dy * dx
	right := dy * line.dx
	if right < left {
		return 0
	}
	return 1
}

func segmentIntersectFrac(ax, ay, bx, by, cx, cy, dx, dy int64) (float64, bool) {
	x1, y1 := float64(ax), float64(ay)
	x2, y2 := float64(bx), float64(by)
	x3, y3 := float64(cx), float64(cy)
	x4, y4 := float64(dx), float64(dy)
	den := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if den == 0 {
		return 0, false
	}
	t := ((x1-x3)*(y3-y4) - (y1-y3)*(x3-x4)) / den
	u := -((x1-x2)*(y1-y3) - (y1-y2)*(x1-x3)) / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return 0, false
	}
	return t, true
}

func fixedDiv(a, b int64) int64 {
	if b == 0 {
		if a >= 0 {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	neg := (a < 0) != (b < 0)
	ua := uint64(abs(a))
	ub := uint64(abs(b))
	hi, lo := bits.Mul64(ua, fracUnit)
	q, _ := bits.Div64(hi, lo, ub)
	if q > uint64(math.MaxInt64) {
		if neg {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	out := int64(q)
	if neg {
		return -out
	}
	return out
}

func interceptVector(v2, v1 divline) int64 {
	den := fixedMul(v1.dy>>8, v2.dx) - fixedMul(v1.dx>>8, v2.dy)
	if den == 0 {
		return 0
	}
	num := fixedMul((v1.x-v2.x)>>8, v1.dy) + fixedMul((v2.y-v1.y)>>8, v1.dx)
	return fixedDiv(num, den)
}

func doomSlopeDiv(num, den uint32) int {
	if den < 512 {
		return doomSlopeRange
	}
	ans := (num << 3) / (den >> 8)
	if ans > doomSlopeRange {
		return doomSlopeRange
	}
	return int(ans)
}

func doomPointToAngle2(x1, y1, x2, y2 int64) uint32 {
	x := x2 - x1
	y := y2 - y1
	if x == 0 && y == 0 {
		return 0
	}
	if x >= 0 {
		if y >= 0 {
			if x > y {
				return doomTanToAngle[doomSlopeDiv(uint32(y), uint32(x))]
			}
			return doomAng90 - 1 - doomTanToAngle[doomSlopeDiv(uint32(x), uint32(y))]
		}
		y = -y
		if x > y {
			return 0 - doomTanToAngle[doomSlopeDiv(uint32(y), uint32(x))]
		}
		return doomAng270 + doomTanToAngle[doomSlopeDiv(uint32(x), uint32(y))]
	}
	x = -x
	if y >= 0 {
		if x > y {
			return doomAng180 - 1 - doomTanToAngle[doomSlopeDiv(uint32(y), uint32(x))]
		}
		return doomAng90 + doomTanToAngle[doomSlopeDiv(uint32(x), uint32(y))]
	}
	y = -y
	if x > y {
		return doomAng180 + doomTanToAngle[doomSlopeDiv(uint32(y), uint32(x))]
	}
	return doomAng270 - 1 - doomTanToAngle[doomSlopeDiv(uint32(x), uint32(y))]
}

func fixedMul(a, b int64) int64 {
	return (a * b) >> fracBits
}

func floatToFixed(v float64) int64 {
	return int64(v * fracUnit)
}

func angleToRadians(a uint32) float64 {
	return float64(a) * (2 * math.Pi / 4294967296.0)
}

func degToAngle(deg int16) uint32 {
	return uint32((float64(deg) / 360.0) * 4294967296.0)
}

func thingDegToWorldAngle(deg int16) uint32 {
	// Doom THINGS angles already match our internal convention:
	// 0=east, 90=north, 180=west, 270=south.
	world := float64(deg)
	for world < 0 {
		world += 360
	}
	for world >= 360 {
		world -= 360
	}
	return degToAngle(int16(world))
}

func worldAngleToThingDeg(angle uint32) int16 {
	deg := float64(angle) * (360.0 / 4294967296.0)
	for deg < 0 {
		deg += 360
	}
	for deg >= 360 {
		deg -= 360
	}
	return int16(math.Round(deg)) % 360
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func approxDistance(dx, dy int64) int64 {
	dx = abs(dx)
	dy = abs(dy)
	if dx < dy {
		return dx + dy - (dx >> 1)
	}
	return dx + dy - (dy >> 1)
}

func vectorToAngle(dx, dy int64) uint32 {
	return doomPointToAngle2(0, 0, dx, dy)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
