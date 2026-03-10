package automap

import (
	"image/color"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview/presenter"
)

func styleForThing(t mapdata.Thing) presenter.ThingStyle {
	if isPlayerStart(t.Type) {
		return presenter.ThingStyle{Glyph: presenter.GlyphSquare, Color: presenter.ThingPlayerColor}
	}
	if isMonster(t.Type) {
		return presenter.ThingStyle{Glyph: presenter.GlyphTriangle, Color: presenter.ThingMonsterColor}
	}
	if k, ok := keyColorForType(t.Type); ok {
		return presenter.ThingStyle{Glyph: presenter.GlyphStar, Color: k}
	}
	if isItemOrPickup(t.Type) {
		return presenter.ThingStyle{Glyph: presenter.GlyphDiamond, Color: presenter.ThingItemColor}
	}
	return presenter.ThingStyle{Glyph: presenter.GlyphCross, Color: presenter.ThingMiscColor}
}

func isPlayerStart(typ int16) bool {
	return typ >= 1 && typ <= 4
}

func isMonster(typ int16) bool {
	switch typ {
	case 7, 9, 16, 58, 64, 65, 66, 67, 68, 69, 71, 84:
		return true
	case 3001, 3002, 3003, 3004, 3005, 3006:
		return true
	default:
		return false
	}
}

func keyColorForType(typ int16) (color.RGBA, bool) {
	switch typ {
	case 5, 40:
		return presenter.ThingKeyBlue, true
	case 13, 38:
		return presenter.ThingKeyRed, true
	case 6, 39:
		return presenter.ThingKeyYellow, true
	default:
		return color.RGBA{}, false
	}
}

func isItemOrPickup(typ int16) bool {
	switch typ {
	case 8, 17, 83, 2011, 2012, 2013, 2014, 2015, 2018, 2019, 2022, 2023, 2024, 2025, 2026, 2045, 2046, 2047, 2048:
		return true
	default:
		return false
	}
}

func relativeThingAngle(thingAngle int16, viewAngle uint32) int16 {
	return relativeWorldAngle(thingDegToWorldAngle(thingAngle), viewAngle)
}

func worldThingAngle(thingAngle int16) int16 {
	return worldAngleToGlyphAngle(thingDegToWorldAngle(thingAngle))
}

func relativeWorldAngle(worldAngle, viewAngle uint32) int16 {
	viewDeg := float64(viewAngle) * (360.0 / 4294967296.0)
	thingDeg := float64(worldAngle) * (360.0 / 4294967296.0)
	delta := viewDeg - thingDeg
	return int16(normalizeDegrees(delta))
}

func worldAngleToGlyphAngle(worldAngle uint32) int16 {
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
