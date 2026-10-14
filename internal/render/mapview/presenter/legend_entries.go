package presenter

import (
	"fmt"
	"image/color"
	"strings"
)

type ThingLegendEntry struct {
	Label string
	Glyph Glyph
	Color color.RGBA
}
type LineLegendEntry struct {
	Label string
	Color color.Color
}

func ThingLegendEntries(in LegendInputs, c LegendColors) []ThingLegendEntry {
	entries := []ThingLegendEntry{{"player starts", GlyphSquare, c.ThingPlayer}, {"monsters", GlyphTriangle, c.ThingMonster}, {"items/pickups", GlyphDiamond, c.ThingItem}, {"keys", GlyphStar, c.ThingKey}, {"misc", GlyphCross, c.ThingMisc}}
	if in.SourcePortMode {
		entries = append(entries, ThingLegendEntry{fmt.Sprintf("render: %s", strings.ToLower(in.SourcePortThingLabel)), GlyphCross, c.ThingMisc})
	}
	return entries
}
func LineLegendEntries(c LegendColors) []LineLegendEntry {
	return []LineLegendEntry{{"one-sided wall", c.WallOneSided}, {"floor delta", c.WallFloor}, {"ceiling delta", c.WallCeil}, {"teleporter", c.WallTeleport}, {"use switch/button", c.WallUse}, {"unrevealed (allmap)", c.WallHidden}}
}
