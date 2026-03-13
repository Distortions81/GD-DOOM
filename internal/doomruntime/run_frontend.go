package doomruntime

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"gddoom/internal/runtimehost"
	"gddoom/internal/sessionaudio"
	"gddoom/internal/sessionflow"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const frontendMainMenuTitle = "GD-DOOM [ALPHA]"

func (sg *sessionGame) shouldStartInFrontend() bool {
	if sg == nil {
		return false
	}
	if sg.opts.StartInMapMode || sg.opts.DemoScript != nil || strings.TrimSpace(sg.opts.RecordDemoPath) != "" {
		return false
	}
	return true
}

func (sg *sessionGame) startFrontend() {
	if sg == nil {
		return
	}
	sg.frontend = frontendState(sessionflow.StartFrontend())
	sg.frontendMenuPending = false
	if sg.opts.OpenMenuOnFrontendStart && len(sg.opts.AttractDemos) == 0 {
		sg.frontend.AttractPage = "TITLEPIC"
		sg.frontendMenuPending = true
		sg.stopAndClearMusic()
		sg.playTitleMusic()
		return
	}
	if !sg.advanceFrontendAttract() {
		sg.stopAndClearMusic()
		sg.playTitleMusic()
	}
}

func (sg *sessionGame) startAttractDemoByName(name string) bool {
	if sg == nil || len(sg.opts.AttractDemos) == 0 || sg.opts.DemoMapLoader == nil {
		if sg != nil {
			fmt.Printf("attract-error step=%s reason=no-demo-loader-or-demos\n", strings.ToUpper(strings.TrimSpace(name)))
		}
		return false
	}
	want := strings.ToUpper(strings.TrimSpace(name))
	found := false
	for _, demo := range sg.opts.AttractDemos {
		if demo == nil || !strings.EqualFold(strings.TrimSpace(demo.Path), want) {
			continue
		}
		found = true
		m, err := sg.opts.DemoMapLoader(demo)
		if err != nil {
			fmt.Printf("attract-error step=%s reason=demo-map-load err=%v\n", want, err)
			continue
		}
		if m == nil {
			fmt.Printf("attract-error step=%s reason=demo-map-load returned nil map\n", want)
			continue
		}
		sg.capturePersistentSettings()
		sg.applyPersistentSettingsToOptions()
		demoOpts := sg.opts
		demoOpts.DemoScript = demo
		demoOpts.DemoQuitOnComplete = false
		demoOpts.RecordDemoPath = ""
		ng := sg.buildGame(cloneMapForRestart(m), demoOpts)
		sg.applyPersistentSettingsToGame(ng)
		sg.g = ng
		sg.rt = ng
		sg.stopAndClearMusic()
		sg.playMusicForMap(m.Name)
		return true
	}
	if !found {
		fmt.Printf("attract-error step=%s reason=missing-demo-lump\n", want)
	}
	return false
}

func (sg *sessionGame) frontendAttractSequence() []string {
	if sg == nil {
		return nil
	}
	return sessionflow.FrontendAttractSequence(sg.bootMap.Name, sg.opts.Episodes, sg.hasAttractDemo("DEMO4"))
}

func (sg *sessionGame) hasAttractDemo(name string) bool {
	if sg == nil {
		return false
	}
	return sessionflow.HasAttractDemo(sg.opts.AttractDemos, name)
}

func (sg *sessionGame) advanceFrontendAttract() bool {
	if sg == nil || !sg.frontend.Active {
		return false
	}
	seq := sg.frontendAttractSequence()
	if len(seq) == 0 {
		fmt.Printf("attract-error reason=empty-sequence\n")
		return false
	}
	commercial := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(sg.bootMap.Name))), "MAP")
	for i := 0; i < len(seq); i++ {
		var action sessionflow.AttractAction
		var ok bool
		sg.frontend, action, ok = sessionflow.AdvanceAttract(
			sessionflow.Frontend(sg.frontend),
			seq,
			commercial,
			attractPageTitleCommercial,
			attractPageTitleNonCommercial,
			attractPageInfo,
		)
		if !ok {
			break
		}
		switch action.Kind {
		case sessionflow.AttractActionDemo:
			if sg.startAttractDemoByName(action.Name) {
				return true
			}
			continue
		case sessionflow.AttractActionPage:
			if action.PlayTitle {
				sg.playTitleMusic()
			}
			if sg.g != nil {
				sg.g.opts.DemoScript = nil
			}
			return true
		}
	}
	fmt.Printf("attract-error reason=no-playable-attract-step sequence=%v\n", seq)
	return false
}

func (sg *sessionGame) playTitleMusic() {
	if sg == nil || sg.musicCtl == nil {
		return
	}
	sg.musicCtl.PlayTitle(clampVolume(sg.opts.MusicVolume))
	sg.setNowPlayingLevel("")
	sg.setNowPlayingMusic("Title Screen")
}

func (sg *sessionGame) frontendStatus(msg string, tics int) {
	if sg == nil {
		return
	}
	sg.frontend.Status = strings.TrimSpace(msg)
	sg.frontend.StatusTic = tics
}

func (sg *sessionGame) playMenuMoveSound() {
	if sg != nil && sg.menuSfx != nil {
		sg.menuSfx.PlayMove()
	}
}

func (sg *sessionGame) playMenuConfirmSound() {
	if sg != nil && sg.menuSfx != nil {
		sg.menuSfx.PlayConfirm()
	}
}

func (sg *sessionGame) playMenuBackSound() {
	if sg != nil && sg.menuSfx != nil {
		sg.menuSfx.PlayBack()
	}
}

func (sg *sessionGame) requestQuitPrompt() {
	if sg == nil {
		return
	}
	sg.quitPrompt, sg.quitMessageSeq = sessionflow.StartQuitPrompt(sg.quitMessageSeq, doomQuitMessages)
}

func (sg *sessionGame) clearQuitPrompt() {
	if sg == nil {
		return
	}
	sg.quitPrompt = quitPromptState{}
}

func (sg *sessionGame) handleQuitPromptInput() error {
	if sg == nil || !sg.quitPrompt.Active {
		return nil
	}
	confirm := inpututil.IsKeyJustPressed(ebiten.KeyY)
	cancel := inpututil.IsKeyJustPressed(ebiten.KeyN) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace)
	if confirm {
		sg.playQuitPromptSound()
	}
	if cancel {
		sg.playMenuBackSound()
	}
	var done bool
	sg.quitPrompt, done = sessionflow.TickQuitPrompt(sg.quitPrompt, confirm, cancel)
	if done {
		return ebiten.Termination
	}
	return nil
}

func (sg *sessionGame) anyQuitPromptTrigger() bool {
	if sg == nil {
		return false
	}
	return inpututil.IsKeyJustPressed(ebiten.KeyF4) || inpututil.IsKeyJustPressed(ebiten.KeyF10)
}

func (sg *sessionGame) nextQuitPromptLines() []string {
	if sg == nil {
		return defaultQuitPromptLines()
	}
	sg.quitMessageSeq++
	msg := "are you sure you want to\nquit this great game?"
	if len(doomQuitMessages) > 0 {
		msg = doomQuitMessages[(sg.quitMessageSeq-1)%len(doomQuitMessages)]
	}
	lines := strings.Split(strings.ToUpper(msg), "\n")
	lines = append(lines, "(PRESS Y TO QUIT)")
	return lines
}

func defaultQuitPromptLines() []string {
	return sessionflow.DefaultQuitPromptLines()
}

func (sg *sessionGame) playQuitPromptSound() {
	if sg == nil || sg.menuSfx == nil {
		return
	}
	sg.menuSfx.PlayQuit(len(sg.opts.Episodes) == 0, sg.quitMessageSeq-1)
}

func (sg *sessionGame) startGameFromFrontend(skill int) {
	if sg == nil || sg.g == nil {
		return
	}
	sg.capturePersistentSettings()
	startMap := sessionflow.NewGameStartMap(
		sg.bootMap.Name,
		sg.availableFrontendEpisodeChoices(),
		sg.frontend.SelectedEpisode,
		sg.opts.NewGameLoader != nil,
	)
	if sg.opts.NewGameLoader != nil {
		if m, err := sg.opts.NewGameLoader(startMap); err == nil && m != nil {
			sg.bootMap = m
		}
	}
	sg.frontend = frontendState{}
	sg.stopAndClearMusic()
	sg.applyPersistentSettingsToOptions()
	gameOpts := sg.opts
	gameOpts.DemoScript = nil
	gameOpts.DemoQuitOnComplete = false
	gameOpts.RecordDemoPath = ""
	gameOpts.SkillLevel = normalizeSkillLevel(skill)
	sg.g = sg.buildGame(cloneMapForRestart(sg.bootMap), gameOpts)
	sg.applyPersistentSettingsToGame(sg.g)
	sg.rt = sg.g
	sg.current = sg.g.sessionSignals().MapName
	sg.currentTemplate = cloneMapForRestart(sg.g.m)
	sg.playMusicForMap(sg.current)
	sg.announceMapMusic(sg.current)
	ebiten.SetWindowTitle(runtimehost.WindowTitle(sg.current))
	sg.queueTransition(transitionLevel, 0)
}

func (sg *sessionGame) availableFrontendEpisodeChoices() []int {
	if sg == nil {
		return nil
	}
	return sessionflow.AvailableEpisodeChoices(sg.opts.Episodes)
}

func (sg *sessionGame) openReadThis(fromGame bool) {
	if sg == nil {
		return
	}
	sg.frontend = frontendState(sessionflow.OpenReadThis(sessionflow.Frontend(sg.frontend), fromGame))
}

func (sg *sessionGame) closeReadThis() {
	if sg == nil {
		return
	}
	sg.frontend = frontendState(sessionflow.CloseReadThis(sessionflow.Frontend(sg.frontend)))
}

func (sg *sessionGame) readThisPageNames() []string {
	if sg == nil {
		return []string{"HELP2", "HELP1"}
	}
	pages := make([]string, 0, 2)
	if _, _, ok := sg.intermissionPatch("HELP2"); ok {
		pages = append(pages, "HELP2")
	}
	if _, _, ok := sg.intermissionPatch("HELP1"); ok {
		pages = append(pages, "HELP1")
	}
	if len(pages) == 0 {
		if _, _, ok := sg.intermissionPatch("HELP"); ok {
			pages = append(pages, "HELP")
		} else if _, _, ok := sg.intermissionPatch("CREDIT"); ok {
			pages = append(pages, "CREDIT")
		}
	}
	if len(pages) == 0 {
		return []string{"HELP2", "HELP1"}
	}
	return pages
}

func (sg *sessionGame) readThisPageName(page int) string {
	pages := sg.readThisPageNames()
	if page < 0 || page >= len(pages) {
		return pages[0]
	}
	return pages[page]
}

func (sg *sessionGame) frontendChangeMessages() {
	if sg == nil {
		return
	}
	switch {
	case sg.g != nil:
		sg.g.hudMessagesEnabled = !sg.g.hudMessagesEnabled
		sg.settings.HUDMessages = sg.g.hudMessagesEnabled
		sg.g.publishRuntimeSettingsIfChanged()
	case sg.rt != nil:
		sg.settings.HUDMessages = sg.rt.sessionToggleHUDMessages()
		sg.rt.sessionPublishRuntimeSettings()
	default:
		return
	}
	if sg.settings.HUDMessages {
		sg.frontendStatus("MESSAGES ON", doomTicsPerSecond)
	} else {
		sg.frontendStatus("MESSAGES OFF", doomTicsPerSecond)
	}
}

func (sg *sessionGame) tickFrontendMusicPlayer() error {
	if sg == nil || !sg.frontend.Active {
		return nil
	}
	var advanceAttract bool
	sg.frontend, advanceAttract = sessionflow.AdvanceFrontendFrame(sessionflow.Frontend(sg.frontend), menuSkullBlinkTics)
	if advanceAttract {
		_ = sg.advanceFrontendAttract()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		sg.frontendMusicPlayerClose()
		sg.playMenuBackSound()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		if sg.frontendMusicPlayerMoveRow(-1) {
			sg.playMenuMoveSound()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		if sg.frontendMusicPlayerMoveRow(1) {
			sg.playMenuMoveSound()
		}
	}
	dir := 0
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dir = -1
	} else if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dir = 1
	}
	if dir != 0 && sg.frontendMusicPlayerAdjust(dir) {
		sg.playMenuMoveSound()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		if sg.frontendMusicPlayerPlaySelected() {
			sg.playMenuConfirmSound()
		} else {
			sg.playMenuBackSound()
		}
	}
	return nil
}

func (sg *sessionGame) frontendChangeScreenSize(dir int) {
	if sg == nil || sg.g == nil || dir == 0 {
		return
	}
	sg.g.adjustScreenBlocks(dir)
}

func (sg *sessionGame) frontendCycleScreenSize() {
	if sg == nil || sg.g == nil {
		return
	}
	minBlocks, maxBlocks := allowedScreenBlocksRange(sg.g.opts)
	if sg.g.screenBlocks >= maxBlocks {
		sg.g.screenBlocks = minBlocks
		sg.g.setHUDMessage(fmt.Sprintf("Status bar %s", sg.g.screenSizeLabel()), 70)
		return
	}
	sg.g.adjustScreenBlocks(1)
}

func (sg *sessionGame) frontendChangeHUDScale(dir int) {
	if sg == nil || sg.g == nil || dir == 0 {
		return
	}
	sg.g.adjustHUDScale(dir)
}

func (sg *sessionGame) frontendCycleHUDScale() {
	if sg == nil || sg.g == nil || len(sourcePortHUDScaleSteps) == 0 {
		return
	}
	if sg.g.hudScaleStep >= len(sourcePortHUDScaleSteps)-1 {
		sg.g.hudScaleStep = 0
		sg.g.statusBarCacheValid = false
		sg.g.setHUDMessage(fmt.Sprintf("HUD size %s", sg.g.hudScaleLabel()), 70)
		return
	}
	sg.g.adjustHUDScale(1)
}

func (sg *sessionGame) frontendChangePerfOverlay() {
	if sg == nil || sg.rt == nil {
		return
	}
	if sg.rt.sessionTogglePerfOverlay() {
		sg.frontendStatus("FPS ON", doomTicsPerSecond)
	} else {
		sg.frontendStatus("FPS OFF", doomTicsPerSecond)
	}
}

func (sg *sessionGame) frontendChangeMouseSensitivity(dir int) {
	if sg == nil || sg.rt == nil || dir == 0 {
		return
	}
	cur := sg.rt.sessionMouseLookSpeed()
	_, mouseThermoCount, _ := sg.frontendMouseSensitivityLayout(36, "M_MSENS")
	next := sessionflow.NextMouseSensitivityForCount(cur, dir, mouseThermoCount)
	if next == cur {
		return
	}
	sg.rt.sessionSetMouseLookSpeed(next)
	sg.opts.MouseLookSpeed = next
	sg.settings.MouseLookSpeed = next
	sg.frontendStatus(fmt.Sprintf("MOUSE SENSITIVITY %.2f", next), doomTicsPerSecond)
}

func (sg *sessionGame) frontendCycleMouseSensitivity() {
	if sg == nil || sg.rt == nil {
		return
	}
	cur := sg.rt.sessionMouseLookSpeed()
	_, mouseThermoCount, _ := sg.frontendMouseSensitivityLayout(36, "MOUSE SENSITIVITY")
	next := sessionflow.NextMouseSensitivityForCount(cur, 1, mouseThermoCount)
	if next == cur {
		next = sessionflow.MouseSensitivitySpeedForDotCount(0, mouseThermoCount)
	}
	sg.rt.sessionSetMouseLookSpeed(next)
	sg.opts.MouseLookSpeed = next
	sg.settings.MouseLookSpeed = next
	sg.frontendStatus(fmt.Sprintf("MOUSE SENSITIVITY %.2f", next), doomTicsPerSecond)
}

func (sg *sessionGame) frontendChangeMusicVolume(dir int) {
	if sg == nil || sg.rt == nil || dir == 0 {
		return
	}
	cur := sg.rt.sessionMusicVolume()
	prev := clampVolume(cur)
	next := clampVolume(cur + float64(dir)*0.1)
	if next == cur {
		return
	}
	sg.rt.sessionSetMusicVolume(next)
	sg.opts.MusicVolume = next
	sg.settings.MusicVolume = next
	switch {
	case next <= 0:
		sg.stopAndClearMusic()
	case prev <= 0:
		if sg.frontend.Active && !sg.frontend.InGame {
			sg.playTitleMusic()
		} else {
			sg.playMusicForMap(sg.current)
		}
	case sg.musicCtl != nil:
		sg.musicCtl.SetVolume(next)
	}
	sg.rt.sessionPublishRuntimeSettings()
}

func (sg *sessionGame) frontendCycleMusicVolume() {
	if sg == nil || sg.rt == nil {
		return
	}
	cur := sg.rt.sessionMusicVolume()
	next := clampVolume(cur + 0.1)
	if next == cur {
		next = 0
	}
	sg.rt.sessionSetMusicVolume(next)
	sg.opts.MusicVolume = next
	sg.settings.MusicVolume = next
	switch {
	case next <= 0:
		sg.stopAndClearMusic()
	case cur <= 0:
		if sg.frontend.Active && !sg.frontend.InGame {
			sg.playTitleMusic()
		} else {
			sg.playMusicForMap(sg.current)
		}
	case sg.musicCtl != nil:
		sg.musicCtl.SetVolume(next)
	}
	sg.rt.sessionPublishRuntimeSettings()
}

func (sg *sessionGame) frontendChangeSFXVolume(dir int) {
	if sg == nil || sg.rt == nil || dir == 0 {
		return
	}
	cur := sg.rt.sessionSFXVolume()
	next := clampVolume(cur + float64(dir)*0.1)
	if next == cur {
		return
	}
	sg.rt.sessionSetSFXVolume(next)
	sg.opts.SFXVolume = next
	sg.settings.SFXVolume = next
	sg.menuSfx = sessionaudio.NewMenuController(sg.opts.SoundBank, next)
	sg.rt.sessionPublishRuntimeSettings()
	sg.playMenuMoveSound()
}

func (sg *sessionGame) frontendCycleSFXVolume() {
	if sg == nil || sg.rt == nil {
		return
	}
	cur := sg.rt.sessionSFXVolume()
	next := clampVolume(cur + 0.1)
	if next == cur {
		next = 0
	}
	sg.rt.sessionSetSFXVolume(next)
	sg.opts.SFXVolume = next
	sg.settings.SFXVolume = next
	sg.menuSfx = sessionaudio.NewMenuController(sg.opts.SoundBank, next)
	sg.rt.sessionPublishRuntimeSettings()
	sg.playMenuMoveSound()
}

func (sg *sessionGame) tickFrontend() error {
	if sg == nil || !sg.frontend.Active {
		return nil
	}
	if sg.frontendMenuPending {
		sg.frontend.MenuActive = true
		sg.frontendMenuPending = false
	}
	if sg.frontend.Mode == frontendModeMusicPlayer {
		return sg.tickFrontendMusicPlayer()
	}
	var advanceAttract bool
	sg.frontend, advanceAttract = sessionflow.AdvanceFrontendFrame(sessionflow.Frontend(sg.frontend), menuSkullBlinkTics)
	if advanceAttract {
		_ = sg.advanceFrontendAttract()
	}
	input := sessionflow.FrontendInput{
		Escape: inpututil.IsKeyJustPressed(ebiten.KeyEscape),
		Up:     inpututil.IsKeyJustPressed(ebiten.KeyArrowUp),
		Down:   inpututil.IsKeyJustPressed(ebiten.KeyArrowDown),
		Left:   inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft),
		Right:  inpututil.IsKeyJustPressed(ebiten.KeyArrowRight),
		Select: inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter),
		Skip:   anyIntermissionSkipInput(),
	}
	if sg.frontend.Mode == frontendModeOptions && sg.frontend.OptionsOn == frontendOptionsRowMusicPlayer && input.Select {
		if sg.frontendMusicPlayerOpen() {
			sg.playMenuConfirmSound()
		} else {
			sg.frontendStatus("NO MUSIC CATALOG", doomTicsPerSecond*2)
			sg.playMenuBackSound()
		}
		return nil
	}
	result := sessionflow.StepFrontend(
		sessionflow.Frontend(sg.frontend),
		input,
		sessionflow.FrontendConfig{
			ReadThisPageCount: len(sg.readThisPageNames()),
			EpisodeChoices:    sg.availableFrontendEpisodeChoices(),
			OptionRows:        frontendOptionsSelectableRows[:],
			MainMenuCount:     len(frontendMainMenuNames),
			SkillMenuCount:    len(frontendSkillMenuNames),
			StatusTics:        doomTicsPerSecond,
		},
	)
	sg.frontend = frontendState(result.State)
	switch result.Sound {
	case sessionflow.FrontendSoundMove:
		sg.playMenuMoveSound()
	case sessionflow.FrontendSoundConfirm:
		sg.playMenuConfirmSound()
	case sessionflow.FrontendSoundBack:
		sg.playMenuBackSound()
	}
	if result.StatusMessage != "" {
		sg.frontendStatus(result.StatusMessage, result.StatusMessageTic)
	}
	if result.ChangeMessages {
		sg.frontendChangeMessages()
	}
	if result.ChangeDetail {
		if input.Select {
			switch sg.frontend.OptionsOn {
			case 2:
				sg.frontendCycleHUDScale()
			default:
				sg.frontendCycleScreenSize()
			}
		} else {
			dir := 1
			if input.Left && !input.Right {
				dir = -1
			}
			switch sg.frontend.OptionsOn {
			case 2:
				sg.frontendChangeHUDScale(dir)
			default:
				sg.frontendChangeScreenSize(dir)
			}
		}
	}
	if result.ChangePerf {
		sg.frontendChangePerfOverlay()
	}
	if result.ChangeMouse != 0 {
		if input.Select {
			sg.frontendCycleMouseSensitivity()
		} else {
			sg.frontendChangeMouseSensitivity(result.ChangeMouse)
		}
	}
	if result.ChangeMusic != 0 {
		if input.Select {
			sg.frontendCycleMusicVolume()
		} else {
			sg.frontendChangeMusicVolume(result.ChangeMusic)
		}
	}
	if result.ChangeSFX != 0 {
		if input.Select {
			sg.frontendCycleSFXVolume()
		} else {
			sg.frontendChangeSFXVolume(result.ChangeSFX)
		}
	}
	if result.RequestQuit {
		sg.requestQuitPrompt()
	}
	if result.StartGameSkill > 0 {
		sg.startGameFromFrontend(result.StartGameSkill)
	}
	return nil
}

func (sg *sessionGame) drawFrontendTransitionSurface(dst *ebiten.Image) {
	if dst == nil {
		return
	}
	if sg.opts.SourcePortMode {
		dw := max(dst.Bounds().Dx(), 1)
		dh := max(dst.Bounds().Dy(), 1)
		dst.Fill(color.Black)
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != dw || sg.presentSurface.Bounds().Dy() != dh {
			sg.presentSurface = newUnmanagedImage(dw, dh)
		}
		sg.presentSurface.Clear()
		sg.drawFrontend(sg.presentSurface)
		sg.drawSourcePortPresented(dst, sg.presentSurface, dw, dh)
		return
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != dw || sg.presentSurface.Bounds().Dy() != dh {
		sg.presentSurface = newUnmanagedImage(dw, dh)
	}
	sg.presentSurface.Clear()
	sg.drawFrontend(sg.presentSurface)
	dst.Fill(color.Black)
	dst.DrawImage(sg.presentSurface, nil)
}

func (sg *sessionGame) drawFrontend(screen *ebiten.Image) {
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

	switch sg.frontend.Mode {
	case frontendModeReadThis:
		sg.drawFrontendAttractBackground(screen)
		name := sg.readThisPageName(sg.frontend.ReadThisPage)
		if !sg.drawIntermissionPatch(screen, name, 0, 0, scale, ox, oy, false) && !sg.drawFrontendPage(screen, "TITLEPIC") {
			screen.Fill(color.Black)
		}
		if (sg.frontend.Tic/16)&1 == 0 {
			prompt := "PRESS ANY KEY TO RETURN"
			if sg.frontend.ReadThisPage+1 < len(sg.readThisPageNames()) {
				prompt = "PRESS ANY KEY TO CONTINUE"
			}
			sg.drawIntermissionText(screen, prompt, 160, 186, scale, ox, oy, true)
		}
		return
	case frontendModeSound:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		sg.drawFrontendSoundMenu(screen, scale, ox, oy)
		return
	case frontendModeOptions:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		sg.drawFrontendOptionsMenu(screen, scale, ox, oy)
		return
	case frontendModeMusicPlayer:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		sg.drawFrontendMusicPlayerMenu(screen, scale, ox, oy)
		return
	case frontendModeEpisode:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		_ = sg.drawMenuPatch(screen, "M_NEWG", 96, 14, scale, ox, oy, false)
		_ = sg.drawMenuPatch(screen, "M_EPISOD", 54, 38, scale, ox, oy, false)
		episodes := sg.availableFrontendEpisodeChoices()
		for i, ep := range episodes {
			if name, ok := frontendEpisodeMenuNames[ep]; ok {
				_ = sg.drawMenuPatch(screen, name, 48, 63+i*16, scale, ox, oy, false)
			}
		}
		sg.drawMenuSkull(screen, 16, 63+sg.frontend.EpisodeOn*16, scale, ox, oy)
		return
	case frontendModeSkill:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		_ = sg.drawMenuPatch(screen, "M_NEWG", 96, 14, scale, ox, oy, false)
		_ = sg.drawMenuPatch(screen, "M_SKILL", 54, 38, scale, ox, oy, false)
		for i, name := range frontendSkillMenuNames {
			_ = sg.drawMenuPatch(screen, name, 48, 63+i*16, scale, ox, oy, false)
		}
		sg.drawMenuSkull(screen, 16, 63+sg.frontend.SkillOn*16, scale, ox, oy)
		return
	default:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.Active {
			return
		}
		if sg.frontend.MenuActive {
			if sg.frontend.InGame {
				_ = sg.drawMenuPatch(screen, "M_PAUSE", 126, 4, scale, ox, oy, false)
				for i, name := range inGamePauseMenuNames {
					_ = sg.drawMenuPatch(screen, name, 97, 64+i*16, scale, ox, oy, false)
				}
			} else {
				sg.drawFrontendMainMenuTitle(screen, scale, ox, oy)
				for i, name := range frontendMainMenuNames {
					_ = sg.drawMenuPatch(screen, name, 97, 64+i*16, scale, ox, oy, false)
				}
			}
			sg.drawMenuSkull(screen, 65, 64+sg.frontend.ItemOn*16, scale, ox, oy)
		}
		if msg := strings.TrimSpace(sg.frontend.Status); msg != "" {
			sg.drawIntermissionText(screen, msg, 160, 178, scale, ox, oy, true)
		}
	}
}

func (sg *sessionGame) drawFrontendMainMenuTitle(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil || screen == nil {
		return
	}
	_ = sg.drawMenuPatch(screen, "M_DOOM", 94, 2, scale, ox, oy, false)
}

func (sg *sessionGame) drawFrontendBackdrop(screen *ebiten.Image, showLogo bool) {
	if sg == nil || screen == nil {
		return
	}
	sg.drawFrontendAttractBackground(screen)
	if sg.frontend.MenuActive || sg.frontend.InGame {
		ebitenutil.DrawRect(screen, 0, 0, float64(max(screen.Bounds().Dx(), 1)), float64(max(screen.Bounds().Dy(), 1)), color.RGBA{A: 128})
	}
	if showLogo && sg.frontend.Mode == frontendModeTitle && sg.frontend.MenuActive {
		return
	}
}

func (sg *sessionGame) drawFrontendAttractBackground(screen *ebiten.Image) {
	if sg == nil || screen == nil {
		return
	}
	if sg.frontend.InGame && sg.g != nil {
		sg.drawGamePresented(screen, sg.g)
		return
	}
	if sg.g != nil && sg.g.sessionSignals().DemoActive {
		sg.drawGamePresented(screen, sg.g)
		return
	}
	if sg.drawFrontendPage(screen, sg.frontend.AttractPage) {
		return
	}
	screen.Fill(color.Black)
}

func (sg *sessionGame) drawFrontendPage(screen *ebiten.Image, name string) bool {
	if sg == nil || screen == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "TITLEPIC":
		sg.drawBootSplashPresented(screen)
		return true
	case "CREDIT", "HELP1", "HELP2":
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
		screen.Fill(color.Black)
		return sg.drawIntermissionPatch(screen, name, 0, 0, scale, ox, oy, false)
	default:
		return false
	}
}

func (sg *sessionGame) drawFrontendOptionsMenu(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil || sg.g == nil {
		return
	}
	sig := sg.g.sessionSignals()
	const menuX = 36
	const menuY = 37
	const lineHeight = 16
	_ = sg.drawMenuPatch(screen, "M_OPTTTL", menuX, 15, scale, ox, oy, false)
	backLabel := "BACK: ESC"
	backX := 320 - 8 - int(math.Ceil(float64(sg.intermissionTextWidth(backLabel))*1.2))
	sg.rt.sessionDrawHUTextAt(screen, backLabel, ox+float64(backX)*scale, oy+float64(17)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "MESSAGES", ox+float64(menuX)*scale, oy+float64(menuY+0*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "STATUS BAR MODE", ox+float64(menuX)*scale, oy+float64(menuY+1*lineHeight+2)*scale, scale*1.2, scale*1.2)
	msgLabel := "OFF"
	if sig.HUDMessages {
		msgLabel = "ON"
	}
	sg.rt.sessionDrawHUTextAt(screen, msgLabel, ox+float64(menuX+215)*scale, oy+float64(menuY+0*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, sg.g.screenSizeLabel(), ox+float64(menuX+215)*scale, oy+float64(menuY+1*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "HUD SIZE", ox+float64(menuX)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, sg.g.hudScaleLabel(), ox+float64(menuX+215)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "FPS", ox+float64(menuX)*scale, oy+float64(menuY+3*lineHeight+2)*scale, scale*1.2, scale*1.2)
	fpsLabel := "OFF"
	if sig.ShowPerf {
		fpsLabel = "ON"
	}
	sg.rt.sessionDrawHUTextAt(screen, fpsLabel, ox+float64(menuX+215)*scale, oy+float64(menuY+3*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "MOUSE SENSITIVITY", ox+float64(menuX)*scale, oy+float64(menuY+4*lineHeight+2)*scale, scale*1.2, scale*1.2)
	optionsSkullX := sg.frontendOptionsSkullX(menuX)
	sg.rt.sessionDrawHUTextAt(screen, formatFloat2(sig.MouseLookSpeed), ox+float64(menuX+215)*scale, oy+float64(menuY+4*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "EFFECTS VOLUME", ox+float64(menuX)*scale, oy+float64(menuY+5*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, formatInt(sessionflow.VolumeDot(sig.SFXVolume)), ox+float64(menuX+215)*scale, oy+float64(menuY+5*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "MUSIC VOLUME", ox+float64(menuX)*scale, oy+float64(menuY+6*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, formatInt(sessionflow.VolumeDot(sig.MusicVolume)), ox+float64(menuX+215)*scale, oy+float64(menuY+6*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, "MUSIC PLAYER", ox+float64(menuX)*scale, oy+float64(menuY+7*lineHeight+2)*scale, scale*1.2, scale*1.2)
	playerLabel := "OPEN"
	if !sg.frontendMusicPlayerAvailable() {
		playerLabel = "N/A"
	}
	sg.rt.sessionDrawHUTextAt(screen, playerLabel, ox+float64(menuX+215)*scale, oy+float64(menuY+7*lineHeight+2)*scale, scale*1.2, scale*1.2)
	sg.drawMenuSkull(screen, optionsSkullX, menuY+sg.frontend.OptionsOn*lineHeight, scale, ox, oy)
}

func (sg *sessionGame) drawFrontendMusicPlayerMenu(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil || sg.g == nil {
		return
	}
	sg.frontendMusicPlayerClamp()
	const menuX = 24
	const menuY = 42
	const lineHeight = 16
	const valueX = 70
	backLabel := "BACK: ESC"
	backX := 320 - 8 - int(math.Ceil(float64(sg.intermissionTextWidth(backLabel))*1.2))
	sg.rt.sessionDrawHUTextAt(screen, "MUSIC PLAYER", ox+float64(menuX)*scale, oy+float64(18)*scale, scale*1.4, scale*1.4)
	sg.rt.sessionDrawHUTextAt(screen, backLabel, ox+float64(backX)*scale, oy+float64(18)*scale, scale*1.2, scale*1.2)
	wad := sg.frontendMusicPlayerWAD()
	episode := sg.frontendMusicPlayerEpisode()
	track := sg.frontendMusicPlayerTrack()
	values := [frontendMusicPlayerRowCount]string{"-", "-", "-"}
	if wad != nil {
		values[frontendMusicPlayerRowWAD] = strings.ToUpper(strings.TrimSpace(wad.Label))
	}
	if episode != nil {
		values[frontendMusicPlayerRowGroup] = strings.ToUpper(strings.TrimSpace(episode.Label))
	}
	if track != nil {
		values[frontendMusicPlayerRowTrack] = strings.ToUpper(strings.TrimSpace(track.Label))
	}
	labels := [frontendMusicPlayerRowCount]string{"WAD", "GROUP", "TRACK"}
	for i, label := range labels {
		y := menuY + i*lineHeight + 2
		sg.rt.sessionDrawHUTextAt(screen, label, ox+float64(menuX)*scale, oy+float64(y)*scale, scale*1.2, scale*1.2)
		sg.rt.sessionDrawHUTextAt(screen, values[i], ox+float64(menuX+valueX)*scale, oy+float64(y)*scale, scale*1.2, scale*1.2)
	}
	sg.rt.sessionDrawHUTextAt(screen, "CURRENTLY PLAYING", ox+float64(menuX)*scale, oy+float64(116)*scale, scale*1.0, scale*1.0)
	sg.rt.sessionDrawHUTextAt(screen, sg.nowPlayingMusicLabel(), ox+float64(menuX)*scale, oy+float64(128)*scale, scale*1.0, scale*1.0)
	sg.rt.sessionDrawHUTextAt(screen, "LEFT/RIGHT CHANGE  ENTER PLAY", ox+float64(menuX)*scale, oy+float64(160)*scale, scale*1.0, scale*1.0)
	if msg := strings.TrimSpace(sg.frontend.Status); msg != "" {
		sg.drawIntermissionText(screen, msg, 160, 182, scale, ox, oy, true)
	}
	sg.drawMenuSkull(screen, sg.frontendMusicPlayerSkullX(menuX), menuY+sg.musicPlayer.Row*lineHeight, scale, ox, oy)
}

func (sg *sessionGame) drawFrontendSoundMenu(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil || sg.g == nil {
		return
	}
	sig := sg.g.sessionSignals()
	const menuX = 80
	const menuY = 64
	const lineHeight = 16
	_ = sg.drawMenuPatch(screen, "M_SVOL", 60, 38, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, "M_SFXVOL", menuX, menuY, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, "M_MUSVOL", menuX, menuY+2*lineHeight, scale, ox, oy, false)
	sg.rt.sessionDrawHUTextAt(screen, formatInt(sessionflow.VolumeDot(sig.SFXVolume)), ox+float64(235)*scale, oy+float64(menuY+2)*scale, scale*1.2, scale*1.2)
	sg.rt.sessionDrawHUTextAt(screen, formatInt(sessionflow.VolumeDot(sig.MusicVolume)), ox+float64(235)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.2, scale*1.2)
	skullY := menuY
	if sg.frontend.SoundOn != 0 {
		skullY += 2 * lineHeight
	}
	sg.drawMenuSkull(screen, menuX-32, skullY, scale, ox, oy)
}

func (sg *sessionGame) drawFrontendThermo(screen *ebiten.Image, x, y, width, dot int, scale, ox, oy float64) {
	if sg == nil {
		return
	}
	if width < 1 {
		width = 1
	}
	if dot < 0 {
		dot = 0
	}
	if dot > width-1 {
		dot = width - 1
	}
	if !sg.drawMenuPatch(screen, "M_THERML", x, y, scale, ox, oy, false) {
		return
	}
	for i := 0; i < width; i++ {
		_ = sg.drawMenuPatch(screen, "M_THERMM", x+8+i*8, y, scale, ox, oy, false)
	}
	_ = sg.drawMenuPatch(screen, "M_THERMR", x+8+width*8, y, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, "M_THERMO", x+8+dot*8, y, scale, ox, oy, false)
}

func (sg *sessionGame) frontendMouseSensitivityLayout(menuX int, label string) (thermoX, thermoCount, valueX int) {
	const (
		menuRightEdge = 320
		rightMargin   = 8
		valueWidth    = 28
		labelGap      = 2
	)
	thermoCount = sessionflow.MouseSensitivitySliderDots()
	labelRight := menuX + int(math.Ceil(float64(sg.intermissionTextWidth(label))*1.2))
	thermoX = labelRight + labelGap
	maxAvailable := menuRightEdge - rightMargin - valueWidth - thermoX
	fitCount := (maxAvailable - 16) / 8
	if fitCount < 3 {
		fitCount = 3
	}
	if fitCount > thermoCount {
		fitCount = thermoCount
	}
	if fitCount%2 == 0 {
		fitCount--
	}
	if fitCount < 3 {
		fitCount = 3
	}
	thermoCount = fitCount
	valueX = thermoX + 16 + thermoCount*8 + 4
	return thermoX, thermoCount, valueX
}

func (sg *sessionGame) frontendOptionsSkullX(menuX int) int {
	const gap = 8
	leftEdge := menuX
	haveLabel := false
	for _, name := range frontendOptionsMenuNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		_, p, ok := sg.menuPatch(name)
		if !ok {
			continue
		}
		labelLeft := menuX - p.OffsetX
		if !haveLabel || labelLeft < leftEdge {
			leftEdge = labelLeft
			haveLabel = true
		}
	}
	if _, p, ok := sg.menuPatch("M_SKULL1"); ok {
		return leftEdge - gap - p.Width + p.OffsetX
	}
	return leftEdge - 32
}

func (sg *sessionGame) frontendMusicPlayerSkullX(menuX int) int {
	const gap = 8
	if _, p, ok := sg.menuPatch("M_SKULL1"); ok {
		x := menuX - gap - p.Width + p.OffsetX
		if x < p.OffsetX {
			x = p.OffsetX
		}
		return x
	}
	if menuX < 32 {
		return menuX
	}
	return menuX - 32
}

func (sg *sessionGame) drawQuitPrompt(screen *ebiten.Image) {
	if sg == nil || !sg.quitPrompt.Active || screen == nil {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	scale := quitPromptScaleForSize(sw, sh)
	ox := (float64(sw) - 320.0*scale) * 0.5
	oy := (float64(sh) - 200.0*scale) * 0.5
	ebitenutil.DrawRect(screen, 0, 0, float64(sw), float64(sh), color.RGBA{R: 8, G: 8, B: 8, A: 128})
	lines := sg.quitPromptLinesForRenderSize(sw, sh)
	startY := 84 - ((len(lines) - 2) * 7)
	for i, line := range lines {
		sg.drawIntermissionText(screen, line, 160, startY+i*14, scale, ox, oy, true)
	}
}

func quitPromptScaleForSize(sw, sh int) float64 {
	sw = max(sw, 1)
	sh = max(sh, 1)
	scale := float64(sw) / 320.0
	scaleY := float64(sh) / 200.0
	if scaleY < scale {
		scale = scaleY
	}
	if scale <= 0 {
		return 1
	}
	return scale
}

func (sg *sessionGame) quitPromptLinesForRenderSize(sw, sh int) []string {
	lines := sg.quitPrompt.Lines
	if len(lines) == 0 {
		lines = defaultQuitPromptLines()
	}
	if sg.quitPromptFitsRenderSize(lines, sw, sh) {
		return lines
	}
	fallback := defaultQuitPromptLines()
	if sg.quitPromptFitsRenderSize(fallback, sw, sh) {
		return fallback
	}
	return lines
}

func (sg *sessionGame) quitPromptFitsRenderSize(lines []string, sw, sh int) bool {
	if len(lines) == 0 {
		lines = defaultQuitPromptLines()
	}
	scale := quitPromptScaleForSize(sw, sh)
	startY := 84 - ((len(lines) - 2) * 7)
	endY := startY + (len(lines)-1)*14 + sg.intermissionTextLineHeight()
	if startY < 0 || endY > 200 {
		return false
	}
	maxWidth := 0
	for _, line := range lines {
		if w := sg.intermissionTextWidth(line); w > maxWidth {
			maxWidth = w
		}
	}
	return float64(maxWidth)*scale <= 320.0*scale
}

func drawCenteredIntegerScaledLogo(screen, img *ebiten.Image) {
	if screen == nil || img == nil {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	lw := max(img.Bounds().Dx(), 1)
	lh := max(img.Bounds().Dy(), 1)
	scaleW := int(0.7 * float64(sw) / float64(lw))
	scaleH := int(0.6 * float64(sh) / float64(lh))
	scale := min(max(scaleW, 1), max(scaleH, 1))
	if scale < 1 {
		scale = 1
	}
	dw := lw * scale
	dh := lh * scale
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(float64((sw-dw)/2), float64((sh-dh)/2))
	screen.DrawImage(img, op)
}

func (sg *sessionGame) drawMenuSkull(screen *ebiten.Image, x, y int, scale, ox, oy float64) {
	name := "M_SKULL1"
	if sg.frontend.WhichSkull != 0 {
		name = "M_SKULL2"
	}
	_ = sg.drawMenuPatch(screen, name, x, y, scale, ox, oy, false)
}

func (sg *sessionGame) drawMenuPatch(screen *ebiten.Image, name string, x, y int, scale, ox, oy float64, centered bool) bool {
	img, p, ok := sg.menuPatch(name)
	if !ok {
		return false
	}
	px := float64(x)*scale + ox
	py := float64(y)*scale + oy
	if centered {
		px -= float64(p.Width) * scale * 0.5
		py -= float64(p.Height) * scale * 0.5
	}
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(px-float64(p.OffsetX)*scale, py-float64(p.OffsetY)*scale)
	screen.DrawImage(img, op)
	return true
}

func (sg *sessionGame) menuPatch(name string) (*ebiten.Image, WallTexture, bool) {
	if sg == nil || sg.opts.MenuPatchBank == nil {
		return nil, WallTexture{}, false
	}
	key := strings.ToUpper(strings.TrimSpace(name))
	p, ok := sg.opts.MenuPatchBank[key]
	if !ok {
		return nil, WallTexture{}, false
	}
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	return img, p, true
}
