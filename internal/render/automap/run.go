package automap

import (
	"errors"
	"fmt"
	"image/color"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

const (
	bootSplashHoldTics = 3 * doomTicsPerSecond
	meltVirtualH       = 200
	// Sourceport melt uses Doom-like 2-pixel column pairs over a 320-wide
	// virtual layout, i.e. 160 moving slices.
	sourcePortMeltInitCols = 160
	sourcePortMeltMoveCols = sourcePortMeltInitCols
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
	// Convert from ~2.2 encoded source to ~2.4 encoded domain before quantization.
	pre := vec3(pow(c.r, GammaRatio), pow(c.g, GammaRatio), pow(c.b, GammaRatio))
	best := vec4(pre, 1.0)
	if EnableQuantize >= 0.5 {
		// 16x16x16 RGB cube LUT flattened into a 256x16 block at source1 top-left.
		ri := int(clamp(pre.r*15.0+0.5, 0.0, 15.0))
		gi := int(clamp(pre.g*15.0+0.5, 0.0, 15.0))
		bi := int(clamp(pre.b*15.0+0.5, 0.0, 15.0))
		idx := ri + gi*16 + bi*256
		lx := float(idx%256) + 0.5
		ly := float(idx/256) + 0.5
		best = imageSrc1At(vec2(lx, ly))
	}
	return vec4(best.rgb, 1.0)
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
	active  bool
	phase   int
	waitTic int
	tic     int
	show    intermissionStats
	target  intermissionStats
	nextMap *mapdata.Map
}

type sessionGame struct {
	g               *game
	current         mapdata.MapName
	opts            Options
	nextMap         NextMapFunc
	err             error
	faithfulSurface *ebiten.Image
	faithfulPost    *ebiten.Image
	faithfulLUT     *ebiten.Image
	faithfulLUTPix  []byte
	faithfulLUTW    int
	faithfulLUTH    int
	faithfulShader  *ebiten.Shader
	presentSurface  *ebiten.Image
	lastFrame       *ebiten.Image
	bootSplashImage *ebiten.Image
	transition      sessionTransition
	intermission    sessionIntermission
}

func RunAutomap(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	windowW := opts.Width
	windowH := opts.Height
	if opts.SourcePortMode {
		if opts.Width <= 0 {
			opts.Width = 1280
		}
		if opts.Height <= 0 {
			opts.Height = 800
		}
	} else {
		// Faithful mode: keep internal render fixed at Doom logical resolution.
		opts.Width = doomLogicalW
		opts.Height = doomLogicalH
		// Window size honors requested width/height but is snapped to integer
		// multiples of the logical resolution to preserve aspect and pixel-perfect scaling.
		if windowW <= 0 {
			windowW = 1280
		}
		if windowH <= 0 {
			windowH = 960
		}
		scaleX := windowW / doomLogicalW
		scaleY := windowH / doomLogicalH
		scale := scaleX
		if scaleY < scale {
			scale = scaleY
		}
		if scale < 1 {
			scale = 1
		}
		windowW = doomLogicalW * scale
		windowH = doomLogicalH * scale
	}
	sg := &sessionGame{
		g:       newGame(m, opts),
		current: m.Name,
		opts:    opts,
		nextMap: nextMap,
	}
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
		if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
			return ebiten.Termination
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) &&
			(ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)) {
			return ebiten.Termination
		}
		sg.tickTransition()
		return nil
	}
	if sg.intermission.active {
		if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
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
			sg.g = newGame(sg.g.m, sg.opts)
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
		if sg.palettePostEnabled() {
			if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sw || sg.presentSurface.Bounds().Dy() != sh {
				sg.presentSurface = ebiten.NewImage(sw, sh)
			}
			sg.g.Layout(sw, sh)
			sg.g.Draw(sg.presentSurface)
			src := sg.applyFaithfulPalettePost(sg.presentSurface)
			screen.DrawImage(src, nil)
			sg.captureLastFrame(src)
			return
		}
		// Render directly to the actual screen target when no postprocess is active.
		sg.g.Layout(sw, sh)
		sg.g.Draw(screen)
		sg.captureLastFrame(screen)
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
		src := sg.applyFaithfulPalettePost(sg.faithfulSurface)
		sg.drawFaithfulPresented(dst, src)
		sg.captureLastFrame(src)
		return
	}
	g.Layout(max(dst.Bounds().Dx(), 1), max(dst.Bounds().Dy(), 1))
	g.Draw(dst)
	if sg.palettePostEnabled() {
		dst.Clear()
		dst.DrawImage(sg.applyFaithfulPalettePost(dst), nil)
	}
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
		g.Layout(max(dst.Bounds().Dx(), 1), max(dst.Bounds().Dy(), 1))
		g.Draw(dst)
		if sg.palettePostEnabled() {
			src := sg.applyFaithfulPalettePost(dst)
			dst.Clear()
			dst.DrawImage(src, nil)
		}
		return
	}
	vw := max(g.viewW, 1)
	vh := max(g.viewH, 1)
	if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
		sg.faithfulSurface = ebiten.NewImage(vw, vh)
	}
	g.Draw(sg.faithfulSurface)
	src := sg.applyFaithfulPalettePost(sg.faithfulSurface)
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
	stats := collectIntermissionStats(sg.g, sg.current, nextName)
	sg.intermission = sessionIntermission{
		active:  true,
		phase:   0,
		waitTic: 0,
		tic:     0,
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
}

func (sg *sessionGame) tickIntermission() bool {
	if !sg.intermission.active {
		return false
	}
	im := &sg.intermission
	im.tic++
	if anyIntermissionSkipInput() {
		im.show.killsPct = im.target.killsPct
		im.show.itemsPct = im.target.itemsPct
		im.show.secretsPct = im.target.secretsPct
		im.show.timeSec = im.target.timeSec
		return true
	}
	if im.waitTic > 0 {
		im.waitTic--
		return false
	}
	switch im.phase {
	case 0:
		im.show.killsPct = intermissionStepCounter(im.show.killsPct, im.target.killsPct, 2)
		if im.show.killsPct >= im.target.killsPct {
			im.phase = 1
			im.waitTic = 8
		}
	case 1:
		im.show.itemsPct = intermissionStepCounter(im.show.itemsPct, im.target.itemsPct, 2)
		if im.show.itemsPct >= im.target.itemsPct {
			im.phase = 2
			im.waitTic = 8
		}
	case 2:
		im.show.secretsPct = intermissionStepCounter(im.show.secretsPct, im.target.secretsPct, 2)
		if im.show.secretsPct >= im.target.secretsPct {
			im.phase = 3
			im.waitTic = 8
		}
	case 3:
		im.show.timeSec = intermissionStepCounter(im.show.timeSec, im.target.timeSec, 3)
		if im.show.timeSec >= im.target.timeSec {
			im.phase = 4
			im.waitTic = doomTicsPerSecond * 2
		}
	default:
		if im.waitTic <= 0 {
			return true
		}
	}
	return false
}

func (sg *sessionGame) finishIntermission() {
	im := &sg.intermission
	if !im.active || im.nextMap == nil {
		return
	}
	sg.current = im.target.nextMapName
	sg.g = newGame(im.nextMap, sg.opts)
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
	sg.drawIntermissionText(screen, fmt.Sprintf("FINISHED %s", im.target.mapName), 160, 24, scale, ox, oy, true)
	sg.drawIntermissionText(screen, fmt.Sprintf("KILLS   %3d%%", im.show.killsPct), 80, 70, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("ITEMS   %3d%%", im.show.itemsPct), 80, 90, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("SECRETS %3d%%", im.show.secretsPct), 80, 110, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("TIME %s", formatIntermissionTime(im.show.timeSec)), 80, 138, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("ENTERING %s", im.target.nextMapName), 160, 168, scale, ox, oy, true)
	if (im.tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
	}
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
	if len(sg.opts.DoomPaletteRGBA) != 256*4 {
		return
	}
	sh, err := ebiten.NewShader(faithfulPaletteShaderSrc)
	if err != nil {
		fmt.Printf("warning: palette shader disabled: %v\n", err)
		return
	}
	sg.faithfulShader = sh
}

func (sg *sessionGame) palettePostEnabled() bool {
	if sg.faithfulShader == nil || sg.g == nil {
		return false
	}
	return sg.g.paletteLUTEnabled || sg.g.gammaLevel > 0
}

func (sg *sessionGame) applyFaithfulPalettePost(src *ebiten.Image) *ebiten.Image {
	if src == nil || sg.faithfulShader == nil {
		return src
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return src
	}
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
	if sg.g != nil && sg.g.paletteLUTEnabled {
		enableQuant = 1
	}
	op.Uniforms = map[string]any{
		"GammaRatio":     gammaRatioForLevel(sg.g.gammaLevel),
		"EnableQuantize": enableQuant,
	}
	sg.faithfulPost.DrawRectShader(w, h, sg.faithfulShader, op)
	return sg.faithfulPost
}

func gammaRatioForLevel(level int) float32 {
	if level <= 0 {
		return 1.0
	}
	return float32(2.2 / 2.4)
}

func (sg *sessionGame) ensureFaithfulLUTSurface(w, h int) {
	if w <= 0 || h <= 0 || len(sg.opts.DoomPaletteRGBA) != 256*4 {
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
	const lutW = 256
	const lutH = 16
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
		if !thingSpawnsForSkill(th, g.opts.SkillLevel) {
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
		// Keep game internals synced, but always expose native output size
		// in sourceport mode so transition and steady-state render targets match.
		sg.g.Layout(w, h)
		return w, h
	}
	return sg.g.Layout(outsideWidth, outsideHeight)
}
