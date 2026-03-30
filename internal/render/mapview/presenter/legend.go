package presenter

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	LineColorMode        string
}

func ShouldDrawThings(iddt int) bool {
	return true
}

func DrawThingGlyph(screen *ebiten.Image, style ThingStyle, sx, sy float64, angleDeg int16, size float64, antiAlias bool) {
	switch style.Glyph {
	case GlyphSquare:
		drawSquareGlyph(screen, sx, sy, size*0.90, style.Color, antiAlias)
	case GlyphDiamond:
		drawDiamondGlyph(screen, sx, sy, size, style.Color, antiAlias)
	case GlyphTriangle:
		drawTriangleGlyph(screen, sx, sy, size*1.15, angleDeg, style.Color, antiAlias)
	case GlyphStar:
		drawStarGlyph(screen, sx, sy, size*1.10, style.Color, antiAlias)
	default:
		drawCrossGlyph(screen, sx, sy, size*0.80, style.Color, antiAlias)
	}
}

func DrawThingLegend(screen *ebiten.Image, in LegendInputs, colors LegendColors) {
	if screen == nil {
		return
	}

	type legendEntry struct {
		label string
		glyph Glyph
		clr   color.RGBA
	}
	entries := []legendEntry{
		{label: "player starts", glyph: GlyphSquare, clr: colors.ThingPlayer},
		{label: "monsters", glyph: GlyphTriangle, clr: colors.ThingMonster},
		{label: "items/pickups", glyph: GlyphDiamond, clr: colors.ThingItem},
		{label: "keys", glyph: GlyphStar, clr: colors.ThingKey},
		{label: "misc", glyph: GlyphCross, clr: colors.ThingMisc},
	}
	if in.SourcePortMode {
		entries = append(entries, legendEntry{
			label: fmt.Sprintf("render: %s", strings.ToLower(in.SourcePortThingLabel)),
			glyph: GlyphCross,
			clr:   colors.ThingMisc,
		})
	}

	type lineLegendEntry struct {
		label string
		clr   color.Color
	}
	lineEntries := []lineLegendEntry{
		{label: "one-sided wall", clr: colors.WallOneSided},
		{label: "floor delta", clr: colors.WallFloor},
		{label: "ceiling delta", clr: colors.WallCeil},
		{label: "teleporter", clr: colors.WallTeleport},
		{label: "use switch/button", clr: colors.WallUse},
	}
	if in.LineColorMode == "parity" {
		lineEntries = append(lineEntries, lineLegendEntry{label: "unrevealed (allmap)", clr: colors.WallHidden})
	}

	maxLen := len("THING LEGEND")
	for _, e := range entries {
		if len(e.label) > maxLen {
			maxLen = len(e.label)
		}
	}
	if len("LINE COLORS") > maxLen {
		maxLen = len("LINE COLORS")
	}
	for _, e := range lineEntries {
		if len(e.label) > maxLen {
			maxLen = len(e.label)
		}
	}

	x := in.ViewWidth - maxLen*7 - 36
	if x < 10 {
		x = 10
	}
	y := 28

	ebitenutil.DebugPrintAt(screen, "THING LEGEND", x, y)
	for i, e := range entries {
		ly := y + 16 + i*14
		DrawThingGlyph(screen, ThingStyle{Glyph: e.glyph, Color: e.clr}, float64(x+8), float64(ly+5), 0, 4.6, in.AntiAlias)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}

	ly0 := y + 16 + len(entries)*14 + 8
	ebitenutil.DebugPrintAt(screen, "LINE COLORS", x, ly0)
	for i, e := range lineEntries {
		ly := ly0 + 16 + i*14
		vector.StrokeLine(screen, float32(x+2), float32(ly+5), float32(x+14), float32(ly+5), 2.4, e.clr, in.AntiAlias)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}
}

func drawCrossGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA, antiAlias bool) {
	vector.StrokeLine(screen, float32(sx-r), float32(sy), float32(sx+r), float32(sy), 1.5, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx), float32(sy-r), float32(sx), float32(sy+r), 1.5, clr, antiAlias)
}

func drawSquareGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA, antiAlias bool) {
	vector.StrokeLine(screen, float32(sx-r), float32(sy-r), float32(sx+r), float32(sy-r), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx+r), float32(sy-r), float32(sx+r), float32(sy+r), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx+r), float32(sy+r), float32(sx-r), float32(sy+r), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx-r), float32(sy+r), float32(sx-r), float32(sy-r), 1.4, clr, antiAlias)
}

func drawDiamondGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA, antiAlias bool) {
	vector.StrokeLine(screen, float32(sx), float32(sy-r), float32(sx+r), float32(sy), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx+r), float32(sy), float32(sx), float32(sy+r), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx), float32(sy+r), float32(sx-r), float32(sy), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx-r), float32(sy), float32(sx), float32(sy-r), 1.4, clr, antiAlias)
}

func drawTriangleGlyph(screen *ebiten.Image, sx, sy, r float64, angleDeg int16, clr color.RGBA, antiAlias bool) {
	a := float64(angleDeg) * math.Pi / 180.0
	p1x, p1y := rotatePoint(0, -r, a)
	p2x, p2y := rotatePoint(r*0.85, r*0.8, a)
	p3x, p3y := rotatePoint(-r*0.85, r*0.8, a)
	vector.StrokeLine(screen, float32(sx+p1x), float32(sy+p1y), float32(sx+p2x), float32(sy+p2y), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx+p2x), float32(sy+p2y), float32(sx+p3x), float32(sy+p3y), 1.4, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx+p3x), float32(sy+p3y), float32(sx+p1x), float32(sy+p1y), 1.4, clr, antiAlias)
}

func drawStarGlyph(screen *ebiten.Image, sx, sy, r float64, clr color.RGBA, antiAlias bool) {
	drawCrossGlyph(screen, sx, sy, r, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx-r*0.7), float32(sy-r*0.7), float32(sx+r*0.7), float32(sy+r*0.7), 1.3, clr, antiAlias)
	vector.StrokeLine(screen, float32(sx-r*0.7), float32(sy+r*0.7), float32(sx+r*0.7), float32(sy-r*0.7), 1.3, clr, antiAlias)
}

func rotatePoint(x, y, angleRad float64) (float64, float64) {
	c := math.Cos(angleRad)
	s := math.Sin(angleRad)
	return x*c - y*s, x*s + y*c
}
