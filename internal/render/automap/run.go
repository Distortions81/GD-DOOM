package automap

import (
	"errors"
	"fmt"
	"image/color"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/session"

	"github.com/hajimehoshi/ebiten/v2"
)

var frontendMainMenuNames = [...]string{
	"M_NGAME",
	"M_OPTION",
	"M_LOADG",
	"M_SAVEG",
	"M_RDTHIS",
	"M_QUITG",
}

var frontendSkillMenuNames = [...]string{
	"M_JKILL",
	"M_ROUGH",
	"M_HURT",
	"M_ULTRA",
	"M_NMARE",
}

var frontendEpisodeMenuNames = map[int]string{
	1: "M_EPI1",
	2: "M_EPI2",
	3: "M_EPI3",
	4: "M_EPI4",
}

var frontendOptionsMenuNames = [...]string{
	"M_ENDGAM",
	"M_MESSG",
	"M_DETAIL",
	"",
	"",
	"M_MSENS",
	"",
	"M_SVOL",
}

var frontendOptionsSelectableRows = [...]int{0, 1, 2, 5, 7}

func RunAutomap(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	sess := NewSession(m, opts, nextMap)
	defer sess.Close()
	if err := session.Run(sess.runner); err != nil {
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sess.Options().RecordDemoPath; p != "" {
		rec := sess.EffectiveDemoRecord()
		demo, derr := BuildRecordedDemo(sess.sg.bootMap.Name, sess.Options(), rec)
		if derr != nil {
			return fmt.Errorf("build demo recording: %w", derr)
		}
		if werr := SaveDemoScript(p, demo); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(rec))
	}
	return sess.Err()
}

type Session struct {
	sg     *sessionGame
	runner *session.Game
}

func NewSession(m *mapdata.Map, opts Options, nextMap NextMapFunc) *Session {
	opts, windowW, windowH := normalizeRunDimensions(opts)
	sg := &sessionGame{
		gameFactory:     newGame,
		bootMap:         m,
		current:         m.Name,
		currentTemplate: cloneMapForRestart(m),
		opts:            opts,
		nextMap:         nextMap,
	}
	if prev := opts.OnRuntimeSettingsChanged; true {
		sg.opts.OnRuntimeSettingsChanged = func(s RuntimeSettings) {
			sg.applyRuntimeSettings(s)
			if prev != nil {
				prev(s)
			}
		}
	}
	sg.menuSfx = NewMenuSoundPlayer(opts.SoundBank, opts.SFXVolume)
	sg.initSession()
	ebiten.SetTPS(doomTicsPerSecond)
	ebiten.SetVsyncEnabled(!opts.NoVsync)
	if opts.SourcePortMode {
		ebiten.SetWindowSize(opts.Width, opts.Height)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	} else {
		ebiten.SetWindowSize(windowW, windowH)
		// Faithful mode keeps corrected presentation while allowing live resize.
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	}
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	ebiten.SetScreenClearedEveryFrame(false)
	return &Session{sg: sg, runner: session.New(sg)}
}

func (s *Session) Update() error {
	if s == nil || s.runner == nil {
		return ebiten.Termination
	}
	return s.runner.Update()
}

func (s *Session) Draw(screen *ebiten.Image) {
	if s == nil || s.runner == nil {
		return
	}
	s.runner.Draw(screen)
}

func (s *Session) Layout(outsideWidth, outsideHeight int) (int, int) {
	if s == nil || s.runner == nil {
		return max(outsideWidth, 1), max(outsideHeight, 1)
	}
	return s.runner.Layout(outsideWidth, outsideHeight)
}

func (s *Session) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if s == nil || s.runner == nil {
		return
	}
	s.runner.DrawFinalScreen(screen, offscreen, geoM)
}

func (s *Session) Close() {
	if s == nil || s.sg == nil {
		return
	}
	if s.sg.menuSfx != nil {
		s.sg.menuSfx.StopAll()
	}
	s.sg.closeMusicPlayback()
}

func (s *Session) Err() error {
	if s == nil || s.sg == nil {
		return nil
	}
	return s.sg.err
}

func (s *Session) EffectiveDemoRecord() []DemoTic {
	if s == nil || s.sg == nil {
		return nil
	}
	return s.sg.effectiveDemoRecord()
}

func (s *Session) Options() Options {
	if s == nil || s.sg == nil {
		return Options{}
	}
	return s.sg.opts
}

func (s *Session) StartMapName() mapdata.MapName {
	if s == nil || s.sg == nil || s.sg.bootMap == nil {
		return ""
	}
	return s.sg.bootMap.Name
}

func (sg *sessionGame) Update() error {
	if sg.quitPrompt.active {
		return sg.handleQuitPromptInput()
	}
	if sg.anyQuitPromptTrigger() {
		sg.requestQuitPrompt()
		return nil
	}
	if sg.transitionActive() {
		if sg.transition.kind == transitionBoot && sg.transition.holdTics > 0 && anyIntermissionSkipInput() {
			sg.transition.holdTics = 0
		}
		sg.tickTransition()
		return nil
	}
	if sg.finale.active {
		if sg.tickFinale() {
			return ebiten.Termination
		}
		return nil
	}
	if sg.frontend.active {
		if sg.rt != nil && sg.rt.sessionSignals().DemoActive {
			err := sg.rt.Update()
			switch {
			case err == nil:
			case errors.Is(err, ebiten.Termination):
				_ = sg.advanceFrontendAttract()
			default:
				sg.err = err
				return ebiten.Termination
			}
		}
		return sg.tickFrontend()
	}
	if sg.intermission.active {
		if sg.tickIntermission() {
			sg.finishIntermission()
		}
		return nil
	}

	err := sg.rt.Update()
	if err == nil {
		sig := sg.rt.sessionSignals()
		if sig.NewGameMap != nil {
			sg.stopAndClearMusic()
			sg.rt.clearPendingSoundState()
			sg.capturePersistentSettings()
			sg.opts.SkillLevel = normalizeSkillLevel(sig.NewGameSkill)
			sg.rebuildGameWithPersistentSettings(sig.NewGameMap)
			sig = sg.rt.sessionSignals()
			sg.current = sig.MapName
			sg.currentTemplate = cloneMapForRestart(sg.g.m)
			sg.playMusicForMap(sg.current)
			ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
			sg.queueTransition(transitionLevel, 0)
			sg.rt.sessionAcknowledgeNewGameRequest()
			return nil
		}
		if sig.QuitPrompt {
			sg.rt.sessionAcknowledgeQuitPrompt()
			sg.requestQuitPrompt()
			return nil
		}
		if sig.ReadThis {
			sg.rt.sessionAcknowledgeReadThis()
			sg.openReadThis(true)
			return nil
		}
		if sig.LevelRestart {
			sg.stopAndClearMusic()
			sg.rt.clearPendingSoundState()
			sg.rebuildGameWithPersistentSettings(sg.restartMapForRespawn())
			sg.playMusicForMap(sg.rt.sessionSignals().MapName)
			ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
			sg.queueTransition(transitionLevel, 0)
		}
		return nil
	}
	if !errors.Is(err, ebiten.Termination) {
		sg.err = err
		return ebiten.Termination
	}
	sig := sg.rt.sessionSignals()
	if !sig.LevelExit {
		return ebiten.Termination
	}
	if sg.startEpisodeFinale(sg.current, sig.SecretLevelExit) {
		return nil
	}
	if sg.nextMap == nil {
		return ebiten.Termination
	}
	next, nextName, nerr := sg.nextMap(sg.current, sig.SecretLevelExit)
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
	if sg.rt != nil {
		sg.rt.sessionSetQuitPromptActive(sg.quitPrompt.active)
	}
	if sg.g == nil {
		screen.Fill(color.Black)
		return
	}
	tw, th := sg.transitionSurfaceSize(sw, sh)
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
			if sg.quitPrompt.active {
				sg.drawQuitPrompt(screen)
			}
			return
		}
		sg.clearTransition()
	}
	if sg.intermission.active {
		sg.drawIntermission(screen)
		if sg.quitPrompt.active {
			sg.drawQuitPrompt(screen)
		}
		sg.captureLastFrame(screen)
		return
	}
	if sg.frontend.active {
		sg.drawFrontend(screen)
		if sg.quitPrompt.active {
			sg.drawQuitPrompt(screen)
		}
		sg.captureLastFrame(screen)
		return
	}
	if sg.finale.active {
		sg.drawFinale(screen)
		if sg.quitPrompt.active {
			sg.drawQuitPrompt(screen)
		}
		sg.captureLastFrame(screen)
		return
	}
	if sg.opts.SourcePortMode {
		sig := sg.g.sessionSignals()
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sig.ViewWidth || sg.presentSurface.Bounds().Dy() != sig.ViewHeight {
			sg.presentSurface = ebiten.NewImage(max(sig.ViewWidth, 1), max(sig.ViewHeight, 1))
		}
		sg.rt.Draw(sg.presentSurface)
		src := sg.presentSurface
		if sg.palettePostEnabled() {
			src = sg.applyFaithfulPalettePost(sg.presentSurface)
		}
		sg.drawSourcePortPresented(screen, src, sw, sh)
		if sg.quitPrompt.active {
			sg.drawQuitPrompt(screen)
		}
		sg.captureLastFrame(src)
		return
	}
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sw || sg.presentSurface.Bounds().Dy() != sh {
		sg.presentSurface = ebiten.NewImage(sw, sh)
	}
	sg.drawGamePresented(sg.presentSurface, sg.g)
	screen.DrawImage(sg.presentSurface, nil)
	if sg.quitPrompt.active {
		sg.drawQuitPrompt(screen)
	}
}

func (sg *sessionGame) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if screen == nil || offscreen == nil {
		return
	}
	if sg == nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM = geoM
		op.Filter = ebiten.FilterLinear
		screen.DrawImage(offscreen, op)
		return
	}
	if sg.opts.SourcePortMode {
		op := &ebiten.DrawImageOptions{}
		op.GeoM = geoM
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(offscreen, op)
		return
	}

	aspectH := faithfulAspectLogicalH
	if sg.opts.DisableAspectCorrection {
		aspectH = doomLogicalH
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	rw, rh, ox, oy := fitRect(sw, sh, doomLogicalW, aspectH)

	screen.Fill(color.Black)
	ow := max(offscreen.Bounds().Dx(), 1)
	oh := max(offscreen.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(float64(rw)/float64(ow), float64(rh)/float64(oh))
	op.GeoM.Translate(float64(ox), float64(oy))
	screen.DrawImage(offscreen, op)
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
	sig := sg.g.sessionSignals()
	return sig.PaletteLUT || !isNeutralGammaLevel(sig.GammaLevel) || sig.CRTEnabled
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
	sig := gameplay.SessionSignals{}
	if sg.g != nil {
		sig = sg.g.sessionSignals()
	}
	needsPaletteGamma := sg.g != nil && (sig.PaletteLUT || !isNeutralGammaLevel(sig.GammaLevel))
	needsCRT := sg.g != nil && sig.CRTEnabled
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
		if sg.g != nil && sig.PaletteLUT && w >= quantizeLUTW && h >= quantizeLUTH {
			enableQuant = 1
		}
		useGamma := true
		if sg.g != nil && isNeutralGammaLevel(sig.GammaLevel) {
			useGamma = false
		}
		if useGamma {
			op.Uniforms = map[string]any{
				"GammaRatio":     gammaRatioForLevel(sig.GammaLevel),
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
		"Time": float32(sig.WorldTic) / float32(doomTicsPerSecond),
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

func fitRect(w, h, baseW, baseH int) (rw, rh, ox, oy int) {
	w = max(w, 1)
	h = max(h, 1)
	baseW = max(baseW, 1)
	baseH = max(baseH, 1)
	rw = w
	rh = h
	if rw*baseH <= rh*baseW {
		rh = (rw * baseH) / baseW
	} else {
		rw = (rh * baseW) / baseH
	}
	rw = max(rw, 1)
	rh = max(rh, 1)
	ox = (w - rw) / 2
	oy = (h - rh) / 2
	return rw, rh, ox, oy
}

func (sg *sessionGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	if sg == nil || sg.g == nil {
		return max(outsideWidth, 1), max(outsideHeight, 1)
	}
	aspectH := faithfulAspectLogicalH
	if sg.opts.DisableAspectCorrection {
		aspectH = doomLogicalH
	}
	if sg.opts.SourcePortMode {
		w := max(outsideWidth, 1)
		h := max(outsideHeight, 1)
		sg.rt.setSkyOutputSize(w, h)
		// Sourceport mode renders/presents natively to the current window size,
		// with detail level controlling internal divisor only.
		div := sg.g.sessionSignals().SourcePortDetail
		if div < 1 {
			div = 1
		}
		rw := max(w/div, 1)
		rh := max(h/div, 1)
		sg.rt.Layout(rw, rh)
		return w, h
	}
	// Faithful mode renders game internals at 320x200 and presents at an
	// auto integer-scaled corrected layout (320*n x aspect*n).
	sg.rt.Layout(doomLogicalW, doomLogicalH)
	w := max(outsideWidth, 1)
	h := max(outsideHeight, 1)
	w, h, _, _ = fitRect(w, h, doomLogicalW, aspectH)
	scale := w / doomLogicalW
	scaleY := h / aspectH
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	return doomLogicalW * scale, aspectH * scale
}
