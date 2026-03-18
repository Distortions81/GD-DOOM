package presenter

func RelativeThingAngle(thingAngle int16, viewAngle uint32) int16 {
	return RelativeWorldAngle(thingDegToWorldAngle(thingAngle), viewAngle)
}

func WorldThingAngle(thingAngle int16) int16 {
	return WorldAngleToGlyphAngle(thingDegToWorldAngle(thingAngle))
}

func ThingGlyphSize(zoom float64) float64 {
	// Doom-like behavior: thing markers scale with map zoom (map-space vectors).
	const worldHalfUnits = 16.0
	s := worldHalfUnits * zoom
	if s < 1.5 {
		return 1.5
	}
	if s > 40 {
		return 40
	}
	return s
}

func thingDegToWorldAngle(thingAngle int16) uint32 {
	return uint32(int64(thingAngle) * 4294967296 / 360)
}

func RelativeWorldAngle(worldAngle, viewAngle uint32) int16 {
	viewDeg := float64(viewAngle) * (360.0 / 4294967296.0)
	thingDeg := float64(worldAngle) * (360.0 / 4294967296.0)
	delta := viewDeg - thingDeg
	return int16(normalizeDegrees(delta))
}

func WorldAngleToGlyphAngle(worldAngle uint32) int16 {
	// Doom things use 0=east, 90=north; glyphs use 0=up, +90=right.
	deg := float64(worldAngle) * (360.0 / 4294967296.0)
	delta := 90.0 - deg
	return int16(normalizeDegrees(delta))
}

func normalizeDegrees(deg float64) float64 {
	for deg <= -180 {
		deg += 360
	}
	for deg > 180 {
		deg -= 360
	}
	return deg
}
