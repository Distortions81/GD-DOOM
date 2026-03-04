package automap

import (
	"errors"
	"fmt"
	"image/color"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/music"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

const (
	bootSplashHoldTics = 2 * doomTicsPerSecond
	meltVirtualH       = 200
	quantizeLUTW       = 256
	quantizeLUTH       = 16
	// Sourceport melt uses Doom-like 2-pixel column pairs over a 320-wide
	// virtual layout, i.e. 160 moving slices.
	sourcePortMeltInitCols = 160
	sourcePortMeltMoveCols = sourcePortMeltInitCols

	intermissionPhaseWaitTics      = 8
	intermissionEnteringWaitTics   = doomTicsPerSecond
	intermissionYouAreHereWaitTics = doomTicsPerSecond * 2
	intermissionSkipInputDelayTics = doomTicsPerSecond / 3
	intermissionSkipExitHoldTics   = 12
	intermissionCounterSoundPeriod = 6
)

var faithfulPaletteShaderSrc = []byte(`//kage:unit pixels
package main

var GammaRatio float
var EnableQuantize float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	c := imageSrc0At(texCoord)
	if c.a <= 0.0 {
		return vec4(0.0, 0.0, 0.0, 1.0)
	}
	outRGB := c.rgb
	if EnableQuantize >= 0.5 {
		// 16x16x16 RGB cube LUT flattened into a 256x16 block at source1 top-left.
		ri := int(clamp(c.r*15.0+0.5, 0.0, 15.0))
		gi := int(clamp(c.g*15.0+0.5, 0.0, 15.0))
		bi := int(clamp(c.b*15.0+0.5, 0.0, 15.0))
		idx := ri + gi*16 + bi*256
		lx := float(idx%256) + 0.5
		ly := float(idx/256) + 0.5
		outRGB = imageSrc1At(vec2(lx, ly) + imageSrc0Origin()).rgb
	}
	// Apply display gamma after optional quantization.
	post := vec3(pow(outRGB.r, GammaRatio), pow(outRGB.g, GammaRatio), pow(outRGB.b, GammaRatio))
	return vec4(post, 1.0)
}
`)

var faithfulPaletteNoGammaShaderSrc = []byte(`//kage:unit pixels
package main

var EnableQuantize float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	c := imageSrc0At(texCoord)
	if c.a <= 0.0 {
		return vec4(0.0, 0.0, 0.0, 1.0)
	}
	outRGB := c.rgb
	if EnableQuantize >= 0.5 {
		// 16x16x16 RGB cube LUT flattened into a 256x16 block at source1 top-left.
		ri := int(clamp(c.r*15.0+0.5, 0.0, 15.0))
		gi := int(clamp(c.g*15.0+0.5, 0.0, 15.0))
		bi := int(clamp(c.b*15.0+0.5, 0.0, 15.0))
		idx := ri + gi*16 + bi*256
		lx := float(idx%256) + 0.5
		ly := float(idx/256) + 0.5
		outRGB = imageSrc1At(vec2(lx, ly) + imageSrc0Origin()).rgb
	}
	return vec4(outRGB, 1.0)
}
`)

var crtPostShaderSrc = []byte(`//kage:unit pixels
package main

var Time float

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	size := imageSrc0Size()
	if size.x <= 0.0 || size.y <= 0.0 {
		return vec4(0.0, 0.0, 0.0, 1.0)
	}
	origin := imageSrc0Origin()
	local := texCoord - origin
	uv := local / size
	p := uv*2.0 - vec2(1.0, 1.0)
	p *= 1.0 + 0.04*dot(p, p)
	uv = (p + vec2(1.0, 1.0)) * 0.5
	if uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0 {
		return vec4(0.0, 0.0, 0.0, 1.0)
	}
	srcPos := uv*size + origin
	c := imageSrc0At(srcPos)
	scan := 0.90 + 0.10*sin((uv.y*size.y+Time*2.0)*3.14159265)
	maskPhase := floor(mod(uv.x*size.x, 3.0))
	mask := vec3(0.90, 0.90, 0.90)
	if maskPhase < 1.0 {
		mask = vec3(1.00, 0.88, 0.88)
	} else if maskPhase < 2.0 {
		mask = vec3(0.88, 1.00, 0.88)
	} else {
		mask = vec3(0.88, 0.88, 1.00)
	}
	v := uv*(1.0-uv)
	vig := clamp(pow(v.x*v.y*20.0, 0.35), 0.0, 1.0)
	outRGB := c.rgb * scan * mask * (0.65 + 0.35*vig)
	return vec4(outRGB, 1.0)
}
`)

type transitionKind int

const (
	transitionNone transitionKind = iota
	transitionBoot
	transitionLevel
)

type sessionTransition struct {
	kind        transitionKind
	pending     bool
	initialized bool
	holdTics    int
	width       int
	height      int
	y           []int
	fromPix     []byte
	toPix       []byte
	workPix     []byte
	from        *ebiten.Image
	to          *ebiten.Image
	work        *ebiten.Image
}

type intermissionStats struct {
	mapName      mapdata.MapName
	nextMapName  mapdata.MapName
	killsPct     int
	itemsPct     int
	secretsPct   int
	timeSec      int
	killsFound   int
	killsTotal   int
	itemsFound   int
	itemsTotal   int
	secretsFound int
	secretsTotal int
}

type sessionIntermission struct {
	active            bool
	phase             int
	waitTic           int
	tic               int
	stageSoundCounter int
	show              intermissionStats
	target            intermissionStats
	nextMap           *mapdata.Map
}

const (
	intermissionPhaseKills = iota
	intermissionPhaseItems
	intermissionPhaseSecrets
	intermissionPhaseTime
	intermissionPhaseEntering
	intermissionPhaseYouAreHere
)

type sessionGame struct {
	g               *game
	current         mapdata.MapName
	currentTemplate *mapdata.Map
	opts            Options
	settings        sessionPersistentSettings
	nextMap         NextMapFunc
	err             error
	musicDriver     *music.Driver
	musicPlayer     *music.ChunkPlayer
	faithfulSurface *ebiten.Image
	faithfulPost    *ebiten.Image
	faithfulLUT     *ebiten.Image
	faithfulLUTPix  []byte
	faithfulLUTW    int
	faithfulLUTH    int
	faithfulShader  *ebiten.Shader
	noGammaShader   *ebiten.Shader
	crtShader       *ebiten.Shader
	crtPost         *ebiten.Image
	presentSurface  *ebiten.Image
	lastFrame       *ebiten.Image
	bootSplashImage *ebiten.Image
	interPatchCache map[string]*ebiten.Image
	transition      sessionTransition
	intermission    sessionIntermission
}

func cloneMapForRestart(src *mapdata.Map) *mapdata.Map {
	if src == nil {
		return nil
	}
	dup := *src
	dup.Things = append([]mapdata.Thing(nil), src.Things...)
	dup.Vertexes = append([]mapdata.Vertex(nil), src.Vertexes...)
	dup.Linedefs = append([]mapdata.Linedef(nil), src.Linedefs...)
	dup.Sidedefs = append([]mapdata.Sidedef(nil), src.Sidedefs...)
	dup.Sectors = append([]mapdata.Sector(nil), src.Sectors...)
	dup.Segs = append([]mapdata.Seg(nil), src.Segs...)
	dup.SubSectors = append([]mapdata.SubSector(nil), src.SubSectors...)
	dup.Nodes = append([]mapdata.Node(nil), src.Nodes...)
	dup.Reject = append([]byte(nil), src.Reject...)
	dup.Blockmap = append([]int16(nil), src.Blockmap...)
	if src.RejectMatrix != nil {
		rm := *src.RejectMatrix
		rm.Data = append([]byte(nil), src.RejectMatrix.Data...)
		dup.RejectMatrix = &rm
	}
	if src.BlockMap != nil {
		bm := *src.BlockMap
		bm.Offsets = append([]uint16(nil), src.BlockMap.Offsets...)
		bm.Cells = make([][]int16, len(src.BlockMap.Cells))
		for i, cell := range src.BlockMap.Cells {
			bm.Cells[i] = append([]int16(nil), cell...)
		}
		dup.BlockMap = &bm
	}
	return &dup
}

type sessionPersistentSettings struct {
	detailLevel      int
	rotateView       bool
	mouseLook        bool
	walkRender       walkRendererMode
	alwaysRun        bool
	autoWeaponSwitch bool
	lineColorMode    string
	showLegend       bool
	mapTexDiag       bool
	floor2DPath      floor2DPathMode
	paletteLUT       bool
	gammaLevel       int
	crtEnabled       bool
	reveal           revealMode
	iddt             int
}

func clampDetailLevelForMode(level int, sourcePort bool) int {
	if sourcePort {
		if len(sourcePortDetailDivisors) == 0 {
			return 0
		}
		if level < 0 {
			return 0
		}
		maxLevel := len(sourcePortDetailDivisors) - 1
		if level > maxLevel {
			return maxLevel
		}
		return level
	}
	if len(detailPresets) == 0 {
		return 0
	}
	if level < 0 {
		return 0
	}
	maxLevel := len(detailPresets) - 1
	if level > maxLevel {
		return maxLevel
	}
	return level
}

func normalizeFloor2DPath(path floor2DPathMode) floor2DPathMode {
	switch path {
	case floor2DPathRasterized, floor2DPathCached, floor2DPathSubsector:
		return path
	default:
		return floor2DPathRasterized
	}
}

func normalizeRevealForMode(mode revealMode, sourcePort bool) revealMode {
	switch mode {
	case revealNormal, revealAllMap:
		return mode
	default:
		if sourcePort {
			return revealAllMap
		}
		return revealNormal
	}
}

func clampIDDT(v int) int {
	if v < 0 {
		return 0
	}
	if v > 2 {
		return 2
	}
	return v
}

func clampGamma(level int) int {
	if level < 0 {
		return 0
	}
	maxLevel := len(gammaTargets) - 1
	if maxLevel < 0 {
		return 0
	}
	if level > maxLevel {
		return maxLevel
	}
	return level
}

func (sg *sessionGame) capturePersistentSettings() {
	if sg == nil || sg.g == nil {
		return
	}
	g := sg.g
	sg.settings = sessionPersistentSettings{
		detailLevel:      g.detailLevel,
		rotateView:       g.rotateView,
		mouseLook:        g.opts.MouseLook,
		walkRender:       g.walkRender,
		alwaysRun:        g.alwaysRun,
		autoWeaponSwitch: g.autoWeaponSwitch,
		lineColorMode:    g.opts.LineColorMode,
		showLegend:       g.showLegend,
		mapTexDiag:       g.mapTexDiag,
		floor2DPath:      g.floor2DPath,
		paletteLUT:       g.paletteLUTEnabled,
		gammaLevel:       g.gammaLevel,
		crtEnabled:       g.crtEnabled,
		reveal:           g.parity.reveal,
		iddt:             g.parity.iddt,
	}
}

func (sg *sessionGame) applyPersistentSettingsToOptions() {
	sg.opts.MouseLook = sg.settings.mouseLook
	sg.opts.AlwaysRun = sg.settings.alwaysRun
	sg.opts.AutoWeaponSwitch = sg.settings.autoWeaponSwitch
	sg.opts.LineColorMode = sg.settings.lineColorMode
}

func (sg *sessionGame) applyPersistentSettingsToGame(g *game) {
	if sg == nil || g == nil {
		return
	}
	s := sg.settings
	g.detailLevel = clampDetailLevelForMode(s.detailLevel, g.opts.SourcePortMode)
	g.rotateView = s.rotateView
	g.opts.MouseLook = s.mouseLook
	g.alwaysRun = s.alwaysRun
	g.autoWeaponSwitch = s.autoWeaponSwitch
	g.opts.LineColorMode = s.lineColorMode
	g.showLegend = s.showLegend
	g.mapTexDiag = s.mapTexDiag
	g.floor2DPath = normalizeFloor2DPath(s.floor2DPath)
	g.paletteLUTEnabled = s.paletteLUT && g.opts.KageShader && len(g.opts.DoomPaletteRGBA) == 256*4
	g.gammaLevel = clampGamma(s.gammaLevel)
	g.crtEnabled = s.crtEnabled && g.opts.KageShader
	g.parity.reveal = normalizeRevealForMode(s.reveal, g.opts.SourcePortMode)
	g.parity.iddt = clampIDDT(s.iddt)
	if g.opts.SourcePortMode && s.walkRender == walkRendererPseudo {
		g.walkRender = walkRendererPseudo
		g.pseudo3D = true
	} else {
		g.walkRender = walkRendererDoomBasic
		g.pseudo3D = false
	}
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
}

func (sg *sessionGame) rebuildGameWithPersistentSettings(next *mapdata.Map) {
	if sg == nil || next == nil {
		return
	}
	sg.capturePersistentSettings()
	sg.applyPersistentSettingsToOptions()
	ng := newGame(next, sg.opts)
	sg.applyPersistentSettingsToGame(ng)
	sg.g = ng
}

func (sg *sessionGame) restartMapForRespawn() *mapdata.Map {
	if sg == nil || sg.g == nil {
		return nil
	}
	if normalizeGameMode(sg.opts.GameMode) != gameModeSingle {
		return sg.g.m
	}
	return cloneMapForRestart(sg.currentTemplate)
}

func (sg *sessionGame) initMusicPlayback() {
	if sg == nil || sg.opts.MapMusicLoader == nil {
		return
	}
	p, err := music.NewChunkPlayer()
	if err != nil {
		return
	}
	sg.musicPlayer = p
	sg.musicDriver = music.NewDriver(p.SampleRate(), sg.opts.MusicPatchBank)
}

func (sg *sessionGame) closeMusicPlayback() {
	if sg == nil || sg.musicPlayer == nil {
		return
	}
	_ = sg.musicPlayer.Close()
	sg.musicPlayer = nil
}

func (sg *sessionGame) stopAndClearMusic() {
	if sg == nil || sg.musicPlayer == nil {
		return
	}
	_ = sg.musicPlayer.Stop()
	_ = sg.musicPlayer.ClearBuffer()
}

func (sg *sessionGame) playMusicForMap(name mapdata.MapName) {
	if sg == nil || sg.musicPlayer == nil || sg.musicDriver == nil || sg.opts.MapMusicLoader == nil {
		return
	}
	sg.stopAndClearMusic()
	data, err := sg.opts.MapMusicLoader(string(name))
	if err != nil || len(data) == 0 {
		return
	}
	sg.musicDriver.Reset()
	chunk, err := sg.musicDriver.RenderMUSS16LE(data)
	if err != nil || len(chunk) == 0 {
		return
	}
	_ = sg.musicPlayer.EnqueueBytesS16LE(chunk)
	_ = sg.musicPlayer.Start()
}

func RunAutomap(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	opts, windowW, windowH := normalizeRunDimensions(opts)
	sg := &sessionGame{
		g:               newGame(m, opts),
		current:         m.Name,
		currentTemplate: cloneMapForRestart(m),
		opts:            opts,
		nextMap:         nextMap,
	}
	sg.initMusicPlayback()
	defer sg.closeMusicPlayback()
	sg.playMusicForMap(m.Name)
	sg.capturePersistentSettings()
	if sg.shouldShowBootSplash() {
		sg.queueTransition(transitionBoot, bootSplashHoldTics)
	}
	sg.initFaithfulPalettePost()
	ebiten.SetTPS(doomTicsPerSecond)
	ebiten.SetVsyncEnabled(!opts.NoVsync)
	if opts.SourcePortMode {
		ebiten.SetWindowSize(opts.Width, opts.Height)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	} else {
		ebiten.SetWindowSize(windowW, windowH)
		// Faithful mode uses fixed integer scaling and aspect, so keep a fixed window.
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	}
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	ebiten.SetScreenClearedEveryFrame(false)
	if err := ebiten.RunGame(sg); err != nil {
		if errors.Is(err, ebiten.Termination) {
			if p := sg.opts.RecordDemoPath; p != "" {
				if werr := SaveDemoScript(p, sg.g.demoRecord); werr != nil {
					return fmt.Errorf("write demo recording: %w", werr)
				}
				fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(sg.g.demoRecord))
			}
			if sg.err != nil {
				return sg.err
			}
			return nil
		}
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sg.opts.RecordDemoPath; p != "" {
		if werr := SaveDemoScript(p, sg.g.demoRecord); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(sg.g.demoRecord))
	}
	return sg.err
}

func (sg *sessionGame) Update() error {
	if sg.transitionActive() {
		if inpututil.IsKeyJustPressed(ebiten.KeyF4) || inpututil.IsKeyJustPressed(ebiten.KeyF10) {
			return ebiten.Termination
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) &&
			(ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)) {
			return ebiten.Termination
		}
		if sg.transition.kind == transitionBoot && sg.transition.holdTics > 0 && anyIntermissionSkipInput() {
			sg.transition.holdTics = 0
		}
		sg.tickTransition()
		return nil
	}
	if sg.intermission.active {
		if inpututil.IsKeyJustPressed(ebiten.KeyF4) || inpututil.IsKeyJustPressed(ebiten.KeyF10) {
			return ebiten.Termination
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) &&
			(ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)) {
			return ebiten.Termination
		}
		if sg.tickIntermission() {
			sg.finishIntermission()
		}
		return nil
	}

	err := sg.g.Update()
	if err == nil {
		if sg.g.levelRestartRequested {
			sg.stopAndClearMusic()
			sg.rebuildGameWithPersistentSettings(sg.restartMapForRespawn())
			sg.playMusicForMap(sg.g.m.Name)
			ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
			sg.queueTransition(transitionLevel, 0)
		}
		return nil
	}
	if !errors.Is(err, ebiten.Termination) {
		sg.err = err
		return ebiten.Termination
	}
	if !sg.g.levelExitRequested {
		return ebiten.Termination
	}
	if sg.nextMap == nil {
		return ebiten.Termination
	}
	next, nextName, nerr := sg.nextMap(sg.current, sg.g.secretLevelExit)
	if nerr != nil {
		sg.err = nerr
		return ebiten.Termination
	}
	sg.startIntermission(next, nextName)
	return nil
}

func (sg *sessionGame) Draw(screen *ebiten.Image) {
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	tw := sw
	th := sh
	if sg.transitionActive() {
		if sg.opts.SourcePortMode && sg.transition.initialized &&
			(sg.transition.width != tw || sg.transition.height != th) {
			// View size changed while transitioning; rebuild transition buffers.
			sg.transition.initialized = false
			sg.transition.pending = true
			sg.transition.y = nil
		}
		sg.ensureTransitionReady(tw, th)
		if sg.transition.initialized {
			sg.drawTransitionFrame(screen, sw, sh)
			return
		}
		sg.clearTransition()
	}
	if sg.intermission.active {
		sg.drawIntermission(screen)
		sg.captureLastFrame(screen)
		return
	}
	if sg.opts.SourcePortMode {
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sg.g.viewW || sg.presentSurface.Bounds().Dy() != sg.g.viewH {
			sg.presentSurface = ebiten.NewImage(max(sg.g.viewW, 1), max(sg.g.viewH, 1))
		}
		sg.g.Draw(sg.presentSurface)
		src := sg.presentSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.presentSurface)
		}
		sg.drawSourcePortPresented(screen, src, sw, sh)
		sg.captureLastFrame(src)
		return
	}
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sw || sg.presentSurface.Bounds().Dy() != sh {
		sg.presentSurface = ebiten.NewImage(sw, sh)
	}
	sg.drawGamePresented(sg.presentSurface, sg.g)
	screen.DrawImage(sg.presentSurface, nil)
}

func (sg *sessionGame) drawGamePresented(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
	if !sg.opts.SourcePortMode {
		vw := max(g.viewW, 1)
		vh := max(g.viewH, 1)
		if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
			sg.faithfulSurface = ebiten.NewImage(vw, vh)
		}
		g.Draw(sg.faithfulSurface)
		src := sg.faithfulSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
		}
		sg.drawFaithfulPresented(dst, src)
		sg.captureLastFrame(src)
		return
	}
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != g.viewW || sg.presentSurface.Bounds().Dy() != g.viewH {
		sg.presentSurface = ebiten.NewImage(max(g.viewW, 1), max(g.viewH, 1))
	}
	g.Draw(sg.presentSurface)
	src := sg.presentSurface
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(sg.presentSurface)
	}
	sg.drawSourcePortPresented(dst, src, max(dst.Bounds().Dx(), 1), max(dst.Bounds().Dy(), 1))
}

func (sg *sessionGame) drawSourcePortPresented(dst, src *ebiten.Image, sw, sh int) {
	if dst == nil || src == nil {
		return
	}
	vw := max(src.Bounds().Dx(), 1)
	vh := max(src.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(sw)/float64(vw), float64(sh)/float64(vh))
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawFaithfulPresented(dst, src *ebiten.Image) {
	if dst == nil || src == nil {
		return
	}
	sw := max(dst.Bounds().Dx(), 1)
	sh := max(dst.Bounds().Dy(), 1)
	vw := max(src.Bounds().Dx(), 1)
	vh := max(src.Bounds().Dy(), 1)
	dst.Fill(color.Black)
	ix := sw / vw
	iy := sh / vh
	n := ix
	if iy < n {
		n = iy
	}
	if n < 1 {
		n = 1
	}
	scaleX := float64(n)
	scaleY := float64(n)
	offX := (float64(sw) - float64(vw)*scaleX) * 0.5
	offY := (float64(sh) - float64(vh)*scaleY) * 0.5
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, scaleY)
	op.GeoM.Translate(offX, offY)
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawBootSplashPresented(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = ebiten.NewImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	if !sg.opts.SourcePortMode {
		sg.drawFaithfulPresented(dst, sg.bootSplashImage)
		return
	}
	sw := max(dst.Bounds().Dx(), 1)
	sh := max(dst.Bounds().Dy(), 1)
	bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
	bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
	dst.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(bw), float64(sh)/float64(bh))
	dst.DrawImage(sg.bootSplashImage, op)
}

func (sg *sessionGame) drawGameTransitionSurface(dst *ebiten.Image, g *game) {
	if dst == nil || g == nil {
		return
	}
	if sg.opts.SourcePortMode {
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != g.viewW || sg.presentSurface.Bounds().Dy() != g.viewH {
			sg.presentSurface = ebiten.NewImage(max(g.viewW, 1), max(g.viewH, 1))
		}
		g.Draw(sg.presentSurface)
		src := sg.presentSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.presentSurface)
		}
		dw := max(dst.Bounds().Dx(), 1)
		dh := max(dst.Bounds().Dy(), 1)
		dst.Fill(color.Black)
		sg.drawSourcePortPresented(dst, src, dw, dh)
		return
	}
	vw := max(g.viewW, 1)
	vh := max(g.viewH, 1)
	if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
		sg.faithfulSurface = ebiten.NewImage(vw, vh)
	}
	g.Draw(sg.faithfulSurface)
	src := sg.faithfulSurface
	if sg.palettePostEnabled() {
		src = sg.applyFaithfulPalettePost(sg.faithfulSurface)
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(dw)/float64(vw), float64(dh)/float64(vh))
	dst.Fill(color.Black)
	dst.DrawImage(src, op)
}

func (sg *sessionGame) drawBootSplashTransitionSurface(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.bootSplashImage == nil && sg.opts.BootSplash.Width > 0 && sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4 {
		sg.bootSplashImage = ebiten.NewImage(sg.opts.BootSplash.Width, sg.opts.BootSplash.Height)
		sg.bootSplashImage.WritePixels(sg.opts.BootSplash.RGBA)
	}
	if sg.bootSplashImage == nil {
		dst.Fill(color.Black)
		return
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	bw := max(sg.bootSplashImage.Bounds().Dx(), 1)
	bh := max(sg.bootSplashImage.Bounds().Dy(), 1)
	dst.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(dw)/float64(bw), float64(dh)/float64(bh))
	dst.DrawImage(sg.bootSplashImage, op)
}

func (sg *sessionGame) queueTransition(kind transitionKind, holdTics int) {
	if kind == transitionNone {
		sg.clearTransition()
		return
	}
	sg.transition.kind = kind
	sg.transition.pending = true
	sg.transition.initialized = false
	if holdTics < 0 {
		holdTics = 0
	}
	sg.transition.holdTics = holdTics
	sg.transition.y = nil
}

func (sg *sessionGame) shouldShowBootSplash() bool {
	if sg.opts.DemoScript != nil {
		return false
	}
	return sg.opts.BootSplash.Width > 0 &&
		sg.opts.BootSplash.Height > 0 &&
		len(sg.opts.BootSplash.RGBA) == sg.opts.BootSplash.Width*sg.opts.BootSplash.Height*4
}

func (sg *sessionGame) transitionActive() bool {
	return sg.transition.kind != transitionNone
}

func (sg *sessionGame) ensureTransitionReady(width, height int) {
	t := &sg.transition
	if t.kind == transitionNone || t.initialized || !t.pending {
		return
	}
	tw := width
	th := height
	if tw <= 0 || th <= 0 {
		return
	}
	if t.from == nil || t.from.Bounds().Dx() != tw || t.from.Bounds().Dy() != th {
		t.from = ebiten.NewImage(tw, th)
	}
	if t.to == nil || t.to.Bounds().Dx() != tw || t.to.Bounds().Dy() != th {
		t.to = ebiten.NewImage(tw, th)
	}
	if t.work == nil || t.work.Bounds().Dx() != tw || t.work.Bounds().Dy() != th {
		t.work = ebiten.NewImage(tw, th)
	}
	switch t.kind {
	case transitionBoot:
		sg.drawBootSplashTransitionSurface(t.from)
		sg.drawGameTransitionSurface(t.to, sg.g)
	case transitionLevel:
		if sg.lastFrame != nil {
			t.from.Clear()
			op := &ebiten.DrawImageOptions{}
			lw := max(sg.lastFrame.Bounds().Dx(), 1)
			lh := max(sg.lastFrame.Bounds().Dy(), 1)
			op.Filter = ebiten.FilterNearest
			op.GeoM.Scale(float64(tw)/float64(lw), float64(th)/float64(lh))
			t.from.DrawImage(sg.lastFrame, op)
		} else {
			sg.drawGameTransitionSurface(t.from, sg.g)
		}
		sg.drawGameTransitionSurface(t.to, sg.g)
	default:
		sg.clearTransition()
		return
	}
	need := tw * th * 4
	if len(t.fromPix) != need {
		t.fromPix = make([]byte, need)
	}
	if len(t.toPix) != need {
		t.toPix = make([]byte, need)
	}
	if len(t.workPix) != need {
		t.workPix = make([]byte, need)
	}
	t.from.ReadPixels(t.fromPix)
	t.to.ReadPixels(t.toPix)
	copy(t.workPix, t.fromPix)
	t.work.WritePixels(t.workPix)
	t.width = tw
	t.height = th
	t.initialized = true
	t.pending = false
	if t.holdTics <= 0 {
		if sg.opts.SourcePortMode {
			t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
		} else {
			t.y = initMeltColumns(tw)
		}
	}
}

func (sg *sessionGame) tickTransition() {
	t := &sg.transition
	if t.kind == transitionNone || !t.initialized {
		return
	}
	if t.holdTics > 0 {
		t.holdTics--
		if t.holdTics == 0 {
			if sg.opts.SourcePortMode {
				t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
			} else {
				t.y = initMeltColumns(t.width)
			}
		}
		return
	}
	if len(t.y) == 0 {
		if sg.opts.SourcePortMode {
			t.y = initMeltColumnsScaled(sourcePortMeltInitColumns(), sourcePortMeltRNGScale(t.height))
		} else {
			t.y = initMeltColumns(t.width)
		}
	}
	// Advance wipe by Doom tics (one melt step per game tic) in both modes.
	meltTicks := 1
	done := false
	if sg.opts.SourcePortMode {
		done = stepMeltSlicesVirtual(t.y, meltVirtualH, t.width, t.height, t.fromPix, t.toPix, t.workPix, meltTicks, sourcePortMeltMoveColumns())
	} else {
		done = stepMeltColumns(t.y, t.width, t.height, t.fromPix, t.toPix, t.workPix, meltTicks)
	}
	if done {
		t.work.WritePixels(t.toPix)
		sg.captureLastFrame(t.to)
		sg.clearTransition()
		return
	}
	t.work.WritePixels(t.workPix)
}

func sourcePortMeltRNGScale(height int) int {
	scale := height / meltVirtualH
	if scale < 1 {
		return 1
	}
	return scale
}

func sourcePortMeltInitColumns() int {
	return sourcePortMeltInitCols
}

func sourcePortMeltMoveColumns() int {
	return sourcePortMeltMoveCols
}

func (sg *sessionGame) drawTransitionFrame(screen *ebiten.Image, sw, sh int) {
	t := &sg.transition
	if t.work == nil {
		screen.Fill(color.Black)
		return
	}
	tw := max(t.width, 1)
	th := max(t.height, 1)
	if tw == sw && th == sh {
		screen.DrawImage(t.work, nil)
		return
	}
	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(tw), float64(sh)/float64(th))
	screen.DrawImage(t.work, op)
}

func (sg *sessionGame) startIntermission(next *mapdata.Map, nextName mapdata.MapName) {
	sg.stopAndClearMusic()
	stats := collectIntermissionStats(sg.g, sg.current, nextName)
	sg.intermission = sessionIntermission{
		active:            true,
		phase:             intermissionPhaseKills,
		waitTic:           0,
		tic:               0,
		stageSoundCounter: 0,
		show: intermissionStats{
			mapName:      stats.mapName,
			nextMapName:  stats.nextMapName,
			killsFound:   stats.killsFound,
			killsTotal:   stats.killsTotal,
			itemsFound:   stats.itemsFound,
			itemsTotal:   stats.itemsTotal,
			secretsFound: stats.secretsFound,
			secretsTotal: stats.secretsTotal,
		},
		target:  stats,
		nextMap: next,
	}
	sg.playIntermissionSound(soundEventIntermissionTick)
}

func (sg *sessionGame) tickIntermission() bool {
	if !sg.intermission.active {
		return false
	}
	im := &sg.intermission
	im.tic++
	skipPressed := anyIntermissionSkipInput()
	if skipPressed && im.tic <= intermissionSkipInputDelayTics {
		skipPressed = false
	}
	if skipPressed {
		im.show.killsPct = im.target.killsPct
		im.show.itemsPct = im.target.itemsPct
		im.show.secretsPct = im.target.secretsPct
		im.show.timeSec = im.target.timeSec
		im.phase = intermissionPhaseYouAreHere
		im.waitTic = intermissionSkipExitHoldTics
		sg.playIntermissionSound(soundEventIntermissionDone)
		return false
	}
	sg.tickIntermissionSoundSystem()
	if im.waitTic > 0 {
		im.waitTic--
		return false
	}
	switch im.phase {
	case intermissionPhaseKills:
		im.show.killsPct = intermissionStepCounter(im.show.killsPct, im.target.killsPct, 2)
		sg.tickIntermissionCounterSound(im.show.killsPct, im.target.killsPct)
		if im.show.killsPct >= im.target.killsPct {
			im.phase = intermissionPhaseItems
			im.waitTic = intermissionPhaseWaitTics
			im.stageSoundCounter = 0
			sg.playIntermissionSound(soundEventIntermissionTick)
		}
	case intermissionPhaseItems:
		im.show.itemsPct = intermissionStepCounter(im.show.itemsPct, im.target.itemsPct, 2)
		sg.tickIntermissionCounterSound(im.show.itemsPct, im.target.itemsPct)
		if im.show.itemsPct >= im.target.itemsPct {
			im.phase = intermissionPhaseSecrets
			im.waitTic = intermissionPhaseWaitTics
			im.stageSoundCounter = 0
			sg.playIntermissionSound(soundEventIntermissionTick)
		}
	case intermissionPhaseSecrets:
		im.show.secretsPct = intermissionStepCounter(im.show.secretsPct, im.target.secretsPct, 2)
		sg.tickIntermissionCounterSound(im.show.secretsPct, im.target.secretsPct)
		if im.show.secretsPct >= im.target.secretsPct {
			im.phase = intermissionPhaseTime
			im.waitTic = intermissionPhaseWaitTics
			im.stageSoundCounter = 0
			sg.playIntermissionSound(soundEventIntermissionTick)
		}
	case intermissionPhaseTime:
		im.show.timeSec = intermissionStepCounter(im.show.timeSec, im.target.timeSec, 3)
		sg.tickIntermissionCounterSound(im.show.timeSec, im.target.timeSec)
		if im.show.timeSec >= im.target.timeSec {
			im.phase = intermissionPhaseEntering
			im.waitTic = intermissionEnteringWaitTics
			im.stageSoundCounter = 0
			sg.playIntermissionSound(soundEventIntermissionDone)
		}
	case intermissionPhaseEntering:
		if shouldShowYouAreHere(im.target.mapName, im.target.nextMapName) {
			im.phase = intermissionPhaseYouAreHere
			im.waitTic = intermissionYouAreHereWaitTics
			sg.playIntermissionSound(soundEventIntermissionTick)
		} else {
			im.phase = intermissionPhaseYouAreHere
			im.waitTic = 1
		}
	default:
		if im.waitTic <= 0 {
			sg.playIntermissionSound(soundEventIntermissionDone)
			return true
		}
	}
	return false
}

func (sg *sessionGame) playIntermissionSound(ev soundEvent) {
	if sg == nil || sg.g == nil || sg.g.snd == nil {
		return
	}
	sg.g.snd.playEvent(ev)
}

func (sg *sessionGame) tickIntermissionSoundSystem() {
	if sg == nil || sg.g == nil || sg.g.snd == nil {
		return
	}
	sg.g.snd.tick()
}

func (sg *sessionGame) tickIntermissionCounterSound(cur, target int) {
	if cur >= target {
		return
	}
	sg.intermission.stageSoundCounter++
	if sg.intermission.stageSoundCounter%intermissionCounterSoundPeriod == 0 {
		sg.playIntermissionSound(soundEventIntermissionTick)
	}
}

func (sg *sessionGame) finishIntermission() {
	im := &sg.intermission
	if !im.active || im.nextMap == nil {
		return
	}
	sg.current = im.target.nextMapName
	sg.currentTemplate = cloneMapForRestart(im.nextMap)
	sg.rebuildGameWithPersistentSettings(im.nextMap)
	sg.playMusicForMap(im.nextMap.Name)
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", im.nextMap.Name))
	sg.intermission = sessionIntermission{}
	sg.queueTransition(transitionLevel, 0)
}

func (sg *sessionGame) drawIntermission(screen *ebiten.Image) {
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	scale := float64(sw) / 320.0
	scaleY := float64(sh) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	ox := (float64(sw) - 320.0*scale) * 0.5
	oy := (float64(sh) - 200.0*scale) * 0.5
	im := &sg.intermission

	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.target.mapName)
	sg.drawIntermissionText(screen, fmt.Sprintf("FINISHED %s", im.target.mapName), 160, 24, scale, ox, oy, true)
	sg.drawIntermissionText(screen, fmt.Sprintf("KILLS   %3d%%", im.show.killsPct), 80, 70, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("ITEMS   %3d%%", im.show.itemsPct), 80, 90, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("SECRETS %3d%%", im.show.secretsPct), 80, 110, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("TIME %s", formatIntermissionTime(im.show.timeSec)), 80, 138, scale, ox, oy, false)
	if im.phase >= intermissionPhaseEntering {
		sg.drawIntermissionText(screen, fmt.Sprintf("ENTERING %s", im.target.nextMapName), 160, 168, scale, ox, oy, true)
	}
	if im.phase == intermissionPhaseYouAreHere && shouldShowYouAreHere(im.target.mapName, im.target.nextMapName) {
		sg.drawYouAreHerePanel(screen, scale, ox, oy, im.target.mapName, im.target.nextMapName)
	}
	if (im.tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
	}
}

func (sg *sessionGame) drawIntermissionBackdrop(screen *ebiten.Image, scale, ox, oy float64, current mapdata.MapName) {
	if bg, ok := sg.intermissionBackgroundName(current); ok {
		_ = sg.drawIntermissionPatch(screen, bg, 0, 0, scale, ox, oy, false)
		return
	}
	_ = sg.drawIntermissionPatch(screen, "INTERPIC", 0, 0, scale, ox, oy, false)
}

func (sg *sessionGame) drawYouAreHerePanel(screen *ebiten.Image, scale, ox, oy float64, current, next mapdata.MapName) {
	if !sg.drawIntermissionPatch(screen, "WIURH0", 208, 38, scale, ox, oy, false) {
		sg.drawIntermissionText(screen, "YOU ARE HERE", 240, 46, scale, ox, oy, true)
	}
	epCur, mapCur, okCur := episodeMapSlot(current)
	epNext, mapNext, okNext := episodeMapSlot(next)
	if !okCur || !okNext || epCur != epNext {
		return
	}
	nodes := intermissionEpisodeNodePos(epCur)
	if len(nodes) != 9 {
		return
	}
	if mapCur >= 1 && mapCur <= 9 {
		pt := nodes[mapCur-1]
		if !sg.drawIntermissionPatch(screen, "WISPLAT", pt.x, pt.y, scale, ox, oy, true) {
			sg.drawIntermissionText(screen, "X", pt.x, pt.y, scale, ox, oy, true)
		}
	}
	if mapNext >= 1 && mapNext <= 9 && (sg.intermission.tic/8)&1 == 0 {
		pt := nodes[mapNext-1]
		if !sg.drawIntermissionPatch(screen, "WIURH0", pt.x, pt.y, scale, ox, oy, true) {
			sg.drawIntermissionText(screen, ">", pt.x, pt.y, scale, ox, oy, true)
		}
	}
}

type interNodePos struct {
	x int
	y int
}

func intermissionEpisodeNodePos(ep int) []interNodePos {
	switch ep {
	case 1:
		return []interNodePos{{185, 164}, {148, 143}, {69, 122}, {209, 102}, {116, 89}, {166, 55}, {71, 56}, {135, 29}, {71, 24}}
	case 2:
		return []interNodePos{{254, 25}, {97, 50}, {188, 64}, {128, 78}, {214, 92}, {133, 130}, {208, 136}, {148, 140}, {235, 158}}
	case 3:
		return []interNodePos{{156, 168}, {48, 154}, {174, 95}, {265, 75}, {130, 48}, {279, 23}, {198, 48}, {140, 25}, {281, 136}}
	default:
		return nil
	}
}

func (sg *sessionGame) intermissionBackgroundName(current mapdata.MapName) (string, bool) {
	ep, _, ok := episodeMapSlot(current)
	if !ok {
		return "", false
	}
	switch ep {
	case 1:
		return "WIMAP0", true
	case 2:
		return "WIMAP1", true
	case 3:
		return "WIMAP2", true
	default:
		return "", false
	}
}

func (sg *sessionGame) drawIntermissionPatch(screen *ebiten.Image, name string, x, y int, scale, ox, oy float64, centered bool) bool {
	img, p, ok := sg.intermissionPatch(name)
	if !ok || img == nil || p.Width <= 0 || p.Height <= 0 {
		return false
	}
	px := ox + float64(x)*scale
	py := oy + float64(y)*scale
	if centered {
		px -= float64(p.Width) * scale * 0.5
		py -= float64(p.Height) * scale * 0.5
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(px-float64(p.OffsetX)*scale, py-float64(p.OffsetY)*scale)
	screen.DrawImage(img, op)
	return true
}

func (sg *sessionGame) intermissionPatch(name string) (*ebiten.Image, WallTexture, bool) {
	if sg == nil || sg.g == nil {
		return nil, WallTexture{}, false
	}
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := sg.g.opts.IntermissionPatchBank[key]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, WallTexture{}, false
	}
	if sg.interPatchCache == nil {
		sg.interPatchCache = make(map[string]*ebiten.Image, 64)
	}
	if img, ok := sg.interPatchCache[key]; ok {
		return img, p, true
	}
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	sg.interPatchCache[key] = img
	return img, p, true
}

func (sg *sessionGame) drawIntermissionText(screen *ebiten.Image, text string, x, y int, scale, ox, oy float64, centered bool) {
	px := ox + float64(x)*scale
	py := oy + float64(y)*scale
	if centered {
		px -= float64(sg.intermissionTextWidth(text)) * scale * 0.5
	}
	if len(sg.g.opts.MessageFontBank) == 0 {
		ebitenutil.DebugPrintAt(screen, text, int(px), int(py))
		return
	}
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			px += 4 * scale
			continue
		}
		img, w, _, gx, gy, ok := sg.g.messageFontGlyph(uc)
		if !ok {
			px += 4 * scale
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(px-float64(gx)*scale, py-float64(gy)*scale)
		screen.DrawImage(img, op)
		px += float64(w) * scale
	}
}

func (sg *sessionGame) intermissionTextWidth(text string) int {
	if len(sg.g.opts.MessageFontBank) == 0 {
		return len(text) * 7
	}
	w := 0
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < huFontStart || uc > huFontEnd {
			w += 4
			continue
		}
		_, gw, _, _, _, ok := sg.g.messageFontGlyph(uc)
		if !ok {
			w += 4
			continue
		}
		w += gw
	}
	return w
}

func shouldShowYouAreHere(current, next mapdata.MapName) bool {
	epCur, _, okCur := episodeMapSlot(current)
	epNext, _, okNext := episodeMapSlot(next)
	if !okCur || !okNext {
		return false
	}
	return epCur == epNext
}

func episodeMapSlot(name mapdata.MapName) (episode int, slot int, ok bool) {
	s := string(name)
	if len(s) != 4 || s[0] != 'E' || s[2] != 'M' {
		return 0, 0, false
	}
	e := int(s[1] - '0')
	m := int(s[3] - '0')
	if e < 1 || e > 9 || m < 1 || m > 9 {
		return 0, 0, false
	}
	return e, m, true
}

func (sg *sessionGame) captureLastFrame(src *ebiten.Image) {
	if src == nil {
		return
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return
	}
	if sg.lastFrame == nil || sg.lastFrame.Bounds().Dx() != w || sg.lastFrame.Bounds().Dy() != h {
		sg.lastFrame = ebiten.NewImage(w, h)
	}
	sg.lastFrame.Clear()
	sg.lastFrame.DrawImage(src, nil)
}

func (sg *sessionGame) clearTransition() {
	sg.transition.kind = transitionNone
	sg.transition.pending = false
	sg.transition.initialized = false
	sg.transition.holdTics = 0
	sg.transition.y = nil
}

func (sg *sessionGame) initFaithfulPalettePost() {
	if !sg.opts.KageShader {
		return
	}
	if len(sg.opts.DoomPaletteRGBA) != 256*4 {
		return
	}
	sh, err := ebiten.NewShader(faithfulPaletteShaderSrc)
	if err != nil {
		fmt.Printf("warning: palette shader disabled: %v\n", err)
		return
	}
	noGammaSh, err := ebiten.NewShader(faithfulPaletteNoGammaShaderSrc)
	if err != nil {
		fmt.Printf("warning: no-gamma palette shader disabled: %v\n", err)
		return
	}
	crtSh, err := ebiten.NewShader(crtPostShaderSrc)
	if err != nil {
		fmt.Printf("warning: crt shader disabled: %v\n", err)
		return
	}
	sg.faithfulShader = sh
	sg.noGammaShader = noGammaSh
	sg.crtShader = crtSh
}

func (sg *sessionGame) palettePostEnabled() bool {
	if sg.g == nil {
		return false
	}
	if !sg.opts.KageShader {
		return false
	}
	if sg.faithfulShader == nil || sg.noGammaShader == nil || sg.crtShader == nil {
		return false
	}
	return sg.g.paletteLUTEnabled || !isNeutralGammaLevel(sg.g.gammaLevel) || sg.g.crtEnabled
}

func (sg *sessionGame) applyFaithfulPalettePost(src *ebiten.Image) *ebiten.Image {
	if !sg.opts.KageShader {
		return src
	}
	if src == nil || sg.faithfulShader == nil || sg.noGammaShader == nil || sg.crtShader == nil {
		return src
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	needsPaletteGamma := sg.g != nil && (sg.g.paletteLUTEnabled || !isNeutralGammaLevel(sg.g.gammaLevel))
	needsCRT := sg.g != nil && sg.g.crtEnabled
	if !needsPaletteGamma && !needsCRT {
		return src
	}
	stage := src
	if needsPaletteGamma {
		if sg.faithfulPost == nil || sg.faithfulPost.Bounds().Dx() != w || sg.faithfulPost.Bounds().Dy() != h {
			sg.faithfulPost = ebiten.NewImage(w, h)
		}
		sg.ensureFaithfulLUTSurface(w, h)
		if sg.faithfulLUT == nil {
			return src
		}
		op := &ebiten.DrawRectShaderOptions{}
		op.Images[0] = src
		op.Images[1] = sg.faithfulLUT
		enableQuant := float32(0)
		if sg.g != nil && sg.g.paletteLUTEnabled && w >= quantizeLUTW && h >= quantizeLUTH {
			enableQuant = 1
		}
		useGamma := true
		if sg.g != nil && isNeutralGammaLevel(sg.g.gammaLevel) {
			useGamma = false
		}
		if useGamma {
			op.Uniforms = map[string]any{
				"GammaRatio":     gammaRatioForLevel(sg.g.gammaLevel),
				"EnableQuantize": enableQuant,
			}
			sg.faithfulPost.DrawRectShader(w, h, sg.faithfulShader, op)
		} else {
			op.Uniforms = map[string]any{
				"EnableQuantize": enableQuant,
			}
			sg.faithfulPost.DrawRectShader(w, h, sg.noGammaShader, op)
		}
		stage = sg.faithfulPost
	}
	if !needsCRT {
		return stage
	}
	if sg.crtPost == nil || sg.crtPost.Bounds().Dx() != w || sg.crtPost.Bounds().Dy() != h {
		sg.crtPost = ebiten.NewImage(w, h)
	}
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = stage
	op.Uniforms = map[string]any{
		"Time": float32(sg.g.worldTic) / float32(doomTicsPerSecond),
	}
	sg.crtPost.DrawRectShader(w, h, sg.crtShader, op)
	return sg.crtPost
}

func gammaRatioForLevel(level int) float32 {
	targetGamma := gammaTargetForLevel(level)
	return float32(targetGamma / 2.2)
}

var gammaTargets = [...]float64{3.2, 2.8, 2.4, 2.2, 1.8, 1.5, 1.4}

func gammaTargetForLevel(level int) float64 {
	if level < 0 {
		level = 0
	}
	if level >= len(gammaTargets) {
		level = len(gammaTargets) - 1
	}
	return gammaTargets[level]
}

func isNeutralGammaLevel(level int) bool {
	return gammaTargetForLevel(level) == 2.2
}

func (sg *sessionGame) ensureFaithfulLUTSurface(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	if len(sg.opts.DoomPaletteRGBA) != 256*4 {
		return
	}
	if sg.faithfulLUT == nil || sg.faithfulLUTW != w || sg.faithfulLUTH != h {
		sg.faithfulLUT = ebiten.NewImage(w, h)
		sg.faithfulLUTW = w
		sg.faithfulLUTH = h
		sg.faithfulLUTPix = make([]byte, w*h*4)
		buildQuantizeLUT16x16x16(sg.faithfulLUTPix, w, h, sg.opts.DoomPaletteRGBA)
		sg.faithfulLUT.WritePixels(sg.faithfulLUTPix)
	}
}

func buildQuantizeLUT16x16x16(dst []byte, w, h int, pal []byte) {
	if len(dst) < w*h*4 || len(pal) < 256*4 {
		return
	}
	const lutW = quantizeLUTW
	const lutH = quantizeLUTH
	if w < lutW || h < lutH {
		return
	}
	for b := 0; b < 16; b++ {
		bv := uint8(b * 17)
		for g := 0; g < 16; g++ {
			gv := uint8(g * 17)
			for r := 0; r < 16; r++ {
				rv := uint8(r * 17)
				best := 0
				bestDist := int(^uint(0) >> 1)
				for i := 0; i < 256; i++ {
					pi := i * 4
					dr := int(rv) - int(pal[pi+0])
					dg := int(gv) - int(pal[pi+1])
					db := int(bv) - int(pal[pi+2])
					d := dr*dr + dg*dg + db*db
					if d < bestDist {
						bestDist = d
						best = i
					}
				}
				idx := r + g*16 + b*256
				x := idx % lutW
				y := idx / lutW
				di := (y*w + x) * 4
				si := best * 4
				dst[di+0] = pal[si+0]
				dst[di+1] = pal[si+1]
				dst[di+2] = pal[si+2]
				dst[di+3] = 0xFF
			}
		}
	}
}

func collectIntermissionStats(g *game, mapName, nextName mapdata.MapName) intermissionStats {
	out := intermissionStats{
		mapName:     mapName,
		nextMapName: nextName,
	}
	if g == nil || g.m == nil {
		return out
	}
	for i, th := range g.m.Things {
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode) {
			continue
		}
		if isMonster(th.Type) {
			out.killsTotal++
			if i >= 0 && i < len(g.thingHP) && g.thingHP[i] <= 0 {
				out.killsFound++
			}
			continue
		}
		if isPickupType(th.Type) {
			out.itemsTotal++
			if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
				out.itemsFound++
			}
		}
	}
	for _, sec := range g.m.Sectors {
		if sec.Special == 9 {
			out.secretsTotal++
		}
	}
	out.secretsFound = g.secretsFound
	if out.secretsFound > out.secretsTotal {
		out.secretsFound = out.secretsTotal
	}
	out.killsPct = intermissionPercent(out.killsFound, out.killsTotal)
	out.itemsPct = intermissionPercent(out.itemsFound, out.itemsTotal)
	out.secretsPct = intermissionPercent(out.secretsFound, out.secretsTotal)
	out.timeSec = g.worldTic / doomTicsPerSecond
	return out
}

func intermissionPercent(n, d int) int {
	if d <= 0 || n <= 0 {
		return 0
	}
	if n >= d {
		return 100
	}
	return (n * 100) / d
}

func intermissionStepCounter(cur, target, step int) int {
	if step < 1 {
		step = 1
	}
	if cur >= target {
		return target
	}
	cur += step
	if cur > target {
		cur = target
	}
	return cur
}

func formatIntermissionTime(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}

func anyIntermissionSkipInput() bool {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) {
		return true
	}
	var keys []ebiten.Key
	keys = inpututil.AppendJustPressedKeys(keys)
	return len(keys) > 0
}

func (sg *sessionGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	if sg.opts.SourcePortMode {
		w := max(outsideWidth, 1)
		h := max(outsideHeight, 1)
		sg.g.setSkyOutputSize(w, h)
		// Sourceport mode keeps a native-sized output while rendering game internals
		// at clean integer divisors for detail levels, then nearest-upscaling.
		div := sg.g.sourcePortDetailDivisor()
		if div < 1 {
			div = 1
		}
		rw := max(w/div, 1)
		rh := max(h/div, 1)
		sg.g.Layout(rw, rh)
		return w, h
	}
	return sg.g.Layout(outsideWidth, outsideHeight)
}
