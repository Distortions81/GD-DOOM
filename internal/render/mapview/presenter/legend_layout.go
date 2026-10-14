package presenter

import "gddoom/internal/render/mapview"

type LegendLabel struct {
	X, Y int
	Text string
}

type LegendLayout struct {
	Segments []mapview.Segment
	Labels   []LegendLabel
}

// LayoutThingLegend supplies both hosts with the main automap's exact header,
// text and glyph positions. Width follows the longest label, including mode.
func LayoutThingLegend(in LegendInputs, colors LegendColors) LegendLayout {
	entries, lines := ThingLegendEntries(in, colors), LineLegendEntries(colors)
	maxLen := len("THING LEGEND")
	for _, e := range entries {
		maxLen = max(maxLen, len(e.Label))
	}
	for _, e := range lines {
		maxLen = max(maxLen, len(e.Label))
	}
	x, y := max(10, in.ViewWidth-maxLen*7-36), 28
	f := LegendLayout{Labels: []LegendLabel{{x, y, "THING LEGEND"}}}
	for i, e := range entries {
		ly := y + 16 + i*14
		f.Segments = AppendThingGlyph(f.Segments, ThingStyle{Glyph: e.Glyph, Color: e.Color}, float64(x+8), float64(ly+5), 0, 4.6)
		f.Labels = append(f.Labels, LegendLabel{x + 18, ly, e.Label})
	}
	ly0 := y + 16 + len(entries)*14 + 8
	f.Labels = append(f.Labels, LegendLabel{x, ly0, "LINE COLORS"})
	for i, e := range lines {
		ly := ly0 + 16 + i*14
		f.Segments = append(f.Segments, mapview.Segment{X1: float64(x + 2), Y1: float64(ly + 5), X2: float64(x + 14), Y2: float64(ly + 5), Width: 2.4, Color: e.Color})
		f.Labels = append(f.Labels, LegendLabel{x + 18, ly, e.Label})
	}
	return f
}
