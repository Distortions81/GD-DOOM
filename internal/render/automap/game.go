package automap

import (
	"fmt"
	"image/color"
	"math"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	lineOneSidedWidth = 1.8
	lineTwoSidedWidth = 1.2
)

var (
	bgColor          = color.RGBA{R: 5, G: 7, B: 9, A: 255}
	wallOneSided     = color.RGBA{R: 220, G: 58, B: 48, A: 255}
	wallSecret       = color.RGBA{R: 160, G: 100, B: 220, A: 255}
	wallFloorChange  = color.RGBA{R: 170, G: 120, B: 60, A: 255}
	wallCeilChange   = color.RGBA{R: 220, G: 200, B: 70, A: 255}
	wallNoHeightDiff = color.RGBA{R: 86, G: 86, B: 86, A: 255}
)

type game struct {
	m       *mapdata.Map
	opts    Options
	bounds  bounds
	viewW   int
	viewH   int
	camX    float64
	camY    float64
	zoom    float64
	fitZoom float64
}

func newGame(m *mapdata.Map, opts Options) *game {
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 720
	}
	g := &game{
		m:      m,
		opts:   opts,
		bounds: mapBounds(m),
		viewW:  opts.Width,
		viewH:  opts.Height,
	}
	g.resetView()
	if opts.StartZoom > 0 {
		g.zoom = opts.StartZoom
	}
	return g
}

func (g *game) resetView() {
	g.camX = (g.bounds.minX + g.bounds.maxX) / 2
	g.camY = (g.bounds.minY + g.bounds.maxY) / 2

	worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	margin := 0.9
	zx := float64(max(g.viewW, 1)) * margin / worldW
	zy := float64(max(g.viewH, 1)) * margin / worldH
	g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
	g.zoom = g.fitZoom
}

func (g *game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	if inpututilIsPressedAny(ebiten.Key0, ebiten.KeyKP0) {
		g.resetView()
	}

	zoomStep := 1.03
	if ebiten.IsKeyPressed(ebiten.KeyEqual) || ebiten.IsKeyPressed(ebiten.KeyKPAdd) {
		g.zoom *= zoomStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyMinus) || ebiten.IsKeyPressed(ebiten.KeyKPSubtract) {
		g.zoom /= zoomStep
	}
	_, wheelY := ebiten.Wheel()
	if wheelY > 0 {
		g.zoom *= 1.1
	}
	if wheelY < 0 {
		g.zoom /= 1.1
	}
	if g.zoom < g.fitZoom*0.05 {
		g.zoom = g.fitZoom * 0.05
	}
	if g.zoom > g.fitZoom*200 {
		g.zoom = g.fitZoom * 200
	}

	panStep := 14.0 / g.zoom
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.camY += panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.camY -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.camX -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.camX += panStep
	}

	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(bgColor)

	for i := range g.m.Linedefs {
		ld := g.m.Linedefs[i]
		if int(ld.V1) >= len(g.m.Vertexes) || int(ld.V2) >= len(g.m.Vertexes) {
			continue
		}
		v1 := g.m.Vertexes[ld.V1]
		v2 := g.m.Vertexes[ld.V2]
		x1, y1 := g.worldToScreen(float64(v1.X), float64(v1.Y))
		x2, y2 := g.worldToScreen(float64(v2.X), float64(v2.Y))
		if x1 == x2 && y1 == y2 {
			continue
		}
		c, w := g.linedefStyle(ld)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), float32(w), c, true)
	}

	overlay := fmt.Sprintf("%s | lines %d verts %d | zoom %.2f | pan WASD/arrows zoom +/- wheel reset 0 exit Esc",
		g.m.Name,
		len(g.m.Linedefs),
		len(g.m.Vertexes),
		g.zoom,
	)
	ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.viewW = max(outsideWidth, 1)
	g.viewH = max(outsideHeight, 1)
	return g.viewW, g.viewH
}

func (g *game) worldToScreen(x, y float64) (float64, float64) {
	sx := (x-g.camX)*g.zoom + float64(g.viewW)/2
	sy := float64(g.viewH)/2 - (y-g.camY)*g.zoom
	return sx, sy
}

func (g *game) linedefStyle(ld mapdata.Linedef) (color.Color, float64) {
	const (
		mlSecret = 0x20
	)

	if ld.SideNum[1] < 0 {
		if ld.Flags&mlSecret != 0 {
			return wallSecret, lineOneSidedWidth
		}
		return wallOneSided, lineOneSidedWidth
	}
	if ld.SideNum[0] < 0 || ld.SideNum[1] < 0 {
		return wallOneSided, lineOneSidedWidth
	}
	if int(ld.SideNum[0]) >= len(g.m.Sidedefs) || int(ld.SideNum[1]) >= len(g.m.Sidedefs) {
		return wallOneSided, lineOneSidedWidth
	}

	s0 := g.m.Sidedefs[ld.SideNum[0]].Sector
	s1 := g.m.Sidedefs[ld.SideNum[1]].Sector
	if int(s0) >= len(g.m.Sectors) || int(s1) >= len(g.m.Sectors) {
		return wallOneSided, lineOneSidedWidth
	}

	sec0 := g.m.Sectors[s0]
	sec1 := g.m.Sectors[s1]
	if sec0.FloorHeight != sec1.FloorHeight {
		return wallFloorChange, lineTwoSidedWidth
	}
	if sec0.CeilingHeight != sec1.CeilingHeight {
		return wallCeilChange, lineTwoSidedWidth
	}
	if ld.Flags&mlSecret != 0 {
		return wallSecret, lineTwoSidedWidth
	}
	return wallNoHeightDiff, 1
}

func inpututilIsPressedAny(keys ...ebiten.Key) bool {
	for _, k := range keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
