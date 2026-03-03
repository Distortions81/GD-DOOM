package automap

import (
	"fmt"
	"image/color"
	"math"
	"sort"
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

	mode          viewMode
	walkRender    walkRendererMode
	followMode    bool
	rotateView    bool
	showHelp      bool
	pseudo3D      bool
	parity        automapParityState
	showGrid      bool
	showLegend    bool
	showMapFloors bool
	bigMap        bool
	savedView     savedMapView
	marks         []mapMark
	nextMarkID    int
	p             player
	localSlot     int
	peerStarts    []playerStart

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

	lastMouseX   int
	mouseLookSet bool

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

	mapFloorLayer *ebiten.Image
	mapFloorPix   []byte
	mapFloorW     int
	mapFloorH     int
	flatImgCache  map[string]*ebiten.Image
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
		showMapFloors: opts.SourcePortMode && opts.MapFloorTex2D,
		bigMap:        false,
		marks:         make([]mapMark, 0, 16),
		nextMarkID:    1,
		p:             p,
		localSlot:     localSlot,
		peerStarts:    nonLocalStarts(starts, localSlot),
	}
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
			g.setHUDMessage("Automap Closed", 35)
		}
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.rotateView = !g.rotateView
		if g.rotateView {
			g.setHUDMessage("Rotate Mode ON", 70)
		} else {
			g.setHUDMessage("Rotate Mode OFF", 70)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.showHelp = !g.showHelp
	}
	if g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.pseudo3D = !g.pseudo3D
		if g.pseudo3D {
			g.walkRender = walkRendererPseudo
			g.setHUDMessage("Pseudo3D ON", 70)
		} else {
			g.walkRender = walkRendererDoomBasic
			g.setHUDMessage("Pseudo3D OFF", 70)
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
		if g.mouseLookSet {
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
		if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
			g.applyCheatLevel((g.cheatLevel+1)%4, true)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
			g.invulnerable = !g.invulnerable
			if g.invulnerable {
				g.setHUDMessage("IDDQD ON", 70)
			} else {
				g.setHUDMessage("IDDQD OFF", 70)
			}
		}
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
		if inpututil.IsKeyJustPressed(ebiten.KeyJ) {
			g.showMapFloors = !g.showMapFloors
			if g.showMapFloors {
				g.setHUDMessage("Map Floor Textures ON", 70)
			} else {
				g.setHUDMessage("Map Floor Textures OFF", 70)
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
			ebitenutil.DebugPrintAt(screen, "renderer=pseudo3d | P toggle | TAB automap", 12, 28)
		} else {
			g.prepareRenderState()
			g.drawDoomBasic3D(screen)
			ebitenutil.DebugPrintAt(screen, fmt.Sprintf("profile=%s", g.profileLabel()), 12, 28)
			ebitenutil.DebugPrintAt(screen, "renderer=doom-basic | P pseudo3d | TAB automap", 12, 12)
			ebitenutil.DebugPrintAt(screen, "TAB open automap | F1 help", 12, 44)
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
	if g.showMapFloors && len(g.opts.FlatBank) > 0 {
		g.drawMapFloorTextures2D(screen)
	}
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

	depthPix := make([]float64, g.viewW*g.viewH)
	for i := range depthPix {
		depthPix[i] = math.Inf(1)
	}
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
		if f1 <= near && f2 <= near {
			continue
		}

		if f1 <= near || f2 <= near {
			t := (near - f1) / (f2 - f1)
			if f1 < near {
				f1 = near
				s1 = s1 + (s2-s1)*t
			} else {
				f2 = near
				s2 = s1 + (s2-s1)*t
			}
		}
		if f1 <= near || f2 <= near {
			continue
		}
		sx1 := float64(g.viewW)/2 - (s1/f1)*focal
		sx2 := float64(g.viewW)/2 - (s2/f2)*focal

		base, _ := g.decisionStyle(d)
		baseRGBA := color.RGBAModel.Convert(base).(color.RGBA)
		front, back := g.segSectors(si)
		if front == nil {
			continue
		}
		if back == nil {
			g.drawBasicWallColumnRange(screen, depthPix, sx1, sx2, f1, f2, float64(front.CeilingHeight), float64(front.FloorHeight), eyeZ, focal, baseRGBA)
			continue
		}
		openTop := math.Min(float64(front.CeilingHeight), float64(back.CeilingHeight))
		openBottom := math.Max(float64(front.FloorHeight), float64(back.FloorHeight))
		if float64(front.CeilingHeight) > openTop {
			g.drawBasicWallColumnRange(screen, depthPix, sx1, sx2, f1, f2, float64(front.CeilingHeight), openTop, eyeZ, focal, baseRGBA)
		}
		if float64(front.FloorHeight) < openBottom {
			g.drawBasicWallColumnRange(screen, depthPix, sx1, sx2, f1, f2, openBottom, float64(front.FloorHeight), eyeZ, focal, baseRGBA)
		}
	}
}

func (g *game) drawBasicWallColumnRange(screen *ebiten.Image, depthPix []float64, sx1, sx2, f1, f2, zTop, zBot, eyeZ, focal float64, base color.RGBA) {
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

		if f1 <= near && f2 <= near {
			continue
		}
		if f1 <= near || f2 <= near {
			t := (near - f1) / (f2 - f1)
			if f1 < near {
				f1 = near
				s1 = s1 + (s2-s1)*t
			} else {
				f2 = near
				s2 = s1 + (s2-s1)*t
			}
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

		// Use right-handed screen projection so turn/mouselook directions match controls.
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
	g.viewW = max(outsideWidth, 1)
	g.viewH = max(outsideHeight, 1)
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

func (g *game) drawMapFloorTextures2D(screen *ebiten.Image) {
	if g.m == nil || len(g.m.SubSectors) == 0 || len(g.m.Segs) == 0 || len(g.opts.FlatBank) == 0 {
		return
	}
	if g.flatImgCache == nil {
		g.flatImgCache = make(map[string]*ebiten.Image, len(g.opts.FlatBank))
	}
	triOpts := &ebiten.DrawTrianglesOptions{
		Filter:  ebiten.FilterNearest,
		Address: ebiten.AddressRepeat,
	}
	triIdx := []uint16{0, 1, 2}
	for ss := range g.m.SubSectors {
		poly, worldVerts, cx, cy, _, ok := g.subSectorScreenPolygon(ss)
		if !ok {
			continue
		}
		secIdx, ok := g.subSectorSectorIndex(ss)
		if !ok || secIdx < 0 || secIdx >= len(g.m.Sectors) {
			secIdx = g.sectorAt(int64(cx*fracUnit), int64(cy*fracUnit))
			if secIdx < 0 || secIdx >= len(g.m.Sectors) {
				continue
			}
		}
		flatName := g.m.Sectors[secIdx].FloorPic
		flatImg, ok := g.flatImage(flatName)
		if !ok || flatImg == nil {
			continue
		}
		if len(poly) < 3 {
			continue
		}
		v0 := poly[0]
		w0 := worldVerts[0]
		for i := 1; i+1 < len(poly); i++ {
			v1 := poly[i]
			v2 := poly[i+1]
			w1 := worldVerts[i]
			w2 := worldVerts[i+1]
			verts := []ebiten.Vertex{
				{
					DstX:   float32(v0.x),
					DstY:   float32(v0.y),
					SrcX:   float32(w0.x),
					SrcY:   float32(w0.y),
					ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
				},
				{
					DstX:   float32(v1.x),
					DstY:   float32(v1.y),
					SrcX:   float32(w1.x),
					SrcY:   float32(w1.y),
					ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
				},
				{
					DstX:   float32(v2.x),
					DstY:   float32(v2.y),
					SrcX:   float32(w2.x),
					SrcY:   float32(w2.y),
					ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
				},
			}
			screen.DrawTriangles(verts, triIdx, flatImg, triOpts)
		}
	}
}

type worldPt struct {
	x float64
	y float64
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
	chain, closed := chainSubsectorEdges(edges)
	if !closed {
		chain = rawSubsectorVertexOrder(g.m, sub)
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
	si := int(sub.FirstSeg)
	if si < 0 || si >= len(g.m.Segs) {
		return 0, false
	}
	sg := g.m.Segs[si]
	if int(sg.Linedef) >= len(g.m.Linedefs) {
		return 0, false
	}
	ld := g.m.Linedefs[sg.Linedef]
	side := ld.SideNum[0]
	if sg.Direction != 0 {
		side = ld.SideNum[1]
	}
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		// Fallback to whatever side exists.
		if ld.SideNum[0] >= 0 && int(ld.SideNum[0]) < len(g.m.Sidedefs) {
			side = ld.SideNum[0]
		} else if ld.SideNum[1] >= 0 && int(ld.SideNum[1]) < len(g.m.Sidedefs) {
			side = ld.SideNum[1]
		} else {
			return 0, false
		}
	}
	sec := int(g.m.Sidedefs[side].Sector)
	return sec, sec >= 0 && sec < len(g.m.Sectors)
}

type polyBBox struct {
	minX int
	minY int
	maxX int
	maxY int
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
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return nil
	}
	sec := g.m.Sidedefs[int(side)].Sector
	if int(sec) >= len(g.m.Sectors) {
		return nil
	}
	return &g.m.Sectors[sec]
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
		"TAB  WALK/MAP MODE",
		"WASD  MOVE",
		"Q/E  TURN (MAP MODE)",
		"SHIFT  RUN",
		"SPACE  USE",
		"ARROWS  PAN (FOLLOW OFF)",
		"F  FOLLOW TOGGLE",
		"0  BIG MAP",
		"G  GRID TOGGLE",
		"M  ADD MARK",
		"C  CLEAR MARKS",
		"+/- OR WHEEL  ZOOM",
		"ESC  QUIT",
	}
	if g.opts.SourcePortMode {
		lines = append(lines,
			"SOURCEPORT EXTRAS",
			"R  ROTATE/FOLLOW HEADING",
			"P  TOGGLE WALK RENDERER",
			"B  BIG MAP (ALIAS)",
			"O  TOGGLE NORMAL/ALLMAP",
			"I  CYCLE IDDT",
			"L  TOGGLE COLOR MODE",
			"V  TOGGLE THING LEGEND",
			"J  TOGGLE 2D FLOOR FLATS",
			"HOME  RESET VIEW",
		)
	} else {
		lines = append(lines,
			"DOOM PARITY NOTES",
			"R/B/O/I/L/HOME DISABLED",
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
