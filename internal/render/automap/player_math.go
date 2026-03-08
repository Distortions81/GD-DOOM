package automap

import "math"

type divline struct {
	x  int64
	y  int64
	dx int64
	dy int64
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
	if dx == 0 && dy == 0 {
		return 0
	}
	ang := math.Atan2(float64(dy), float64(dx))
	if ang < 0 {
		ang += 2 * math.Pi
	}
	return uint32((ang / (2 * math.Pi)) * 4294967296.0)
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
