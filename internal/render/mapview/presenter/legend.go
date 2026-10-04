package presenter

import (
	"gddoom/internal/render/mapview"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type Glyph int

const (
	GlyphCross Glyph = iota
	GlyphSquare
	GlyphDiamond
	GlyphTriangle
	GlyphStar
)

type ThingStyle struct {
	Glyph Glyph
	Color color.RGBA
}

var (
	ThingPlayerColor  = color.RGBA{R: 120, G: 220, B: 255, A: 255}
	ThingMonsterColor = color.RGBA{R: 255, G: 120, B: 120, A: 255}
	ThingItemColor    = color.RGBA{R: 255, G: 220, B: 120, A: 255}
	ThingKeyBlue      = color.RGBA{R: 90, G: 150, B: 255, A: 255}
	ThingKeyRed       = color.RGBA{R: 255, G: 90, B: 90, A: 255}
	ThingKeyYellow    = color.RGBA{R: 255, G: 220, B: 70, A: 255}
	ThingMiscColor    = color.RGBA{R: 170, G: 170, B: 170, A: 255}
)

func StyleForThingType(typ int16, isPlayerStart, isMonster bool) ThingStyle {
	if isPlayerStart {
		return ThingStyle{Glyph: GlyphSquare, Color: ThingPlayerColor}
	}
	if isMonster {
		return ThingStyle{Glyph: GlyphTriangle, Color: ThingMonsterColor}
	}
	if k, ok := keyColorForType(typ); ok {
		return ThingStyle{Glyph: GlyphStar, Color: k}
	}
	if IsItemOrPickupType(typ) {
		return ThingStyle{Glyph: GlyphDiamond, Color: ThingItemColor}
	}
	return ThingStyle{Glyph: GlyphCross, Color: ThingMiscColor}
}

func IsItemOrPickupType(typ int16) bool {
	switch typ {
	case 8, 17, 83, 2011, 2012, 2013, 2014, 2015, 2018, 2019, 2022, 2023, 2024, 2025, 2026, 2045, 2046, 2047, 2048:
		return true
	default:
		return false
	}
}

func keyColorForType(typ int16) (color.RGBA, bool) {
	switch typ {
	case 5, 40:
		return ThingKeyBlue, true
	case 13, 38:
		return ThingKeyRed, true
	case 6, 39:
		return ThingKeyYellow, true
	default:
		return color.RGBA{}, false
	}
}

type LegendColors struct {
	ThingPlayer  color.RGBA
	ThingMonster color.RGBA
	ThingItem    color.RGBA
	ThingKey     color.RGBA
	ThingMisc    color.RGBA
	WallOneSided color.RGBA
	WallFloor    color.RGBA
	WallCeil     color.RGBA
	WallTeleport color.RGBA
	WallUse      color.RGBA
	WallHidden   color.RGBA
}

type LegendInputs struct {
	ViewWidth            int
	AntiAlias            bool
	SourcePortMode       bool
	SourcePortThingLabel string
}

func ShouldDrawThings(iddt int) bool {
	return true
}

func DrawThingGlyph(screen *ebiten.Image, style ThingStyle, sx, sy float64, angleDeg int16, size float64, antiAlias bool) {
	var segments [4]mapview.Segment
	mapview.DrawSegments(screen, AppendThingGlyph(segments[:0], style, sx, sy, angleDeg, size), antiAlias)
}

func DrawThingLegend(screen *ebiten.Image, in LegendInputs, colors LegendColors) {
	if screen == nil {
		return
	}

	f := LayoutThingLegend(in, colors)
	mapview.DrawSegments(screen, f.Segments, in.AntiAlias)
	for _, label := range f.Labels {
		ebitenutil.DebugPrintAt(screen, label.Text, label.X, label.Y)
	}
}
