package raymesh

import (
	"fmt"
	"math"

	"gddoom/internal/render/levelmesh"
)

type LightingMode string

const (
	DoomLighting       LightingMode = "doom"
	SectorLighting     LightingMode = "sector"
	FullbrightLighting LightingMode = "fullbright"
)

func (m LightingMode) Validate() error {
	if m != DoomLighting && m != SectorLighting && m != FullbrightLighting {
		return fmt.Errorf("lighting must be doom, sector or fullbright")
	}
	return nil
}

func linearLightRamp() (ramp [32]float32) {
	for row := range ramp {
		ramp[row] = 1 - float32(row)/31
	}
	return ramp
}

// The second UV attribute carries static surface lighting metadata: planes
// use 4, walls use their Doom axis bias (-1, 0, +1). Raw sector light travels
// in vertex-color alpha, independently of RGB used by the diagnostic views.
func surfaceLightTag(tri levelmesh.Triangle) float32 {
	if tri.Kind == levelmesh.Billboard {
		return 0
	}
	if tri.Kind == levelmesh.EmissiveBillboard {
		return 6
	}
	if tri.Kind == levelmesh.ShadowBillboard {
		return 7
	}
	if tri.Kind == levelmesh.Floor || tri.Kind == levelmesh.Ceiling {
		return 4
	}
	minX, maxX := tri.Vertices[0].X, tri.Vertices[0].X
	minY, maxY := tri.Vertices[0].Y, tri.Vertices[0].Y
	for _, v := range tri.Vertices[1:] {
		minX, maxX = math.Min(minX, v.X), math.Max(maxX, v.X)
		minY, maxY = math.Min(minY, v.Y), math.Max(maxY, v.Y)
	}
	if minY == maxY && minX < maxX {
		return -1
	}
	if minX == maxX && minY < maxY {
		return 1
	}
	return 0
}
