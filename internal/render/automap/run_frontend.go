package automap

import (
	"fmt"
	"image/color"
	"strings"

	"gddoom/internal/runtimehost"
	"gddoom/internal/sessionaudio"
	"gddoom/internal/sessionflow"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

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
	if sg == nil || sg.musicCtl == nil || sg.opts.TitleMusicLoader == nil {
		return
	}
	sg.stopAndClearMusic()
	if clampVolume(sg.opts.MusicVolume) <= 0 {
		return
	}
	data, err := sg.opts.TitleMusicLoader()
	if err != nil || len(data) == 0 {
		return
	}
	sg.musicCtl.PlayMUS(data)
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

func (sg *sessionGame) frontendDetailLow() bool {
	if sg == nil || sg.g == nil {
		return false
	}
	sig := sg.g.sessionSignals()
	if sig.SourcePortMode {
		return sig.SourcePortDetail > 1
	}
	return sig.LowDetail
}

func (sg *sessionGame) frontendSourcePortDetailLabel() string {
	if sg == nil || sg.g == nil {
		return "FULL"
	}
	return sessionflow.SourcePortDetailLabel(sg.g.sessionSignals().SourcePortDetail)
}

func (sg *sessionGame) frontendChangeMessages() {
	if sg == nil || sg.rt == nil {
		return
	}
	sg.settings.HUDMessages = sg.rt.sessionToggleHUDMessages()
	if sg.settings.HUDMessages {
		sg.frontendStatus("MESSAGES ON", doomTicsPerSecond)
	} else {
		sg.frontendStatus("MESSAGES OFF", doomTicsPerSecond)
	}
}

func (sg *sessionGame) frontendChangeDetail() {
	if sg == nil || sg.rt == nil {
		return
	}
	sg.settings.DetailLevel = sg.rt.sessionCycleDetail()
	sg.opts.InitialDetailLevel = sg.settings.DetailLevel
}

func (sg *sessionGame) frontendChangeMouseSensitivity(dir int) {
	if sg == nil || sg.rt == nil || dir == 0 {
		return
	}
	cur := sg.rt.sessionMouseLookSpeed()
	next := sessionflow.NextMouseSensitivity(cur, dir)
	if next == cur {
		return
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
	next := clampVolume(cur + float64(dir)*(1.0/15.0))
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
		if sg.frontend.Active {
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
	next := clampVolume(cur + float64(dir)*(1.0/15.0))
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

func (sg *sessionGame) tickFrontend() error {
	if sg == nil || !sg.frontend.Active {
		return nil
	}
	var advanceAttract bool
	sg.frontend, advanceAttract = sessionflow.AdvanceFrontendFrame(sessionflow.Frontend(sg.frontend), menuSkullBlinkTics)
	if advanceAttract {
		_ = sg.advanceFrontendAttract()
	}
	result := sessionflow.StepFrontend(
		sessionflow.Frontend(sg.frontend),
		sessionflow.FrontendInput{
			Escape: inpututil.IsKeyJustPressed(ebiten.KeyEscape),
			Up:     inpututil.IsKeyJustPressed(ebiten.KeyArrowUp),
			Down:   inpututil.IsKeyJustPressed(ebiten.KeyArrowDown),
			Left:   inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft),
			Right:  inpututil.IsKeyJustPressed(ebiten.KeyArrowRight),
			Select: inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter),
			Skip:   anyIntermissionSkipInput(),
		},
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
		sg.frontendChangeDetail()
	}
	if result.ChangeMouse != 0 {
		sg.frontendChangeMouseSensitivity(result.ChangeMouse)
	}
	if result.ChangeMusic != 0 {
		sg.frontendChangeMusicVolume(result.ChangeMusic)
	}
	if result.ChangeSFX != 0 {
		sg.frontendChangeSFXVolume(result.ChangeSFX)
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
			sg.presentSurface = ebiten.NewImage(dw, dh)
		}
		sg.presentSurface.Clear()
		sg.drawFrontend(sg.presentSurface)
		sg.drawSourcePortPresented(dst, sg.presentSurface, dw, dh)
		return
	}
	dw := max(dst.Bounds().Dx(), 1)
	dh := max(dst.Bounds().Dy(), 1)
	if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != dw || sg.presentSurface.Bounds().Dy() != dh {
		sg.presentSurface = ebiten.NewImage(dw, dh)
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
			_ = sg.drawMenuPatch(screen, "M_DOOM", 94, 2, scale, ox, oy, false)
			for i, name := range frontendMainMenuNames {
				_ = sg.drawMenuPatch(screen, name, 97, 64+i*16, scale, ox, oy, false)
			}
			sg.drawMenuSkull(screen, 65, 64+sg.frontend.ItemOn*16, scale, ox, oy)
		}
		if msg := strings.TrimSpace(sg.frontend.Status); msg != "" {
			sg.drawIntermissionText(screen, msg, 160, 178, scale, ox, oy, true)
		}
	}
}

func (sg *sessionGame) drawFrontendBackdrop(screen *ebiten.Image, showLogo bool) {
	if sg == nil || screen == nil {
		return
	}
	sg.drawFrontendAttractBackground(screen)
	if showLogo && sg.frontend.Mode == frontendModeTitle && sg.frontend.MenuActive {
		return
	}
}

func (sg *sessionGame) drawFrontendAttractBackground(screen *ebiten.Image) {
	if sg == nil || screen == nil {
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
	const menuX = 60
	const menuY = 37
	const lineHeight = 16
	_ = sg.drawMenuPatch(screen, "M_OPTTTL", 108, 15, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, sessionflow.MessagesPatch(sig.HUDMessages), menuX+120, menuY+1*lineHeight, scale, ox, oy, false)
	if sig.SourcePortMode {
		sg.rt.sessionDrawHUTextAt(screen, sg.frontendSourcePortDetailLabel(), ox+float64(menuX+175)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.6, scale*1.6)
	} else {
		_ = sg.drawMenuPatch(screen, sessionflow.DetailPatch(sg.frontendDetailLow()), menuX+175, menuY+2*lineHeight, scale, ox, oy, false)
	}
	sg.drawFrontendThermo(screen, menuX, menuY+6*lineHeight, 10, frontendMouseSensitivityDot(sig.MouseLookSpeed), scale, ox, oy)
	for i, name := range frontendOptionsMenuNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		_ = sg.drawMenuPatch(screen, name, menuX, menuY+i*lineHeight, scale, ox, oy, false)
	}
	sg.drawMenuSkull(screen, menuX-32, menuY+sg.frontend.OptionsOn*lineHeight, scale, ox, oy)
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
	sg.drawFrontendThermo(screen, menuX, menuY+1*lineHeight, 16, sessionflow.VolumeDot(sig.SFXVolume), scale, ox, oy)
	sg.drawFrontendThermo(screen, menuX, menuY+3*lineHeight, 16, sessionflow.VolumeDot(sig.MusicVolume), scale, ox, oy)
	_ = sg.drawMenuPatch(screen, "M_SFXVOL", menuX, menuY, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, "M_MUSVOL", menuX, menuY+2*lineHeight, scale, ox, oy, false)
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
	if sg.menuPatchCache == nil {
		sg.menuPatchCache = make(map[string]*ebiten.Image, 32)
	}
	if img, ok := sg.menuPatchCache[key]; ok {
		return img, p, true
	}
	img := ebiten.NewImage(p.Width, p.Height)
	img.WritePixels(p.RGBA)
	sg.menuPatchCache[key] = img
	return img, p, true
}
