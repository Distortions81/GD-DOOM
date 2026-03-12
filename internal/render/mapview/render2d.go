package mapview

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Segment struct {
	X1    float64
	Y1    float64
	X2    float64
	Y2    float64
	Width float32
	Color color.Color
}

func DrawCachedLines(screen *ebiten.Image, items []CachedLine, antiAlias bool) {
	if screen == nil {
		return
	}
	for _, ln := range items {
		vector.StrokeLine(screen, ln.X1, ln.Y1, ln.X2, ln.Y2, ln.W, ln.Clr, antiAlias)
	}
}

func DrawGrid(screen *ebiten.Image, view Snapshot, vp Viewport, worldToScreen func(float64, float64) (float64, float64), antiAlias bool) {
	if screen == nil || worldToScreen == nil {
		return
	}
	const cell = 128.0
	left, right, bottom, top := view.BoundsForViewport(vp)
	grid := color.RGBA{R: 40, G: 50, B: 60, A: 255}

	startX := math.Floor(left/cell) * cell
	for x := startX; x <= right; x += cell {
		x1, y1 := worldToScreen(x, bottom)
		x2, y2 := worldToScreen(x, top)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, antiAlias)
	}
	startY := math.Floor(bottom/cell) * cell
	for y := startY; y <= top; y += cell {
		x1, y1 := worldToScreen(left, y)
		x2, y2 := worldToScreen(right, y)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, antiAlias)
	}
}

func DrawMarks(screen *ebiten.Image, items []Mark, worldToScreen func(float64, float64) (float64, float64), antiAlias bool) {
	if screen == nil || worldToScreen == nil {
		return
	}
	mc := color.RGBA{R: 120, G: 210, B: 255, A: 255}
	for _, mk := range items {
		sx, sy := worldToScreen(mk.X, mk.Y)
		r := 5.0
		vector.StrokeLine(screen, float32(sx-r), float32(sy-r), float32(sx+r), float32(sy+r), 1.3, mc, antiAlias)
		vector.StrokeLine(screen, float32(sx-r), float32(sy+r), float32(sx+r), float32(sy-r), 1.3, mc, antiAlias)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", mk.ID), int(sx)+6, int(sy)-6)
	}
}

func DrawSegments(screen *ebiten.Image, segments []Segment, antiAlias bool) {
	if screen == nil {
		return
	}
	for _, seg := range segments {
		vector.StrokeLine(screen, float32(seg.X1), float32(seg.Y1), float32(seg.X2), float32(seg.Y2), seg.Width, seg.Color, antiAlias)
	}
}
