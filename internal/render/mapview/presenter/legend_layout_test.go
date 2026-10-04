package presenter

import "testing"

func TestThingLegendLayoutTracksMainWidthAndRows(t *testing.T) {
	for _, width := range []int{160, 320, 640, 1280} {
		f := LayoutThingLegend(LegendInputs{ViewWidth: width, SourcePortMode: true, SourcePortThingLabel: "sprites"}, LegendColors{})
		// The longest label is "unrevealed (allmap)" (19 characters).
		x := max(10, width-19*7-36)
		if len(f.Labels) != 14 || f.Labels[0] != (LegendLabel{x, 28, "THING LEGEND"}) || f.Labels[1] != (LegendLabel{x + 18, 44, "player starts"}) || f.Labels[7] != (LegendLabel{x, 136, "LINE COLORS"}) || f.Labels[13] != (LegendLabel{x + 18, 222, "unrevealed (allmap)"}) {
			t.Fatalf("width=%d incorrect legend layout: %+v", width, f.Labels)
		}
		if len(f.Segments) == 0 || f.Segments[len(f.Segments)-1].Width != 2.4 {
			t.Fatal("legend lost glyph/line geometry")
		}
	}
}
