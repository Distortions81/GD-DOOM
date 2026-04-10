package doomruntime

import (
	"errors"
	"fmt"
	"image/color"
	"strings"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimehost"
	"gddoom/internal/session"
	"gddoom/internal/sessionaudio"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
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
	"",
	"",
	"",
	"",
	"",
	"",
	"",
	"",
}

var frontendOptionsTextLabels = [...]string{
	"MESSAGES",
	"STATUS BAR MODE",
	"HUD SIZE",
	"FPS",
	"MOUSE SENSITIVITY",
	"SOUND OPTIONS",
	"VOICE OPTIONS",
	"KEY BINDINGS",
}

var frontendOptionsSelectableRows = [...]int{0, 1, 2, 3, 4, 5, 6, 7}

func NewRuntime(m *mapdata.Map, opts Options, nextMap runtimehost.NextMapFunc) (session.Runtime, runtimehost.Meta) {
	sg := runtimehost.Init(runtimehost.Initializer[*sessionGame]{
		Base: func() *sessionGame {
			return &sessionGame{
				gameFactory:     newGame,
				bootMap:         m,
				current:         m.Name,
				currentTemplate: cloneMapForRestart(m),
				opts:            opts,
				nextMap:         nextMap,
			}
		},
		Config: []func(*sessionGame){
			func(sg *sessionGame) {
				if prev := opts.OnRuntimeSettingsChanged; true {
					sg.opts.OnRuntimeSettingsChanged = func(s RuntimeSettings) {
						sg.applyRuntimeSettings(s)
						if prev != nil {
							prev(s)
						}
					}
				}
			},
			func(sg *sessionGame) {
				sg.opts.PlayCheatMusic = func(currentMapName string, code string) (bool, error) {
					return sg.playCheatMusic(currentMapName, code)
				}
			},
			func(sg *sessionGame) {
				sg.opts.VoiceTransmitActive = func() bool {
					return sg.voiceTransmitHeld.Load()
				}
			},
			func(sg *sessionGame) {
				if !sg.headlessDemoPlayback() {
					sg.menuSfx = sessionaudio.NewMenuController(opts.SoundBank, opts.PCSpeakerBank, opts.SFXVolume, opts.PCSpeakerVariant)
				}
			},
		},
		Start: func(sg *sessionGame) {
			sg.initSession()
		},
	})
	return runtimehost.NewGame(sg, runtimehost.Accessors{
		Close: func() {
			if sg.menuSfx != nil {
				sg.menuSfx.Close()
			}
			sg.closeMusicPlayback()
		},
		Err: func() error {
			return sg.err
		},
		EffectiveDemoRecord: sg.effectiveDemoRecord,
		Options: func() Options {
			opts := sg.opts
			if strings.TrimSpace(opts.RecordDemoPath) == "" && strings.TrimSpace(sg.frozenDemoPath) != "" {
				opts.RecordDemoPath = sg.frozenDemoPath
			}
			return opts
		},
		StartMapName: func() mapdata.MapName {
			if sg.demoRecordingMap != "" {
				return sg.demoRecordingMap
			}
			if sg.bootMap == nil {
				return ""
			}
			return sg.bootMap.Name
		},
		CurrentWorldTic: func() int {
			if sg.g == nil {
				return 0
			}
			return sg.g.sessionSignals().WorldTic
		},
		CaptureKeyframe: sg.marshalNetplayKeyframe,
		LoadKeyframe:    sg.unmarshalNetplayKeyframe,
	})
}

func (sg *sessionGame) headlessDemoPlayback() bool {
	if sg == nil {
		return false
	}
	return sg.opts.DemoScript != nil && sg.opts.DemoQuitOnComplete
}

func frontendShouldUpdateRuntime(sig gameplay.SessionSignals) bool {
	return sig.DemoActive || sig.WatchActive
}

func (sg *sessionGame) Update() error {
	sg.releaseStartupMusicIfReady()
	sg.releaseTransitionMusicIfReady()
	err := runtimehost.RunUpdate(runtimehost.Update{
		QuitPromptActive:    func() bool { return sg.quitPrompt.Active },
		HandleQuitPrompt:    sg.handleQuitPromptInput,
		QuitPromptTriggered: sg.anyQuitPromptTrigger,
		RequestQuitPrompt:   sg.requestQuitPrompt,
		TransitionActive:    sg.transitionActive,
		TransitionIsBootHolding: func() bool {
			return sg.transition.Kind() == transitionBoot && sg.transition.HoldTics() > 0
		},
		SkipRequested:      sg.anyIntermissionSkipInput,
		SkipTransitionHold: sg.transition.SkipHold,
		TickTransition:     sg.tickTransition,
		FinaleActive: func() bool {
			return sg.finale.Active
		},
		TickFinale: sg.tickFinale,
		FrontendActive: func() bool {
			return sg.frontend.Active
		},
		DemoActive: func() bool {
			return sg.rt != nil && frontendShouldUpdateRuntime(sg.rt.sessionSignals())
		},
		UpdateRuntimeForDemo: func() error {
			err := sg.rt.Update()
			switch {
			case err == nil:
				return nil
			case errors.Is(err, ebiten.Termination):
				_ = sg.advanceFrontendAttract()
				return nil
			default:
				sg.err = err
				return ebiten.Termination
			}
		},
		AdvanceFrontendAttract: sg.advanceFrontendAttract,
		TickFrontend:           sg.tickFrontend,
		IntermissionActive: func() bool {
			return sg.intermission.state.Active
		},
		TickIntermission:   sg.tickIntermission,
		FinishIntermission: sg.finishIntermission,
		UpdateRuntime: func() error {
			if err := sg.applyMandatoryWatchKeyframes(); err != nil {
				return err
			}
			return sg.rt.Update()
		},
		HandleRuntimeProgress: func() (bool, error) {
			sig := sg.rt.sessionSignals()
			if sig.SaveGame {
				sg.rt.sessionAcknowledgeSaveGame()
				if err := sg.SaveGameToSlot(1); err != nil {
					if sg.g != nil {
						sg.g.setHUDMessage(strings.ToUpper(err.Error()), 70)
					}
				} else if sg.g != nil {
					sg.g.setHUDMessage("GAME SAVED", 70)
				}
				return true, nil
			}
			if sig.LoadGame {
				sg.rt.sessionAcknowledgeLoadGame()
				if err := sg.LoadGameFromSlot(1); err != nil {
					if sg.g != nil {
						sg.g.setHUDMessage(strings.ToUpper(err.Error()), 70)
					}
				}
				return true, nil
			}
			if sig.FrontendMenu {
				sg.rt.sessionAcknowledgeFrontendMenu()
				sg.consumeFrontendOpenInput()
				sg.openFrontendMenuFromSignal(sig)
				return true, nil
			}
			if sig.SoundMenu {
				sg.rt.sessionAcknowledgeSoundMenu()
				sg.consumeFrontendOpenInput()
				sg.openFrontendSoundMenuFromSignal(sig)
				return true, nil
			}
			if sig.MusicPlayer {
				sg.rt.sessionAcknowledgeMusicPlayer()
				sg.frontend = frontendState{Active: true, InGame: true, MenuActive: true, Mode: frontendModeSound, SoundOn: frontendSoundMenuRowPlayer}
				return true, nil
			}
			return runtimehost.HandleProgress(
				runtimehost.ProgressSignals{
					HasNewGame:    sig.NewGameMap != nil,
					HasQuitPrompt: sig.QuitPrompt,
					HasReadThis:   sig.ReadThis,
					HasRestart:    sig.LevelRestart,
				},
				runtimehost.ProgressHandlers{
					OnNewGame: func() error {
						sg.stopAndClearMusic()
						sg.rt.clearPendingSoundState()
						sg.capturePersistentSettings()
						sg.levelCarryover = nil
						sg.secretVisited = false
						sg.opts.SkillLevel = normalizeSkillLevel(sig.NewGameSkill)
						sg.rebuildGameWithPersistentSettings(sig.NewGameMap)
						sig = sg.rt.sessionSignals()
						sg.current = sig.MapName
						sg.currentTemplate = cloneMapForRestart(sg.g.m)
						if err := sg.broadcastMandatoryRuntimeKeyframe(); err != nil {
							return err
						}
						sg.queueTransition(transitionLevel, 0)
						sg.playMusicForMap(sg.current)
						sg.announceMapMusic(sg.current)
						ebiten.SetWindowTitle(runtimehost.WindowTitle(sg.current))
						sg.rt.sessionAcknowledgeNewGameRequest()
						return nil
					},
					OnQuitPrompt: func() error {
						sg.rt.sessionAcknowledgeQuitPrompt()
						sg.requestQuitPrompt()
						return nil
					},
					OnReadThis: func() error {
						sg.rt.sessionAcknowledgeReadThis()
						sg.openReadThis(true)
						return nil
					},
					OnRestart: func() error {
						sg.freezeDemoRecord()
						restartMap := sg.restartMapForRespawn()
						if sg.opts.DebugEvents {
							if restartMap != nil {
								fmt.Printf(
									"level-restart-exec current=%s next=%s single=%t\n",
									sg.current,
									restartMap.Name,
									normalizeGameMode(sg.opts.GameMode) == gameModeSingle,
								)
							} else {
								fmt.Printf(
									"level-restart-exec current=%s next=<nil> single=%t\n",
									sg.current,
									normalizeGameMode(sg.opts.GameMode) == gameModeSingle,
								)
							}
						}
						sg.stopAndClearMusic()
						sg.rt.clearPendingSoundState()
						sg.rt.sessionAcknowledgeLevelRestart()
						sg.levelCarryover = nil
						sg.rebuildGameWithPersistentSettings(restartMap)
						sg.queueTransition(transitionLevel, 0)
						sg.playMusicForMap(sg.rt.sessionSignals().MapName)
						sg.announceMapMusic(sg.rt.sessionSignals().MapName)
						ebiten.SetWindowTitle(runtimehost.WindowTitle(sg.current))
						return nil
					},
				},
			)
		},
		HandleRuntimeTermination: func() (bool, error) {
			err := sg.handleGameplayTermination()
			switch {
			case err == nil:
				return true, nil
			case errors.Is(err, runtimehost.ErrTerminate):
				return false, err
			default:
				return true, err
			}
		},
	})
	if errors.Is(err, runtimehost.ErrTerminate) {
		return ebiten.Termination
	}
	return err
}

func (sg *sessionGame) Draw(screen *ebiten.Image) {
	yieldWASMRenderTime()
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	tw, th := sg.transitionSurfaceSize(sw, sh)
	runtimehost.RunDraw(screen, runtimehost.Draw{
		Prepare: func() {
			if sg.rt != nil {
				sg.rt.sessionSetQuitPromptActive(sg.quitPrompt.Active)
				sg.rt.sessionSetFrontendActive(sg.frontend.Active && sg.frontend.InGame)
			}
		},
		HasGame: func() bool {
			return sg.g != nil
		},
		DrawEmpty: func(screen *ebiten.Image) {
			screen.Fill(color.Black)
		},
		TransitionActive: func() bool {
			return sg.transitionActive()
		},
		TransitionNeedsResize: func() bool {
			return sg.opts.SourcePortMode && sg.transition.NeedsResize(tw, th)
		},
		InvalidateTransition: sg.transition.Invalidate,
		EnsureTransitionReady: func() {
			sg.ensureTransitionReady(tw, th)
		},
		TransitionInitialized: sg.transition.Initialized,
		DrawTransitionFrame: func(screen *ebiten.Image) {
			sg.drawTransitionFrame(screen, sw, sh)
		},
		ClearTransition: sg.transition.Clear,
		IntermissionActive: func() bool {
			return sg.intermission.state.Active
		},
		DrawIntermission: sg.drawIntermissionPresented,
		FrontendActive: func() bool {
			return sg.frontend.Active
		},
		DrawFrontend: func(screen *ebiten.Image) {
			sg.drawFrontendPresented(screen)
		},
		FinaleActive: func() bool {
			return sg.finale.Active
		},
		DrawFinale: sg.drawFinalePresented,
		DrawGameplay: func(screen *ebiten.Image) {
			if sg.opts.SourcePortMode {
				sg.drawGamePresented(screen, sg.g)
				if sg.quitPrompt.Active {
					sg.drawQuitPrompt(screen)
				}
				return
			}
			present := sg.ensureFrontendSurface(sw, sh)
			sg.drawGamePresented(present, sg.g)
			screen.DrawImage(present, nil)
			if sg.quitPrompt.Active {
				sg.drawQuitPrompt(screen)
			}
		},
		QuitPromptActive: func() bool {
			return sg.quitPrompt.Active
		},
		DrawQuitPrompt: sg.drawQuitPrompt,
	})
	if sg.startupMusicLocked && !sg.transitionActive() {
		sg.startupMusicVisualReady = true
	}
}

func (sg *sessionGame) handleGameplayTermination() error {
	sig := sg.rt.sessionSignals()
	if !sig.LevelExit {
		return runtimehost.ErrTerminate
	}
	if sg.startEpisodeFinale(sg.current, sig.SecretLevelExit) {
		sg.freezeDemoRecord()
		return nil
	}
	if sg.nextMap == nil {
		return runtimehost.ErrTerminate
	}
	next, nextName, err := sg.nextMap(sg.current, sig.SecretLevelExit)
	if err != nil {
		sg.err = err
		return ebiten.Termination
	}
	sg.startIntermission(next, nextName, sig.SecretLevelExit)
	return nil
}

func (sg *sessionGame) SampleInput() {
	if sg == nil {
		return
	}
	if sg.shouldSampleRuntimeInput() && sg.rt != nil {
		sg.rt.SampleInput()
	}
	if sg.g != nil && sg.shouldSampleRuntimeInput() {
		sg.voiceTransmitHeld.Store(sg.g.bindingHeld(bindingVoice))
	} else {
		sg.voiceTransmitHeld.Store(false)
	}
	if !sg.shouldSampleSessionInput() {
		sg.clearSampledInput()
		return
	}
	sg.input.justPressedKeys = addPressCounts(sg.input.justPressedKeys, inpututil.AppendJustPressedKeys(nil))
	sg.input.justPressedMouseButtons = addPressCounts(
		sg.input.justPressedMouseButtons,
		justPressedMouseButtons(),
	)
}

func (sg *sessionGame) clearSampledInput() {
	if sg == nil {
		return
	}
	sg.input = sessionInputSnapshot{}
}

func (sg *sessionGame) keyJustPressed(key ebiten.Key) bool {
	if sg == nil {
		return inpututil.IsKeyJustPressed(key)
	}
	return consumePress(sg.input.justPressedKeys, key)
}

func (sg *sessionGame) bindingJustPressed(action bindingAction) bool {
	if sg == nil {
		return false
	}
	binds := bindingValue(sg.opts.InputBindings, action)
	return bindingPressedCounts(sg.input.justPressedKeys, binds) || bindingMousePressedCounts(sg.input.justPressedMouseButtons, binds)
}

func (sg *sessionGame) openFrontendMenuFromSignal(sig gameplay.SessionSignals) {
	if sg == nil {
		return
	}
	inGame := !sig.DemoActive
	itemOn := 0
	if inGame && sg.frontendWatchMode() {
		itemOn = frontendWatchMenuSelectableRows[0]
	}
	sg.frontend = frontendState{
		Active:     true,
		InGame:     inGame,
		Attract:    sig.DemoActive,
		Mode:       frontendModeTitle,
		MenuActive: true,
		ItemOn:     itemOn,
	}
}

func (sg *sessionGame) openFrontendSoundMenuFromSignal(sig gameplay.SessionSignals) {
	if sg == nil {
		return
	}
	inGame := !sig.DemoActive
	sg.frontend = frontendState{
		Active:     true,
		InGame:     inGame,
		Attract:    sig.DemoActive,
		Mode:       frontendModeSound,
		MenuActive: true,
		SoundOn:    frontendSoundMenuRowSFX,
	}
}

func (sg *sessionGame) consumeAnyKeyPress() bool {
	if sg == nil || len(sg.input.justPressedKeys) == 0 {
		return false
	}
	sg.input.justPressedKeys = nil
	return true
}

func (sg *sessionGame) skipInputTriggered() bool {
	return sg.consumeAnyKeyPress() ||
		sg.mouseJustPressed(ebiten.MouseButtonLeft) ||
		sg.mouseJustPressed(ebiten.MouseButtonRight) ||
		sg.mouseJustPressed(ebiten.MouseButtonMiddle) ||
		sg.mouseJustPressed(ebiten.MouseButton3) ||
		sg.mouseJustPressed(ebiten.MouseButton4)
}

func (sg *sessionGame) mouseJustPressed(button ebiten.MouseButton) bool {
	if sg == nil {
		return inpututil.IsMouseButtonJustPressed(button)
	}
	return consumePress(sg.input.justPressedMouseButtons, button)
}

func justPressedMouseButtons() []ebiten.MouseButton {
	buttons := [...]ebiten.MouseButton{
		ebiten.MouseButtonLeft,
		ebiten.MouseButtonRight,
		ebiten.MouseButtonMiddle,
		ebiten.MouseButton3,
		ebiten.MouseButton4,
	}
	dst := make([]ebiten.MouseButton, 0, len(buttons))
	for _, button := range buttons {
		if inpututil.IsMouseButtonJustPressed(button) {
			dst = append(dst, button)
		}
	}
	return dst
}

func addPressCounts[T comparable](dst map[T]int, items []T) map[T]int {
	if len(items) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[T]int, len(items))
	}
	for _, item := range items {
		dst[item]++
	}
	return dst
}

func consumePress[T comparable](counts map[T]int, key T) bool {
	if len(counts) == 0 {
		return false
	}
	v := counts[key]
	if v <= 0 {
		return false
	}
	if v == 1 {
		delete(counts, key)
	} else {
		counts[key] = v - 1
	}
	return true
}

func (sg *sessionGame) consumeFrontendOpenInput() {
	if sg == nil {
		return
	}
	sg.input.justPressedKeys = nil
}

func (sg *sessionGame) shouldSampleRuntimeInput() bool {
	if sg == nil {
		return false
	}
	frontendBlocksRuntimeInput := sg.frontend.Active
	if frontendBlocksRuntimeInput && sg.rt != nil && sg.rt.sessionSignals().DemoActive && !sg.frontend.MenuActive {
		frontendBlocksRuntimeInput = false
	}
	return !sg.quitPrompt.Active &&
		!sg.transitionActive() &&
		!sg.finale.Active &&
		!frontendBlocksRuntimeInput &&
		!sg.intermission.state.Active
}

func (sg *sessionGame) shouldSampleSessionInput() bool {
	if sg == nil {
		return false
	}
	return sg.quitPrompt.Active ||
		sg.transitionActive() ||
		sg.finale.Active ||
		sg.frontend.Active ||
		sg.intermission.state.Active
}

func (sg *sessionGame) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if screen == nil || offscreen == nil {
		return
	}
	if sg == nil {
		var op ebiten.DrawImageOptions
		op.GeoM = geoM
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(offscreen, &op)
		return
	}
	if sg.opts.SourcePortMode {
		op := &sg.finalScreenDrawOp
		*op = ebiten.DrawImageOptions{}
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
	op := &sg.finalScreenDrawOp
	*op = ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(rw)/float64(ow), float64(rh)/float64(oh))
	op.GeoM.Translate(float64(ox), float64(oy))
	screen.DrawImage(offscreen, op)
}

func (sg *sessionGame) initFaithfulPalettePost() {
	if sg == nil || sg.headlessDemoPlayback() {
		return
	}
	if !sg.opts.KageShader {
		return
	}
	crtSh, err := ebiten.NewShader(crtPostShaderSrc)
	if err != nil {
		fmt.Printf("warning: crt shader disabled: %v\n", err)
		return
	}
	sg.crtShader = crtSh
}

func (sg *sessionGame) palettePostEnabled() bool {
	if sg.g == nil {
		return false
	}
	if !sg.opts.KageShader {
		return false
	}
	if sg.crtShader == nil {
		return false
	}
	sig := sg.g.sessionSignals()
	return sig.CRTEnabled
}

func (sg *sessionGame) applyFaithfulPalettePost(src *ebiten.Image) *ebiten.Image {
	if !sg.opts.KageShader {
		return src
	}
	if src == nil || sg.crtShader == nil {
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
	needsCRT := sg.g != nil && sig.CRTEnabled
	if !needsCRT {
		return src
	}
	if sg.crtPost == nil || sg.crtPost.Bounds().Dx() != w || sg.crtPost.Bounds().Dy() != h {
		sg.crtPost = newUnmanagedImage(w, h)
	}
	op := &ebiten.DrawRectShaderOptions{}
	op.Images[0] = src
	if sg.crtUniforms == nil {
		sg.crtUniforms = make(map[string]any, 1)
	}
	sg.crtUniforms["Time"] = float32(sig.WorldTic) / float32(doomTicsPerSecond)
	op.Uniforms = sg.crtUniforms
	sg.crtPost.DrawRectShader(w, h, sg.crtShader, op)
	return sg.crtPost
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
		layoutW := max(outsideWidth, 1)
		layoutH := max(outsideHeight, 1)
		sg.g.mouseInputScaleX = float64(layoutW) / float64(max(layoutW, 1))
		if sg.g.skyOutputW != layoutW || sg.g.skyOutputH != layoutH {
			sg.rt.setSkyOutputSize(layoutW, layoutH)
		}
		// Sourceport mode renders/presents natively to the current window size,
		// with detail level controlling internal divisor only.
		div := sg.g.sessionSignals().SourcePortDetail
		if div < 1 {
			div = 1
		}
		renderW := max(outsideWidth, 1)
		renderH := max(outsideHeight, 1)
		rw := max(renderW/div, 1)
		rh := max(renderH/div, 1)
		rw, rh = clampSourcePortGameSizeForPlatform(rw, rh, isWASMBuild())
		if sg.g.viewW != rw || sg.g.viewH != rh {
			sg.rt.Layout(rw, rh)
		}
		return layoutW, layoutH
	}
	// Faithful mode renders game internals at 320x200 and presents at an
	// fixed 640x400 logical buffer, with detail level selecting the internal
	// game buffer size and final-screen presentation applying aspect correction.
	rw, rh := faithfulDetailPresetSize(sg.g.detailLevel)
	sg.g.mouseInputScaleX = float64(max(outsideWidth, 1)) / float64(faithfulBufferW)
	sg.rt.Layout(rw, rh)
	_ = aspectH
	return faithfulBufferW, faithfulBufferH
}
