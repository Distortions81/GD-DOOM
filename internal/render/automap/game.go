package automap

import (
	"fmt"
	"image/color"
	"math"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	playerColor      = color.RGBA{R: 120, G: 240, B: 130, A: 255}
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

	mode       viewMode
	followMode bool
	p          player

	lines       []physLine
	lineValid   []int
	validCount  int
	bmapOriginX int64
	bmapOriginY int64
	bmapWidth   int
	bmapHeight  int
	physForLine []int
	renderSeen  []int
	renderEpoch int
	visibleBuf  []int

	lastMouseX   int
	mouseLookSet bool
}

func newGame(m *mapdata.Map, opts Options) *game {
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 720
	}
	g := &game{
		m:          m,
		opts:       opts,
		bounds:     mapBounds(m),
		viewW:      opts.Width,
		viewH:      opts.Height,
		mode:       viewMap,
		followMode: true,
		p:          spawnPlayer(m),
	}
	g.mode = viewWalk
	g.initPhysics()
	g.physForLine = make([]int, len(g.m.Linedefs))
	for i := range g.physForLine {
		g.physForLine[i] = -1
	}
	for i, pl := range g.lines {
		if pl.idx >= 0 && pl.idx < len(g.physForLine) {
			g.physForLine[pl.idx] = i
		}
	}
	g.renderSeen = make([]int, len(g.m.Linedefs))
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
	g.mode = viewWalk
	ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	g.updateWalkMode()
	return nil
}

func (g *game) updateMapMode() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.followMode = !g.followMode
	}
	if inpututil.IsKeyJustPressed(ebiten.Key0) || inpututil.IsKeyJustPressed(ebiten.KeyKP0) {
		g.resetView()
	}
	g.updateZoom()

	if g.followMode {
		g.camX = float64(g.p.x) / fracUnit
		g.camY = float64(g.p.y) / fracUnit
		return
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
}

func (g *game) updateWalkMode() {
	g.updateZoom()
	cmd := moveCmd{}
	speed := 0
	if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		speed = 1
	}
	strafeMod := ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		cmd.forward += forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		cmd.forward -= forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		cmd.side -= sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		cmd.side += sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		if strafeMod {
			cmd.side -= sideMove[speed]
		} else {
			cmd.turn -= 1
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		if strafeMod {
			cmd.side += sideMove[speed]
		} else {
			cmd.turn += 1
		}
	}

	mx, _ := ebiten.CursorPosition()
	if g.mouseLookSet {
		dx := mx - g.lastMouseX
		// Keep vanilla-feeling turn quantization while using modern mouse-look default.
		cmd.turnRaw += int64(dx) * (40 << 16)
	}
	g.lastMouseX = mx
	g.mouseLookSet = true

	cmd.run = speed == 1
	g.updatePlayer(cmd)
	g.camX = float64(g.p.x) / fracUnit
	g.camY = float64(g.p.y) / fracUnit
}

func (g *game) updateZoom() {
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
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(bgColor)

	for _, li := range g.visibleLineIndices() {
		pi := g.physForLine[li]
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		ld := g.m.Linedefs[li]
		pl := g.lines[pi]
		x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
		x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
		if x1 == x2 && y1 == y2 {
			continue
		}
		c, w := g.linedefStyle(ld)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), float32(w), c, true)
	}

	g.drawPlayer(screen)

	modeText := "MAP"
	if g.mode == viewWalk {
		modeText = "WALK"
	}
	overlay := fmt.Sprintf("%s | mode %s | zoom %.2f | move WASD | mouse turn | arrows move/turn | Alt+arrows strafe | Shift run",
		g.m.Name,
		modeText,
		g.zoom,
	)
	ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
}

func (g *game) drawPlayer(screen *ebiten.Image) {
	px := float64(g.p.x) / fracUnit
	py := float64(g.p.y) / fracUnit
	sx, sy := g.worldToScreen(px, py)
	ang := angleToRadians(g.p.angle)
	forwardX := sx + math.Cos(ang)*12
	forwardY := sy - math.Sin(ang)*12
	leftX := sx + math.Cos(ang+2.6)*8
	leftY := sy - math.Sin(ang+2.6)*8
	rightX := sx + math.Cos(ang-2.6)*8
	rightY := sy - math.Sin(ang-2.6)*8
	vector.StrokeLine(screen, float32(sx), float32(sy), float32(forwardX), float32(forwardY), 2, playerColor, true)
	vector.StrokeLine(screen, float32(leftX), float32(leftY), float32(forwardX), float32(forwardY), 2, playerColor, true)
	vector.StrokeLine(screen, float32(rightX), float32(rightY), float32(forwardX), float32(forwardY), 2, playerColor, true)
	vector.StrokeCircle(screen, float32(sx), float32(sy), float32((float64(playerRadius)/fracUnit)*g.zoom), 1, playerColor, false)
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

	s0 := g.m.Sidedefs[int(ld.SideNum[0])].Sector
	s1 := g.m.Sidedefs[int(ld.SideNum[1])].Sector
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

func (g *game) visibleLineIndices() []int {
	margin := 2.0 / g.zoom
	viewHalfW := float64(g.viewW) / (2 * g.zoom)
	viewHalfH := float64(g.viewH) / (2 * g.zoom)
	minX := floatToFixed(g.camX - viewHalfW - margin)
	maxX := floatToFixed(g.camX + viewHalfW + margin)
	minY := floatToFixed(g.camY - viewHalfH - margin)
	maxY := floatToFixed(g.camY + viewHalfH + margin)

	g.visibleBuf = g.visibleBuf[:0]
	g.renderEpoch++
	if g.renderEpoch == 0 {
		for i := range g.renderSeen {
			g.renderSeen[i] = 0
		}
		g.renderEpoch = 1
	}

	if g.m.BlockMap != nil && g.bmapWidth > 0 && g.bmapHeight > 0 {
		bx0 := int((minX - g.bmapOriginX) >> (fracBits + 7))
		bx1 := int((maxX - g.bmapOriginX) >> (fracBits + 7))
		by0 := int((minY - g.bmapOriginY) >> (fracBits + 7))
		by1 := int((maxY - g.bmapOriginY) >> (fracBits + 7))
		for bx := bx0; bx <= bx1; bx++ {
			for by := by0; by <= by1; by++ {
				if bx < 0 || by < 0 || bx >= g.bmapWidth || by >= g.bmapHeight {
					continue
				}
				cellIdx := by*g.bmapWidth + bx
				if cellIdx < 0 || cellIdx >= len(g.m.BlockMap.Cells) {
					continue
				}
				for _, lw := range g.m.BlockMap.Cells[cellIdx] {
					li := int(lw)
					if li < 0 || li >= len(g.m.Linedefs) {
						continue
					}
					if g.renderSeen[li] == g.renderEpoch {
						continue
					}
					g.renderSeen[li] = g.renderEpoch
					if g.lineVisibleInBox(li, minX, minY, maxX, maxY) {
						g.visibleBuf = append(g.visibleBuf, li)
					}
				}
			}
		}
		return g.visibleBuf
	}

	for _, pl := range g.lines {
		if !bboxIntersects(pl.bbox, minX, minY, maxX, maxY) {
			continue
		}
		g.visibleBuf = append(g.visibleBuf, pl.idx)
	}
	return g.visibleBuf
}

func (g *game) lineVisibleInBox(lineIdx int, minX, minY, maxX, maxY int64) bool {
	pi := g.physForLine[lineIdx]
	if pi < 0 || pi >= len(g.lines) {
		return false
	}
	return bboxIntersects(g.lines[pi].bbox, minX, minY, maxX, maxY)
}

func bboxIntersects(lineBBox [4]int64, minX, minY, maxX, maxY int64) bool {
	lineMaxY := lineBBox[0]
	lineMinY := lineBBox[1]
	lineMaxX := lineBBox[2]
	lineMinX := lineBBox[3]
	if lineMaxX < minX || lineMinX > maxX {
		return false
	}
	if lineMaxY < minY || lineMinY > maxY {
		return false
	}
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
