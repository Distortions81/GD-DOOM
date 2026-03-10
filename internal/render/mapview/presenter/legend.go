package presenter

import (
	"fmt"
	"image/color"
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

type LegendHooks struct {
	DrawGlyph func(screen *ebiten.Image, glyph Glyph, clr color.RGBA, x, y, size float64, antiAlias bool)
}

func ShouldDrawThings(iddt int) bool {
	return iddt >= 2
}

func DrawThingLegend(screen *ebiten.Image, in LegendInputs, colors LegendColors, hooks LegendHooks) {
	if screen == nil || hooks.DrawGlyph == nil {
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
		hooks.DrawGlyph(screen, e.glyph, e.clr, float64(x+8), float64(ly+5), 4.6, in.AntiAlias)
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
