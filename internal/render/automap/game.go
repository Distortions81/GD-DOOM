package automap

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	lineOneSidedWidth  = 1.8
	lineTwoSidedWidth  = 1.2
	doomInitialZoomMul = 1.0 / 0.7
	// Give cursor capture/resizing a couple of frames to settle after detail changes.
	detailMouseSuppressTicks = 3
	mlDontPegTop             = 1 << 3
	mlDontPegBottom          = 1 << 4
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
	wallUseSpecial   = color.RGBA{R: 255, G: 80, B: 170, A: 255}
	playerColor      = color.RGBA{R: 120, G: 240, B: 130, A: 255}
	otherPlayerColor = color.RGBA{R: 90, G: 170, B: 255, A: 255}
	useTargetColor   = color.RGBA{R: 255, G: 210, B: 70, A: 255}
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

var detailPresets = [][2]int{
	{320, 200},
	{640, 400},
	{960, 600},
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
	walkRender walkRendererMode
	followMode bool
	rotateView bool
	showHelp   bool
	pseudo3D   bool
	parity     automapParityState
	showGrid   bool
	showLegend bool
	bigMap     bool
	savedView  savedMapView
	marks      []mapMark
	nextMarkID int
	p          player
	localSlot  int
	peerStarts []playerStart

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
	turnHeld    int
	snd         *soundSystem
	soundQueue  []soundEvent
	delayedSfx  []delayedSoundEvent

	prevCamX  float64
	prevCamY  float64
	prevPX    int64
	prevPY    int64
	prevAngle uint32

	renderCamX  float64
	renderCamY  float64
	renderPX    float64
	renderPY    float64
	renderAngle uint32

	lastUpdate time.Time

	lastMouseX             int
	mouseLookSet           bool
	mouseLookSuppressTicks int

	levelExitRequested    bool
	secretLevelExit       bool
	levelRestartRequested bool

	thingCollected []bool
	thingHP        []int
	thingAggro     []bool
	thingCooldown  []int
	projectiles    []projectile
	cheatLevel     int
	invulnerable   bool
	inventory      playerInventory
	stats          playerStats
	worldTic       int
	isDead         bool
	damageFlashTic int
	bonusFlashTic  int
	subSectorSec   []int
	subSectorPoly  [][]worldPt
	subSectorTris  [][][3]int
	subSectorBBox  []worldBBox
	holeFillPolys  []holeFillPoly

	mapFloorLayer      *ebiten.Image
	mapFloorPix        []byte
	mapFloorW          int
	mapFloorH          int
	mapFloorWorldLayer *ebiten.Image
	mapFloorWorldInit  bool
	mapFloorWorldMinX  float64
	mapFloorWorldMaxY  float64
	mapFloorWorldStep  float64
	mapFloorWorldStats floorFrameStats
	mapFloorWorldState string
	wallLayer          *ebiten.Image
	wallPix            []byte
	wallW              int
	wallH              int
	flatImgCache       map[string]*ebiten.Image
	whitePixel         *ebiten.Image
	cullLogBudget      int
	floorDbgMode       floorDebugMode
	floor2DPath        floor2DPathMode
	floorVisDiag       floorVisDiagMode
	floorFrame         floorFrameStats
	floorClip          []int16
	ceilingClip        []int16
	floorPlanes        map[floorPlaneKey][]*floorVisplane
	floorPlaneOrd      []*floorVisplane
	floorSpans         []floorSpan
	detailLevel        int
	plane3DFrame       plane3DFrameStats
}

type savedMapView struct {
	camX   float64
	camY   float64
	zoom   float64
	follow bool
	valid  bool
}

type mapMark struct {
	id int
	x  float64
	y  float64
}

type delayedSoundEvent struct {
	ev   soundEvent
	tics int
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

type walkRendererMode int

const (
	walkRendererDoomBasic walkRendererMode = iota
	walkRendererPseudo
)

type floorDebugMode int

const (
	floorDebugTextured floorDebugMode = iota
	floorDebugSolid
	floorDebugUV
)

type floor2DPathMode int

const (
	floor2DPathLegacy floor2DPathMode = iota
	floor2DPathVisplane
)

type floorVisDiagMode int

const (
	floorVisDiagOff floorVisDiagMode = iota
	floorVisDiagClip
	floorVisDiagSpan
	floorVisDiagBoth
)

type floorFrameStats struct {
	markedCols       int
	emittedSpans     int
	rejectedSpan     int
	rejectNoSector   int
	rejectNoPoly     int
	rejectDegenerate int
	rejectSpanClip   int
}

type plane3DFrameStats struct {
	buckets      int
	inputSpans   int
	outputSpans  int
	clippedSpans int
	texturedPix  int
	fallbackPix  int
}

func newGame(m *mapdata.Map, opts Options) *game {
	if opts.Width <= 0 {
		opts.Width = 1280
	}
	if opts.Height <= 0 {
		opts.Height = 720
	}
	opts.SkillLevel = normalizeSkillLevel(opts.SkillLevel)
	if !opts.SourcePortMode {
		// Doom mode keeps strict parity color semantics.
		opts.LineColorMode = "parity"
	}
	if opts.PlayerSlot < 1 || opts.PlayerSlot > 4 {
		opts.PlayerSlot = 1
	}
	p, localSlot, starts := spawnPlayer(m, opts.PlayerSlot)
	g := &game{
		m:          m,
		opts:       opts,
		bounds:     mapBounds(m),
		viewW:      opts.Width,
		viewH:      opts.Height,
		mode:       viewMap,
		walkRender: walkRendererDoomBasic,
		followMode: true,
		rotateView: opts.SourcePortMode,
		pseudo3D:   false,
		parity: automapParityState{
			reveal: revealNormal,
			iddt:   0,
		},
		showGrid:      false,
		showLegend:    opts.SourcePortMode,
		bigMap:        false,
		marks:         make([]mapMark, 0, 16),
		nextMarkID:    1,
		p:             p,
		localSlot:     localSlot,
		peerStarts:    nonLocalStarts(starts, localSlot),
		cullLogBudget: 0,
		floorDbgMode:  floorDebugTextured,
		// Keep sourceport map textures on the stable legacy path by default.
		floor2DPath:  floor2DPathLegacy,
		floorVisDiag: floorVisDiagOff,
	}
	g.detailLevel = detailPresetIndex(g.viewW, g.viewH)
	g.initPlayerState()
	g.thingCollected = make([]bool, len(m.Things))
	g.thingHP = make([]int, len(m.Things))
	g.thingAggro = make([]bool, len(m.Things))
	g.thingCooldown = make([]int, len(m.Things))
	g.initThingCombatState()
	g.applySkillThingFiltering()
	g.cheatLevel = normalizeCheatLevel(opts.CheatLevel)
	g.invulnerable = opts.Invulnerable
	if !g.opts.StartInMapMode {
		g.mode = viewWalk
	}
	g.initPhysics()
	g.initSubSectorSectorCache()
	g.snd = newSoundSystem(opts.SoundBank)
	g.soundQueue = make([]soundEvent, 0, 8)
	g.delayedSfx = make([]delayedSoundEvent, 0, 8)
	if g.opts.SourcePortMode {
		// Source-port defaults: reveal full map style and heading-follow at startup.
		g.parity.reveal = revealAllMap
	}
	if g.opts.AllCheats {
		// Backward compatible legacy switch.
		if g.cheatLevel < 3 {
			g.cheatLevel = 3
		}
		g.invulnerable = true
	}
	g.applyCheatLevel(g.cheatLevel, false)
	if g.invulnerable {
		g.setHUDMessage("IDDQD ON", 70)
	}
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
	g.syncRenderState()
	if g.mode == viewWalk {
		// Avoid startup cursor-capture deltas rotating the initial spawn heading.
		g.mouseLookSet = false
		g.mouseLookSuppressTicks = detailMouseSuppressTicks
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
	g.zoom = g.fitZoom * doomInitialZoomMul
}

func detailPresetIndex(w, h int) int {
	for i, p := range detailPresets {
		if p[0] == w && p[1] == h {
			return i
		}
	}
	return 0
}

func (g *game) cycleDetailLevel() {
	if len(detailPresets) == 0 {
		return
	}
	g.detailLevel = (g.detailLevel + 1) % len(detailPresets)
	p := detailPresets[g.detailLevel]
	oldFit := g.fitZoom
	g.viewW = p[0]
	g.viewH = p[1]

	worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	margin := 0.9
	zx := float64(max(g.viewW, 1)) * margin / worldW
	zy := float64(max(g.viewH, 1)) * margin / worldH
	g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
	if oldFit > 0 {
		g.zoom = (g.zoom / oldFit) * g.fitZoom
	} else {
		g.zoom = g.fitZoom * doomInitialZoomMul
	}
	g.setHUDMessage(fmt.Sprintf("Detail: %dx%d", g.viewW, g.viewH), 70)
	// Avoid a large turn delta on the next walk-mode update after viewport size changes.
	g.mouseLookSet = false
	g.mouseLookSuppressTicks = detailMouseSuppressTicks
	// Keep interpolation state aligned to current state to prevent one-frame render pops.
	g.syncRenderState()
}

func (g *game) Update() error {
	g.capturePrevState()
	if g.levelExitRequested {
		return ebiten.Termination
	}
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		if g.mode == viewWalk {
			g.mode = viewMap
			g.setHUDMessage("Automap Opened", 35)
		} else {
			g.mode = viewWalk
			// Reset mouse baseline when entering walk mode to avoid turn spikes.
			g.mouseLookSet = false
			g.mouseLookSuppressTicks = detailMouseSuppressTicks
			g.setHUDMessage("Automap Closed", 35)
		}
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.rotateView = !g.rotateView
		if g.rotateView {
			g.setHUDMessage("Heading-Up ON", 70)
		} else {
			g.setHUDMessage("Heading-Up OFF", 70)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.showHelp = !g.showHelp
	}
	if !g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.cycleDetailLevel()
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.pseudo3D = !g.pseudo3D
		if g.pseudo3D {
			g.walkRender = walkRendererPseudo
			g.setHUDMessage("Wireframe Mode ON", 70)
		} else {
			g.walkRender = walkRendererDoomBasic
			g.setHUDMessage("Wireframe Mode OFF", 70)
		}
	}
	if g.isDead && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
		g.requestLevelRestart()
	}
	if g.mode == viewMap {
		if g.opts.SourcePortMode {
			ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
		g.updateMapMode()
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		g.updateWalkMode()
	}
	if g.useFlash > 0 {
		g.useFlash--
	}
	if g.damageFlashTic > 0 {
		g.damageFlashTic--
	}
	if g.bonusFlashTic > 0 {
		g.bonusFlashTic--
	}
	g.tickDelayedSounds()
	g.flushSoundEvents()
	g.lastUpdate = time.Now()
	return nil
}

func (g *game) requestLevelExit(secret bool, msg string) {
	g.levelExitRequested = true
	g.secretLevelExit = secret
	g.setHUDMessage(msg, 35)
}

func (g *game) requestLevelRestart() {
	g.levelRestartRequested = true
	g.setHUDMessage("Restarting level...", 20)
}

func (g *game) updateMapMode() {
	g.updateParityControls()
	g.updateWeaponHotkeys()
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.followMode = !g.followMode
		if g.followMode {
			g.setHUDMessage("Follow ON", 70)
		} else {
			g.setHUDMessage("Follow OFF", 70)
		}
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyB) {
		g.toggleBigMap()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key0) || inpututil.IsKeyJustPressed(ebiten.KeyKP0) {
		g.toggleBigMap()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.addMark()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.clearMarks()
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		g.resetView()
	}
	g.updateZoom()

	// Keep gameplay simulation active while automap is open.
	cmd := moveCmd{}
	speed := 0
	if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		speed = 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		cmd.forward += forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		cmd.forward -= forwardMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		cmd.side -= sideMove[speed]
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		cmd.side += sideMove[speed]
	}
	// Keep map panning on arrow keys; use Q/E turning in map mode.
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		cmd.turn += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		cmd.turn -= 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.handleUse()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyControlLeft) || inpututil.IsKeyJustPressed(ebiten.KeyControlRight) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.handleFire()
	}
	if g.opts.SourcePortMode {
		mx, _ := ebiten.CursorPosition()
		if g.mouseLookSuppressTicks > 0 {
			g.mouseLookSuppressTicks--
		} else if g.mouseLookSet {
			dx := mx - g.lastMouseX
			cmd.turnRaw -= int64(dx) * (40 << 16)
		}
		g.lastMouseX = mx
		g.mouseLookSet = true
	} else {
		g.mouseLookSet = false
	}
	cmd.run = speed == 1
	g.updatePlayer(cmd)
	g.discoverLinesAroundPlayer()

	if g.followMode {
		g.camX = float64(g.p.x) / fracUnit
		g.camY = float64(g.p.y) / fracUnit
		return
	}

	panStep := 14.0 / g.zoom
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.camY += panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.camY -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.camX -= panStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.camX += panStep
	}
}

func (g *game) updateWalkMode() {
	g.updateParityControls()
	g.updateWeaponHotkeys()
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
	if inpututil.IsKeyJustPressed(ebiten.KeyControlLeft) || inpututil.IsKeyJustPressed(ebiten.KeyControlRight) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.handleFire()
	}

	mx, _ := ebiten.CursorPosition()
	if g.mouseLookSuppressTicks > 0 {
		g.mouseLookSuppressTicks--
	} else if g.mouseLookSet {
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

func (g *game) updateWeaponHotkeys() {
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.selectWeaponSlot(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.selectWeaponSlot(2)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.selectWeaponSlot(3)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.selectWeaponSlot(4)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		g.selectWeaponSlot(5)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key6) {
		g.selectWeaponSlot(6)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key7) {
		g.selectWeaponSlot(7)
	}
}

func (g *game) updateParityControls() {
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.showGrid = !g.showGrid
		if g.showGrid {
			g.setHUDMessage("Grid ON", 70)
		} else {
			g.setHUDMessage("Grid OFF", 70)
		}
	}
	if g.opts.SourcePortMode {
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			if g.parity.reveal == revealNormal {
				g.parity.reveal = revealAllMap
				g.setHUDMessage("Allmap ON", 70)
			} else {
				g.parity.reveal = revealNormal
				g.setHUDMessage("Allmap OFF", 70)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			g.parity.iddt = (g.parity.iddt + 1) % 3
			g.setHUDMessage(fmt.Sprintf("IDDT %d", g.parity.iddt), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyL) {
			g.opts.LineColorMode = toggledLineColorMode(g.opts.LineColorMode)
			g.setHUDMessage(fmt.Sprintf("Line Colors: %s", g.opts.LineColorMode), 70)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyV) {
			g.showLegend = !g.showLegend
			if g.showLegend {
				g.setHUDMessage("Thing Legend ON", 70)
			} else {
				g.setHUDMessage("Thing Legend OFF", 70)
			}
		}
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
	if g.mode != viewMap {
		if g.walkRender == walkRendererPseudo {
			g.prepareRenderState()
			g.drawPseudo3D(screen)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("profile=%s", g.profileLabel()), 12, 12)
			ebitenutil.DebugPrintAt(screen, "renderer=wireframe | P toggle | TAB automap", 12, 28)
		} else {
			g.prepareRenderState()
			g.drawDoomBasic3D(screen)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("profile=%s", g.profileLabel()), 12, 28)
			if g.opts.SourcePortMode {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | P wireframe | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | J planes | P wireframe | F1 help", 12, 44)
			} else {
				ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | TAB automap", 12, 12)
				ebitenutil.DebugPrintAt(screen, "TAB automap | F5 detail | F1 help", 12, 44)
			}
			planes3DOn := len(g.opts.FlatBank) > 0
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("planes3d=%t flats=%d detail=%dx%d", planes3DOn, len(g.opts.FlatBank), g.viewW, g.viewH), 12, 60)
			if planes3DOn {
				ebitenutil.DebugPrintAt(screen, fmt.Sprintf("plane3d buckets=%d in=%d out=%d clip=%d texpx=%d fbpx=%d", g.plane3DFrame.buckets, g.plane3DFrame.inputSpans, g.plane3DFrame.outputSpans, g.plane3DFrame.clippedSpans, g.plane3DFrame.texturedPix, g.plane3DFrame.fallbackPix), 12, 76)
			}
		}
		if g.isDead {
			g.drawDeathOverlay(screen)
		}
		g.drawFlashOverlay(screen)
		if g.useFlash > 0 {
			ebitenutil.DebugPrintAt(screen, g.useText, 12, 44)
		}
		g.drawHelpUI(screen)
		return
	}
	g.prepareRenderState()
	if g.showGrid {
		g.drawGrid(screen)
	}

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
	if g.opts.SourcePortMode {
		g.drawUseSpecialLines(screen)
	}
	if g.opts.SourcePortMode {
		g.drawUseTargetHighlight(screen)
	}

	if shouldDrawThings(g.parity) {
		g.drawThings(screen)
	}
	g.drawMarks(screen)
	g.drawPlayer(screen)
	g.drawPeerPlayers(screen)

	modeText := "MAP"
	if g.mode == viewWalk {
		modeText = "WALK"
	}
	revealText := "normal"
	if g.parity.reveal == revealAllMap {
		revealText = "allmap"
	}
	if g.opts.SourcePortMode {
		overlay := fmt.Sprintf("map=%s mode=%s skill=%d zoom=%.2f reveal=%s iddt=%d grid=%t marks=%d colors=%s",
			g.m.Name,
			modeText,
			g.opts.SkillLevel,
			g.zoom,
			revealText,
			g.parity.iddt,
			g.showGrid,
			len(g.marks),
			g.opts.LineColorMode,
		)
		ebitenutil.DebugPrintAt(screen, overlay, 12, 12)
		stats := fmt.Sprintf("hp=%d ar=%d am=%d sh=%d ro=%d ce=%d keys=%s wp=%s",
			g.stats.Health,
			g.stats.Armor,
			g.stats.Bullets,
			g.stats.Shells,
			g.stats.Rockets,
			g.stats.Cells,
			g.inventory.keySummary(),
			weaponName(g.inventory.ReadyWeapon),
		)
		ebitenutil.DebugPrintAt(screen, stats, 12, 28)
		cheat := fmt.Sprintf("cheat=%d invuln=%t", g.cheatLevel, g.invulnerable)
		ebitenutil.DebugPrintAt(screen, cheat, 12, 60)
		if g.showLegend {
			g.drawThingLegend(screen)
		}
	}
	if g.useFlash > 0 {
		msgY := 12
		if g.opts.SourcePortMode {
			msgY = 44
		}
		ebitenutil.DebugPrintAt(screen, g.useText, 12, msgY)
	}
	g.drawHelpUI(screen)
	if g.isDead {
		g.drawDeathOverlay(screen)
	}
	g.drawFlashOverlay(screen)
}

func (g *game) profileLabel() string {
	if g.opts.SourcePortMode {
		return "sourceport"
	}
	return "doom"
}

func (g *game) emitSoundEvent(ev soundEvent) {
	g.soundQueue = append(g.soundQueue, ev)
}

func (g *game) emitSoundEventDelayed(ev soundEvent, tics int) {
	if tics <= 0 {
		g.emitSoundEvent(ev)
		return
	}
	g.delayedSfx = append(g.delayedSfx, delayedSoundEvent{ev: ev, tics: tics})
}

func (g *game) tickDelayedSounds() {
	if len(g.delayedSfx) == 0 {
		return
	}
	keep := g.delayedSfx[:0]
	for _, d := range g.delayedSfx {
		d.tics--
		if d.tics <= 0 {
			g.emitSoundEvent(d.ev)
			continue
		}
		keep = append(keep, d)
	}
	g.delayedSfx = keep
}

func (g *game) setHUDMessage(msg string, tics int) {
	g.useText = msg
	g.useFlash = tics
}

func (g *game) applySkillThingFiltering() {
	for i, th := range g.m.Things {
		if !thingSpawnsForSkill(th, g.opts.SkillLevel) {
			g.thingCollected[i] = true
		}
	}
}

func (g *game) flushSoundEvents() {
	if g.snd != nil {
		for _, ev := range g.soundQueue {
			g.snd.playEvent(ev)
		}
		g.snd.tick()
	}
	g.soundQueue = g.soundQueue[:0]
}

func shouldDrawThings(st automapParityState) bool {
	return st.iddt >= 2
}

func toggledLineColorMode(mode string) string {
	if mode == "parity" {
		return "doom"
	}
	return "parity"
}

func (g *game) drawThingLegend(screen *ebiten.Image) {
	type legendEntry struct {
		label string
		style thingStyle
	}
	entries := []legendEntry{
		{label: "player starts", style: thingStyle{glyph: thingGlyphSquare, clr: thingPlayerColor}},
		{label: "monsters", style: thingStyle{glyph: thingGlyphTriangle, clr: thingMonsterColor}},
		{label: "items/pickups", style: thingStyle{glyph: thingGlyphDiamond, clr: thingItemColor}},
		{label: "keys", style: thingStyle{glyph: thingGlyphStar, clr: thingKeyBlue}},
		{label: "misc", style: thingStyle{glyph: thingGlyphCross, clr: thingMiscColor}},
	}
	type lineLegendEntry struct {
		label string
		clr   color.Color
	}
	lineEntries := []lineLegendEntry{
		{label: "one-sided wall", clr: wallOneSided},
		{label: "floor delta", clr: wallFloorChange},
		{label: "ceiling delta", clr: wallCeilChange},
		{label: "teleporter", clr: wallTeleporter},
		{label: "use switch/button", clr: wallUseSpecial},
	}
	if g.opts.LineColorMode == "parity" {
		lineEntries = append(lineEntries, lineLegendEntry{label: "unrevealed (allmap)", clr: wallUnrevealed})
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
	x := g.viewW - maxLen*7 - 36
	if x < 10 {
		x = 10
	}
	y := 28
	ebitenutil.DebugPrintAt(screen, "THING LEGEND", x, y)
	for i, e := range entries {
		ly := y + 16 + i*14
		drawThingGlyph(screen, e.style, float64(x+8), float64(ly+5), 0, 4.6)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}

	ly0 := y + 16 + len(entries)*14 + 8
	ebitenutil.DebugPrintAt(screen, "LINE COLORS", x, ly0)
	for i, e := range lineEntries {
		ly := ly0 + 16 + i*14
		vector.StrokeLine(screen, float32(x+2), float32(ly+5), float32(x+14), float32(ly+5), 2.4, e.clr, true)
		ebitenutil.DebugPrintAt(screen, e.label, x+18, ly)
	}
}

func (g *game) addMark() {
	if len(g.marks) >= 10 {
		g.setHUDMessage("Marks Full", 70)
		return
	}
	id := g.nextMarkID
	g.marks = append(g.marks, mapMark{
		id: g.nextMarkID,
		x:  g.camX,
		y:  g.camY,
	})
	g.nextMarkID++
	g.setHUDMessage(fmt.Sprintf("Marked Spot %d", id), 70)
}

func (g *game) clearMarks() {
	g.marks = g.marks[:0]
	g.setHUDMessage("Marks Cleared", 70)
}

func (g *game) toggleBigMap() {
	if !g.bigMap {
		g.savedView = savedMapView{
			camX:   g.camX,
			camY:   g.camY,
			zoom:   g.zoom,
			follow: g.followMode,
			valid:  true,
		}
		g.bigMap = true
		g.followMode = false
		g.camX = (g.bounds.minX + g.bounds.maxX) / 2
		g.camY = (g.bounds.minY + g.bounds.maxY) / 2
		g.zoom = g.fitZoom
		g.setHUDMessage("Big Map ON", 70)
		return
	}
	g.bigMap = false
	if g.savedView.valid {
		g.camX = g.savedView.camX
		g.camY = g.savedView.camY
		g.zoom = g.savedView.zoom
		g.followMode = g.savedView.follow
	}
	g.setHUDMessage("Big Map OFF", 70)
}

func (g *game) drawGrid(screen *ebiten.Image) {
	const cell = 128.0
	left := g.renderCamX - float64(g.viewW)/(2*g.zoom)
	right := g.renderCamX + float64(g.viewW)/(2*g.zoom)
	bottom := g.renderCamY - float64(g.viewH)/(2*g.zoom)
	top := g.renderCamY + float64(g.viewH)/(2*g.zoom)
	grid := color.RGBA{R: 40, G: 50, B: 60, A: 255}

	startX := math.Floor(left/cell) * cell
	for x := startX; x <= right; x += cell {
		x1, y1 := g.worldToScreen(x, bottom)
		x2, y2 := g.worldToScreen(x, top)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, true)
	}
	startY := math.Floor(bottom/cell) * cell
	for y := startY; y <= top; y += cell {
		x1, y1 := g.worldToScreen(left, y)
		x2, y2 := g.worldToScreen(right, y)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 1, grid, true)
	}
}

func (g *game) drawThings(screen *ebiten.Image) {
	for i, th := range g.m.Things {
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		x := float64(th.X)
		y := float64(th.Y)
		sx, sy := g.worldToScreen(x, y)
		size := thingGlyphSize(g.zoom)
		angle := worldThingAngle(th.Angle)
		if g.rotateView {
			angle = relativeThingAngle(th.Angle, g.renderAngle)
		}
		drawThingGlyph(screen, styleForThing(th), sx, sy, angle, size)
	}
}

func thingGlyphSize(zoom float64) float64 {
	// Doom-like behavior: thing markers scale with map zoom (map-space vectors).
	const worldHalfUnits = 16.0
	s := worldHalfUnits * zoom
	if s < 1.5 {
		return 1.5
	}
	if s > 40 {
		return 40
	}
	return s
}

func (g *game) drawMarks(screen *ebiten.Image) {
	mc := color.RGBA{R: 120, G: 210, B: 255, A: 255}
	for _, mk := range g.marks {
		sx, sy := g.worldToScreen(mk.x, mk.y)
		r := 5.0
		vector.StrokeLine(screen, float32(sx-r), float32(sy-r), float32(sx+r), float32(sy+r), 1.3, mc, true)
		vector.StrokeLine(screen, float32(sx-r), float32(sy+r), float32(sx+r), float32(sy-r), 1.3, mc, true)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", mk.id), int(sx)+6, int(sy)-6)
	}
}

func (g *game) drawPlayer(screen *ebiten.Image) {
	px := g.renderPX
	py := g.renderPY
	sx, sy := g.worldToScreen(px, py)
	if g.rotateView {
		// Heading-follow: keep icon fixed-up in screen-space.
		g.drawPlayerArrowScreen(screen, sx, sy, math.Pi/2)
		return
	}
	ang := angleToRadians(g.renderAngle)
	g.drawPlayerArrowWorld(screen, px, py, ang, playerColor)
}

func (g *game) drawPlayerArrowWorld(screen *ebiten.Image, px, py, ang float64, clr color.Color) {
	ca := math.Cos(ang)
	sa := math.Sin(ang)
	for _, seg := range doomPlayerArrow {
		ax := seg[0]*ca - seg[1]*sa
		ay := seg[0]*sa + seg[1]*ca
		bx := seg[2]*ca - seg[3]*sa
		by := seg[2]*sa + seg[3]*ca
		x1, y1 := g.worldToScreen(px+ax, py+ay)
		x2, y2 := g.worldToScreen(px+bx, py+by)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2, clr, true)
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

func (g *game) drawPeerPlayers(screen *ebiten.Image) {
	if len(g.peerStarts) == 0 {
		return
	}
	for _, ps := range g.peerStarts {
		px := float64(ps.x) / fracUnit
		py := float64(ps.y) / fracUnit
		ang := angleToRadians(ps.angle)
		g.drawPlayerArrowWorld(screen, px, py, ang, otherPlayerColor)
	}
}

func (g *game) drawUseTargetHighlight(screen *ebiten.Image) {
	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 || lineIdx >= len(g.physForLine) {
		return
	}
	pi := g.physForLine[lineIdx]
	if pi < 0 || pi >= len(g.lines) {
		return
	}
	pl := g.lines[pi]
	x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
	x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
	vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 3.0, useTargetColor, true)
}

func (g *game) drawDoomBasic3D(screen *ebiten.Image) {
	g.plane3DFrame = plane3DFrameStats{}
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := float64(g.p.z)/fracUnit + 41.0
	focal := float64(g.viewW) * 0.75
	near := 2.0

	ceilClr, floorClr := g.basicPlaneColors()
	ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH)/2, ceilClr)
	ebitenutil.DrawRect(screen, 0, float64(g.viewH)/2, float64(g.viewW), float64(g.viewH)/2, floorClr)
	g.ensureWallLayer()
	for i := 0; i < len(g.wallPix); i += 4 {
		g.wallPix[i+0] = 0
		g.wallPix[i+1] = 0
		g.wallPix[i+2] = 0
		g.wallPix[i+3] = 0
	}

	depthPix := make([]float64, g.viewW*g.viewH)
	for i := range depthPix {
		depthPix[i] = math.Inf(1)
	}
	wallTop := make([]int, g.viewW)
	wallBottom := make([]int, g.viewW)
	for i := 0; i < g.viewW; i++ {
		wallTop[i] = g.viewH
		wallBottom[i] = -1
	}
	planesEnabled := len(g.opts.FlatBank) > 0
	ceilingClip := make([]int, g.viewW)
	floorClip := make([]int, g.viewW)
	for i := 0; i < g.viewW; i++ {
		ceilingClip[i] = -1
		floorClip[i] = g.viewH
	}
	planeVis := make(map[plane3DKey][]*plane3DVisplane, 64)
	planeOrder := make([]*plane3DVisplane, 0, 64)
	solid := make([]solidSpan, 0, 16)
	for _, si := range g.visibleSegIndicesPseudo3D() {
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		seg := g.m.Segs[si]
		li := int(seg.Linedef)
		if li < 0 || li >= len(g.m.Linedefs) {
			continue
		}
		ld := g.m.Linedefs[li]
		d := g.linedefDecisionPseudo3D(ld)
		if !d.visible {
			continue
		}
		x1w, y1w, x2w, y2w, ok := g.segWorldEndpoints(si)
		if !ok {
			continue
		}
		frontSide := int(seg.Direction)
		if frontSide < 0 || frontSide > 1 {
			frontSide = 0
		}
		var frontSideDef *mapdata.Sidedef
		if sn := ld.SideNum[frontSide]; sn >= 0 && int(sn) < len(g.m.Sidedefs) {
			frontSideDef = &g.m.Sidedefs[int(sn)]
		}
		segLen := math.Hypot(x2w-x1w, y2w-y1w)
		u1 := float64(seg.Offset)
		if frontSideDef != nil {
			u1 += float64(frontSideDef.TextureOffset)
		}
		u2 := u1 + segLen
		if frontSide == 1 {
			u2 = u1 - segLen
		}

		x1 := x1w - camX
		y1 := y1w - camY
		x2 := x2w - camX
		y2 := y2w - camY
		f1 := x1*ca + y1*sa
		s1 := -x1*sa + y1*ca
		f2 := x2*ca + y2*sa
		s2 := -x2*sa + y2*ca
		origF1, origS1, origF2, origS2 := f1, s1, f2, s2
		preSX1 := float64(g.viewW) / 2
		preSX2 := float64(g.viewW) / 2
		if math.Abs(origF1) > 1e-9 {
			preSX1 -= (origS1 / origF1) * focal
		}
		if math.Abs(origF2) > 1e-9 {
			preSX2 -= (origS2 / origF2) * focal
		}
		f1, s1, u1, f2, s2, u2, ok = clipSegmentToNearWithAttr(f1, s1, u1, f2, s2, u2, near)
		if !ok {
			g.logWallCull(si, "BEHIND", origF1, origF2, preSX1, preSX2)
			continue
		}
		// Backface cull after near clipping for stable edge behavior.
		if f1*s2-s1*f2 >= 0 {
			g.logWallCull(si, "BACKFACE", f1, f2, s1, s2)
			continue
		}
		sx1 := float64(g.viewW)/2 - (s1/f1)*focal
		sx2 := float64(g.viewW)/2 - (s2/f2)*focal
		if !isFinite(sx1) || !isFinite(sx2) {
			g.logWallCull(si, "FLIPPED", f1, f2, sx1, sx2)
			continue
		}
		minSX := int(math.Floor(math.Min(sx1, sx2)))
		maxSX := int(math.Ceil(math.Max(sx1, sx2)))
		if minSX < 0 {
			minSX = 0
		}
		if maxSX >= g.viewW {
			maxSX = g.viewW - 1
		}
		if minSX > maxSX {
			g.logWallCull(si, "OFFSCREEN", f1, f2, sx1, sx2)
			continue
		}
		// Doom-style solid column clipping: skip segs fully covered by prior solid walls.
		if solidFullyCovered(solid, minSX, maxSX) {
			g.logWallCull(si, "OCCLUDED", f1, f2, sx1, sx2)
			continue
		}

		base, _ := g.decisionStyle(d)
		baseRGBA := color.RGBAModel.Convert(base).(color.RGBA)
		front, back := g.segSectors(si)
		if front == nil {
			continue
		}
		worldTop := float64(front.CeilingHeight) - eyeZ
		worldBottom := float64(front.FloorHeight) - eyeZ
		worldHigh := worldTop
		worldLow := worldBottom
		topWall := false
		bottomWall := false
		markCeiling := true
		markFloor := true
		solidWall := back == nil

		if back != nil {
			worldHigh = float64(back.CeilingHeight) - eyeZ
			worldLow = float64(back.FloorHeight) - eyeZ
			if isSkyFlatName(front.CeilingPic) && isSkyFlatName(back.CeilingPic) {
				// Doom sky hack: keep upper portal open when both sides are sky.
				worldTop = worldHigh
			}
			markFloor = worldLow != worldBottom ||
				normalizeFlatName(back.FloorPic) != normalizeFlatName(front.FloorPic) ||
				back.Light != front.Light
			markCeiling = worldHigh != worldTop ||
				normalizeFlatName(back.CeilingPic) != normalizeFlatName(front.CeilingPic) ||
				back.Light != front.Light
			if back.CeilingHeight <= front.FloorHeight || back.FloorHeight >= front.CeilingHeight {
				markFloor = true
				markCeiling = true
				solidWall = true
			}
			topWall = worldHigh < worldTop
			bottomWall = worldLow > worldBottom
		}
		if float64(front.FloorHeight) >= eyeZ {
			markFloor = false
		}
		if float64(front.CeilingHeight) <= eyeZ && !isSkyFlatName(front.CeilingPic) {
			markCeiling = false
		}
		invF1 := 1.0 / f1
		invF2 := 1.0 / f2
		uOverF1 := u1 * invF1
		uOverF2 := u2 * invF2
		var midTex WallTexture
		var topTex WallTexture
		var botTex WallTexture
		hasMidTex := false
		hasTopTex := false
		hasBotTex := false
		midTexMid := 0.0
		topTexMid := 0.0
		botTexMid := 0.0
		if frontSideDef != nil {
			rowOffset := float64(frontSideDef.RowOffset)
			if solidWall {
				midTex, hasMidTex = g.wallTexture(frontSideDef.Mid)
				if hasMidTex {
					if (ld.Flags & mlDontPegBottom) != 0 {
						midTexMid = float64(front.FloorHeight) + float64(midTex.Height) - eyeZ
					} else {
						midTexMid = float64(front.CeilingHeight) - eyeZ
					}
					midTexMid += rowOffset
				}
			} else {
				if topWall {
					topTex, hasTopTex = g.wallTexture(frontSideDef.Top)
					if hasTopTex {
						if (ld.Flags & mlDontPegTop) != 0 {
							topTexMid = float64(front.CeilingHeight) - eyeZ
						} else if back != nil {
							topTexMid = float64(back.CeilingHeight) + float64(topTex.Height) - eyeZ
						} else {
							topTexMid = float64(front.CeilingHeight) - eyeZ
						}
						topTexMid += rowOffset
					}
				}
				if bottomWall {
					botTex, hasBotTex = g.wallTexture(frontSideDef.Bottom)
					if hasBotTex {
						if (ld.Flags & mlDontPegBottom) != 0 {
							botTexMid = float64(front.CeilingHeight) - eyeZ
						} else if back != nil {
							botTexMid = float64(back.FloorHeight) - eyeZ
						} else {
							botTexMid = float64(front.FloorHeight) - eyeZ
						}
						botTexMid += rowOffset
					}
				}
			}
		}

		var floorPlane *plane3DVisplane
		var ceilPlane *plane3DVisplane
		if planesEnabled {
			var created bool
			floorPlane, created = ensurePlane3DForRange(planeVis, g.plane3DKeyForSector(front, true), minSX, maxSX, g.viewW)
			if created && floorPlane != nil {
				planeOrder = append(planeOrder, floorPlane)
			}
			ceilPlane, created = ensurePlane3DForRange(planeVis, g.plane3DKeyForSector(front, false), minSX, maxSX, g.viewW)
			if created && ceilPlane != nil {
				planeOrder = append(planeOrder, ceilPlane)
			}
		}

		for x := minSX; x <= maxSX; x++ {
			t := (float64(x) - sx1) / (sx2 - sx1)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			invF := invF1 + (invF2-invF1)*t
			if invF <= 0 {
				continue
			}
			f := 1.0 / invF
			if f <= 0 {
				continue
			}
			texU := (uOverF1 + (uOverF2-uOverF1)*t) * f

			yl := int(math.Ceil(float64(g.viewH)/2 - (worldTop/f)*focal))
			if yl < ceilingClip[x]+1 {
				yl = ceilingClip[x] + 1
			}
			if markCeiling && planesEnabled && ceilPlane != nil {
				top := ceilingClip[x] + 1
				bottom := yl - 1
				if bottom >= floorClip[x] {
					bottom = floorClip[x] - 1
				}
				markPlane3DColumnRange(ceilPlane, x, top, bottom, ceilingClip, floorClip)
			}

			yh := int(math.Floor(float64(g.viewH)/2 - (worldBottom/f)*focal))
			if yh >= floorClip[x] {
				yh = floorClip[x] - 1
			}
			if markFloor && planesEnabled && floorPlane != nil {
				top := yh + 1
				bottom := floorClip[x] - 1
				if top <= ceilingClip[x] {
					top = ceilingClip[x] + 1
				}
				markPlane3DColumnRange(floorPlane, x, top, bottom, ceilingClip, floorClip)
			}

			if solidWall {
				g.drawBasicWallColumn(depthPix, wallTop, wallBottom, x, yl, yh, f, baseRGBA, texU, midTexMid, focal, midTex, hasMidTex)
				ceilingClip[x] = g.viewH
				floorClip[x] = -1
				continue
			}

			if topWall {
				mid := int(math.Floor(float64(g.viewH)/2 - (worldHigh/f)*focal))
				if mid >= floorClip[x] {
					mid = floorClip[x] - 1
				}
				if mid >= yl {
					g.drawBasicWallColumn(depthPix, wallTop, wallBottom, x, yl, mid, f, baseRGBA, texU, topTexMid, focal, topTex, hasTopTex)
					ceilingClip[x] = mid
				} else {
					ceilingClip[x] = yl - 1
				}
			} else if markCeiling {
				ceilingClip[x] = yl - 1
			}

			if bottomWall {
				mid := int(math.Ceil(float64(g.viewH)/2 - (worldLow/f)*focal))
				if mid <= ceilingClip[x] {
					mid = ceilingClip[x] + 1
				}
				if mid <= yh {
					g.drawBasicWallColumn(depthPix, wallTop, wallBottom, x, mid, yh, f, baseRGBA, texU, botTexMid, focal, botTex, hasBotTex)
					floorClip[x] = mid
				} else {
					floorClip[x] = yh + 1
				}
			} else if markFloor {
				floorClip[x] = yh + 1
			}
		}

		if solidWall {
			solid = addSolidSpan(solid, minSX, maxSX)
		}
	}
	g.wallLayer.WritePixels(g.wallPix)
	screen.DrawImage(g.wallLayer, nil)
	if planesEnabled {
		g.drawDoomBasicTexturedPlanesVisplanePass(screen, camX, camY, ca, sa, eyeZ, focal, ceilClr, floorClr, planeOrder)
	}
}

func (g *game) plane3DKeyForSector(sec *mapdata.Sector, floor bool) plane3DKey {
	key := plane3DKey{
		light:    160,
		fallback: true,
		floor:    floor,
	}
	if sec == nil {
		return key
	}
	key.light = sec.Light
	pic := sec.CeilingPic
	key.height = sec.CeilingHeight
	if floor {
		pic = sec.FloorPic
		key.height = sec.FloorHeight
	}
	if !floor && isSkyFlatName(pic) {
		key.sky = true
		key.height = 0
		key.light = 0
		key.flat = "SKY"
		key.fallback = true
		return key
	}
	key.flat = normalizeFlatName(pic)
	key.fallback = len(g.opts.FlatBank[key.flat]) != 64*64*4
	return key
}

func (g *game) drawBasicWallColumn(depthPix []float64, wallTop, wallBottom []int, x, y0, y1 int, depth float64, base color.RGBA, texU, texMid, focal float64, tex WallTexture, useTex bool) {
	if x < 0 || x >= g.viewW || y0 > y1 {
		return
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= g.viewH {
		y1 = g.viewH - 1
	}
	if y0 > y1 {
		return
	}
	sf := shadeFactorByDistance(depth)
	baseR := uint8(float64(base.R) * sf)
	baseG := uint8(float64(base.G) * sf)
	baseB := uint8(float64(base.B) * sf)
	cy := float64(g.viewH) * 0.5
	for y := y0; y <= y1; y++ {
		pi := y*g.viewW + x
		if depth < depthPix[pi] {
			depthPix[pi] = depth
			if y < wallTop[x] {
				wallTop[x] = y
			}
			if y > wallBottom[x] {
				wallBottom[x] = y
			}
			i := pi * 4
			if useTex {
				// Doom wall columns use a view-relative texture mid and per-column scale.
				zRel := (cy - (float64(y) + 0.5)) * depth / focal
				texV := texMid - zRel
				tx := wrapIndex(int(math.Floor(texU)), tex.Width)
				ty := wrapIndex(int(math.Floor(texV)), tex.Height)
				ti := (ty*tex.Width + tx) * 4
				g.wallPix[i+0] = uint8(float64(tex.RGBA[ti+0]) * sf)
				g.wallPix[i+1] = uint8(float64(tex.RGBA[ti+1]) * sf)
				g.wallPix[i+2] = uint8(float64(tex.RGBA[ti+2]) * sf)
				g.wallPix[i+3] = 255
			} else {
				g.wallPix[i+0] = baseR
				g.wallPix[i+1] = baseG
				g.wallPix[i+2] = baseB
				g.wallPix[i+3] = 255
			}
		}
	}
}

func (g *game) drawDoomBasicTexturedPlanesVisplanePass(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, ceilFallback, floorFallback color.RGBA, planes []*plane3DVisplane) {
	if len(planes) == 0 {
		return
	}
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}
	spansByPlane := make([][]plane3DSpan, len(planes))
	active := 0
	for i, pl := range planes {
		spans := makePlane3DSpans(pl, h, nil)
		if len(spans) == 0 {
			continue
		}
		spansByPlane[i] = spans
		g.plane3DFrame.inputSpans += len(spans)
		active++
	}
	g.plane3DFrame.buckets = active
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	flatCache := make(map[string][]byte, len(planes))
	for i, pl := range planes {
		spans := spansByPlane[i]
		if len(spans) == 0 {
			continue
		}
		key := pl.key
		fb := ceilFallback
		if key.floor {
			fb = floorFallback
		}
		tex := flatCache[key.flat]
		if !key.fallback && tex == nil {
			tex = g.opts.FlatBank[key.flat]
			flatCache[key.flat] = tex
		}
		for _, sp := range spans {
			if sp.y < 0 || sp.y >= h {
				continue
			}
			x1 := sp.x1
			x2 := sp.x2
			if x1 < 0 {
				x1 = 0
			}
			if x2 >= w {
				x2 = w - 1
			}
			if x2 < x1 {
				continue
			}
			row := sp.y * w * 4
			g.plane3DFrame.outputSpans++
			if key.sky {
				for x := x1; x <= x2; x++ {
					i := row + x*4
					pix[i+0] = fb.R
					pix[i+1] = fb.G
					pix[i+2] = fb.B
					pix[i+3] = 255
					g.plane3DFrame.fallbackPix++
				}
				continue
			}
			den := cy - (float64(sp.y) + 0.5)
			if math.Abs(den) < 1e-6 {
				continue
			}
			planeZ := float64(key.height)
			depth := ((planeZ - eyeZ) / den) * focal
			if depth <= 0 {
				continue
			}
			wxSpan := camX + depth*ca - ((cx-(float64(x1)+0.5))*depth/focal)*sa
			wySpan := camY + depth*sa + ((cx-(float64(x1)+0.5))*depth/focal)*ca
			stepWX := (depth / focal) * sa
			stepWY := -(depth / focal) * ca
			for x := x1; x <= x2; x++ {
				i := row + x*4
				if key.fallback {
					pix[i+0] = fb.R
					pix[i+1] = fb.G
					pix[i+2] = fb.B
					pix[i+3] = 255
					g.plane3DFrame.fallbackPix++
				} else if len(tex) == 64*64*4 {
					u := int(math.Floor(wxSpan)) & 63
					v := int(math.Floor(wySpan)) & 63
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
					pix[i+3] = 255
					g.plane3DFrame.texturedPix++
				} else {
					pix[i+0] = fb.R
					pix[i+1] = fb.G
					pix[i+2] = fb.B
					pix[i+3] = 255
					g.plane3DFrame.fallbackPix++
				}
				wxSpan += stepWX
				wySpan += stepWY
			}
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) drawDoomBasicTexturedPlanesSpanPass(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, ceilFallback, floorFallback color.RGBA, wallTop, wallBottom []int) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseFloorZ := float64(g.m.Sectors[playerSec].FloorHeight)
	baseCeilZ := float64(g.m.Sectors[playerSec].CeilingHeight)
	flatCache := make(map[string][]byte, 64)
	spanBuckets := make(map[plane3DKey][]plane3DSpan, 64)
	keyOrder := make([]plane3DKey, 0, 64)

	for y := 0; y < h; y++ {
		den := cy - (float64(y) + 0.5)
		if math.Abs(den) < 1e-6 {
			continue
		}
		isFloor := float64(y) > cy
		planeZ := baseCeilZ
		if isFloor {
			planeZ = baseFloorZ
		}
		depth := ((planeZ - eyeZ) / den) * focal
		if depth <= 0 {
			continue
		}
		s := (cx - 0.5) * depth / focal
		wx := camX + depth*ca - s*sa
		wy := camY + depth*sa + s*ca
		stepWX := (depth / focal) * sa
		stepWY := -(depth / focal) * ca
		runStart := -1
		var runKey plane3DKey
		flushRun := func(x int) {
			if runStart >= 0 {
				keyOrder, spanBuckets = bucketSpanByKey(spanBuckets, keyOrder, y, runStart, x-1, runKey)
				runStart = -1
			}
		}
		for x := 0; x < w; x++ {
			if isFloor {
				if x >= 0 && x < len(wallBottom) && y <= wallBottom[x] {
					flushRun(x)
					wx += stepWX
					wy += stepWY
					continue
				}
			} else {
				if x >= 0 && x < len(wallTop) && y >= wallTop[x] {
					flushRun(x)
					wx += stepWX
					wy += stepWY
					continue
				}
			}
			pkey := plane3DKey{
				height:   int16(math.Round(planeZ)),
				light:    160,
				fallback: true,
				floor:    isFloor,
			}
			sec := g.sectorAt(int64(wx*fracUnit), int64(wy*fracUnit))
			if sec >= 0 && sec < len(g.m.Sectors) {
				pic := g.m.Sectors[sec].CeilingPic
				pkey.height = g.m.Sectors[sec].CeilingHeight
				pkey.light = g.m.Sectors[sec].Light
				if isFloor {
					pic = g.m.Sectors[sec].FloorPic
					pkey.height = g.m.Sectors[sec].FloorHeight
				}
				k := normalizeFlatName(pic)
				pkey.flat = k
				pkey.fallback = len(g.opts.FlatBank[k]) != 64*64*4
				if !isFloor && isSkyFlatName(pic) {
					pkey.sky = true
					pkey.fallback = true
				}
			}
			if runStart < 0 {
				runStart = x
				runKey = pkey
			} else if runKey != pkey {
				flushRun(x)
				runStart = x
				runKey = pkey
			}
			wx += stepWX
			wy += stepWY
		}
		flushRun(w)
	}
	sort.Slice(keyOrder, func(i, j int) bool {
		if keyOrder[i].floor != keyOrder[j].floor {
			return !keyOrder[i].floor
		}
		if keyOrder[i].sky != keyOrder[j].sky {
			return !keyOrder[i].sky
		}
		if keyOrder[i].height != keyOrder[j].height {
			return keyOrder[i].height < keyOrder[j].height
		}
		if keyOrder[i].light != keyOrder[j].light {
			return keyOrder[i].light < keyOrder[j].light
		}
		if keyOrder[i].flat != keyOrder[j].flat {
			return keyOrder[i].flat < keyOrder[j].flat
		}
		if keyOrder[i].fallback != keyOrder[j].fallback {
			return keyOrder[j].fallback
		}
		return false
	})
	g.plane3DFrame.buckets = len(keyOrder)
	coveredByRow := make([][]spanRange, h)
	for _, key := range keyOrder {
		fb := ceilFallback
		if key.floor {
			fb = floorFallback
		}
		tex := flatCache[key.flat]
		if !key.fallback && tex == nil {
			tex = g.opts.FlatBank[key.flat]
			flatCache[key.flat] = tex
		}
		for _, sp := range spanBuckets[key] {
			g.plane3DFrame.inputSpans++
			if sp.y < 0 || sp.y >= h {
				continue
			}
			if sp.x1 < 0 {
				sp.x1 = 0
			}
			if sp.x2 >= w {
				sp.x2 = w - 1
			}
			if sp.x2 < sp.x1 {
				continue
			}
			visible := clipRangeAgainstCovered(sp.x1, sp.x2, coveredByRow[sp.y])
			if len(visible) == 0 {
				g.plane3DFrame.clippedSpans++
				continue
			}
			if len(visible) > 1 {
				g.plane3DFrame.clippedSpans += len(visible) - 1
			}
			den := cy - (float64(sp.y) + 0.5)
			if math.Abs(den) < 1e-6 {
				continue
			}
			planeZ := float64(key.height)
			depth := ((planeZ - eyeZ) / den) * focal
			if depth <= 0 {
				continue
			}
			row := sp.y * w * 4
			stepWX := (depth / focal) * sa
			stepWY := -(depth / focal) * ca
			for _, vr := range visible {
				g.plane3DFrame.outputSpans++
				wxSpan := camX + depth*ca - ((cx-(float64(vr.l)+0.5))*depth/focal)*sa
				wySpan := camY + depth*sa + ((cx-(float64(vr.l)+0.5))*depth/focal)*ca
				for x := vr.l; x <= vr.r; x++ {
					i := row + x*4
					if key.fallback {
						pix[i+0] = fb.R
						pix[i+1] = fb.G
						pix[i+2] = fb.B
						pix[i+3] = 255
						g.plane3DFrame.fallbackPix++
					} else if len(tex) == 64*64*4 {
						u := int(math.Floor(wxSpan)) & 63
						v := int(math.Floor(wySpan)) & 63
						ti := (v*64 + u) * 4
						pix[i+0] = tex[ti+0]
						pix[i+1] = tex[ti+1]
						pix[i+2] = tex[ti+2]
						pix[i+3] = 255
						g.plane3DFrame.texturedPix++
					} else {
						pix[i+0] = fb.R
						pix[i+1] = fb.G
						pix[i+2] = fb.B
						pix[i+3] = 255
						g.plane3DFrame.fallbackPix++
					}
					wxSpan += stepWX
					wySpan += stepWY
				}
				coveredByRow[sp.y] = addCoveredRange(coveredByRow[sp.y], vr.l, vr.r)
			}
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) drawDoomBasicTexturedCeilingClipped(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, ceilFallback color.RGBA, wallTop []int, depthPix []float64) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseCeilZ := float64(g.m.Sectors[playerSec].CeilingHeight)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}

	for y := 0; y < h; y++ {
		if float64(y) >= cy {
			break
		}
		rowBase := y * w * 4

		for x := 0; x < w; x++ {
			i := rowBase + x*4
			stopY := int(cy)
			if x >= 0 && x < len(wallTop) && wallTop[x] < stopY {
				stopY = wallTop[x]
			}
			if y >= stopY {
				continue
			}
			wx, wy, depth, sec, ok := g.refinePlaneSampleAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, baseCeilZ, true)
			if !ok {
				continue
			}
			pi := y*g.viewW + x
			if pi < 0 || pi >= len(depthPix) || depth >= depthPix[pi] {
				continue
			}
			if sec >= 0 && sec < len(g.m.Sectors) {
				name := g.m.Sectors[sec].CeilingPic
				if isSkyFlatName(name) {
					pix[i+0] = ceilFallback.R
					pix[i+1] = ceilFallback.G
					pix[i+2] = ceilFallback.B
					pix[i+3] = 255
				} else if tex, ok := g.flatRGBA(name); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
					pix[i+3] = 255
				} else {
					pix[i+0] = ceilFallback.R
					pix[i+1] = ceilFallback.G
					pix[i+2] = ceilFallback.B
					pix[i+3] = 255
				}
			} else {
				pix[i+0] = ceilFallback.R
				pix[i+1] = ceilFallback.G
				pix[i+2] = ceilFallback.B
				pix[i+3] = 255
			}
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) drawDoomBasicTexturedFloorClipped(screen *ebiten.Image, camX, camY, ca, sa, eyeZ, focal float64, playerSec int, floorFallback color.RGBA, wallBottom []int, depthPix []float64) {
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	w := g.viewW
	h := g.viewH
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return
	}
	cx := float64(w) * 0.5
	cy := float64(h) * 0.5
	baseFloorZ := float64(g.m.Sectors[playerSec].FloorHeight)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}

	for y := 0; y < h; y++ {
		rowBase := y * w * 4
		if float64(y) <= cy {
			continue
		}
		for x := 0; x < w; x++ {
			i := rowBase + x*4
			startY := int(cy)
			if x >= 0 && x < len(wallBottom) && wallBottom[x]+1 > startY {
				startY = wallBottom[x] + 1
			}
			if y < startY {
				continue
			}
			wx, wy, depth, sec, ok := g.refinePlaneSampleAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, baseFloorZ, false)
			if !ok {
				continue
			}
			pi := y*g.viewW + x
			if pi < 0 || pi >= len(depthPix) || depth >= depthPix[pi] {
				continue
			}
			if sec >= 0 && sec < len(g.m.Sectors) {
				if tex, ok := g.flatRGBA(g.m.Sectors[sec].FloorPic); ok {
					u := int(math.Floor(wx)) & 63
					v := int(math.Floor(wy)) & 63
					ti := (v*64 + u) * 4
					pix[i+0] = tex[ti+0]
					pix[i+1] = tex[ti+1]
					pix[i+2] = tex[ti+2]
					pix[i+3] = 255
				} else {
					pix[i+0] = floorFallback.R
					pix[i+1] = floorFallback.G
					pix[i+2] = floorFallback.B
					pix[i+3] = 255
				}
			} else {
				pix[i+0] = floorFallback.R
				pix[i+1] = floorFallback.G
				pix[i+2] = floorFallback.B
				pix[i+3] = 255
			}
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func worldPointForPlaneAtPixel(x, y int, cx, cy, camX, camY, ca, sa, eyeZ, focal, planeZ float64) (wx, wy, depth float64, ok bool) {
	den := cy - (float64(y) + 0.5)
	if math.Abs(den) < 1e-6 {
		return 0, 0, 0, false
	}
	depth = ((planeZ - eyeZ) / den) * focal
	if depth <= 0 {
		return 0, 0, 0, false
	}
	s := (cx - (float64(x) + 0.5)) * depth / focal
	wx = camX + depth*ca - s*sa
	wy = camY + depth*sa + s*ca
	return wx, wy, depth, true
}

func (g *game) refinePlaneSampleAtPixel(x, y int, cx, cy, camX, camY, ca, sa, eyeZ, focal, initialZ float64, ceiling bool) (wx, wy, depth float64, sec int, ok bool) {
	planeZ := initialZ
	lastSec := -1
	for i := 0; i < 4; i++ {
		rwx, rwy, rd, rok := worldPointForPlaneAtPixel(x, y, cx, cy, camX, camY, ca, sa, eyeZ, focal, planeZ)
		if !rok {
			return 0, 0, 0, -1, false
		}
		rsec := g.sectorAt(int64(rwx*fracUnit), int64(rwy*fracUnit))
		if rsec < 0 || rsec >= len(g.m.Sectors) {
			return rwx, rwy, rd, rsec, true
		}
		nextZ := float64(g.m.Sectors[rsec].FloorHeight)
		if ceiling {
			nextZ = float64(g.m.Sectors[rsec].CeilingHeight)
		}
		wx, wy, depth, sec = rwx, rwy, rd, rsec
		if rsec == lastSec || math.Abs(nextZ-planeZ) < 0.001 {
			return wx, wy, depth, sec, true
		}
		lastSec = rsec
		planeZ = nextZ
	}
	return wx, wy, depth, sec, sec >= 0
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (g *game) logWallCull(segIdx int, reason string, z1, z2, x1, x2 float64) {
	if g.cullLogBudget <= 0 {
		return
	}
	g.cullLogBudget--
	fmt.Printf("wall-cull seg=%d reason=%s z1=%.4f z2=%.4f x1=%.2f x2=%.2f\n", segIdx, reason, z1, z2, x1, x2)
}

func clipSegmentToNear(f1, s1, f2, s2, near float64) (float64, float64, float64, float64, bool) {
	const eps = 1e-6
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, false
	}
	// Work from originals so we never interpolate from already-mutated values.
	of1, os1 := f1, s1
	of2, os2 := f2, s2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, false
	}
	return f1, s1, f2, s2, true
}

func clipSegmentToNearWithAttr(f1, s1, a1, f2, s2, a2, near float64) (float64, float64, float64, float64, float64, float64, bool) {
	const eps = 1e-6
	clipNear := near + eps
	if f1 <= near && f2 <= near {
		return 0, 0, 0, 0, 0, 0, false
	}
	of1, os1, oa1 := f1, s1, a1
	of2, os2, oa2 := f2, s2, a2
	if of1 < near {
		den := of2 - of1
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of1) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f1 = clipNear
		s1 = os1 + (os2-os1)*t
		a1 = oa1 + (oa2-oa1)*t
	}
	if of2 < near {
		den := of1 - of2
		if math.Abs(den) < 1e-9 {
			return 0, 0, 0, 0, 0, 0, false
		}
		t := (clipNear - of2) / den
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		f2 = clipNear
		s2 = os2 + (os1-os2)*t
		a2 = oa2 + (oa1-oa2)*t
	}
	if f1 < near || f2 < near {
		return 0, 0, 0, 0, 0, 0, false
	}
	return f1, s1, a1, f2, s2, a2, true
}

type solidSpan struct {
	l int
	r int
}

func solidFullyCovered(spans []solidSpan, l, r int) bool {
	if l > r {
		return true
	}
	cur := l
	for _, s := range spans {
		if s.r < cur {
			continue
		}
		if s.l > cur {
			return false
		}
		if s.r+1 > cur {
			cur = s.r + 1
		}
		if cur > r {
			return true
		}
	}
	return false
}

func addSolidSpan(spans []solidSpan, l, r int) []solidSpan {
	if l > r {
		return spans
	}
	ns := solidSpan{l: l, r: r}
	out := make([]solidSpan, 0, len(spans)+1)
	inserted := false
	for _, s := range spans {
		if s.r+1 < ns.l {
			out = append(out, s)
			continue
		}
		if ns.r+1 < s.l {
			if !inserted {
				out = append(out, ns)
				inserted = true
			}
			out = append(out, s)
			continue
		}
		if s.l < ns.l {
			ns.l = s.l
		}
		if s.r > ns.r {
			ns.r = s.r
		}
	}
	if !inserted {
		out = append(out, ns)
	}
	return out
}

func (g *game) drawBasicWallColumnRange(screen *ebiten.Image, depthPix []float64, wallTop, wallBottom []int, sx1, sx2, f1, f2, zTop, zBot, eyeZ, focal float64, base color.RGBA) {
	if zTop <= zBot {
		return
	}
	if math.Abs(sx2-sx1) < 0.001 {
		return
	}
	minX := int(math.Max(0, math.Floor(math.Min(sx1, sx2))))
	maxX := int(math.Min(float64(g.viewW-1), math.Ceil(math.Max(sx1, sx2))))
	if minX > maxX {
		return
	}
	for x := minX; x <= maxX; x++ {
		t := (float64(x) - sx1) / (sx2 - sx1)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		// Perspective-correct depth interpolation across screen columns.
		// In projected space, 1/z is linear with x (not z itself).
		invF1 := 1.0 / f1
		invF2 := 1.0 / f2
		invF := invF1 + (invF2-invF1)*t
		if invF <= 0 {
			continue
		}
		f := 1.0 / invF
		if f <= 0 {
			continue
		}
		yt := float64(g.viewH)/2 - ((zTop-eyeZ)/f)*focal
		yb := float64(g.viewH)/2 - ((zBot-eyeZ)/f)*focal
		if yb <= yt {
			continue
		}
		y0 := int(math.Max(0, math.Ceil(yt)))
		y1 := int(math.Min(float64(g.viewH-1), math.Floor(yb)))
		if y0 > y1 {
			continue
		}
		clr := shadeByDistance(base, f)
		runStart := -1
		for y := y0; y <= y1; y++ {
			pi := y*g.viewW + x
			if f < depthPix[pi] {
				depthPix[pi] = f
				if x >= 0 && x < len(wallTop) {
					if y < wallTop[x] {
						wallTop[x] = y
					}
					if y > wallBottom[x] {
						wallBottom[x] = y
					}
				}
				if runStart < 0 {
					runStart = y
				}
			} else if runStart >= 0 {
				ebitenutil.DrawRect(screen, float64(x), float64(runStart), 1, float64(y-runStart), clr)
				runStart = -1
			}
		}
		if runStart >= 0 {
			ebitenutil.DrawRect(screen, float64(x), float64(runStart), 1, float64(y1-runStart+1), clr)
		}
	}
}

func (g *game) basicPlaneColors() (color.RGBA, color.RGBA) {
	sec := g.sectorAt(g.p.x, g.p.y)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return color.RGBA{R: 24, G: 24, B: 30, A: 255}, color.RGBA{R: 28, G: 22, B: 18, A: 255}
	}
	s := g.m.Sectors[sec]
	ceilBase := uint8(36 + (int(s.CeilingHeight) & 31))
	floorBase := uint8(28 + (int(s.FloorHeight) & 31))
	return color.RGBA{R: ceilBase, G: ceilBase, B: ceilBase + 8, A: 255}, color.RGBA{R: floorBase + 10, G: floorBase + 4, B: floorBase, A: 255}
}

func shadeByDistance(c color.RGBA, dist float64) color.RGBA {
	n := dist / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	f := 1.0 - 0.72*n
	return color.RGBA{
		R: uint8(float64(c.R) * f),
		G: uint8(float64(c.G) * f),
		B: uint8(float64(c.B) * f),
		A: c.A,
	}
}

func (g *game) drawPseudo3D(screen *ebiten.Image) {
	ceiling := color.RGBA{R: 20, G: 24, B: 36, A: 255}
	floor := color.RGBA{R: 24, G: 18, B: 14, A: 255}
	ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH)/2, ceiling)
	ebitenutil.DrawRect(screen, 0, float64(g.viewH)/2, float64(g.viewW), float64(g.viewH)/2, floor)

	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := float64(g.p.z)/fracUnit + 41.0
	focal := float64(g.viewW) * 0.75
	near := 2.0

	for _, si := range g.visibleSegIndicesPseudo3D() {
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		seg := g.m.Segs[si]
		li := int(seg.Linedef)
		if li < 0 || li >= len(g.m.Linedefs) {
			continue
		}
		ld := g.m.Linedefs[li]
		d := g.linedefDecisionPseudo3D(ld)
		if !d.visible {
			continue
		}
		x1w, y1w, x2w, y2w, ok := g.segWorldEndpoints(si)
		if !ok {
			continue
		}

		x1 := x1w - camX
		y1 := y1w - camY
		x2 := x2w - camX
		y2 := y2w - camY

		f1 := x1*ca + y1*sa
		s1 := -x1*sa + y1*ca
		f2 := x2*ca + y2*sa
		s2 := -x2*sa + y2*ca
		f1, s1, f2, s2, ok = clipSegmentToNear(f1, s1, f2, s2, near)
		if !ok {
			continue
		}
		// Backface cull after near clipping for stable edge behavior.
		if f1*s2-s1*f2 >= 0 {
			continue
		}

		fsec, bsec := g.segSectors(si)
		if fsec == nil {
			continue
		}
		topZ := float64(fsec.CeilingHeight)
		botZ := float64(fsec.FloorHeight)
		if bsec != nil {
			topZ = math.Max(topZ, float64(bsec.CeilingHeight))
			botZ = math.Min(botZ, float64(bsec.FloorHeight))
		}

		sx1 := float64(g.viewW)/2 - (s1/f1)*focal
		sx2 := float64(g.viewW)/2 - (s2/f2)*focal
		yt1 := float64(g.viewH)/2 - ((topZ-eyeZ)/f1)*focal
		yb1 := float64(g.viewH)/2 - ((botZ-eyeZ)/f1)*focal
		yt2 := float64(g.viewH)/2 - ((topZ-eyeZ)/f2)*focal
		yb2 := float64(g.viewH)/2 - ((botZ-eyeZ)/f2)*focal

		c, _ := g.decisionStyle(d)
		vector.StrokeLine(screen, float32(sx1), float32(yt1), float32(sx2), float32(yt2), 1.4, c, true)
		vector.StrokeLine(screen, float32(sx1), float32(yb1), float32(sx2), float32(yb2), 1.4, c, true)
		vector.StrokeLine(screen, float32(sx1), float32(yt1), float32(sx1), float32(yb1), 1.2, c, true)
		vector.StrokeLine(screen, float32(sx2), float32(yt2), float32(sx2), float32(yb2), 1.2, c, true)
	}
	g.drawPseudo3DProjectiles(screen, camX, camY, camAng, focal, near)
	g.drawPseudo3DMonsters(screen, camX, camY, camAng, focal, near)
}

func (g *game) drawPseudo3DProjectiles(screen *ebiten.Image, camX, camY, camAng, focal, near float64) {
	type projectedProjectile struct {
		dist  float64
		sx    float64
		sy    float64
		r     float64
		outer color.RGBA
		inner color.RGBA
	}
	if len(g.projectiles) == 0 {
		return
	}
	items := make([]projectedProjectile, 0, len(g.projectiles))
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := float64(g.p.z)/fracUnit + 41.0

	for _, p := range g.projectiles {
		px := float64(p.x)/fracUnit - camX
		py := float64(p.y)/fracUnit - camY
		f := px*ca + py*sa
		s := -px*sa + py*ca
		if f <= near {
			continue
		}
		// Coarse occlusion check against solid map geometry.
		if !g.monsterHasLOS(g.p.x, g.p.y, p.x, p.y) {
			continue
		}
		sx := float64(g.viewW)/2 - (s/f)*focal
		centerZ := float64(p.z+p.height/2) / fracUnit
		sy := float64(g.viewH)/2 - ((centerZ-eyeZ)/f)*focal
		r := (projectileViewRadius(p) / f) * focal
		if r < 1.2 {
			r = 1.2
		}
		xPad := r + 8
		yPad := r + 8
		if sx+xPad < 0 || sx-xPad > float64(g.viewW) || sy+yPad < 0 || sy-yPad > float64(g.viewH) {
			continue
		}
		cr := projectileColor(p.kind)
		items = append(items, projectedProjectile{
			dist:  f,
			sx:    sx,
			sy:    sy,
			r:     math.Min(48, r),
			outer: color.RGBA{R: cr[0], G: cr[1], B: 24, A: 255},
			inner: color.RGBA{R: 255, G: 236, B: 120, A: 232},
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
	for _, it := range items {
		drawCircleApprox(screen, it.sx, it.sy, it.r, it.outer)
		drawCircleApprox(screen, it.sx, it.sy, it.r*0.52, it.inner)
	}
}

func drawCircleApprox(screen *ebiten.Image, cx, cy, r float64, clr color.RGBA) {
	if r <= 1.2 {
		ebitenutil.DrawRect(screen, cx-1, cy-1, 2, 2, clr)
		return
	}
	const segs = 18
	prevX := cx + r
	prevY := cy
	for i := 1; i <= segs; i++ {
		a := (2 * math.Pi * float64(i)) / segs
		x := cx + math.Cos(a)*r
		y := cy + math.Sin(a)*r
		vector.StrokeLine(screen, float32(prevX), float32(prevY), float32(x), float32(y), 1.2, clr, true)
		prevX = x
		prevY = y
	}
}

func (g *game) drawPseudo3DMonsters(screen *ebiten.Image, camX, camY, camAng, focal, near float64) {
	type projectedMonster struct {
		dist float64
		sx   float64
		yt   float64
		yb   float64
		clr  color.RGBA
	}
	items := make([]projectedMonster, 0, 32)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := float64(g.p.z)/fracUnit + 41.0

	for i, th := range g.m.Things {
		if i < 0 || i >= len(g.thingCollected) || g.thingCollected[i] {
			continue
		}
		if !isMonster(th.Type) {
			continue
		}
		tx := float64(th.X) - camX
		ty := float64(th.Y) - camY
		f := tx*ca + ty*sa
		s := -tx*sa + ty*ca
		if f <= near {
			continue
		}
		// Skip monsters hidden behind solid geometry.
		if !g.monsterHasLOS(g.p.x, g.p.y, int64(th.X)<<fracBits, int64(th.Y)<<fracBits) {
			continue
		}

		sx := float64(g.viewW)/2 - (s/f)*focal
		floorZ := float64(g.thingFloorZ(int64(th.X)<<fracBits, int64(th.Y)<<fracBits) / fracUnit)
		monsterH := monsterRenderHeight(th.Type)
		yt := float64(g.viewH)/2 - ((floorZ+monsterH-eyeZ)/f)*focal
		yb := float64(g.viewH)/2 - ((floorZ-eyeZ)/f)*focal
		if yb <= yt {
			continue
		}
		h := yb - yt
		w := math.Max(6, math.Min(120, h*0.45))
		xPad := w/2 + 8
		if sx+xPad < 0 || sx-xPad > float64(g.viewW) {
			continue
		}
		clr := shadedMonsterColor(f, near)
		items = append(items, projectedMonster{
			dist: f,
			sx:   sx,
			yt:   yt,
			yb:   yb,
			clr:  clr,
		})
	}

	// Draw far-to-near.
	sort.Slice(items, func(i, j int) bool { return items[i].dist > items[j].dist })
	for _, it := range items {
		h := it.yb - it.yt
		w := math.Max(6, math.Min(120, h*0.45))
		lx := it.sx - w/2
		ty := it.yt
		// Body billboard.
		ebitenutil.DrawRect(screen, lx, ty+h*0.22, w, h*0.78, it.clr)
		// Head cap.
		headClr := brighten(it.clr, 18)
		ebitenutil.DrawRect(screen, lx+w*0.16, ty, w*0.68, h*0.26, headClr)
		// Eye slit.
		ebitenutil.DrawRect(screen, lx+w*0.26, ty+h*0.10, w*0.48, math.Max(1, h*0.03), color.RGBA{R: 20, G: 14, B: 14, A: 220})
		// Foot shadow/ground cue.
		ebitenutil.DrawRect(screen, lx-w*0.08, it.yb-math.Max(1, h*0.03), w*1.16, math.Max(1, h*0.03), color.RGBA{R: 30, G: 20, B: 20, A: 180})
	}
}

func monsterRenderHeight(typ int16) float64 {
	switch typ {
	case 3002:
		return 56
	case 3006:
		return 56
	case 3005:
		return 56
	case 3003:
		return 64
	case 16:
		return 110
	case 7:
		return 100
	default:
		return 56
	}
}

func shadedMonsterColor(dist, near float64) color.RGBA {
	// Distance fog-ish shading for pseudo-3D readability.
	n := (dist - near) / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	f := 1.0 - 0.65*n
	return color.RGBA{
		R: uint8(float64(thingMonsterColor.R) * f),
		G: uint8(float64(thingMonsterColor.G) * f),
		B: uint8(float64(thingMonsterColor.B) * f),
		A: 245,
	}
}

func brighten(c color.RGBA, add uint8) color.RGBA {
	return color.RGBA{
		R: uint8(min(255, int(c.R)+int(add))),
		G: uint8(min(255, int(c.G)+int(add))),
		B: uint8(min(255, int(c.B)+int(add))),
		A: c.A,
	}
}

func (g *game) drawUseSpecialLines(screen *ebiten.Image) {
	for _, li := range g.visibleLineIndices() {
		if li < 0 || li >= len(g.lineSpecial) || !buttonHighlightEligible(g.lineSpecial[li]) {
			continue
		}
		pi := g.physForLine[li]
		if pi < 0 || pi >= len(g.lines) {
			continue
		}
		ld := g.m.Linedefs[li]
		d := g.linedefDecision(ld)
		if !d.visible {
			continue
		}
		pl := g.lines[pi]
		x1, y1 := g.worldToScreen(float64(pl.x1)/fracUnit, float64(pl.y1)/fracUnit)
		x2, y2 := g.worldToScreen(float64(pl.x2)/fracUnit, float64(pl.y2)/fracUnit)
		vector.StrokeLine(screen, float32(x1), float32(y1), float32(x2), float32(y2), 2.4, wallUseSpecial, true)
	}
}

func buttonHighlightEligible(special uint16) bool {
	if special == 0 {
		return false
	}
	info := mapdata.LookupLineSpecial(special)
	return info.Trigger == mapdata.TriggerUse
}

func (g *game) drawDeathOverlay(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 25, G: 0, B: 0, A: 130})
	msg1 := "YOU DIED"
	msg2 := "PRESS ENTER TO RESTART"
	x1 := g.viewW/2 - len(msg1)*7/2
	x2 := g.viewW/2 - len(msg2)*7/2
	y := g.viewH / 2
	ebitenutil.DebugPrintAt(screen, msg1, x1, y)
	ebitenutil.DebugPrintAt(screen, msg2, x2, y+16)
}

func (g *game) drawFlashOverlay(screen *ebiten.Image) {
	if g.damageFlashTic > 0 {
		a := uint8(40 + min(120, g.damageFlashTic*8))
		ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 180, G: 20, B: 20, A: a})
	}
	if g.bonusFlashTic > 0 {
		a := uint8(20 + min(80, g.bonusFlashTic*6))
		ebitenutil.DrawRect(screen, 0, 0, float64(g.viewW), float64(g.viewH), color.RGBA{R: 210, G: 190, B: 80, A: a})
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.opts.SourcePortMode {
		w := max(outsideWidth, 1)
		h := max(outsideHeight, 1)
		if w != g.viewW || h != g.viewH {
			oldFit := g.fitZoom
			g.viewW = w
			g.viewH = h
			worldW := math.Max(g.bounds.maxX-g.bounds.minX, 1)
			worldH := math.Max(g.bounds.maxY-g.bounds.minY, 1)
			margin := 0.9
			zx := float64(g.viewW) * margin / worldW
			zy := float64(g.viewH) * margin / worldH
			g.fitZoom = math.Max(math.Min(zx, zy), 0.0001)
			if oldFit > 0 {
				g.zoom = (g.zoom / oldFit) * g.fitZoom
			} else {
				g.zoom = g.fitZoom * doomInitialZoomMul
			}
			g.mouseLookSet = false
			g.mouseLookSuppressTicks = detailMouseSuppressTicks
			g.syncRenderState()
		}
		return g.viewW, g.viewH
	}
	if g.viewW < 1 {
		g.viewW = 1
	}
	if g.viewH < 1 {
		g.viewH = 1
	}
	return g.viewW, g.viewH
}

func (g *game) worldToScreen(x, y float64) (float64, float64) {
	dx := x - g.renderCamX
	dy := y - g.renderCamY
	if g.rotateView {
		rot := (math.Pi / 2) - angleToRadians(g.renderAngle)
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

func (g *game) screenToWorld(sx, sy float64) (float64, float64) {
	dx := (sx - float64(g.viewW)/2) / g.zoom
	dy := (float64(g.viewH)/2 - sy) / g.zoom
	if g.rotateView {
		rot := (math.Pi / 2) - angleToRadians(g.renderAngle)
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		// Inverse of worldToScreen's rotation.
		wdx := dx*cr + dy*sr
		wdy := -dx*sr + dy*cr
		dx = wdx
		dy = wdy
	}
	return g.renderCamX + dx, g.renderCamY + dy
}

func (g *game) ensureMapFloorLayer() {
	need := g.viewW * g.viewH * 4
	if g.mapFloorLayer == nil || g.mapFloorW != g.viewW || g.mapFloorH != g.viewH || len(g.mapFloorPix) != need {
		g.mapFloorLayer = ebiten.NewImage(g.viewW, g.viewH)
		g.mapFloorPix = make([]byte, need)
		g.mapFloorW = g.viewW
		g.mapFloorH = g.viewH
	}
}

func (g *game) buildMapFloorWorldLayer() {
	g.mapFloorWorldInit = false
	g.mapFloorWorldLayer = nil
	g.mapFloorWorldStats = floorFrameStats{}
	g.mapFloorWorldMinX = 0
	g.mapFloorWorldMaxY = 0
	g.mapFloorWorldStep = 1
	g.mapFloorWorldState = "init"

	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 || len(g.opts.FlatBank) == 0 {
		g.mapFloorWorldState = "no-map-or-flats"
		g.mapFloorWorldInit = true
		return
	}
	if len(g.subSectorPoly) != len(g.m.SubSectors) || len(g.subSectorTris) != len(g.m.SubSectors) {
		g.mapFloorWorldState = "poly-cache-mismatch"
		g.mapFloorWorldInit = true
		return
	}

	minX := math.Floor(g.bounds.minX)
	maxX := math.Ceil(g.bounds.maxX)
	minY := math.Floor(g.bounds.minY)
	maxY := math.Ceil(g.bounds.maxY)
	if !isFinite(minX) || !isFinite(minY) || !isFinite(maxX) || !isFinite(maxY) {
		g.mapFloorWorldState = "bad-bounds"
		g.mapFloorWorldInit = true
		return
	}
	spanX := maxX - minX
	spanY := maxY - minY
	if spanX <= 0 || spanY <= 0 {
		g.mapFloorWorldState = "empty-bounds"
		g.mapFloorWorldInit = true
		return
	}

	step := 1.0
	w := int(math.Ceil(spanX / step))
	h := int(math.Ceil(spanY / step))
	const maxDim = 16384
	const maxPixels int64 = 120_000_000
	for (w > maxDim || h > maxDim || int64(w)*int64(h) > maxPixels) && step < 1024 {
		step *= 2
		w = int(math.Ceil(spanX / step))
		h = int(math.Ceil(spanY / step))
	}
	if w < 1 || h < 1 {
		g.mapFloorWorldState = "size-invalid"
		g.mapFloorWorldInit = true
		return
	}
	if w > maxDim || h > maxDim || int64(w)*int64(h) > maxPixels {
		g.mapFloorWorldState = fmt.Sprintf("size-too-large %dx%d step=%.0f", w, h, step)
		g.mapFloorWorldInit = true
		return
	}

	pix := make([]byte, w*h*4)
	const fallbackR = 90
	const fallbackG = 125
	const fallbackB = 160
	secTex := make([][]byte, len(g.m.Sectors))
	secTexLoaded := make([]bool, len(g.m.Sectors))
	stats := floorFrameStats{}

	paintPixel := func(idx int, wx, wy float64, tex []byte) {
		if len(tex) == 64*64*4 {
			u := int(math.Floor(wx)) & 63
			v := int(math.Floor(wy)) & 63
			ti := (v*64 + u) * 4
			pix[idx+0] = tex[ti+0]
			pix[idx+1] = tex[ti+1]
			pix[idx+2] = tex[ti+2]
			pix[idx+3] = 255
			return
		}
		pix[idx+0] = fallbackR
		pix[idx+1] = fallbackG
		pix[idx+2] = fallbackB
		pix[idx+3] = 255
	}

	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			stats.rejectedSpan++
			stats.rejectNoSector++
			continue
		}
		if !secTexLoaded[sec] {
			secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
			secTexLoaded[sec] = true
		}
		tex := secTex[sec]

		if !g.ensureSubSectorPolyAndTris(ss) {
			stats.rejectedSpan++
			stats.rejectNoPoly++
			continue
		}
		verts := g.subSectorPoly[ss]
		bbox := worldBBox{}
		hasBBox := false
		if ss < len(g.subSectorBBox) {
			b := g.subSectorBBox[ss]
			if isFinite(b.minX) && isFinite(b.minY) && isFinite(b.maxX) && isFinite(b.maxY) && b.minX <= b.maxX && b.minY <= b.maxY {
				bbox = b
				hasBBox = true
			}
		}

		tris := g.subSectorTris[ss]
		for _, tri := range tris {
			a := verts[tri[0]]
			b := verts[tri[1]]
			c := verts[tri[2]]

			ax := (a.x - minX) / step
			ay := (maxY - a.y) / step
			bx := (b.x - minX) / step
			by := (maxY - b.y) / step
			cx := (c.x - minX) / step
			cy := (maxY - c.y) / step
			area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if math.Abs(area) < 1e-9 {
				stats.rejectedSpan++
				stats.rejectDegenerate++
				continue
			}

			minPX := int(math.Floor(math.Min(ax, math.Min(bx, cx))))
			maxPX := int(math.Ceil(math.Max(ax, math.Max(bx, cx))))
			minPY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
			maxPY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
			if minPX < 0 {
				minPX = 0
			}
			if minPY < 0 {
				minPY = 0
			}
			if maxPX >= w {
				maxPX = w - 1
			}
			if maxPY >= h {
				maxPY = h - 1
			}
			if minPX > maxPX || minPY > maxPY {
				stats.rejectedSpan++
				stats.rejectSpanClip++
				continue
			}

			stats.emittedSpans++
			for y := minPY; y <= maxPY; y++ {
				py := float64(y) + 0.5
				row := y * w * 4
				for x := minPX; x <= maxPX; x++ {
					px := float64(x) + 0.5
					w0 := (cx-bx)*(py-by) - (cy-by)*(px-bx)
					w1 := (ax-cx)*(py-cy) - (ay-cy)*(px-cx)
					w2 := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
					if area > 0 {
						if w0 < -1e-6 || w1 < -1e-6 || w2 < -1e-6 {
							continue
						}
					} else {
						if w0 > 1e-6 || w1 > 1e-6 || w2 > 1e-6 {
							continue
						}
					}
					wx := minX + (float64(x)+0.5)*step
					wy := maxY - (float64(y)+0.5)*step
					if hasBBox && (wx < bbox.minX || wx > bbox.maxX || wy < bbox.minY || wy > bbox.maxY) {
						continue
					}
					idx := row + x*4
					paintPixel(idx, wx, wy, tex)
					stats.markedCols++
				}
			}
		}
	}

	for _, hp := range g.holeFillPolys {
		sec := hp.sector
		if sec < 0 || sec >= len(g.m.Sectors) {
			stats.rejectedSpan++
			stats.rejectNoSector++
			continue
		}
		if len(hp.verts) < 3 || len(hp.tris) == 0 {
			stats.rejectedSpan++
			stats.rejectDegenerate++
			continue
		}
		if !secTexLoaded[sec] {
			secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
			secTexLoaded[sec] = true
		}
		tex := secTex[sec]
		verts := hp.verts
		bbox := hp.bbox
		hasBBox := isFinite(bbox.minX) && isFinite(bbox.minY) && isFinite(bbox.maxX) && isFinite(bbox.maxY) && bbox.minX <= bbox.maxX && bbox.minY <= bbox.maxY

		for _, tri := range hp.tris {
			a := verts[tri[0]]
			b := verts[tri[1]]
			c := verts[tri[2]]

			ax := (a.x - minX) / step
			ay := (maxY - a.y) / step
			bx := (b.x - minX) / step
			by := (maxY - b.y) / step
			cx := (c.x - minX) / step
			cy := (maxY - c.y) / step
			area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if math.Abs(area) < 1e-9 {
				stats.rejectedSpan++
				stats.rejectDegenerate++
				continue
			}

			minPX := int(math.Floor(math.Min(ax, math.Min(bx, cx))))
			maxPX := int(math.Ceil(math.Max(ax, math.Max(bx, cx))))
			minPY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
			maxPY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
			if minPX < 0 {
				minPX = 0
			}
			if minPY < 0 {
				minPY = 0
			}
			if maxPX >= w {
				maxPX = w - 1
			}
			if maxPY >= h {
				maxPY = h - 1
			}
			if minPX > maxPX || minPY > maxPY {
				stats.rejectedSpan++
				stats.rejectSpanClip++
				continue
			}

			stats.emittedSpans++
			for y := minPY; y <= maxPY; y++ {
				py := float64(y) + 0.5
				row := y * w * 4
				for x := minPX; x <= maxPX; x++ {
					px := float64(x) + 0.5
					w0 := (cx-bx)*(py-by) - (cy-by)*(px-bx)
					w1 := (ax-cx)*(py-cy) - (ay-cy)*(px-cx)
					w2 := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
					if area > 0 {
						if w0 < -1e-6 || w1 < -1e-6 || w2 < -1e-6 {
							continue
						}
					} else {
						if w0 > 1e-6 || w1 > 1e-6 || w2 > 1e-6 {
							continue
						}
					}
					wx := minX + (float64(x)+0.5)*step
					wy := maxY - (float64(y)+0.5)*step
					if hasBBox && (wx < bbox.minX || wx > bbox.maxX || wy < bbox.minY || wy > bbox.maxY) {
						continue
					}
					idx := row + x*4
					paintPixel(idx, wx, wy, tex)
					stats.markedCols++
				}
			}
		}
	}

	// Resolve any remaining holes once, in world space.
	if stats.rejectSpanClip > 0 || stats.rejectNoPoly > 0 {
		for y := 0; y < h; y++ {
			row := y * w * 4
			for x := 0; x < w; x++ {
				idx := row + x*4
				if pix[idx+3] != 0 {
					continue
				}
				wx := minX + (float64(x)+0.5)*step
				wy := maxY - (float64(y)+0.5)*step
				ss := g.subSectorAtFixed(int64(wx*fracUnit), int64(wy*fracUnit))
				if ss < 0 || ss >= len(g.m.SubSectors) {
					continue
				}
				if !g.ensureSubSectorPolyAndTris(ss) {
					continue
				}
				poly := g.subSectorPoly[ss]
				if ss < len(g.subSectorBBox) {
					b := g.subSectorBBox[ss]
					if isFinite(b.minX) && isFinite(b.minY) && isFinite(b.maxX) && isFinite(b.maxY) {
						if wx < b.minX || wx > b.maxX || wy < b.minY || wy > b.maxY {
							continue
						}
					}
				}
				if !pointInWorldPoly(worldPt{x: wx, y: wy}, poly) {
					continue
				}
				sec := -1
				if ss < len(g.subSectorSec) {
					sec = g.subSectorSec[ss]
				}
				if sec < 0 || sec >= len(g.m.Sectors) {
					if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
						sec = s
					}
				}
				if sec < 0 || sec >= len(g.m.Sectors) {
					continue
				}
				if !secTexLoaded[sec] {
					secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
					secTexLoaded[sec] = true
				}
				paintPixel(idx, wx, wy, secTex[sec])
				stats.markedCols++
			}
		}
	}

	img := ebiten.NewImage(w, h)
	img.WritePixels(pix)
	g.mapFloorWorldLayer = img
	g.mapFloorWorldMinX = minX
	g.mapFloorWorldMaxY = maxY
	g.mapFloorWorldStep = step
	g.mapFloorWorldStats = stats
	g.mapFloorWorldState = fmt.Sprintf("ready %dx%d step=%.0f holes=%d", w, h, step, len(g.holeFillPolys))
	g.mapFloorWorldInit = true
}

func (g *game) drawMapFloorWorldLayer(screen *ebiten.Image) bool {
	if g.mapFloorWorldLayer == nil {
		return false
	}
	step := g.mapFloorWorldStep
	if step <= 0 {
		step = 1
	}

	cr := 1.0
	sr := 0.0
	if g.rotateView {
		rot := (math.Pi / 2) - angleToRadians(g.renderAngle)
		cr = math.Cos(rot)
		sr = math.Sin(rot)
	}

	dx0 := g.mapFloorWorldMinX - g.renderCamX
	dy0 := g.mapFloorWorldMaxY - g.renderCamY
	halfW := float64(g.viewW) / 2
	halfH := float64(g.viewH) / 2
	scale := g.zoom * step

	a := scale * cr
	b := scale * sr
	c := -scale * sr
	d := scale * cr
	tx := g.zoom*(cr*dx0-sr*dy0) + halfW
	ty := -g.zoom*(sr*dx0+cr*dy0) + halfH

	op := &ebiten.DrawImageOptions{}
	op.GeoM.SetElement(0, 0, a)
	op.GeoM.SetElement(0, 1, b)
	op.GeoM.SetElement(0, 2, tx)
	op.GeoM.SetElement(1, 0, c)
	op.GeoM.SetElement(1, 1, d)
	op.GeoM.SetElement(1, 2, ty)
	screen.DrawImage(g.mapFloorWorldLayer, op)
	return true
}

func (g *game) ensureWallLayer() {
	need := g.viewW * g.viewH * 4
	if g.wallLayer == nil || g.wallW != g.viewW || g.wallH != g.viewH || len(g.wallPix) != need {
		g.wallLayer = ebiten.NewImage(g.viewW, g.viewH)
		g.wallPix = make([]byte, need)
		g.wallW = g.viewW
		g.wallH = g.viewH
	}
}

func (g *game) wallTexture(name string) (WallTexture, bool) {
	key := normalizeFlatName(name)
	if key == "" || key == "-" {
		return WallTexture{}, false
	}
	tex, ok := g.opts.WallTexBank[key]
	if !ok || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return WallTexture{}, false
	}
	return tex, true
}

func wrapIndex(x, size int) int {
	if size <= 0 {
		return 0
	}
	m := x % size
	if m < 0 {
		m += size
	}
	return m
}

func shadeFactorByDistance(dist float64) float64 {
	n := dist / 1200.0
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	return 1.0 - 0.72*n
}

func (g *game) drawMapFloorTextures2D(screen *ebiten.Image) {
	_ = screen
	g.floorFrame = floorFrameStats{}
}

func (g *game) drawMapFloorTextures2DGZDoom(screen *ebiten.Image) {
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 || len(g.opts.FlatBank) == 0 {
		return
	}
	secTex := make([]*ebiten.Image, len(g.m.Sectors))
	secTexLoaded := make([]bool, len(g.m.Sectors))
	if g.whitePixel == nil {
		g.whitePixel = ebiten.NewImage(1, 1)
		g.whitePixel.Fill(color.White)
	}

	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectNoSector++
			continue
		}

		if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectNoPoly++
			continue
		}
		if ss >= len(g.subSectorTris) || len(g.subSectorTris[ss]) == 0 {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectDegenerate++
			continue
		}
		verts := g.subSectorPoly[ss]
		tris := g.subSectorTris[ss]

		drawImg := g.whitePixel
		addressMode := ebiten.AddressUnsafe
		if g.floorDbgMode == floorDebugTextured {
			if !secTexLoaded[sec] {
				if img, ok := g.flatImage(g.m.Sectors[sec].FloorPic); ok {
					secTex[sec] = img
				}
				secTexLoaded[sec] = true
			}
			if secTex[sec] == nil {
				g.floorFrame.rejectedSpan++
				g.floorFrame.rejectNoPoly++
				continue
			}
			drawImg = secTex[sec]
			addressMode = ebiten.AddressRepeat
		}

		vtx := make([]ebiten.Vertex, len(verts))
		for i, v := range verts {
			sx, sy := g.worldToScreen(v.x, v.y)
			vtx[i].DstX = float32(sx)
			vtx[i].DstY = float32(sy)
			switch g.floorDbgMode {
			case floorDebugSolid:
				vtx[i].SrcX = 0
				vtx[i].SrcY = 0
				vtx[i].ColorR = 0.55
				vtx[i].ColorG = 0.70
				vtx[i].ColorB = 0.95
				vtx[i].ColorA = 1
			case floorDebugUV:
				vtx[i].SrcX = 0
				vtx[i].SrcY = 0
				u := frac01(v.x / 64.0)
				w := frac01(v.y / 64.0)
				vtx[i].ColorR = float32(u)
				vtx[i].ColorG = float32(w)
				vtx[i].ColorB = 0
				vtx[i].ColorA = 1
			default:
				vtx[i].SrcX = float32(v.x)
				vtx[i].SrcY = float32(v.y)
				vtx[i].ColorR = 1
				vtx[i].ColorG = 1
				vtx[i].ColorB = 1
				vtx[i].ColorA = 1
			}
		}

		idx := make([]uint16, 0, len(tris)*3)
		for _, tri := range tris {
			if tri[0] < 0 || tri[1] < 0 || tri[2] < 0 || tri[0] >= len(vtx) || tri[1] >= len(vtx) || tri[2] >= len(vtx) {
				continue
			}
			idx = append(idx, uint16(tri[0]), uint16(tri[1]), uint16(tri[2]))
		}
		if len(idx) == 0 {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectDegenerate++
			continue
		}

		op := &ebiten.DrawTrianglesOptions{
			Address:   addressMode,
			Filter:    ebiten.FilterNearest,
			AntiAlias: false,
		}
		screen.DrawTriangles(vtx, idx, drawImg, op)
		g.floorFrame.emittedSpans += len(tris)
		g.floorFrame.markedCols += len(vtx)
	}
}

func (g *game) drawMapFloorTextures2DLegacy(screen *ebiten.Image) {
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 || len(g.opts.FlatBank) == 0 {
		return
	}
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}
	rowWX0 := make([]float64, g.viewH)
	rowWY0 := make([]float64, g.viewH)
	rowStepWX := make([]float64, g.viewH)
	rowStepWY := make([]float64, g.viewH)
	for y := 0; y < g.viewH; y++ {
		wx0, wy0 := g.screenToWorld(0.5, float64(y)+0.5)
		wx1, wy1 := g.screenToWorld(1.5, float64(y)+0.5)
		rowWX0[y] = wx0
		rowWY0[y] = wy0
		rowStepWX[y] = wx1 - wx0
		rowStepWY[y] = wy1 - wy0
	}

	const fallbackR = 90
	const fallbackG = 125
	const fallbackB = 160
	secTex := make([][]byte, len(g.m.Sectors))
	secTexLoaded := make([]bool, len(g.m.Sectors))
	const edgeEps = 1e-4

	paintPixel := func(idx int, wx, wy float64, tex []byte) {
		switch g.floorDbgMode {
		case floorDebugSolid:
			pix[idx+0] = 95
			pix[idx+1] = 145
			pix[idx+2] = 215
			pix[idx+3] = 255
		case floorDebugUV:
			u := frac01(wx / 64.0)
			v := frac01(wy / 64.0)
			pix[idx+0] = uint8(u * 255)
			pix[idx+1] = uint8(v * 255)
			pix[idx+2] = 0
			pix[idx+3] = 255
		default:
			if len(tex) == 64*64*4 {
				u := int(math.Floor(wx)) & 63
				v := int(math.Floor(wy)) & 63
				ti := (v*64 + u) * 4
				pix[idx+0] = tex[ti+0]
				pix[idx+1] = tex[ti+1]
				pix[idx+2] = tex[ti+2]
				pix[idx+3] = 255
			} else {
				pix[idx+0] = fallbackR
				pix[idx+1] = fallbackG
				pix[idx+2] = fallbackB
				pix[idx+3] = 255
			}
		}
	}

	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectNoSector++
			continue
		}
		if !secTexLoaded[sec] {
			secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
			secTexLoaded[sec] = true
		}
		tex := secTex[sec]

		if !g.ensureSubSectorPolyAndTris(ss) {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectNoPoly++
			continue
		}
		verts := g.subSectorPoly[ss]
		bbox := worldBBox{}
		hasBBox := false
		if ss < len(g.subSectorBBox) {
			b := g.subSectorBBox[ss]
			if isFinite(b.minX) && isFinite(b.minY) && isFinite(b.maxX) && isFinite(b.maxY) && b.minX <= b.maxX && b.minY <= b.maxY {
				bbox = b
				hasBBox = true
			}
		}

		tris := g.subSectorTris[ss]
		_ = edgeEps // retained for parity with old path tuning.
		for _, tri := range tris {
			a := verts[tri[0]]
			b := verts[tri[1]]
			c := verts[tri[2]]
			ax, ay := g.worldToScreen(a.x, a.y)
			bx, by := g.worldToScreen(b.x, b.y)
			cx, cy := g.worldToScreen(c.x, c.y)
			area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if math.Abs(area) < 1e-9 {
				g.floorFrame.rejectedSpan++
				g.floorFrame.rejectDegenerate++
				continue
			}
			minX := int(math.Floor(math.Min(ax, math.Min(bx, cx))))
			maxX := int(math.Ceil(math.Max(ax, math.Max(bx, cx))))
			minY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
			maxY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
			if minX < 0 {
				minX = 0
			}
			if minY < 0 {
				minY = 0
			}
			if maxX >= g.viewW {
				maxX = g.viewW - 1
			}
			if maxY >= g.viewH {
				maxY = g.viewH - 1
			}
			if minX > maxX || minY > maxY {
				g.floorFrame.rejectedSpan++
				g.floorFrame.rejectSpanClip++
				continue
			}
			g.floorFrame.emittedSpans++
			for y := minY; y <= maxY; y++ {
				py := float64(y) + 0.5
				row := y * g.viewW * 4
				for x := minX; x <= maxX; x++ {
					px := float64(x) + 0.5
					w0 := (cx-bx)*(py-by) - (cy-by)*(px-bx)
					w1 := (ax-cx)*(py-cy) - (ay-cy)*(px-cx)
					w2 := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
					if area > 0 {
						if w0 < -1e-6 || w1 < -1e-6 || w2 < -1e-6 {
							continue
						}
					} else {
						if w0 > 1e-6 || w1 > 1e-6 || w2 > 1e-6 {
							continue
						}
					}
					wx := rowWX0[y] + rowStepWX[y]*float64(x)
					wy := rowWY0[y] + rowStepWY[y]*float64(x)
					if hasBBox && (wx < bbox.minX || wx > bbox.maxX || wy < bbox.minY || wy > bbox.maxY) {
						continue
					}
					idx := row + x*4
					paintPixel(idx, wx, wy, tex)
					g.floorFrame.markedCols++
				}
			}
		}
	}

	for _, hp := range g.holeFillPolys {
		sec := hp.sector
		if sec < 0 || sec >= len(g.m.Sectors) {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectNoSector++
			continue
		}
		if len(hp.verts) < 3 || len(hp.tris) == 0 {
			g.floorFrame.rejectedSpan++
			g.floorFrame.rejectDegenerate++
			continue
		}
		if !secTexLoaded[sec] {
			secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
			secTexLoaded[sec] = true
		}
		tex := secTex[sec]
		verts := hp.verts
		bbox := hp.bbox
		hasBBox := isFinite(bbox.minX) && isFinite(bbox.minY) && isFinite(bbox.maxX) && isFinite(bbox.maxY) && bbox.minX <= bbox.maxX && bbox.minY <= bbox.maxY

		for _, tri := range hp.tris {
			a := verts[tri[0]]
			b := verts[tri[1]]
			c := verts[tri[2]]
			ax, ay := g.worldToScreen(a.x, a.y)
			bx, by := g.worldToScreen(b.x, b.y)
			cx, cy := g.worldToScreen(c.x, c.y)
			area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
			if math.Abs(area) < 1e-9 {
				g.floorFrame.rejectedSpan++
				g.floorFrame.rejectDegenerate++
				continue
			}
			minX := int(math.Floor(math.Min(ax, math.Min(bx, cx))))
			maxX := int(math.Ceil(math.Max(ax, math.Max(bx, cx))))
			minY := int(math.Floor(math.Min(ay, math.Min(by, cy))))
			maxY := int(math.Ceil(math.Max(ay, math.Max(by, cy))))
			if minX < 0 {
				minX = 0
			}
			if minY < 0 {
				minY = 0
			}
			if maxX >= g.viewW {
				maxX = g.viewW - 1
			}
			if maxY >= g.viewH {
				maxY = g.viewH - 1
			}
			if minX > maxX || minY > maxY {
				g.floorFrame.rejectedSpan++
				g.floorFrame.rejectSpanClip++
				continue
			}
			g.floorFrame.emittedSpans++
			for y := minY; y <= maxY; y++ {
				py := float64(y) + 0.5
				row := y * g.viewW * 4
				for x := minX; x <= maxX; x++ {
					px := float64(x) + 0.5
					w0 := (cx-bx)*(py-by) - (cy-by)*(px-bx)
					w1 := (ax-cx)*(py-cy) - (ay-cy)*(px-cx)
					w2 := (bx-ax)*(py-ay) - (by-ay)*(px-ax)
					if area > 0 {
						if w0 < -1e-6 || w1 < -1e-6 || w2 < -1e-6 {
							continue
						}
					} else {
						if w0 > 1e-6 || w1 > 1e-6 || w2 > 1e-6 {
							continue
						}
					}
					wx := rowWX0[y] + rowStepWX[y]*float64(x)
					wy := rowWY0[y] + rowStepWY[y]*float64(x)
					if hasBBox && (wx < bbox.minX || wx > bbox.maxX || wy < bbox.minY || wy > bbox.maxY) {
						continue
					}
					idx := row + x*4
					paintPixel(idx, wx, wy, tex)
					g.floorFrame.markedCols++
				}
			}
		}
	}
	// Final deterministic recovery pass: sample any still-empty pixel in world
	// space and fill it only if the sample is inside a valid subsector polygon.
	// This avoids angle-dependent flashing from screen-space component heuristics.
	if g.floorFrame.rejectSpanClip > 0 || g.floorFrame.rejectNoPoly > 0 {
		for y := 0; y < g.viewH; y++ {
			row := y * g.viewW * 4
			for x := 0; x < g.viewW; x++ {
				idx := row + x*4
				if pix[idx+3] != 0 {
					continue
				}
				wx := rowWX0[y] + rowStepWX[y]*float64(x)
				wy := rowWY0[y] + rowStepWY[y]*float64(x)
				ss := g.subSectorAtFixed(int64(wx*fracUnit), int64(wy*fracUnit))
				if ss < 0 || ss >= len(g.m.SubSectors) {
					continue
				}
				if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
					continue
				}
				poly := g.subSectorPoly[ss]
				if ss < len(g.subSectorBBox) {
					b := g.subSectorBBox[ss]
					if isFinite(b.minX) && isFinite(b.minY) && isFinite(b.maxX) && isFinite(b.maxY) {
						if wx < b.minX || wx > b.maxX || wy < b.minY || wy > b.maxY {
							continue
						}
					}
				}
				if !pointInWorldPoly(worldPt{x: wx, y: wy}, poly) {
					continue
				}
				sec := -1
				if ss < len(g.subSectorSec) {
					sec = g.subSectorSec[ss]
				}
				if sec < 0 || sec >= len(g.m.Sectors) {
					if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
						sec = s
					}
				}
				if sec < 0 || sec >= len(g.m.Sectors) {
					continue
				}
				if !secTexLoaded[sec] {
					secTex[sec] = g.opts.FlatBank[normalizeFlatName(g.m.Sectors[sec].FloorPic)]
					secTexLoaded[sec] = true
				}
				paintPixel(idx, wx, wy, secTex[sec])
				g.floorFrame.markedCols++
			}
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, nil)
}

func (g *game) drawMapFloorTextures2DVisplane(screen *ebiten.Image) {
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 || len(g.opts.FlatBank) == 0 {
		return
	}
	g.resetFloorVisplaneFrame()
	g.buildFloorVisplaneMarks()
	g.buildFloorVisplaneSpans()
	g.renderFloorVisplaneSpans(screen)
}

func (g *game) subSectorVerticesFromSegList(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	verts := make([]worldPt, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		// Use subsector seg order directly (Doom BSP output).
		vi := sg.StartVertex
		if int(vi) >= len(g.m.Vertexes) {
			continue
		}
		v := g.m.Vertexes[vi]
		p := worldPt{x: float64(v.X), y: float64(v.Y)}
		if len(verts) > 0 {
			last := verts[len(verts)-1]
			if last.x == p.x && last.y == p.y {
				continue
			}
		}
		verts = append(verts, p)
	}
	if len(verts) >= 2 {
		a := verts[0]
		b := verts[len(verts)-1]
		if a.x == b.x && a.y == b.y {
			verts = verts[:len(verts)-1]
		}
	}
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 1e-6 {
		return nil, 0, 0, false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func (g *game) subSectorConvexVertices(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	chain, closed := subsectorVertexLoopFromSegOrder(g.m, sub)
	if !closed {
		// Some WAD subsectors reuse geometry/lines; fall back to unique vertices.
		verts, ok := uniqueSubsectorVertices(g.m, sub)
		if !ok || len(verts) < 3 {
			return nil, 0, 0, false
		}
		cx, cy := 0.0, 0.0
		for _, v := range verts {
			cx += v.x
			cy += v.y
		}
		cx /= float64(len(verts))
		cy /= float64(len(verts))
		sort.Slice(verts, func(i, j int) bool {
			ai := math.Atan2(verts[i].y-cy, verts[i].x-cx)
			aj := math.Atan2(verts[j].y-cy, verts[j].x-cx)
			return ai < aj
		})
		if math.Abs(polygonArea2(verts)) < 1e-6 {
			return nil, 0, 0, false
		}
		return verts, cx, cy, true
	}
	verts := vertexChainToWorld(g.m, chain)
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 1e-6 {
		return nil, 0, 0, false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func (g *game) floorDebugTriVertices(world []worldPt, poly []screenPt, i0, i1, i2, texW, texH int) []ebiten.Vertex {
	mk := func(i int) ebiten.Vertex {
		v := ebiten.Vertex{
			DstX: float32(poly[i].x),
			DstY: float32(poly[i].y),
			SrcX: float32(world[i].x),
			SrcY: float32(world[i].y),
		}
		switch g.floorDbgMode {
		case floorDebugSolid:
			v.SrcX = 0
			v.SrcY = 0
			v.ColorR, v.ColorG, v.ColorB, v.ColorA = 0.55, 0.7, 0.95, 1.0
		case floorDebugUV:
			u := frac01(world[i].x / float64(max(texW, 1)))
			w := frac01(world[i].y / float64(max(texH, 1)))
			v.SrcX = 0
			v.SrcY = 0
			v.ColorR, v.ColorG, v.ColorB, v.ColorA = float32(u), float32(w), 0.0, 1.0
		default:
			v.ColorR, v.ColorG, v.ColorB, v.ColorA = 1, 1, 1, 1
		}
		return v
	}
	return []ebiten.Vertex{mk(i0), mk(i1), mk(i2)}
}

func frac01(x float64) float64 {
	return x - math.Floor(x)
}

func (g *game) floorDebugLabel() string {
	switch g.floorDbgMode {
	case floorDebugSolid:
		return "solid"
	case floorDebugUV:
		return "uv"
	default:
		return "textured"
	}
}

func (g *game) floorPathLabel() string {
	switch g.floor2DPath {
	case floor2DPathVisplane:
		return "visplane"
	default:
		return "legacy"
	}
}

func (g *game) floorVisDiagLabel() string {
	switch g.floorVisDiag {
	case floorVisDiagClip:
		return "clip"
	case floorVisDiagSpan:
		return "span"
	case floorVisDiagBoth:
		return "both"
	default:
		return "off"
	}
}

type worldPt struct {
	x float64
	y float64
}

type holeFillPoly struct {
	sector int
	verts  []worldPt
	tris   [][3]int
	bbox   worldBBox
}

type holeQuantPt struct {
	x int64
	y int64
}

type holeEdgeKey struct {
	ax int64
	ay int64
	bx int64
	by int64
}

type holeBoundaryEdge struct {
	a  holeQuantPt
	b  holeQuantPt
	aw worldPt
	bw worldPt
}

type holeEdgeDirBucket struct {
	ab []holeBoundaryEdge
	ba []holeBoundaryEdge
}

type subsectorEdge struct {
	a uint16
	b uint16
}

func (g *game) subSectorWorldVertices(ss int) ([]worldPt, float64, float64, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return nil, 0, 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount < 3 {
		return nil, 0, 0, false
	}
	chain, closed := subsectorVertexLoopFromSegOrder(g.m, sub)
	if !closed {
		edges := make([]subsectorEdge, 0, sub.SegCount)
		for i := 0; i < int(sub.SegCount); i++ {
			si := int(sub.FirstSeg) + i
			if si < 0 || si >= len(g.m.Segs) {
				continue
			}
			sg := g.m.Segs[si]
			if int(sg.StartVertex) >= len(g.m.Vertexes) || int(sg.EndVertex) >= len(g.m.Vertexes) {
				continue
			}
			edges = append(edges, subsectorEdge{a: sg.StartVertex, b: sg.EndVertex})
		}
		if len(edges) < 3 {
			return nil, 0, 0, false
		}
		chain, closed = chainSubsectorEdges(edges)
		if !closed {
			chain = rawSubsectorVertexOrder(g.m, sub)
		}
	}
	verts := vertexChainToWorld(g.m, chain)
	if len(verts) < 3 {
		return nil, 0, 0, false
	}
	// If winding is clockwise, reverse to keep a consistent triangle fan.
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 0.001 {
		return nil, 0, 0, false
	}
	if area2 < 0 {
		for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
			verts[i], verts[j] = verts[j], verts[i]
		}
	}
	// Polygon centroid estimate (mean of vertices is enough for convex subsectors).
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(len(verts))
	cy /= float64(len(verts))
	return verts, cx, cy, true
}

func subsectorVertexLoopFromSegOrder(m *mapdata.Map, sub mapdata.SubSector) ([]uint16, bool) {
	if sub.SegCount < 3 {
		return nil, false
	}
	type edge struct {
		a uint16
		b uint16
	}
	edges := make([]edge, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			return nil, false
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) || int(sg.EndVertex) >= len(m.Vertexes) {
			return nil, false
		}
		edges = append(edges, edge{a: sg.StartVertex, b: sg.EndVertex})
	}
	if len(edges) < 3 {
		return nil, false
	}
	used := make([]bool, len(edges))
	chain := make([]uint16, 0, len(edges)+1)
	chain = append(chain, edges[0].a, edges[0].b)
	used[0] = true
	for len(chain) < len(edges)+1 {
		last := chain[len(chain)-1]
		found := false
		for i := 1; i < len(edges); i++ {
			if used[i] {
				continue
			}
			e := edges[i]
			switch {
			case e.a == last:
				chain = append(chain, e.b)
				used[i] = true
				found = true
			case e.b == last:
				chain = append(chain, e.a)
				used[i] = true
				found = true
			}
			if found {
				break
			}
		}
		if !found {
			break
		}
	}
	if len(chain) >= 2 && chain[len(chain)-1] == chain[0] {
		chain = chain[:len(chain)-1]
	}
	if len(chain) < 3 {
		return nil, false
	}
	// Do not require every seg edge to be consumed: some subsectors can contain
	// repeated/redundant edges after node building.
	return chain, true
}

func uniqueSubsectorVertices(m *mapdata.Map, sub mapdata.SubSector) ([]worldPt, bool) {
	seen := make(map[uint16]struct{}, int(sub.SegCount)*2)
	out := make([]worldPt, 0, int(sub.SegCount)*2)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			continue
		}
		sg := m.Segs[si]
		for _, vi := range []uint16{sg.StartVertex, sg.EndVertex} {
			if _, ok := seen[vi]; ok {
				continue
			}
			if int(vi) >= len(m.Vertexes) {
				continue
			}
			v := m.Vertexes[vi]
			out = append(out, worldPt{x: float64(v.X), y: float64(v.Y)})
			seen[vi] = struct{}{}
		}
	}
	return out, len(out) >= 3
}

func chainSubsectorEdges(edges []subsectorEdge) ([]uint16, bool) {
	if len(edges) == 0 {
		return nil, false
	}
	used := make([]bool, len(edges))
	chain := make([]uint16, 0, len(edges)+1)
	chain = append(chain, edges[0].a, edges[0].b)
	used[0] = true
	for len(chain) <= len(edges)+1 {
		last := chain[len(chain)-1]
		progress := false
		for i, e := range edges {
			if used[i] {
				continue
			}
			if e.a == last {
				chain = append(chain, e.b)
				used[i] = true
				progress = true
				break
			}
			if e.b == last {
				chain = append(chain, e.a)
				used[i] = true
				progress = true
				break
			}
		}
		if !progress {
			break
		}
		if len(chain) >= 3 && chain[len(chain)-1] == chain[0] {
			allUsed := true
			for _, u := range used {
				if !u {
					allUsed = false
					break
				}
			}
			if allUsed {
				chain = chain[:len(chain)-1]
				return chain, true
			}
			break
		}
	}
	return nil, false
}

func rawSubsectorVertexOrder(m *mapdata.Map, sub mapdata.SubSector) []uint16 {
	chain := make([]uint16, 0, sub.SegCount)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(m.Segs) {
			continue
		}
		sg := m.Segs[si]
		if int(sg.StartVertex) >= len(m.Vertexes) {
			continue
		}
		chain = append(chain, sg.StartVertex)
	}
	if len(chain) >= 2 && chain[len(chain)-1] == chain[0] {
		chain = chain[:len(chain)-1]
	}
	return chain
}

func vertexChainToWorld(m *mapdata.Map, chain []uint16) []worldPt {
	if len(chain) < 3 {
		return nil
	}
	verts := make([]worldPt, 0, len(chain))
	lastX, lastY := math.Inf(1), math.Inf(1)
	for _, vi := range chain {
		if int(vi) >= len(m.Vertexes) {
			continue
		}
		v := m.Vertexes[vi]
		x, y := float64(v.X), float64(v.Y)
		if x == lastX && y == lastY {
			continue
		}
		verts = append(verts, worldPt{x: x, y: y})
		lastX, lastY = x, y
	}
	if len(verts) >= 2 {
		a := verts[0]
		b := verts[len(verts)-1]
		if a.x == b.x && a.y == b.y {
			verts = verts[:len(verts)-1]
		}
	}
	return verts
}

func polygonArea2(verts []worldPt) float64 {
	area2 := 0.0
	for i := 0; i < len(verts); i++ {
		j := (i + 1) % len(verts)
		area2 += verts[i].x*verts[j].y - verts[j].x*verts[i].y
	}
	return area2
}

func triangulateWorldPolygon(verts []worldPt) ([][3]int, bool) {
	n := len(verts)
	if n < 3 {
		return nil, false
	}
	if !polygonSimple(verts) {
		return nil, false
	}
	area2 := polygonArea2(verts)
	if math.Abs(area2) < 1e-9 {
		return nil, false
	}
	idx := make([]int, n)
	if area2 > 0 {
		for i := 0; i < n; i++ {
			idx[i] = i
		}
	} else {
		for i := 0; i < n; i++ {
			idx[i] = n - 1 - i
		}
	}
	out := make([][3]int, 0, n-2)
	guard := 0
	for len(idx) > 3 && guard < n*n {
		guard++
		earFound := false
		for i := 0; i < len(idx); i++ {
			pi := idx[(i-1+len(idx))%len(idx)]
			ci := idx[i]
			ni := idx[(i+1)%len(idx)]
			if !isCCW(verts[pi], verts[ci], verts[ni]) {
				continue
			}
			if containsAnyPointInTri(verts, idx, pi, ci, ni) {
				continue
			}
			out = append(out, [3]int{pi, ci, ni})
			idx = append(idx[:i], idx[i+1:]...)
			earFound = true
			break
		}
		if !earFound {
			return nil, false
		}
	}
	if len(idx) == 3 {
		out = append(out, [3]int{idx[0], idx[1], idx[2]})
	}
	return out, len(out) > 0
}

func triangulateByAngleFan(verts []worldPt) ([][3]int, bool) {
	n := len(verts)
	if n < 3 {
		return nil, false
	}
	cx, cy := 0.0, 0.0
	for _, v := range verts {
		cx += v.x
		cy += v.y
	}
	cx /= float64(n)
	cy /= float64(n)

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		ai := math.Atan2(verts[order[i]].y-cy, verts[order[i]].x-cx)
		aj := math.Atan2(verts[order[j]].y-cy, verts[order[j]].x-cx)
		return ai < aj
	})

	// Reject clearly degenerate results.
	area2 := 0.0
	for i := 0; i < n; i++ {
		a := verts[order[i]]
		b := verts[order[(i+1)%n]]
		area2 += a.x*b.y - b.x*a.y
	}
	if math.Abs(area2) < 1e-6 {
		return nil, false
	}

	tris := make([][3]int, 0, n-2)
	for i := 1; i+1 < n; i++ {
		tris = append(tris, [3]int{order[0], order[i], order[i+1]})
	}
	return tris, len(tris) > 0
}

func polygonSimple(verts []worldPt) bool {
	n := len(verts)
	if n < 3 {
		return false
	}
	for i := 0; i < n; i++ {
		a1 := verts[i]
		a2 := verts[(i+1)%n]
		for j := i + 1; j < n; j++ {
			// Skip adjacent edges and the same closing edge pair.
			if j == i || (j+1)%n == i || j == (i+1)%n {
				continue
			}
			b1 := verts[j]
			b2 := verts[(j+1)%n]
			if segmentsIntersectStrict(a1, a2, b1, b2) {
				return false
			}
		}
	}
	return true
}

func segmentsIntersectStrict(a1, a2, b1, b2 worldPt) bool {
	o1 := orient2D(a1, a2, b1)
	o2 := orient2D(a1, a2, b2)
	o3 := orient2D(b1, b2, a1)
	o4 := orient2D(b1, b2, a2)
	return (o1*o2 < 0) && (o3*o4 < 0)
}

func orient2D(a, b, c worldPt) float64 {
	return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x)
}

func isCCW(a, b, c worldPt) bool {
	return orient2D(a, b, c) > 1e-9
}

func containsAnyPointInTri(verts []worldPt, idx []int, ai, bi, ci int) bool {
	a, b, c := verts[ai], verts[bi], verts[ci]
	for _, vi := range idx {
		if vi == ai || vi == bi || vi == ci {
			continue
		}
		if pointInTri(verts[vi], a, b, c) {
			return true
		}
	}
	return false
}

func pointInTri(p, a, b, c worldPt) bool {
	ab := orient2D(a, b, p)
	bc := orient2D(b, c, p)
	ca := orient2D(c, a, p)
	const eps = 1e-9
	// Accept edge points to avoid sliver ears.
	return ab >= -eps && bc >= -eps && ca >= -eps
}

func (g *game) subSectorScreenPolygon(ss int) ([]screenPt, []worldPt, float64, float64, polyBBox, bool) {
	verts, cx, cy, ok := g.subSectorWorldVertices(ss)
	if !ok {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	poly := make([]screenPt, 0, len(verts))
	minX, minY := g.viewW-1, g.viewH-1
	maxX, maxY := 0, 0
	for _, v := range verts {
		sx, sy := g.worldToScreen(v.x, v.y)
		poly = append(poly, screenPt{x: sx, y: sy})
		ix := int(math.Round(sx))
		iy := int(math.Round(sy))
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
		}
		if iy < minY {
			minY = iy
		}
		if iy > maxY {
			maxY = iy
		}
	}
	if maxX < 0 || maxY < 0 || minX >= g.viewW || minY >= g.viewH {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= g.viewW {
		maxX = g.viewW - 1
	}
	if maxY >= g.viewH {
		maxY = g.viewH - 1
	}
	if minX > maxX || minY > maxY {
		return nil, nil, 0, 0, polyBBox{}, false
	}
	return poly, verts, cx, cy, polyBBox{minX: minX, minY: minY, maxX: maxX, maxY: maxY}, true
}

func (g *game) subSectorSectorIndex(ss int) (int, bool) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return 0, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount == 0 {
		return 0, false
	}
	// Doom associates a subsector with the sector of its first seg.
	firstSeg := int(sub.FirstSeg)
	if sec, ok := g.subSectorSectorFromSeg(firstSeg); ok {
		return sec, true
	}
	// Fallback for malformed node data.
	for i := 1; i < int(sub.SegCount); i++ {
		if sec, ok := g.subSectorSectorFromSeg(int(sub.FirstSeg) + i); ok {
			return sec, true
		}
	}
	return 0, false
}

func (g *game) initSubSectorSectorCache() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorSec = nil
		g.subSectorPoly = nil
		g.subSectorTris = nil
		g.subSectorBBox = nil
		g.holeFillPolys = nil
		return
	}
	g.subSectorSec = make([]int, len(g.m.SubSectors))
	g.subSectorBBox = make([]worldBBox, len(g.m.SubSectors))
	g.subSectorPoly = nil
	g.subSectorTris = nil
	g.holeFillPolys = nil
	for i := range g.subSectorSec {
		g.subSectorSec[i] = -1
		g.subSectorBBox[i] = worldBBox{
			minX: math.Inf(1),
			minY: math.Inf(1),
			maxX: math.Inf(-1),
			maxY: math.Inf(-1),
		}
	}
	for ss := range g.m.SubSectors {
		if sec, ok := g.subSectorSectorIndex(ss); ok {
			g.subSectorSec[ss] = sec
		}
		if b, ok := g.subSectorSegBBox(ss); ok {
			g.subSectorBBox[ss] = b
		}
	}
}

func (g *game) buildSubSectorPolysFromSegLoops() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		return
	}
	if len(g.subSectorPoly) != len(g.m.SubSectors) {
		return
	}
	for ss := range g.m.SubSectors {
		sub := g.m.SubSectors[ss]
		if sub.SegCount < 3 {
			continue
		}
		verts := make([]worldPt, 0, sub.SegCount)
		for i := 0; i < int(sub.SegCount); i++ {
			si := int(sub.FirstSeg) + i
			if si < 0 || si >= len(g.m.Segs) {
				continue
			}
			sg := g.m.Segs[si]
			if int(sg.StartVertex) >= len(g.m.Vertexes) {
				continue
			}
			v := g.m.Vertexes[sg.StartVertex]
			p := worldPt{x: float64(v.X), y: float64(v.Y)}
			if len(verts) > 0 && nearlyEqualWorldPt(verts[len(verts)-1], p, 1e-6) {
				continue
			}
			verts = append(verts, p)
		}
		if len(verts) >= 2 && nearlyEqualWorldPt(verts[0], verts[len(verts)-1], 1e-6) {
			verts = verts[:len(verts)-1]
		}
		if len(verts) < 3 {
			if fallback, _, _, ok := g.subSectorConvexVertices(ss); ok && len(fallback) >= 3 {
				verts = fallback
			}
		}
		if len(verts) < 3 {
			continue
		}
		area2 := polygonArea2(verts)
		if math.Abs(area2) < 1e-6 {
			if fallback, _, _, ok := g.subSectorConvexVertices(ss); ok && len(fallback) >= 3 {
				verts = fallback
				area2 = polygonArea2(verts)
			}
		}
		if math.Abs(area2) < 1e-6 {
			continue
		}
		if area2 < 0 {
			for i, j := 0, len(verts)-1; i < j; i, j = i+1, j-1 {
				verts[i], verts[j] = verts[j], verts[i]
			}
		}
		g.subSectorPoly[ss] = verts
	}
}

func (g *game) buildSubSectorTriCache() {
	if g.m == nil || len(g.m.SubSectors) == 0 {
		g.subSectorTris = nil
		return
	}
	if len(g.subSectorTris) != len(g.m.SubSectors) {
		g.subSectorTris = make([][][3]int, len(g.m.SubSectors))
	}
	for ss := range g.m.SubSectors {
		g.subSectorTris[ss] = nil
		if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		verts := g.subSectorPoly[ss]
		tris, ok := triangulateWorldPolygon(verts)
		if !ok {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			continue
		}
		g.subSectorTris[ss] = tris
	}
}

func (g *game) ensureSubSectorPolyAndTris(ss int) bool {
	if g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return false
	}
	if ss >= len(g.subSectorPoly) {
		return false
	}
	if len(g.subSectorPoly[ss]) < 3 {
		verts, ok := g.subSectorWorldPolyCached(ss)
		if !ok || len(verts) < 3 {
			return false
		}
		g.subSectorPoly[ss] = verts
		if ss < len(g.subSectorBBox) {
			if b, ok := g.subSectorSegBBox(ss); ok {
				g.subSectorBBox[ss] = b
			}
		}
	}
	if ss >= len(g.subSectorTris) {
		return false
	}
	if len(g.subSectorTris[ss]) == 0 {
		verts := g.subSectorPoly[ss]
		tris, ok := triangulateWorldPolygon(verts)
		if !ok {
			tris, ok = triangulateByAngleFan(verts)
		}
		if !ok || len(tris) == 0 {
			return false
		}
		g.subSectorTris[ss] = tris
	}
	return true
}

func holeQuantizeWorldPt(p worldPt) holeQuantPt {
	const q = 64.0
	return holeQuantPt{
		x: int64(math.Round(p.x * q)),
		y: int64(math.Round(p.y * q)),
	}
}

func holeQuantLess(a, b holeQuantPt) bool {
	if a.x != b.x {
		return a.x < b.x
	}
	return a.y < b.y
}

func canonicalHoleEdgeKey(a, b holeQuantPt) (holeEdgeKey, bool) {
	if holeQuantLess(a, b) {
		return holeEdgeKey{ax: a.x, ay: a.y, bx: b.x, by: b.y}, true
	}
	return holeEdgeKey{ax: b.x, ay: b.y, bx: a.x, by: a.y}, false
}

func (g *game) buildHoleFillPolys() {
	g.holeFillPolys = nil
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.subSectorPoly) == 0 {
		return
	}

	perSector := make(map[int]map[holeEdgeKey]*holeEdgeDirBucket)
	for ss := range g.m.SubSectors {
		sec := -1
		if ss < len(g.subSectorSec) {
			sec = g.subSectorSec[ss]
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			if s, ok := g.subSectorSectorIndex(ss); ok && s >= 0 && s < len(g.m.Sectors) {
				sec = s
			}
		}
		if sec < 0 || sec >= len(g.m.Sectors) {
			continue
		}
		if ss >= len(g.subSectorPoly) || len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		poly := g.subSectorPoly[ss]
		area2 := polygonArea2(poly)
		if math.Abs(area2) < 1e-6 {
			continue
		}
		if area2 < 0 {
			cp := make([]worldPt, len(poly))
			copy(cp, poly)
			for i, j := 0, len(cp)-1; i < j; i, j = i+1, j-1 {
				cp[i], cp[j] = cp[j], cp[i]
			}
			poly = cp
		}

		edgeBuckets, ok := perSector[sec]
		if !ok {
			edgeBuckets = make(map[holeEdgeKey]*holeEdgeDirBucket)
			perSector[sec] = edgeBuckets
		}
		for i := 0; i < len(poly); i++ {
			a := poly[i]
			b := poly[(i+1)%len(poly)]
			qa := holeQuantizeWorldPt(a)
			qb := holeQuantizeWorldPt(b)
			if qa == qb {
				continue
			}
			key, forward := canonicalHoleEdgeKey(qa, qb)
			bucket := edgeBuckets[key]
			if bucket == nil {
				bucket = &holeEdgeDirBucket{}
				edgeBuckets[key] = bucket
			}
			edge := holeBoundaryEdge{a: qa, b: qb, aw: a, bw: b}
			if forward {
				bucket.ab = append(bucket.ab, edge)
			} else {
				bucket.ba = append(bucket.ba, edge)
			}
		}
	}

	out := make([]holeFillPoly, 0, 16)
	for sec, edgeBuckets := range perSector {
		boundary := make([]holeBoundaryEdge, 0, len(edgeBuckets))
		for _, b := range edgeBuckets {
			if len(b.ab) > len(b.ba) {
				boundary = append(boundary, b.ab[:len(b.ab)-len(b.ba)]...)
			} else if len(b.ba) > len(b.ab) {
				boundary = append(boundary, b.ba[:len(b.ba)-len(b.ab)]...)
			}
		}
		if len(boundary) < 3 {
			continue
		}

		adj := make(map[holeQuantPt][]int, len(boundary))
		for i, e := range boundary {
			adj[e.a] = append(adj[e.a], i)
		}
		used := make([]bool, len(boundary))

		for i := range boundary {
			if used[i] {
				continue
			}
			start := boundary[i]
			cur := start
			used[i] = true
			loop := make([]worldPt, 0, 12)
			loop = append(loop, cur.aw)

			closed := false
			guard := 0
			for guard < len(boundary)+4 {
				guard++
				loop = append(loop, cur.bw)
				if cur.b == start.a {
					closed = true
					break
				}
				nextIdx := -1
				for _, cand := range adj[cur.b] {
					if !used[cand] {
						nextIdx = cand
						break
					}
				}
				if nextIdx < 0 {
					break
				}
				used[nextIdx] = true
				cur = boundary[nextIdx]
			}
			if !closed {
				continue
			}
			if len(loop) >= 2 && nearlyEqualWorldPt(loop[0], loop[len(loop)-1], 1e-6) {
				loop = loop[:len(loop)-1]
			}
			if len(loop) < 3 {
				continue
			}
			area2 := polygonArea2(loop)
			if math.Abs(area2) < 1e-6 {
				continue
			}
			// Remaining boundary loops from subsector unions are CCW for outer borders
			// and CW for holes. Only fill CW loops.
			if area2 >= 0 {
				continue
			}
			tris, ok := triangulateWorldPolygon(loop)
			if !ok || len(tris) == 0 {
				continue
			}
			bbox := worldPolyBBox(loop)
			if !isFinite(bbox.minX) || !isFinite(bbox.minY) || !isFinite(bbox.maxX) || !isFinite(bbox.maxY) {
				continue
			}
			out = append(out, holeFillPoly{
				sector: sec,
				verts:  loop,
				tris:   tris,
				bbox:   bbox,
			})
		}
	}

	g.holeFillPolys = out
}

func (g *game) subSectorWorldPolyCached(ss int) ([]worldPt, bool) {
	verts, _, _, ok := g.subSectorWorldVertices(ss)
	if !ok {
		verts, _, _, ok = g.subSectorVerticesFromSegList(ss)
	}
	if !ok {
		verts, _, _, ok = g.subSectorConvexVertices(ss)
	}
	if !ok || len(verts) < 3 {
		return nil, false
	}
	return verts, true
}

func (g *game) subSectorAtFixed(x, y int64) int {
	if len(g.m.Nodes) == 0 {
		if len(g.m.SubSectors) == 0 {
			return -1
		}
		return 0
	}
	child := uint16(len(g.m.Nodes) - 1)
	for {
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss < 0 || ss >= len(g.m.SubSectors) {
				return -1
			}
			return ss
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return -1
		}
		n := g.m.Nodes[ni]
		dl := divline{
			x:  int64(n.X) << fracBits,
			y:  int64(n.Y) << fracBits,
			dx: int64(n.DX) << fracBits,
			dy: int64(n.DY) << fracBits,
		}
		side := pointOnDivlineSide(x, y, dl)
		child = n.ChildID[side]
	}
}

func (g *game) sectorForSubSector(ss int) int {
	if ss >= 0 && ss < len(g.subSectorSec) {
		if sec := g.subSectorSec[ss]; sec >= 0 && sec < len(g.m.Sectors) {
			return sec
		}
	}
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return -1
	}
	s := g.m.SubSectors[ss]
	if int(s.FirstSeg) >= len(g.m.Segs) {
		return -1
	}
	seg := g.m.Segs[s.FirstSeg]
	if int(seg.Linedef) >= len(g.m.Linedefs) {
		return -1
	}
	ld := g.m.Linedefs[seg.Linedef]
	side := int(seg.Direction)
	if side < 0 || side > 1 {
		side = 0
	}
	sideNum := ld.SideNum[side]
	if sideNum < 0 || int(sideNum) >= len(g.m.Sidedefs) {
		return -1
	}
	sec := int(g.m.Sidedefs[int(sideNum)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1
	}
	return sec
}

func pointOnWorldSegment(p, a, b worldPt) bool {
	const eps = 1e-6
	cross := orient2D(a, b, p)
	if math.Abs(cross) > eps {
		return false
	}
	minX := math.Min(a.x, b.x) - eps
	maxX := math.Max(a.x, b.x) + eps
	minY := math.Min(a.y, b.y) - eps
	maxY := math.Max(a.y, b.y) + eps
	return p.x >= minX && p.x <= maxX && p.y >= minY && p.y <= maxY
}

func pointInWorldPoly(p worldPt, poly []worldPt) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a := poly[j]
		b := poly[i]
		if pointOnWorldSegment(p, a, b) {
			return true
		}
		yiAbove := a.y > p.y
		yjAbove := b.y > p.y
		if yiAbove == yjAbove {
			continue
		}
		xInt := (b.x-a.x)*(p.y-a.y)/(b.y-a.y) + a.x
		if xInt > p.x {
			inside = !inside
		}
	}
	return inside
}

type polyBBox struct {
	minX int
	minY int
	maxX int
	maxY int
}

type worldBBox struct {
	minX float64
	minY float64
	maxX float64
	maxY float64
}

func worldPolyBBox(poly []worldPt) worldBBox {
	b := worldBBox{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for _, v := range poly {
		if v.x < b.minX {
			b.minX = v.x
		}
		if v.y < b.minY {
			b.minY = v.y
		}
		if v.x > b.maxX {
			b.maxX = v.x
		}
		if v.y > b.maxY {
			b.maxY = v.y
		}
	}
	return b
}

func nodeBBoxToWorld(bb [4]int16) (worldBBox, bool) {
	// Doom node bbox order is top, bottom, left, right.
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	b := worldBBox{
		minX: math.Min(left, right),
		minY: math.Min(bottom, top),
		maxX: math.Max(left, right),
		maxY: math.Max(bottom, top),
	}
	if !isFinite(b.minX) || !isFinite(b.minY) || !isFinite(b.maxX) || !isFinite(b.maxY) {
		return worldBBox{}, false
	}
	if b.minX > b.maxX || b.minY > b.maxY {
		return worldBBox{}, false
	}
	return b, true
}

func clipWorldPolyByBBox(poly []worldPt, b worldBBox) []worldPt {
	if len(poly) < 3 {
		return nil
	}
	const eps = 1e-6
	clip := func(in []worldPt, inside func(worldPt) bool, intersect func(worldPt, worldPt) worldPt) []worldPt {
		if len(in) < 3 {
			return nil
		}
		out := make([]worldPt, 0, len(in)+2)
		prev := in[len(in)-1]
		prevIn := inside(prev)
		for _, cur := range in {
			curIn := inside(cur)
			if prevIn && curIn {
				out = appendWorldPtUnique(out, cur, eps)
			} else if prevIn && !curIn {
				out = appendWorldPtUnique(out, intersect(prev, cur), eps)
			} else if !prevIn && curIn {
				out = appendWorldPtUnique(out, intersect(prev, cur), eps)
				out = appendWorldPtUnique(out, cur, eps)
			}
			prev = cur
			prevIn = curIn
		}
		if len(out) >= 2 && nearlyEqualWorldPt(out[0], out[len(out)-1], eps) {
			out = out[:len(out)-1]
		}
		if len(out) < 3 {
			return nil
		}
		return out
	}

	out := poly
	out = clip(out, func(p worldPt) bool { return p.x >= b.minX-eps }, func(a, c worldPt) worldPt {
		den := c.x - a.x
		if math.Abs(den) < 1e-12 {
			return worldPt{x: b.minX, y: a.y}
		}
		t := (b.minX - a.x) / den
		return worldPt{x: b.minX, y: a.y + (c.y-a.y)*t}
	})
	out = clip(out, func(p worldPt) bool { return p.x <= b.maxX+eps }, func(a, c worldPt) worldPt {
		den := c.x - a.x
		if math.Abs(den) < 1e-12 {
			return worldPt{x: b.maxX, y: a.y}
		}
		t := (b.maxX - a.x) / den
		return worldPt{x: b.maxX, y: a.y + (c.y-a.y)*t}
	})
	out = clip(out, func(p worldPt) bool { return p.y >= b.minY-eps }, func(a, c worldPt) worldPt {
		den := c.y - a.y
		if math.Abs(den) < 1e-12 {
			return worldPt{x: a.x, y: b.minY}
		}
		t := (b.minY - a.y) / den
		return worldPt{x: a.x + (c.x-a.x)*t, y: b.minY}
	})
	out = clip(out, func(p worldPt) bool { return p.y <= b.maxY+eps }, func(a, c worldPt) worldPt {
		den := c.y - a.y
		if math.Abs(den) < 1e-12 {
			return worldPt{x: a.x, y: b.maxY}
		}
		t := (b.maxY - a.y) / den
		return worldPt{x: a.x + (c.x-a.x)*t, y: b.maxY}
	})
	if len(out) < 3 || math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) subSectorSegBBox(ss int) (worldBBox, bool) {
	if g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) {
		return worldBBox{}, false
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount == 0 {
		return worldBBox{}, false
	}
	b := worldBBox{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		for _, vi := range []uint16{sg.StartVertex, sg.EndVertex} {
			if int(vi) >= len(g.m.Vertexes) {
				continue
			}
			v := g.m.Vertexes[vi]
			x := float64(v.X)
			y := float64(v.Y)
			if x < b.minX {
				b.minX = x
			}
			if y < b.minY {
				b.minY = y
			}
			if x > b.maxX {
				b.maxX = x
			}
			if y > b.maxY {
				b.maxY = y
			}
		}
	}
	if !isFinite(b.minX) || !isFinite(b.minY) || !isFinite(b.maxX) || !isFinite(b.maxY) {
		return worldBBox{}, false
	}
	if b.minX > b.maxX || b.minY > b.maxY {
		return worldBBox{}, false
	}
	return b, true
}

func nearlyEqualWorldPt(a, b worldPt, eps float64) bool {
	return math.Abs(a.x-b.x) <= eps && math.Abs(a.y-b.y) <= eps
}

func appendWorldPtUnique(dst []worldPt, p worldPt, eps float64) []worldPt {
	if len(dst) > 0 && nearlyEqualWorldPt(dst[len(dst)-1], p, eps) {
		return dst
	}
	return append(dst, p)
}

func clipWorldPolyByDivline(poly []worldPt, a, b worldPt, side int) []worldPt {
	if len(poly) < 3 {
		return nil
	}
	const eps = 1e-6
	inside := func(p worldPt) bool {
		o := orient2D(a, b, p)
		if side == 0 {
			return o <= eps
		}
		return o >= -eps
	}
	intersect := func(p1, p2 worldPt) (worldPt, bool) {
		o1 := orient2D(a, b, p1)
		o2 := orient2D(a, b, p2)
		den := o1 - o2
		if math.Abs(den) < 1e-12 {
			return worldPt{}, false
		}
		t := o1 / den
		return worldPt{
			x: p1.x + (p2.x-p1.x)*t,
			y: p1.y + (p2.y-p1.y)*t,
		}, true
	}

	out := make([]worldPt, 0, len(poly)+2)
	prev := poly[len(poly)-1]
	prevIn := inside(prev)
	for _, cur := range poly {
		curIn := inside(cur)
		if prevIn && curIn {
			out = appendWorldPtUnique(out, cur, eps)
		} else if prevIn && !curIn {
			if ip, ok := intersect(prev, cur); ok {
				out = appendWorldPtUnique(out, ip, eps)
			}
		} else if !prevIn && curIn {
			if ip, ok := intersect(prev, cur); ok {
				out = appendWorldPtUnique(out, ip, eps)
			}
			out = appendWorldPtUnique(out, cur, eps)
		}
		prev = cur
		prevIn = curIn
	}
	if len(out) >= 2 && nearlyEqualWorldPt(out[0], out[len(out)-1], eps) {
		out = out[:len(out)-1]
	}
	if len(out) < 3 {
		return nil
	}
	if math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) subSectorSeedPoint(ss int, fallback []worldPt) (worldPt, bool) {
	if _, cx, cy, ok := g.subSectorVerticesFromSegList(ss); ok {
		return worldPt{x: cx, y: cy}, true
	}
	if _, cx, cy, ok := g.subSectorWorldVertices(ss); ok {
		return worldPt{x: cx, y: cy}, true
	}
	if len(fallback) >= 3 {
		cx, cy := 0.0, 0.0
		for _, p := range fallback {
			cx += p.x
			cy += p.y
		}
		return worldPt{x: cx / float64(len(fallback)), y: cy / float64(len(fallback))}, true
	}
	return worldPt{}, false
}

func (g *game) clipSubSectorPolyBySegBounds(ss int, poly []worldPt) []worldPt {
	if ss < 0 || ss >= len(g.m.SubSectors) || len(poly) < 3 {
		return nil
	}
	seed, ok := g.subSectorSeedPoint(ss, poly)
	if !ok {
		return poly
	}
	sub := g.m.SubSectors[ss]
	out := poly
	const sideEps = 1e-7
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si < 0 || si >= len(g.m.Segs) {
			continue
		}
		sg := g.m.Segs[si]
		if int(sg.StartVertex) >= len(g.m.Vertexes) || int(sg.EndVertex) >= len(g.m.Vertexes) {
			continue
		}
		va := g.m.Vertexes[sg.StartVertex]
		vb := g.m.Vertexes[sg.EndVertex]
		a := worldPt{x: float64(va.X), y: float64(va.Y)}
		b := worldPt{x: float64(vb.X), y: float64(vb.Y)}

		seedSide := orient2D(a, b, seed)
		if math.Abs(seedSide) <= sideEps {
			// Ambiguous seed-on-edge case: choose the side that keeps the larger
			// clipped polygon to avoid precision-driven half-plane flips.
			c0 := clipWorldPolyByDivline(out, a, b, 0)
			c1 := clipWorldPolyByDivline(out, a, b, 1)
			a0 := 0.0
			if len(c0) >= 3 {
				a0 = math.Abs(polygonArea2(c0))
			}
			a1 := 0.0
			if len(c1) >= 3 {
				a1 = math.Abs(polygonArea2(c1))
			}
			switch {
			case a0 == 0 && a1 == 0:
				return nil
			case a1 > a0:
				out = c1
			default:
				out = c0
			}
			continue
		}
		side := 0
		if seedSide > 0 {
			side = 1
		}
		clipped := clipWorldPolyByDivline(out, a, b, side)
		if len(clipped) >= 3 && pointInWorldPoly(seed, clipped) {
			out = clipped
			continue
		}
		if len(clipped) < 3 || !pointInWorldPoly(seed, clipped) {
			alt := clipWorldPolyByDivline(out, a, b, side^1)
			if len(alt) >= 3 {
				clipped = alt
			} else {
				return nil
			}
		}
		out = clipped
	}
	if len(out) < 3 || math.Abs(polygonArea2(out)) < 1e-6 {
		return nil
	}
	return out
}

func (g *game) buildSubSectorPolysFromNodes() {
	if g.m == nil || len(g.m.Nodes) == 0 || len(g.m.SubSectors) == 0 {
		return
	}

	w := math.Max(g.bounds.maxX-g.bounds.minX, 1)
	h := math.Max(g.bounds.maxY-g.bounds.minY, 1)
	pad := math.Max(w, h)*2 + 1024
	root := []worldPt{
		{x: g.bounds.minX - pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.minY - pad},
		{x: g.bounds.maxX + pad, y: g.bounds.maxY + pad},
		{x: g.bounds.minX - pad, y: g.bounds.maxY + pad},
	}

	var walk func(child uint16, poly []worldPt)
	walk = func(child uint16, poly []worldPt) {
		if len(poly) < 3 {
			return
		}
		if child&0x8000 != 0 {
			ss := int(child & 0x7fff)
			if ss < 0 || ss >= len(g.m.SubSectors) {
				return
			}
			g.subSectorPoly[ss] = poly
			return
		}
		ni := int(child)
		if ni < 0 || ni >= len(g.m.Nodes) {
			return
		}
		n := g.m.Nodes[ni]
		a := worldPt{x: float64(n.X), y: float64(n.Y)}
		b := worldPt{x: float64(n.X) + float64(n.DX), y: float64(n.Y) + float64(n.DY)}

		p0 := clipWorldPolyByDivline(poly, a, b, 0)
		if len(p0) >= 3 {
			if bb, ok := nodeBBoxToWorld(n.BBoxR); ok {
				p0 = clipWorldPolyByBBox(p0, bb)
			}
		}
		if len(p0) >= 3 {
			walk(n.ChildID[0], p0)
		}
		p1 := clipWorldPolyByDivline(poly, a, b, 1)
		if len(p1) >= 3 {
			if bb, ok := nodeBBoxToWorld(n.BBoxL); ok {
				p1 = clipWorldPolyByBBox(p1, bb)
			}
		}
		if len(p1) >= 3 {
			walk(n.ChildID[1], p1)
		}
	}

	walk(uint16(len(g.m.Nodes)-1), root)

	for ss := range g.m.SubSectors {
		if len(g.subSectorPoly[ss]) < 3 {
			continue
		}
		if clipped := g.clipSubSectorPolyBySegBounds(ss, g.subSectorPoly[ss]); len(clipped) >= 3 {
			g.subSectorPoly[ss] = clipped
		}
	}
}

type screenPt struct {
	x float64
	y float64
}

func (g *game) flatImage(name string) (*ebiten.Image, bool) {
	key := normalizeFlatName(name)
	if img, ok := g.flatImgCache[key]; ok {
		return img, true
	}
	rgba, ok := g.opts.FlatBank[key]
	if !ok || len(rgba) != 64*64*4 {
		return nil, false
	}
	img := ebiten.NewImage(64, 64)
	img.WritePixels(rgba)
	g.flatImgCache[key] = img
	return img, true
}

func (g *game) flatRGBA(name string) ([]byte, bool) {
	key := normalizeFlatName(name)
	rgba, ok := g.opts.FlatBank[key]
	if !ok || len(rgba) != 64*64*4 {
		return nil, false
	}
	return rgba, true
}

func normalizeFlatName(name string) string {
	out := make([]byte, 0, 8)
	for i := 0; i < len(name) && len(out) < 8; i++ {
		c := name[i]
		if c == 0 {
			break
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}

func isSkyFlatName(name string) bool {
	n := normalizeFlatName(name)
	if n == "" {
		return false
	}
	return strings.Contains(n, "SKY")
}

func (g *game) capturePrevState() {
	g.prevCamX = g.camX
	g.prevCamY = g.camY
	g.prevPX = g.p.x
	g.prevPY = g.p.y
	g.prevAngle = g.p.angle
}

func (g *game) syncRenderState() {
	g.capturePrevState()
	g.renderCamX = g.camX
	g.renderCamY = g.camY
	g.renderPX = float64(g.p.x) / fracUnit
	g.renderPY = float64(g.p.y) / fracUnit
	g.renderAngle = g.p.angle
	g.lastUpdate = time.Now()
}

func (g *game) prepareRenderState() {
	alpha := g.interpAlpha()
	if !g.opts.SourcePortMode {
		alpha = 1
	}
	g.renderCamX = lerp(g.prevCamX, g.camX, alpha)
	g.renderCamY = lerp(g.prevCamY, g.camY, alpha)
	g.renderPX = lerp(float64(g.prevPX)/fracUnit, float64(g.p.x)/fracUnit, alpha)
	g.renderPY = lerp(float64(g.prevPY)/fracUnit, float64(g.p.y)/fracUnit, alpha)
	g.renderAngle = lerpAngle(g.prevAngle, g.p.angle, alpha)
}

func (g *game) interpAlpha() float64 {
	if g.lastUpdate.IsZero() {
		return 1
	}
	dt := time.Since(g.lastUpdate).Seconds()
	step := 1.0 / doomTicsPerSecond
	a := dt / step
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func lerpAngle(a, b uint32, t float64) uint32 {
	d := int64(int32(b - a))
	v := float64(int64(a)) + float64(d)*t
	return uint32(int64(v))
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

func (g *game) segWorldEndpoints(segIdx int) (x1, y1, x2, y2 float64, ok bool) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return 0, 0, 0, 0, false
	}
	sg := g.m.Segs[segIdx]
	if int(sg.StartVertex) >= len(g.m.Vertexes) || int(sg.EndVertex) >= len(g.m.Vertexes) {
		return 0, 0, 0, 0, false
	}
	v1 := g.m.Vertexes[sg.StartVertex]
	v2 := g.m.Vertexes[sg.EndVertex]
	return float64(v1.X), float64(v1.Y), float64(v2.X), float64(v2.Y), true
}

func (g *game) segSectors(segIdx int) (*mapdata.Sector, *mapdata.Sector) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return nil, nil
	}
	sg := g.m.Segs[segIdx]
	li := int(sg.Linedef)
	if li < 0 || li >= len(g.m.Linedefs) {
		return nil, nil
	}
	ld := g.m.Linedefs[li]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	front := g.sectorFromSideNum(ld.SideNum[frontSide])
	back := g.sectorFromSideNum(ld.SideNum[backSide])
	// WAD seg direction can point at the missing side on one-sided linedefs.
	// Treat reversed one-sided segs as solid walls using the existing side.
	if front == nil && back != nil && (ld.SideNum[0] < 0 || ld.SideNum[1] < 0) {
		front = back
		back = nil
	}
	return front, back
}

func (g *game) sectorFromSideNum(side int16) *mapdata.Sector {
	secIdx := g.sectorIndexFromSideNum(side)
	if secIdx < 0 || secIdx >= len(g.m.Sectors) {
		return nil
	}
	return &g.m.Sectors[secIdx]
}

func (g *game) subSectorSectorFromSeg(segIdx int) (int, bool) {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return 0, false
	}
	sg := g.m.Segs[segIdx]
	if int(sg.Linedef) < 0 || int(sg.Linedef) >= len(g.m.Linedefs) {
		return 0, false
	}
	ld := g.m.Linedefs[sg.Linedef]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	if sec := g.sectorIndexFromSideNum(ld.SideNum[frontSide]); sec >= 0 {
		return sec, true
	}
	back := g.sectorIndexFromSideNum(ld.SideNum[backSide])
	if back >= 0 && (ld.SideNum[0] < 0 || ld.SideNum[1] < 0) {
		return back, true
	}
	if back >= 0 {
		return back, true
	}
	return 0, false
}

func (g *game) sectorIndexFromSideNum(side int16) int {
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return -1
	}
	sec := int(g.m.Sidedefs[int(side)].Sector)
	if sec < 0 || sec >= len(g.m.Sectors) {
		return -1
	}
	return sec
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
	camX := g.renderCamX
	camY := g.renderCamY
	viewHalfW := float64(g.viewW) / (2 * g.zoom)
	viewHalfH := float64(g.viewH) / (2 * g.zoom)
	minXf := camX - viewHalfW - margin
	maxXf := camX + viewHalfW + margin
	minYf := camY - viewHalfH - margin
	maxYf := camY + viewHalfH + margin
	if g.rotateView {
		// Conservative culling when rotating: circumscribed circle around the viewport.
		r := math.Hypot(viewHalfW, viewHalfH) + margin
		minXf = camX - r
		maxXf = camX + r
		minYf = camY - r
		maxYf = camY + r
	}
	minX := floatToFixed(minXf)
	maxX := floatToFixed(maxXf)
	minY := floatToFixed(minYf)
	maxY := floatToFixed(maxYf)

	g.visibleBuf = g.visibleBuf[:0]
	// Robust automap visibility: trust line bboxes directly.
	// Some BLOCKMAP data can omit candidates and cause line pop/disappear at seams.
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
		fmt.Sprintf("PROFILE  %s", g.profileLabel()),
		"F1  HELP TOGGLE",
		"F5  DETAIL CYCLE",
		"TAB  WALK/MAP MODE",
		"WALK MODE",
		"WASD  MOVE",
		"ARROWS  TURN/STRAFE(ALT)",
		"CTRL/MOUSE1  FIRE",
		"MAP MODE",
		"Q/E  TURN (MAP MODE)",
		"SHIFT  RUN",
		"SPACE  USE",
		"ARROWS  PAN (FOLLOW OFF)",
		"F  FOLLOW TOGGLE",
		"0  BIG MAP",
		"M  ADD MARK",
		"C  CLEAR MARKS",
		"+/- OR WHEEL  ZOOM",
		"ESC  QUIT",
	}
	if g.opts.SourcePortMode {
		lines = append(lines,
			"SOURCEPORT EXTRAS",
			"R  TOGGLE HEADING-UP",
			"P  TOGGLE WIREFRAME",
			"J  TOGGLE 2D FLOOR FLATS",
			"B  BIG MAP (ALIAS)",
			"HOME  RESET VIEW",
			"O  TOGGLE NORMAL/ALLMAP",
			"I  CYCLE IDDT",
			"L  TOGGLE COLOR MODE",
			"V  TOGGLE THING LEGEND",
		)
	} else {
		lines = append(lines,
			"DOOM PARITY NOTES",
			"ONLY CORE CONTROLS ENABLED",
			"USE -sourceport-mode FOR EXTRAS",
		)
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
