package automap

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"gddoom/internal/music"

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
	sg.frontend = frontendState{
		active:     true,
		mode:       frontendModeTitle,
		menuActive: false,
		attractSeq: -1,
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
	commercial := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(sg.bootMap.Name))), "MAP")
	retail := false
	for _, ep := range sg.opts.Episodes {
		if ep == 4 {
			retail = true
			break
		}
	}
	if commercial {
		return []string{"TITLEPIC", "DEMO1", "CREDIT", "DEMO2", "TITLEPIC", "DEMO3"}
	}
	secondPage := "HELP2"
	if retail {
		secondPage = "CREDIT"
	}
	seq := []string{"TITLEPIC", "DEMO1", "CREDIT", "DEMO2", secondPage, "DEMO3"}
	if sg.hasAttractDemo("DEMO4") {
		seq = append(seq, "DEMO4")
	}
	return seq
}

func (sg *sessionGame) hasAttractDemo(name string) bool {
	if sg == nil {
		return false
	}
	for _, demo := range sg.opts.AttractDemos {
		if demo != nil && strings.EqualFold(strings.TrimSpace(demo.Path), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func (sg *sessionGame) advanceFrontendAttract() bool {
	if sg == nil || !sg.frontend.active {
		return false
	}
	seq := sg.frontendAttractSequence()
	if len(seq) == 0 {
		fmt.Printf("attract-error reason=empty-sequence\n")
		return false
	}
	for i := 0; i < len(seq); i++ {
		sg.frontend.attractSeq = (sg.frontend.attractSeq + 1) % len(seq)
		step := seq[sg.frontend.attractSeq]
		sg.frontend.attractPage = ""
		sg.frontend.attractPageTic = 0
		if strings.HasPrefix(step, "DEMO") {
			if sg.startAttractDemoByName(step) {
				return true
			}
			continue
		}
		if step == "TITLEPIC" {
			sg.frontend.attractPageTic = attractPageTitleNonCommercial
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(sg.bootMap.Name))), "MAP") {
				sg.frontend.attractPageTic = attractPageTitleCommercial
			}
			sg.playTitleMusic()
		} else {
			sg.frontend.attractPageTic = attractPageInfo
		}
		sg.frontend.attractPage = step
		if sg.g != nil {
			sg.g.opts.DemoScript = nil
		}
		return true
	}
	fmt.Printf("attract-error reason=no-playable-attract-step sequence=%v\n", seq)
	return false
}

func (sg *sessionGame) playTitleMusic() {
	if sg == nil || sg.musicPlayer == nil || sg.musicDriver == nil || sg.opts.TitleMusicLoader == nil {
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
	stream, err := music.NewMUSStreamRenderer(sg.musicDriver, data)
	if err != nil {
		return
	}
	chunk, done, err := stream.NextChunkS16LE(music.DefaultStreamChunkFrames)
	if err != nil || len(chunk) == 0 {
		return
	}
	_ = sg.musicPlayer.EnqueueBytesS16LE(chunk)
	_ = sg.musicPlayer.Start()
	if done {
		return
	}
	stop := make(chan struct{})
	sg.musicStreamStop = stop
	go sg.streamMapMusic(stop, stream)
}

func (sg *sessionGame) frontendStatus(msg string, tics int) {
	if sg == nil {
		return
	}
	sg.frontend.status = strings.TrimSpace(msg)
	sg.frontend.statusTic = tics
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
	sg.quitPrompt.active = true
	sg.quitPrompt.exitDelayTic = 0
	sg.quitPrompt.lines = sg.nextQuitPromptLines()
}

func (sg *sessionGame) clearQuitPrompt() {
	if sg == nil {
		return
	}
	sg.quitPrompt = quitPromptState{}
}

func (sg *sessionGame) handleQuitPromptInput() error {
	if sg == nil || !sg.quitPrompt.active {
		return nil
	}
	if sg.quitPrompt.exitDelayTic > 0 {
		sg.quitPrompt.exitDelayTic--
		if sg.quitPrompt.exitDelayTic == 0 {
			return ebiten.Termination
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyY) {
		sg.playQuitPromptSound()
		sg.quitPrompt.exitDelayTic = quitPromptExitDelayTics
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		sg.playMenuBackSound()
		sg.clearQuitPrompt()
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
	return []string{"ARE YOU SURE YOU WANT TO", "QUIT THIS GREAT GAME?", "(PRESS Y TO QUIT)"}
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
	startMap := string(sg.bootMap.Name)
	if sg.opts.NewGameLoader != nil {
		startMap = "MAP01"
		if episodes := sg.availableFrontendEpisodeChoices(); len(episodes) > 1 {
			ep := sg.frontend.selectedEpisode
			if ep == 0 {
				ep = episodes[0]
			}
			startMap = fmt.Sprintf("E%dM1", ep)
		}
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
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
	sg.queueTransition(transitionLevel, 0)
}

func (sg *sessionGame) availableFrontendEpisodeChoices() []int {
	if sg == nil || len(sg.opts.Episodes) == 0 {
		return nil
	}
	out := make([]int, 0, len(sg.opts.Episodes))
	for _, ep := range sg.opts.Episodes {
		if ep >= 1 && ep <= 4 {
			out = append(out, ep)
		}
	}
	return out
}

func (sg *sessionGame) openReadThis(fromGame bool) {
	if sg == nil {
		return
	}
	sg.frontend.active = true
	sg.frontend.mode = frontendModeReadThis
	sg.frontend.menuActive = false
	sg.frontend.readThisPage = 0
	sg.frontend.readThisFromGame = fromGame
}

func (sg *sessionGame) closeReadThis() {
	if sg == nil {
		return
	}
	if sg.frontend.readThisFromGame {
		sg.frontend = frontendState{}
		return
	}
	sg.frontend.mode = frontendModeTitle
	sg.frontend.menuActive = true
	sg.frontend.readThisPage = 0
	sg.frontend.readThisFromGame = false
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

func frontendNextSelectableOptionRow(cur, dir int) int {
	rows := frontendOptionsSelectableRows[:]
	if len(rows) == 0 {
		return 0
	}
	idx := 0
	for i, row := range rows {
		if row == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(rows)) % len(rows)
	return rows[idx]
}

func clampFrontendMouseLookSpeed(v float64) float64 {
	if v < 0.5 {
		return 0.5
	}
	if v > 8.0 {
		return 8.0
	}
	return v
}

func frontendMouseSensitivitySpeedForDot(dot int) float64 {
	if dot < 0 {
		dot = 0
	}
	if dot > 9 {
		dot = 9
	}
	const minSpeed = 0.5
	const maxSpeed = 8.0
	if dot == 0 {
		return minSpeed
	}
	if dot == 9 {
		return maxSpeed
	}
	return minSpeed * math.Pow(maxSpeed/minSpeed, float64(dot)/9.0)
}

func frontendMouseSensitivityDot(speed float64) int {
	speed = clampFrontendMouseLookSpeed(speed)
	const minSpeed = 0.5
	const maxSpeed = 8.0
	dot := int(math.Round(math.Log(speed/minSpeed) / math.Log(maxSpeed/minSpeed) * 9.0))
	if dot < 0 {
		return 0
	}
	if dot > 9 {
		return 9
	}
	return dot
}

func frontendNextMouseSensitivity(speed float64, dir int) float64 {
	if dir == 0 {
		return clampFrontendMouseLookSpeed(speed)
	}
	dot := frontendMouseSensitivityDot(speed) + dir
	if dot < 0 {
		dot = 0
	}
	if dot > 9 {
		dot = 9
	}
	return frontendMouseSensitivitySpeedForDot(dot)
}

func frontendVolumeDot(v float64) int {
	dot := int(math.Round(clampVolume(v) * 15.0))
	if dot < 0 {
		return 0
	}
	if dot > 15 {
		return 15
	}
	return dot
}

func frontendMessagesPatch(enabled bool) string {
	if enabled {
		return "M_MSGON"
	}
	return "M_MSGOFF"
}

func frontendDetailPatch(low bool) string {
	if low {
		return "M_GDLOW"
	}
	return "M_GDHIGH"
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
	switch sg.g.sessionSignals().SourcePortDetail {
	case 1:
		return "FULL"
	case 2:
		return "1/2"
	case 3:
		return "1/3"
	case 4:
		return "1/4"
	default:
		return fmt.Sprintf("1/%d", sg.g.sessionSignals().SourcePortDetail)
	}
}

func (sg *sessionGame) frontendChangeMessages() {
	if sg == nil || sg.rt == nil {
		return
	}
	sg.settings.hudMessagesEnabled = sg.rt.sessionToggleHUDMessages()
	if sg.settings.hudMessagesEnabled {
		sg.frontendStatus("MESSAGES ON", doomTicsPerSecond)
	} else {
		sg.frontendStatus("MESSAGES OFF", doomTicsPerSecond)
	}
}

func (sg *sessionGame) frontendChangeDetail() {
	if sg == nil || sg.rt == nil {
		return
	}
	sg.settings.detailLevel = sg.rt.sessionCycleDetail()
	sg.opts.InitialDetailLevel = sg.settings.detailLevel
}

func (sg *sessionGame) frontendChangeMouseSensitivity(dir int) {
	if sg == nil || sg.rt == nil || dir == 0 {
		return
	}
	cur := sg.rt.sessionMouseLookSpeed()
	next := frontendNextMouseSensitivity(cur, dir)
	if next == cur {
		return
	}
	sg.rt.sessionSetMouseLookSpeed(next)
	sg.opts.MouseLookSpeed = next
	sg.settings.mouseLookSpeed = next
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
	sg.settings.musicVolume = next
	switch {
	case next <= 0:
		sg.stopAndClearMusic()
	case prev <= 0:
		if sg.frontend.active {
			sg.playTitleMusic()
		} else {
			sg.playMusicForMap(sg.current)
		}
	case sg.musicPlayer != nil:
		_ = sg.musicPlayer.SetVolume(next)
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
	sg.settings.sfxVolume = next
	sg.menuSfx = NewMenuSoundPlayer(sg.opts.SoundBank, next)
	sg.rt.sessionPublishRuntimeSettings()
	sg.playMenuMoveSound()
}

func (sg *sessionGame) tickFrontend() error {
	if sg == nil || !sg.frontend.active {
		return nil
	}
	f := &sg.frontend
	f.tic++
	f.skullAnimCounter++
	if f.skullAnimCounter >= menuSkullBlinkTics {
		f.skullAnimCounter = 0
		f.whichSkull ^= 1
	}
	if f.statusTic > 0 {
		f.statusTic--
		if f.statusTic == 0 {
			f.status = ""
		}
	}
	if f.attractPageTic > 0 {
		f.attractPageTic--
		if f.attractPageTic == 0 {
			_ = sg.advanceFrontendAttract()
		}
	}
	switch f.mode {
	case frontendModeReadThis:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			sg.closeReadThis()
			sg.playMenuBackSound()
			return nil
		}
		if anyIntermissionSkipInput() {
			if f.readThisPage+1 < len(sg.readThisPageNames()) {
				f.readThisPage++
				sg.playMenuConfirmSound()
			} else {
				sg.closeReadThis()
				sg.playMenuBackSound()
			}
		}
		return nil
	case frontendModeSound:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			f.mode = frontendModeOptions
			sg.playMenuBackSound()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			f.soundOn ^= 1
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if f.soundOn == 0 {
				sg.frontendChangeSFXVolume(-1)
			} else {
				sg.frontendChangeMusicVolume(-1)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if f.soundOn == 0 {
				sg.frontendChangeSFXVolume(1)
			} else {
				sg.frontendChangeMusicVolume(1)
			}
		}
		return nil
	case frontendModeOptions:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			f.mode = frontendModeTitle
			f.menuActive = true
			sg.playMenuBackSound()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			f.optionsOn = frontendNextSelectableOptionRow(f.optionsOn, -1)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			f.optionsOn = frontendNextSelectableOptionRow(f.optionsOn, 1)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			switch f.optionsOn {
			case 2:
				sg.frontendChangeDetail()
			case 5:
				sg.frontendChangeMouseSensitivity(-1)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			switch f.optionsOn {
			case 2:
				sg.frontendChangeDetail()
			case 5:
				sg.frontendChangeMouseSensitivity(1)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			switch f.optionsOn {
			case 0:
				sg.frontendStatus("NOT IN GAME", doomTicsPerSecond)
			case 1:
				sg.frontendChangeMessages()
				sg.playMenuConfirmSound()
			case 2:
				sg.frontendChangeDetail()
				sg.playMenuConfirmSound()
			case 5:
				sg.frontendChangeMouseSensitivity(1)
				sg.playMenuConfirmSound()
			case 7:
				f.mode = frontendModeSound
				sg.playMenuConfirmSound()
			}
		}
		return nil
	case frontendModeEpisode:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			f.mode = frontendModeTitle
			f.menuActive = true
			sg.playMenuBackSound()
			return nil
		}
		episodes := sg.availableFrontendEpisodeChoices()
		if len(episodes) <= 1 {
			f.mode = frontendModeSkill
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			f.episodeOn = (f.episodeOn + len(episodes) - 1) % len(episodes)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			f.episodeOn = (f.episodeOn + 1) % len(episodes)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			if f.episodeOn < 0 || f.episodeOn >= len(episodes) {
				f.episodeOn = 0
			}
			f.selectedEpisode = episodes[f.episodeOn]
			f.mode = frontendModeSkill
			sg.playMenuConfirmSound()
		}
		return nil
	case frontendModeSkill:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			if len(sg.availableFrontendEpisodeChoices()) > 1 {
				f.mode = frontendModeEpisode
			} else {
				f.mode = frontendModeTitle
				f.menuActive = true
			}
			sg.playMenuBackSound()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			f.skillOn = (f.skillOn + len(frontendSkillMenuNames) - 1) % len(frontendSkillMenuNames)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			f.skillOn = (f.skillOn + 1) % len(frontendSkillMenuNames)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			sg.playMenuConfirmSound()
			sg.startGameFromFrontend(f.skillOn + 1)
		}
		return nil
	default:
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			f.menuActive = !f.menuActive
			if f.menuActive {
				sg.playMenuMoveSound()
			} else {
				sg.playMenuBackSound()
			}
		}
		if !f.menuActive {
			if anyIntermissionSkipInput() {
				f.menuActive = true
				sg.playMenuMoveSound()
			}
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			f.itemOn = (f.itemOn + len(frontendMainMenuNames) - 1) % len(frontendMainMenuNames)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			f.itemOn = (f.itemOn + 1) % len(frontendMainMenuNames)
			sg.playMenuMoveSound()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
			sg.playMenuConfirmSound()
			switch f.itemOn {
			case 0:
				if episodes := sg.availableFrontendEpisodeChoices(); len(episodes) > 1 {
					f.episodeOn = 0
					f.selectedEpisode = episodes[0]
					f.mode = frontendModeEpisode
				} else {
					f.mode = frontendModeSkill
				}
			case 1:
				f.mode = frontendModeOptions
				f.optionsOn = frontendOptionsSelectableRows[0]
			case 4:
				sg.openReadThis(false)
			case 2, 3:
				sg.frontendStatus("MENU ITEM NOT WIRED YET", doomTicsPerSecond*2)
			case 5:
				sg.requestQuitPrompt()
			}
		}
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

	switch sg.frontend.mode {
	case frontendModeReadThis:
		sg.drawFrontendAttractBackground(screen)
		name := sg.readThisPageName(sg.frontend.readThisPage)
		if !sg.drawIntermissionPatch(screen, name, 0, 0, scale, ox, oy, false) && !sg.drawFrontendPage(screen, "TITLEPIC") {
			screen.Fill(color.Black)
		}
		if (sg.frontend.tic/16)&1 == 0 {
			prompt := "PRESS ANY KEY TO RETURN"
			if sg.frontend.readThisPage+1 < len(sg.readThisPageNames()) {
				prompt = "PRESS ANY KEY TO CONTINUE"
			}
			sg.drawIntermissionText(screen, prompt, 160, 186, scale, ox, oy, true)
		}
		return
	case frontendModeSound:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.active {
			return
		}
		sg.drawFrontendSoundMenu(screen, scale, ox, oy)
		return
	case frontendModeOptions:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.active {
			return
		}
		sg.drawFrontendOptionsMenu(screen, scale, ox, oy)
		return
	case frontendModeEpisode:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.active {
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
		sg.drawMenuSkull(screen, 16, 63+sg.frontend.episodeOn*16, scale, ox, oy)
		return
	case frontendModeSkill:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.active {
			return
		}
		_ = sg.drawMenuPatch(screen, "M_NEWG", 96, 14, scale, ox, oy, false)
		_ = sg.drawMenuPatch(screen, "M_SKILL", 54, 38, scale, ox, oy, false)
		for i, name := range frontendSkillMenuNames {
			_ = sg.drawMenuPatch(screen, name, 48, 63+i*16, scale, ox, oy, false)
		}
		sg.drawMenuSkull(screen, 16, 63+sg.frontend.skillOn*16, scale, ox, oy)
		return
	default:
		sg.drawFrontendBackdrop(screen, true)
		if sg.quitPrompt.active {
			return
		}
		if sg.frontend.menuActive {
			_ = sg.drawMenuPatch(screen, "M_DOOM", 94, 2, scale, ox, oy, false)
			for i, name := range frontendMainMenuNames {
				_ = sg.drawMenuPatch(screen, name, 97, 64+i*16, scale, ox, oy, false)
			}
			sg.drawMenuSkull(screen, 65, 64+sg.frontend.itemOn*16, scale, ox, oy)
		}
		if msg := strings.TrimSpace(sg.frontend.status); msg != "" {
			sg.drawIntermissionText(screen, msg, 160, 178, scale, ox, oy, true)
		}
	}
}

func (sg *sessionGame) drawFrontendBackdrop(screen *ebiten.Image, showLogo bool) {
	if sg == nil || screen == nil {
		return
	}
	sg.drawFrontendAttractBackground(screen)
	if showLogo && sg.frontend.mode == frontendModeTitle && sg.frontend.menuActive {
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
	if sg.drawFrontendPage(screen, sg.frontend.attractPage) {
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
	_ = sg.drawMenuPatch(screen, frontendMessagesPatch(sig.HUDMessages), menuX+120, menuY+1*lineHeight, scale, ox, oy, false)
	if sig.SourcePortMode {
		sg.rt.sessionDrawHUTextAt(screen, sg.frontendSourcePortDetailLabel(), ox+float64(menuX+175)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.6, scale*1.6)
	} else {
		_ = sg.drawMenuPatch(screen, frontendDetailPatch(sg.frontendDetailLow()), menuX+175, menuY+2*lineHeight, scale, ox, oy, false)
	}
	sg.drawFrontendThermo(screen, menuX, menuY+6*lineHeight, 10, frontendMouseSensitivityDot(sig.MouseLookSpeed), scale, ox, oy)
	for i, name := range frontendOptionsMenuNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		_ = sg.drawMenuPatch(screen, name, menuX, menuY+i*lineHeight, scale, ox, oy, false)
	}
	sg.drawMenuSkull(screen, menuX-32, menuY+sg.frontend.optionsOn*lineHeight, scale, ox, oy)
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
	sg.drawFrontendThermo(screen, menuX, menuY+1*lineHeight, 16, frontendVolumeDot(sig.SFXVolume), scale, ox, oy)
	sg.drawFrontendThermo(screen, menuX, menuY+3*lineHeight, 16, frontendVolumeDot(sig.MusicVolume), scale, ox, oy)
	_ = sg.drawMenuPatch(screen, "M_SFXVOL", menuX, menuY, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, "M_MUSVOL", menuX, menuY+2*lineHeight, scale, ox, oy, false)
	skullY := menuY
	if sg.frontend.soundOn != 0 {
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
	if sg == nil || !sg.quitPrompt.active || screen == nil {
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
	lines := sg.quitPrompt.lines
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
	if sg.frontend.whichSkull != 0 {
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
