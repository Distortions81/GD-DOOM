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
	wallTeleporter   = color.RGBA{R: 40, G: 165, B: 220, A: 255}
	wallFloorChange  = color.RGBA{R: 170, G: 120, B: 60, A: 255}
	wallCeilChange   = color.RGBA{R: 220, G: 200, B: 70, A: 255}
	wallNoHeightDiff = color.RGBA{R: 86, G: 86, B: 86, A: 255}
	wallUnrevealed   = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	playerColor      = color.RGBA{R: 120, G: 240, B: 130, A: 255}
)

var doomPlayerArrow = [][4]float64{
	// Rough port of Doom's AM player_arrow (points right in local space).
	{-16, 0, 18.2857, 0},
	{18.2857, 0, 9.14285, 4.5714},
	{18.2857, 0, 9.14285, -4.5714},
	{-16, 0, -20.5714, 4.5714},
	{-16, 0, -20.5714, -4.5714},
	{-10.2857, 0, -16, 4.5714},
	{-10.2857, 0, -16, -4.5714},
}

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
	rotateView bool
	showHelp   bool
	parity     automapParityState
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
	sectorFloor []int64
	sectorCeil  []int64
	lineSpecial []uint16
	doors       map[int]*doorThinker
	useFlash    int
	useText     string

	lastMouseX   int
	mouseLookSet bool
}

type revealMode int

const (
	revealNormal revealMode = iota
	revealAllMap
)

type automapParityState struct {
	reveal revealMode
	iddt   int
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
		mode:       viewWalk,
		followMode: true,
		rotateView: true,
		parity: automapParityState{
			reveal: revealAllMap,
			iddt:   0,
		},
		p: spawnPlayer(m),
	}
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
	g.discoverLinesAroundPlayer()
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
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if g.mode == viewWalk {
			g.mode = viewMap
		} else {
			g.mode = viewWalk
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.rotateView = !g.rotateView
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.showHelp = !g.showHelp
	}
	if g.mode == viewMap {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
		g.updateMapMode()
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		g.updateWalkMode()
	}
	if g.useFlash > 0 {
		g.useFlash--
	}
	return nil
}

func (g *game) updateMapMode() {
	g.updateParityControls()
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
	g.updateParityControls()
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
			cmd.turn += 1
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		if strafeMod {
			cmd.side += sideMove[speed]
		} else {
			cmd.turn -= 1
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.handleUse()
	}

	mx, _ := ebiten.CursorPosition()
	if g.mouseLookSet {
		dx := mx - g.lastMouseX
		// Keep vanilla-feeling turn quantization while using modern mouse-look default.
		cmd.turnRaw -= int64(dx) * (40 << 16)
	}
	g.lastMouseX = mx
	g.mouseLookSet = true

	cmd.run = speed == 1
	g.updatePlayer(cmd)
	g.discoverLinesAroundPlayer()
	g.camX = float64(g.p.x) / fracUnit
	g.camY = float64(g.p.y) / fracUnit
}

func (g *game) updateParityControls() {
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.parity.reveal == revealNormal {
			g.parity.reveal = revealAllMap
		} else {
			g.parity.reveal = revealNormal
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.parity.iddt = (g.parity.iddt + 1) % 3
	}
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
		d := g.linedefDecision(ld)
		if !d.visible {
			continue
		}
		c, w := g.decisionStyle(d)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), float32(w), c, true)
	}

	g.drawPlayer(screen)

	modeText := "MAP"
	if g.mode == viewWalk {
		modeText = "WALK"
	}
	revealText := "normal"
	if g.parity.reveal == revealAllMap {
		revealText = "allmap"
	}
	overlay := fmt.Sprintf("%s | mode %s | zoom %.2f | reveal %s | iddt %d | move WASD | mouse turn | arrows move/turn | Alt+arrows strafe | Shift run",
		g.m.Name,
		modeText,
		g.zoom,
		revealText,
		g.parity.iddt,
	)
	ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
	if g.useFlash > 0 {
		ebitenutil.DebugPrintAt(screen, g.useText, 12, 28)
	}
	g.drawHelpUI(screen)
}

func (g *game) drawPlayer(screen *ebiten.Image) {
	px := float64(g.p.x) / fracUnit
	py := float64(g.p.y) / fracUnit
	sx, sy := g.worldToScreen(px, py)
	if g.rotateView {
		// Heading-follow: keep icon fixed-up in screen-space.
		g.drawPlayerArrowScreen(screen, sx, sy, math.Pi/2)
		return
	}
	ang := angleToRadians(g.p.angle)
	g.drawPlayerArrowWorld(screen, px, py, ang)
}

func (g *game) drawPlayerArrowWorld(screen *ebiten.Image, px, py, ang float64) {
	ca := math.Cos(ang)
	sa := math.Sin(ang)
	for _, seg := range doomPlayerArrow {
		ax := seg[0]*ca - seg[1]*sa
		ay := seg[0]*sa + seg[1]*ca
		bx := seg[2]*ca - seg[3]*sa
		by := seg[2]*sa + seg[3]*ca
		x1, y1 := g.worldToScreen(px+ax, py+ay)
		x2, y2 := g.worldToScreen(px+bx, py+by)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, playerColor, true)
	}
}

func (g *game) drawPlayerArrowScreen(screen *ebiten.Image, sx, sy, ang float64) {
	ca := math.Cos(ang)
	sa := math.Sin(ang)
	scale := g.zoom
	for _, seg := range doomPlayerArrow {
		ax := seg[0]*ca - seg[1]*sa
		ay := seg[0]*sa + seg[1]*ca
		bx := seg[2]*ca - seg[3]*sa
		by := seg[2]*sa + seg[3]*ca
		x1 := sx + ax*scale
		y1 := sy - ay*scale
		x2 := sx + bx*scale
		y2 := sy - by*scale
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, playerColor, true)
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.viewW = max(outsideWidth, 1)
	g.viewH = max(outsideHeight, 1)
	return g.viewW, g.viewH
}

func (g *game) worldToScreen(x, y float64) (float64, float64) {
	dx := x - g.camX
	dy := y - g.camY
	if g.rotateView {
		rot := (math.Pi / 2) - angleToRadians(g.p.angle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		rdx := dx*cr - dy*sr
		rdy := dx*sr + dy*cr
		dx = rdx
		dy = rdy
	}
	sx := dx*g.zoom + float64(g.viewW)/2
	sy := float64(g.viewH)/2 - dy*g.zoom
	return sx, sy
}

func (g *game) linedefDecision(ld mapdata.Linedef) lineDecision {
	front, back := g.lineSectors(ld)
	return parityLineDecision(ld, front, back, g.parity, g.opts.LineColorMode)
}

func (g *game) lineSectors(ld mapdata.Linedef) (*mapdata.Sector, *mapdata.Sector) {
	if ld.SideNum[0] < 0 || int(ld.SideNum[0]) >= len(g.m.Sidedefs) {
		return nil, nil
	}
	s0 := g.m.Sidedefs[int(ld.SideNum[0])].Sector
	if int(s0) >= len(g.m.Sectors) {
		return nil, nil
	}
	front := &g.m.Sectors[s0]
	if ld.SideNum[1] < 0 || int(ld.SideNum[1]) >= len(g.m.Sidedefs) {
		return front, nil
	}
	s1 := g.m.Sidedefs[int(ld.SideNum[1])].Sector
	if int(s1) >= len(g.m.Sectors) {
		return front, nil
	}
	return front, &g.m.Sectors[s1]
}

func (g *game) decisionStyle(d lineDecision) (color.Color, float64) {
	switch d.appearance {
	case lineAppearanceOneSided:
		return wallOneSided, d.width
	case lineAppearanceSecret:
		return wallSecret, d.width
	case lineAppearanceTeleporter:
		return wallTeleporter, d.width
	case lineAppearanceFloorChange:
		return wallFloorChange, d.width
	case lineAppearanceCeilChange:
		return wallCeilChange, d.width
	case lineAppearanceNoHeightDiff:
		return wallNoHeightDiff, d.width
	case lineAppearanceUnrevealed:
		return wallUnrevealed, d.width
	default:
		return wallNoHeightDiff, d.width
	}
}

func (g *game) visibleLineIndices() []int {
	margin := 2.0 / g.zoom
	viewHalfW := float64(g.viewW) / (2 * g.zoom)
	viewHalfH := float64(g.viewH) / (2 * g.zoom)
	minXf := g.camX - viewHalfW - margin
	maxXf := g.camX + viewHalfW + margin
	minYf := g.camY - viewHalfH - margin
	maxYf := g.camY + viewHalfH + margin
	if g.rotateView {
		// Conservative culling when rotating: circumscribed circle around the viewport.
		r := math.Hypot(viewHalfW, viewHalfH) + margin
		minXf = g.camX - r
		maxXf = g.camX + r
		minYf = g.camY - r
		maxYf = g.camY + r
	}
	minX := floatToFixed(minXf)
	maxX := floatToFixed(maxXf)
	minY := floatToFixed(minYf)
	maxY := floatToFixed(maxYf)

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
		// Safety border to avoid dropping lines that barely cross cell edges.
		for bx := bx0 - 1; bx <= bx1+1; bx++ {
			for by := by0 - 1; by <= by1+1; by++ {
				if bx < 0 || by < 0 || bx >= g.bmapWidth || by >= g.bmapHeight {
					continue
				}
				cellIdx := by*g.bmapWidth + bx
				if cellIdx < 0 || cellIdx >= len(g.m.BlockMap.Cells) {
					continue
				}
				cell := g.m.BlockMap.Cells[cellIdx]
				start := 0
				if len(cell) > 0 && cell[0] == 0 {
					start = 1
				}
				for _, lw := range cell[start:] {
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

func (g *game) drawHelpUI(screen *ebiten.Image) {
	helpHint := "F1 HELP"
	hintX := g.viewW - len(helpHint)*7 - 10
	if hintX < 10 {
		hintX = 10
	}
	ebitenutil.DebugPrintAt(screen, helpHint, hintX, 10)
	if !g.showHelp {
		return
	}
	lines := []string{
		"AUTOMAP KEYS",
		"F1  HELP TOGGLE",
		"TAB  WALK/MAP MODE",
		"R   FOLLOW HEADING",
		"M   TOGGLE NORMAL/ALLMAP",
		"I   CYCLE IDDT (0/1/2)",
		"WASD  MOVE",
		"MOUSE  TURN",
		"ARROWS  MOVE/TURN",
		"ALT+ARROWS  STRAFE",
		"SHIFT  RUN",
		"+/- OR WHEEL  ZOOM",
		"ESC  QUIT",
	}
	maxLen := 0
	for _, l := range lines {
		if len(l) > maxLen {
			maxLen = len(l)
		}
	}
	x := g.viewW - maxLen*7 - 14
	if x < 10 {
		x = 10
	}
	y := 28
	for i, l := range lines {
		ebitenutil.DebugPrintAt(screen, l, x, y+i*14)
	}
}
