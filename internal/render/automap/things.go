package automap

import (
	"image/color"
	"math"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type thingGlyph int

const (
	thingGlyphCross thingGlyph = iota
	thingGlyphSquare
	thingGlyphDiamond
	thingGlyphTriangle
	thingGlyphStar
)

type thingStyle struct {
	glyph thingGlyph
	clr   color.RGBA
}

var (
	thingPlayerColor  = color.RGBA{R: 120, G: 220, B: 255, A: 255}
	thingMonsterColor = color.RGBA{R: 255, G: 120, B: 120, A: 255}
	thingItemColor    = color.RGBA{R: 255, G: 220, B: 120, A: 255}
	thingKeyBlue      = color.RGBA{R: 90, G: 150, B: 255, A: 255}
	thingKeyRed       = color.RGBA{R: 255, G: 90, B: 90, A: 255}
	thingKeyYellow    = color.RGBA{R: 255, G: 220, B: 70, A: 255}
	thingMiscColor    = color.RGBA{R: 170, G: 170, B: 170, A: 255}
)

func styleForThing(t mapdata.Thing) thingStyle {
	if isPlayerStart(t.Type) {
		return thingStyle{glyph: thingGlyphSquare, clr: thingPlayerColor}
	}
	if isMonster(t.Type) {
		return thingStyle{glyph: thingGlyphTriangle, clr: thingMonsterColor}
	}
	if k, ok := keyColorForType(t.Type); ok {
		return thingStyle{glyph: thingGlyphStar, clr: k}
	}
	if isItemOrPickup(t.Type) {
		return thingStyle{glyph: thingGlyphDiamond, clr: thingItemColor}
	}
	return thingStyle{glyph: thingGlyphCross, clr: thingMiscColor}
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
		return thingKeyBlue, true
	case 13, 38:
		return thingKeyRed, true
	case 6, 39:
		return thingKeyYellow, true
	default:
		return color.RGBA{}, false
	}
}

func isItemOrPickup(typ int16) bool {
	switch typ {
	case 8, 17, 2011, 2012, 2013, 2014, 2015, 2018, 2019, 2022, 2023, 2024, 2025, 2026, 2045, 2046, 2047, 2048:
		return true
	default:
		return false
	}
}

func drawThingGlyph(screen *ebiten.Image, style thingStyle, sx, sy float64, angleDeg int16, size float64) {
	switch style.glyph {
	case thingGlyphSquare:
		drawSquareGlyph(screen, sx, sy, size*0.90, style.clr)
	case thingGlyphDiamond:
		drawDiamondGlyph(screen, sx, sy, size, style.clr)
	case thingGlyphTriangle:
		drawTriangleGlyph(screen, sx, sy, size*1.15, angleDeg, style.clr)
	case thingGlyphStar:
		drawStarGlyph(screen, sx, sy, size*1.10, style.clr)
	default:
		drawCrossGlyph(screen, sx, sy, size*0.80, style.clr)
	}
}

func drawCrossGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA) {
	vector.StrokeLine(screen, float32(sx-r), float32(sy), float32(sx+r), float32(sy), 1.5, clr, true)
	vector.StrokeLine(screen, float32(sx), float32(sy-r), float32(sx), float32(sy+r), 1.5, clr, true)
}

func drawSquareGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA) {
	vector.StrokeLine(screen, float32(sx-r), float32(sy-r), float32(sx+r), float32(sy-r), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx+r), float32(sy-r), float32(sx+r), float32(sy+r), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx+r), float32(sy+r), float32(sx-r), float32(sy+r), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx-r), float32(sy+r), float32(sx-r), float32(sy-r), 1.4, clr, true)
}

func drawDiamondGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA) {
	vector.StrokeLine(screen, float32(sx), float32(sy-r), float32(sx+r), float32(sy), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx+r), float32(sy), float32(sx), float32(sy+r), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx), float32(sy+r), float32(sx-r), float32(sy), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx-r), float32(sy), float32(sx), float32(sy-r), 1.4, clr, true)
}

func drawTriangleGlyph(screen *ebiten.Image, sx, sy, r float64, angleDeg int16, clr color.RGBA) {
	a := float64(angleDeg) * math.Pi / 180.0
	p1x, p1y := rotatePoint(0, -r, a)
	p2x, p2y := rotatePoint(r*0.85, r*0.8, a)
	p3x, p3y := rotatePoint(-r*0.85, r*0.8, a)
	vector.StrokeLine(screen, float32(sx+p1x), float32(sy+p1y), float32(sx+p2x), float32(sy+p2y), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx+p2x), float32(sy+p2y), float32(sx+p3x), float32(sy+p3y), 1.4, clr, true)
	vector.StrokeLine(screen, float32(sx+p3x), float32(sy+p3y), float32(sx+p1x), float32(sy+p1y), 1.4, clr, true)
}

func drawStarGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA) {
	drawCrossGlyph(screen, sx, sy, r, clr)
	vector.StrokeLine(screen, float32(sx-r*0.7), float32(sy-r*0.7), float32(sx+r*0.7), float32(sy+r*0.7), 1.3, clr, true)
	vector.StrokeLine(screen, float32(sx-r*0.7), float32(sy+r*0.7), float32(sx+r*0.7), float32(sy-r*0.7), 1.3, clr, true)
}

func rotatePoint(x, y, angleRad float64) (float64, float64) {
	c := math.Cos(angleRad)
	s := math.Sin(angleRad)
	return x*c - y*s, x*s + y*c
}

func relativeThingAngle(thingAngle int16, viewAngle uint32) int16 {
	viewDeg := float64(viewAngle) * (360.0 / 4294967296.0)
	delta := viewDeg - float64(thingAngle)
	return int16(normalizeDegrees(delta))
}

func worldThingAngle(thingAngle int16) int16 {
	// Doom things use 0=east, 90=north; glyphs use 0=up, +90=right.
	delta := 90.0 - float64(thingAngle)
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
