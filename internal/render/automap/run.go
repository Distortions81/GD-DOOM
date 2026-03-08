package automap

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

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
	intermissionEnteringWaitTics   = doomTicsPerSecond * 2
	intermissionYouAreHereWaitTics = doomTicsPerSecond * 3
	intermissionSkipInputDelayTics = doomTicsPerSecond / 3
	intermissionSkipExitHoldTics   = 12
	intermissionCounterSoundPeriod = 6
	finaleHoldTics                 = doomTicsPerSecond * 7
	menuSkullBlinkTics             = 8
)

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
	showEntering      bool
	showYouAreHere    bool
	enteringWait      int
	youAreHereWait    int
	show              intermissionStats
	target            intermissionStats
	nextMap           *mapdata.Map
}

type sessionFinale struct {
	active  bool
	tic     int
	waitTic int
	mapName mapdata.MapName
	screen  string
}

type frontendMode int

const (
	frontendModeNone frontendMode = iota
	frontendModeTitle
	frontendModeReadThis
	frontendModeOptions
	frontendModeSound
	frontendModeEpisode
	frontendModeSkill
)

type frontendState struct {
	mode             frontendMode
	active           bool
	menuActive       bool
	itemOn           int
	optionsOn        int
	soundOn          int
	episodeOn        int
	selectedEpisode  int
	skillOn          int
	readThisPage     int
	readThisFromGame bool
	skullAnimCounter int
	whichSkull       int
	tic              int
	status           string
	statusTic        int
}

type quitPromptState struct {
	active       bool
	lines        []string
	exitDelayTic int
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
	bootMap         *mapdata.Map
	current         mapdata.MapName
	currentTemplate *mapdata.Map
	opts            Options
	demoRecord      []DemoTic
	settings        sessionPersistentSettings
	nextMap         NextMapFunc
	err             error
	musicDriver     *music.Driver
	musicStreamStop chan struct{}
	musicPlayer     *music.ChunkPlayer
	faithfulSurface *ebiten.Image
	faithfulNearest *ebiten.Image
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
	menuPatchCache  map[string]*ebiten.Image
	interPatchCache map[string]*ebiten.Image
	menuSfx         *MenuSoundPlayer
	transition      sessionTransition
	intermission    sessionIntermission
	finale          sessionFinale
	frontend        frontendState
	quitPrompt      quitPromptState
	quitMessageSeq  int
}

const quitPromptExitDelayTics = 53

var doomQuitMessages = []string{
	"please don't leave, there's more\ndemons to toast!",
	"let's beat it -- this is turning\ninto a bloodbath!",
	"i wouldn't leave if i were you.\ndos is much worse.",
	"you're trying to say you like dos\nbetter than me, right?",
	"don't leave yet -- there's a\ndemon around that corner!",
	"ya know, next time you come in here\ni'm gonna toast ya.",
	"go ahead and leave. see if i care.",
	"you want to quit?\nthen, thou hast lost an eighth!",
	"don't go now, there's a \ndimensional shambler waiting\nat the dos prompt!",
	"get outta here and go back\nto your boring programs.",
	"if i were your boss, i'd \n deathmatch ya in a minute!",
	"look, bud. you leave now\nand you forfeit your body count!",
	"just leave. when you come\nback, i'll be waiting with a bat.",
	"you're lucky i don't smack\nyou for thinking about leaving.",
	"fuck you, pussy!\nget the fuck out!",
	"you quit and i'll jizz\nin your cystholes!",
	"if you leave, i'll make\nthe lord drink my jizz.",
	"hey, ron! can we say\n'fuck' in the game?",
	"i'd leave: this is just\nmore monsters and levels.\nwhat a load.",
	"suck it down, asshole!\nyou're a fucking wimp!",
	"don't quit now! we're \nstill spending your money!",
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
	detailLevel             int
	rotateView              bool
	mouseLook               bool
	mouseLookSpeed          float64
	musicVolume             float64
	oplVolume               float64
	sfxVolume               float64
	hudMessagesEnabled      bool
	alwaysRun               bool
	autoWeaponSwitch        bool
	lineColorMode           string
	thingRenderMode         string
	showLegend              bool
	spriteClipDiag          bool
	spriteClipDiagOnly      bool
	spriteClipDiagGreenOnly bool
	paletteLUT              bool
	gammaLevel              int
	crtEnabled              bool
	reveal                  revealMode
	iddt                    int
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

func clampVolume(v float64) float64 {
	if v != v {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampOPLVolume(v float64) float64 {
	if v != v {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > music.MaxOutputGain {
		return music.MaxOutputGain
	}
	return v
}

func (sg *sessionGame) capturePersistentSettings() {
	if sg == nil || sg.g == nil {
		return
	}
	g := sg.g
	sg.settings = sessionPersistentSettings{
		detailLevel:             g.detailLevel,
		rotateView:              g.rotateView,
		mouseLook:               g.opts.MouseLook,
		mouseLookSpeed:          g.opts.MouseLookSpeed,
		musicVolume:             g.opts.MusicVolume,
		oplVolume:               g.opts.OPLVolume,
		sfxVolume:               g.opts.SFXVolume,
		hudMessagesEnabled:      g.hudMessagesEnabled,
		alwaysRun:               g.alwaysRun,
		autoWeaponSwitch:        g.autoWeaponSwitch,
		lineColorMode:           g.opts.LineColorMode,
		thingRenderMode:         g.opts.SourcePortThingRenderMode,
		showLegend:              g.showLegend,
		spriteClipDiag:          g.spriteClipDiag,
		spriteClipDiagOnly:      g.spriteClipDiagOnly,
		spriteClipDiagGreenOnly: g.spriteClipDiagGreenOnly,
		paletteLUT:              g.paletteLUTEnabled,
		gammaLevel:              g.gammaLevel,
		crtEnabled:              g.crtEnabled,
		reveal:                  g.parity.reveal,
		iddt:                    g.parity.iddt,
	}
}

func (sg *sessionGame) applyPersistentSettingsToOptions() {
	sg.opts.MouseLook = sg.settings.mouseLook
	sg.opts.MouseLookSpeed = sg.settings.mouseLookSpeed
	sg.opts.MusicVolume = clampVolume(sg.settings.musicVolume)
	sg.opts.OPLVolume = clampOPLVolume(sg.settings.oplVolume)
	sg.opts.SFXVolume = clampVolume(sg.settings.sfxVolume)
	sg.opts.AlwaysRun = sg.settings.alwaysRun
	sg.opts.AutoWeaponSwitch = sg.settings.autoWeaponSwitch
	sg.opts.LineColorMode = sg.settings.lineColorMode
	sg.opts.SourcePortThingRenderMode = sg.settings.thingRenderMode
}

func (sg *sessionGame) applyPersistentSettingsToGame(g *game) {
	if sg == nil || g == nil {
		return
	}
	s := sg.settings
	g.detailLevel = clampDetailLevelForMode(s.detailLevel, g.opts.SourcePortMode)
	g.rotateView = s.rotateView
	g.opts.MouseLook = s.mouseLook
	g.opts.MouseLookSpeed = s.mouseLookSpeed
	g.opts.MusicVolume = clampVolume(s.musicVolume)
	g.opts.OPLVolume = clampOPLVolume(s.oplVolume)
	g.opts.SFXVolume = clampVolume(s.sfxVolume)
	if g.snd != nil {
		g.snd.setSFXVolume(g.opts.SFXVolume)
	}
	if sg.musicPlayer != nil {
		_ = sg.musicPlayer.SetVolume(g.opts.MusicVolume)
	}
	if sg.musicDriver != nil {
		sg.musicDriver.SetOutputGain(g.opts.OPLVolume)
	}
	g.alwaysRun = s.alwaysRun
	g.autoWeaponSwitch = s.autoWeaponSwitch
	g.hudMessagesEnabled = s.hudMessagesEnabled
	g.opts.LineColorMode = s.lineColorMode
	g.opts.SourcePortThingRenderMode = normalizeSourcePortThingRenderMode(s.thingRenderMode, g.opts.SourcePortMode)
	g.showLegend = s.showLegend
	g.spriteClipDiag = s.spriteClipDiag
	g.spriteClipDiagOnly = s.spriteClipDiagOnly && s.spriteClipDiag
	g.spriteClipDiagGreenOnly = s.spriteClipDiagGreenOnly && g.spriteClipDiagOnly
	g.paletteLUTEnabled = s.paletteLUT && g.opts.KageShader && len(g.opts.DoomPaletteRGBA) == 256*4
	g.gammaLevel = clampGamma(s.gammaLevel)
	g.crtEnabled = s.crtEnabled && g.opts.KageShader
	g.parity.reveal = normalizeRevealForMode(s.reveal, g.opts.SourcePortMode)
	g.parity.iddt = clampIDDT(s.iddt)
	g.runtimeSettingsSeen = true
	g.runtimeSettingsLast = g.runtimeSettingsSnapshot()
}

func (sg *sessionGame) applyRuntimeSettings(s RuntimeSettings) {
	if sg == nil {
		return
	}
	prevMusic := clampVolume(sg.settings.musicVolume)
	sg.settings.detailLevel = clampDetailLevelForMode(s.DetailLevel, sg.opts.SourcePortMode)
	sg.settings.mouseLook = s.MouseLook
	sg.settings.musicVolume = clampVolume(s.MusicVolume)
	sg.settings.oplVolume = clampOPLVolume(s.OPLVolume)
	sg.settings.sfxVolume = clampVolume(s.SFXVolume)
	sg.settings.alwaysRun = s.AlwaysRun
	sg.settings.autoWeaponSwitch = s.AutoWeaponSwitch
	if !sg.opts.SourcePortMode {
		sg.settings.lineColorMode = "parity"
	} else {
		sg.settings.lineColorMode = s.LineColorMode
	}
	sg.settings.thingRenderMode = normalizeSourcePortThingRenderMode(s.ThingRenderMode, sg.opts.SourcePortMode)
	sg.settings.gammaLevel = clampGamma(s.GammaLevel)
	sg.settings.crtEnabled = s.CRTEffect && sg.opts.KageShader
	sg.opts.MusicVolume = sg.settings.musicVolume
	sg.opts.OPLVolume = sg.settings.oplVolume
	sg.opts.SFXVolume = sg.settings.sfxVolume
	if sg.menuSfx != nil {
		sg.menuSfx.volume = sg.settings.sfxVolume
	}
	if sg.musicDriver != nil {
		sg.musicDriver.SetOutputGain(sg.opts.OPLVolume)
	}
	nextMusic := sg.settings.musicVolume
	switch {
	case nextMusic <= 0:
		sg.stopAndClearMusic()
	case prevMusic <= 0:
		if sg.frontend.active {
			sg.playTitleMusic()
		} else if sg.g != nil && sg.g.m != nil {
			sg.playMusicForMap(sg.g.m.Name)
		} else {
			sg.playMusicForMap(sg.current)
		}
	case sg.musicPlayer != nil:
		_ = sg.musicPlayer.SetVolume(nextMusic)
	}
}

func (sg *sessionGame) rebuildGameWithPersistentSettings(next *mapdata.Map) {
	if sg == nil || next == nil {
		return
	}
	sg.collectDemoRecord()
	if sg.g != nil {
		sg.g.clearSpritePatchCache()
	}
	sg.capturePersistentSettings()
	sg.applyPersistentSettingsToOptions()
	ng := newGame(next, sg.opts)
	sg.applyPersistentSettingsToGame(ng)
	sg.g = ng
}

func (sg *sessionGame) collectDemoRecord() {
	if sg == nil || sg.g == nil || len(sg.g.demoRecord) == 0 {
		return
	}
	sg.demoRecord = append(sg.demoRecord, sg.g.demoRecord...)
	sg.g.demoRecord = sg.g.demoRecord[:0]
}

func (sg *sessionGame) effectiveDemoRecord() []DemoTic {
	if sg == nil {
		return nil
	}
	sg.collectDemoRecord()
	return sg.demoRecord
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
	_ = sg.musicPlayer.SetVolume(clampVolume(sg.opts.MusicVolume))
	sg.musicDriver = music.NewDriver(p.SampleRate(), sg.opts.MusicPatchBank)
	sg.musicDriver.SetMUSPanMax(sg.opts.MUSPanMax)
	sg.musicDriver.SetOutputGain(sg.opts.OPLVolume)
}

func (sg *sessionGame) closeMusicPlayback() {
	sg.stopMusicStream()
	if sg == nil || sg.musicPlayer == nil {
		return
	}
	_ = sg.musicPlayer.Close()
	sg.musicPlayer = nil
}

func (sg *sessionGame) stopAndClearMusic() {
	sg.stopMusicStream()
	if sg == nil || sg.musicPlayer == nil {
		return
	}
	_ = sg.musicPlayer.Stop()
	_ = sg.musicPlayer.ClearBuffer()
}

func (sg *sessionGame) stopMusicStream() {
	if sg == nil || sg.musicStreamStop == nil {
		return
	}
	close(sg.musicStreamStop)
	sg.musicStreamStop = nil
}

func (sg *sessionGame) playMusicForMap(name mapdata.MapName) {
	if sg == nil || sg.musicPlayer == nil || sg.musicDriver == nil || sg.opts.MapMusicLoader == nil {
		return
	}
	sg.stopAndClearMusic()
	if clampVolume(sg.opts.MusicVolume) <= 0 {
		return
	}
	data, err := sg.opts.MapMusicLoader(string(name))
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

func (sg *sessionGame) streamMapMusic(stop <-chan struct{}, stream *music.StreamRenderer) {
	if sg == nil || sg.musicPlayer == nil || stream == nil {
		return
	}
	const bytesPerFrame = 4 // s16 stereo
	const checkPeriod = 12 * time.Millisecond
	lookaheadBytes := music.DefaultStreamLookahead * bytesPerFrame
	ticker := time.NewTicker(checkPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		default:
		}
		for sg.musicPlayer.BufferedBytes() >= lookaheadBytes {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
		chunk, done, err := stream.NextChunkS16LE(music.DefaultStreamChunkFrames)
		if err != nil {
			return
		}
		if len(chunk) > 0 {
			_ = sg.musicPlayer.EnqueueBytesS16LE(chunk)
		}
		if done {
			return
		}
	}
}

func (sg *sessionGame) initSession() {
	if sg == nil {
		return
	}
	sg.g = newGame(sg.bootMap, sg.opts)
	sg.g.initSkyLayerShader()
	sg.initMusicPlayback()
	if sg.shouldStartInFrontend() {
		sg.startFrontend()
	} else {
		sg.playMusicForMap(sg.current)
	}
	sg.capturePersistentSettings()
	if sg.shouldShowBootSplash() {
		sg.queueTransition(transitionBoot, bootSplashHoldTics)
	}
	sg.initFaithfulPalettePost()
}

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
		menuActive: true,
	}
	sg.stopAndClearMusic()
	sg.playTitleMusic()
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
	sg.g.opts.SkillLevel = normalizeSkillLevel(skill)
	sg.g = newGame(cloneMapForRestart(sg.bootMap), sg.g.opts)
	sg.applyPersistentSettingsToGame(sg.g)
	sg.current = sg.g.m.Name
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
	if sg.g.opts.SourcePortMode {
		return sg.g.sourcePortDetailDivisor() > 1
	}
	return sg.g.lowDetailMode()
}

func (sg *sessionGame) frontendSourcePortDetailLabel() string {
	if sg == nil || sg.g == nil {
		return "FULL"
	}
	switch sg.g.sourcePortDetailDivisor() {
	case 1:
		return "FULL"
	case 2:
		return "1/2"
	case 3:
		return "1/3"
	case 4:
		return "1/4"
	default:
		return fmt.Sprintf("1/%d", sg.g.sourcePortDetailDivisor())
	}
}

func (sg *sessionGame) frontendChangeMessages() {
	if sg == nil || sg.g == nil {
		return
	}
	sg.g.hudMessagesEnabled = !sg.g.hudMessagesEnabled
	sg.settings.hudMessagesEnabled = sg.g.hudMessagesEnabled
	if sg.g.hudMessagesEnabled {
		sg.frontendStatus("MESSAGES ON", doomTicsPerSecond)
	} else {
		sg.frontendStatus("MESSAGES OFF", doomTicsPerSecond)
	}
}

func (sg *sessionGame) frontendChangeDetail() {
	if sg == nil || sg.g == nil {
		return
	}
	if sg.g.opts.SourcePortMode {
		sg.g.cycleSourcePortDetailLevel()
	} else {
		sg.g.cycleDetailLevel()
	}
	sg.settings.detailLevel = sg.g.detailLevel
	sg.opts.InitialDetailLevel = sg.g.detailLevel
}

func (sg *sessionGame) frontendChangeMouseSensitivity(dir int) {
	if sg == nil || sg.g == nil || dir == 0 {
		return
	}
	next := frontendNextMouseSensitivity(sg.g.opts.MouseLookSpeed, dir)
	if next == sg.g.opts.MouseLookSpeed {
		return
	}
	sg.g.opts.MouseLookSpeed = next
	sg.opts.MouseLookSpeed = next
	sg.settings.mouseLookSpeed = next
	sg.frontendStatus(fmt.Sprintf("MOUSE SENSITIVITY %.2f", next), doomTicsPerSecond)
}

func (sg *sessionGame) frontendChangeMusicVolume(dir int) {
	if sg == nil || sg.g == nil || dir == 0 {
		return
	}
	prev := clampVolume(sg.g.opts.MusicVolume)
	next := clampVolume(sg.g.opts.MusicVolume + float64(dir)*(1.0/15.0))
	if next == sg.g.opts.MusicVolume {
		return
	}
	sg.g.opts.MusicVolume = next
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
	sg.g.publishRuntimeSettingsIfChanged()
}

func (sg *sessionGame) frontendChangeSFXVolume(dir int) {
	if sg == nil || sg.g == nil || dir == 0 {
		return
	}
	next := clampVolume(sg.g.opts.SFXVolume + float64(dir)*(1.0/15.0))
	if next == sg.g.opts.SFXVolume {
		return
	}
	sg.g.opts.SFXVolume = next
	sg.opts.SFXVolume = next
	sg.settings.sfxVolume = next
	sg.menuSfx = NewMenuSoundPlayer(sg.opts.SoundBank, next)
	sg.g.publishRuntimeSettingsIfChanged()
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

func RunAutomap(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	sess := NewSession(m, opts, nextMap)
	defer sess.Close()
	if err := ebiten.RunGame(sess); err != nil {
		if errors.Is(err, ebiten.Termination) {
			if p := sess.Options().RecordDemoPath; p != "" {
				rec := sess.EffectiveDemoRecord()
				if werr := SaveDemoScript(p, rec); werr != nil {
					return fmt.Errorf("write demo recording: %w", werr)
				}
				fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(rec))
			}
			if sess.Err() != nil {
				return sess.Err()
			}
			return nil
		}
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sess.Options().RecordDemoPath; p != "" {
		rec := sess.EffectiveDemoRecord()
		if werr := SaveDemoScript(p, rec); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(rec))
	}
	return sess.Err()
}

type Session struct {
	sg *sessionGame
}

func NewSession(m *mapdata.Map, opts Options, nextMap NextMapFunc) *Session {
	opts, windowW, windowH := normalizeRunDimensions(opts)
	sg := &sessionGame{
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
	return &Session{sg: sg}
}

func (s *Session) Update() error {
	if s == nil || s.sg == nil {
		return ebiten.Termination
	}
	return s.sg.Update()
}

func (s *Session) Draw(screen *ebiten.Image) {
	if s == nil || s.sg == nil {
		return
	}
	s.sg.Draw(screen)
}

func (s *Session) Layout(outsideWidth, outsideHeight int) (int, int) {
	if s == nil || s.sg == nil {
		return max(outsideWidth, 1), max(outsideHeight, 1)
	}
	return s.sg.Layout(outsideWidth, outsideHeight)
}

func (s *Session) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if s == nil || s.sg == nil {
		return
	}
	s.sg.DrawFinalScreen(screen, offscreen, geoM)
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
		return sg.tickFrontend()
	}
	if sg.intermission.active {
		if sg.tickIntermission() {
			sg.finishIntermission()
		}
		return nil
	}

	err := sg.g.Update()
	if err == nil {
		if sg.g.newGameRequestedMap != nil {
			sg.stopAndClearMusic()
			sg.g.clearPendingSoundState()
			sg.capturePersistentSettings()
			sg.opts.SkillLevel = normalizeSkillLevel(sg.g.newGameRequestedSkill)
			sg.rebuildGameWithPersistentSettings(sg.g.newGameRequestedMap)
			sg.current = sg.g.m.Name
			sg.currentTemplate = cloneMapForRestart(sg.g.m)
			sg.playMusicForMap(sg.current)
			ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
			sg.queueTransition(transitionLevel, 0)
			sg.g.newGameRequestedMap = nil
			sg.g.newGameRequestedSkill = 0
			return nil
		}
		if sg.g.quitPromptRequested {
			sg.g.quitPromptRequested = false
			sg.requestQuitPrompt()
			return nil
		}
		if sg.g.readThisRequested {
			sg.g.readThisRequested = false
			sg.openReadThis(true)
			return nil
		}
		if sg.g.levelRestartRequested {
			sg.stopAndClearMusic()
			sg.g.clearPendingSoundState()
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
	if sg.startEpisodeFinale(sg.current, sg.g.secretLevelExit) {
		return nil
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
	if sg.g != nil {
		sg.g.quitPromptActive = sg.quitPrompt.active
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
		if sg.presentSurface == nil || sg.presentSurface.Bounds().Dx() != sg.g.viewW || sg.presentSurface.Bounds().Dy() != sg.g.viewH {
			sg.presentSurface = ebiten.NewImage(max(sg.g.viewW, 1), max(sg.g.viewH, 1))
		}
		sg.g.Draw(sg.presentSurface)
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
	targetH := faithfulAspectLogicalH
	if sg.opts.DisableAspectCorrection {
		targetH = doomLogicalH
	}
	scale := sw / doomLogicalW
	scaleY := sh / targetH
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		scale = 1
	}
	nearestW := doomLogicalW * scale
	nearestH := doomLogicalH * scale
	if sg.faithfulNearest == nil || sg.faithfulNearest.Bounds().Dx() != nearestW || sg.faithfulNearest.Bounds().Dy() != nearestH {
		sg.faithfulNearest = ebiten.NewImage(nearestW, nearestH)
	}
	sg.faithfulNearest.Clear()
	nearestOp := &ebiten.DrawImageOptions{}
	nearestOp.Filter = ebiten.FilterNearest
	nearestOp.GeoM.Scale(float64(nearestW)/float64(vw), float64(nearestH)/float64(vh))
	sg.faithfulNearest.DrawImage(src, nearestOp)

	dst.Clear()
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(float64(sw)/float64(nearestW), float64(sh)/float64(nearestH))
	dst.DrawImage(sg.faithfulNearest, op)
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

func (sg *sessionGame) transitionSurfaceSize(screenW, screenH int) (int, int) {
	if sg.opts.SourcePortMode {
		return max(screenW, 1), max(screenH, 1)
	}
	// Faithful mode keeps Doom-paced melt timing by transitioning at the
	// logical render size rather than the window size.
	w := sg.opts.Width
	h := sg.opts.Height
	if w <= 0 {
		w = doomLogicalW
	}
	if h <= 0 {
		h = doomLogicalH
	}
	return w, h
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
		if sg.frontend.active {
			sg.drawFrontendTransitionSurface(t.to)
		} else {
			sg.drawGameTransitionSurface(t.to, sg.g)
		}
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
		screen.Fill(color.Black)
		name := sg.readThisPageName(sg.frontend.readThisPage)
		if !sg.drawIntermissionPatch(screen, name, 0, 0, scale, ox, oy, false) {
			sg.drawBootSplashPresented(screen)
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
		} else if (sg.frontend.tic/16)&1 == 0 {
			sg.drawIntermissionText(screen, "PRESS ANY KEY", 160, 186, scale, ox, oy, true)
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
	screen.Fill(color.Black)
	backdropTint := color.RGBA{R: 8, G: 8, B: 8, A: 191}
	img, _, ok := sg.menuPatch("M_DOOM")
	if !showLogo || !ok || img == nil {
		ebitenutil.DrawRect(screen, 0, 0, float64(max(screen.Bounds().Dx(), 1)), float64(max(screen.Bounds().Dy(), 1)), backdropTint)
		return
	}
	drawCenteredIntegerScaledLogo(screen, img)
	ebitenutil.DrawRect(screen, 0, 0, float64(max(screen.Bounds().Dx(), 1)), float64(max(screen.Bounds().Dy(), 1)), backdropTint)
}

func (sg *sessionGame) drawFrontendOptionsMenu(screen *ebiten.Image, scale, ox, oy float64) {
	if sg == nil || sg.g == nil {
		return
	}
	const menuX = 60
	const menuY = 37
	const lineHeight = 16
	_ = sg.drawMenuPatch(screen, "M_OPTTTL", 108, 15, scale, ox, oy, false)
	_ = sg.drawMenuPatch(screen, frontendMessagesPatch(sg.g.hudMessagesEnabled), menuX+120, menuY+1*lineHeight, scale, ox, oy, false)
	if sg.g.opts.SourcePortMode {
		sg.g.drawHUTextAt(screen, sg.frontendSourcePortDetailLabel(), ox+float64(menuX+175)*scale, oy+float64(menuY+2*lineHeight+2)*scale, scale*1.6, scale*1.6)
	} else {
		_ = sg.drawMenuPatch(screen, frontendDetailPatch(sg.frontendDetailLow()), menuX+175, menuY+2*lineHeight, scale, ox, oy, false)
	}
	sg.drawFrontendThermo(screen, menuX, menuY+6*lineHeight, 10, frontendMouseSensitivityDot(sg.g.opts.MouseLookSpeed), scale, ox, oy)
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
	const menuX = 80
	const menuY = 64
	const lineHeight = 16
	_ = sg.drawMenuPatch(screen, "M_SVOL", 60, 38, scale, ox, oy, false)
	sg.drawFrontendThermo(screen, menuX, menuY+1*lineHeight, 16, frontendVolumeDot(sg.g.opts.SFXVolume), scale, ox, oy)
	sg.drawFrontendThermo(screen, menuX, menuY+3*lineHeight, 16, frontendVolumeDot(sg.g.opts.MusicVolume), scale, ox, oy)
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

func (sg *sessionGame) startIntermission(next *mapdata.Map, nextName mapdata.MapName) {
	sg.stopAndClearMusic()
	if sg.g != nil {
		sg.g.clearPendingSoundState()
		sg.g.clearSpritePatchCache()
	}
	stats := collectIntermissionStats(sg.g, sg.current, nextName)
	showEntering := shouldShowEnteringScreen(stats.mapName, stats.nextMapName)
	showYouAreHere := showEntering && shouldShowYouAreHere(stats.mapName, stats.nextMapName)
	enteringWait := intermissionEnteringWaitTics
	youAreHereWait := intermissionYouAreHereWaitTics
	if !showEntering {
		enteringWait = 0
		youAreHereWait = 1
	} else if !showYouAreHere {
		youAreHereWait = 1
	}
	sg.intermission = sessionIntermission{
		active:            true,
		phase:             intermissionPhaseKills,
		waitTic:           0,
		tic:               0,
		stageSoundCounter: 0,
		showEntering:      showEntering,
		showYouAreHere:    showYouAreHere,
		enteringWait:      enteringWait,
		youAreHereWait:    youAreHereWait,
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
	return sg.tickIntermissionAdvance(anyIntermissionSkipInput())
}

func (sg *sessionGame) tickIntermissionAdvance(skipPressed bool) bool {
	if !sg.intermission.active {
		return false
	}
	im := &sg.intermission
	im.tic++
	if skipPressed && im.tic <= intermissionSkipInputDelayTics {
		skipPressed = false
	}
	if skipPressed && im.phase != intermissionPhaseYouAreHere {
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
			if im.showEntering {
				im.phase = intermissionPhaseEntering
				im.waitTic = im.enteringWait
				im.stageSoundCounter = 0
				sg.playIntermissionSound(soundEventIntermissionDone)
			} else {
				im.phase = intermissionPhaseYouAreHere
				im.waitTic = im.youAreHereWait
			}
		}
	case intermissionPhaseEntering:
		if im.showYouAreHere {
			im.phase = intermissionPhaseYouAreHere
			im.waitTic = im.youAreHereWait
			sg.playIntermissionSound(soundEventIntermissionTick)
		} else {
			im.phase = intermissionPhaseYouAreHere
			im.waitTic = im.youAreHereWait
		}
	default:
		if im.waitTic <= 0 {
			sg.playIntermissionSound(soundEventIntermissionDone)
			return true
		}
	}
	return false
}

func (sg *sessionGame) startEpisodeFinale(current mapdata.MapName, secret bool) bool {
	screen, ok := episodeFinaleScreen(current, secret)
	if !ok {
		return false
	}
	sg.stopAndClearMusic()
	if sg.g != nil {
		sg.g.clearPendingSoundState()
	}
	sg.finale = sessionFinale{
		active:  true,
		tic:     0,
		waitTic: finaleHoldTics,
		mapName: current,
		screen:  screen,
	}
	return true
}

func (sg *sessionGame) tickFinale() bool {
	return sg.tickFinaleAdvance(anyIntermissionSkipInput())
}

func (sg *sessionGame) tickFinaleAdvance(skipPressed bool) bool {
	if !sg.finale.active {
		return false
	}
	f := &sg.finale
	f.tic++
	if skipPressed && f.tic <= intermissionSkipInputDelayTics {
		skipPressed = false
	}
	if skipPressed && f.waitTic > intermissionSkipExitHoldTics {
		f.waitTic = intermissionSkipExitHoldTics
	}
	if f.waitTic > 0 {
		f.waitTic--
		return false
	}
	sg.finale = sessionFinale{}
	return true
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
	if sg.g != nil {
		sg.g.clearPendingSoundState()
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

	if im.phase >= intermissionPhaseEntering && im.showEntering {
		sg.drawIntermissionMapScreen(screen, scale, ox, oy, im)
		return
	}

	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.target.mapName)
	sg.drawIntermissionText(screen, fmt.Sprintf("FINISHED %s", im.target.mapName), 160, 24, scale, ox, oy, true)
	sg.drawIntermissionText(screen, fmt.Sprintf("KILLS   %3d%%", im.show.killsPct), 80, 70, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("ITEMS   %3d%%", im.show.itemsPct), 80, 90, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("SECRETS %3d%%", im.show.secretsPct), 80, 110, scale, ox, oy, false)
	sg.drawIntermissionText(screen, fmt.Sprintf("TIME %s", formatIntermissionTime(im.show.timeSec)), 80, 138, scale, ox, oy, false)
	if (im.tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
	}
}

func (sg *sessionGame) drawIntermissionMapScreen(screen *ebiten.Image, scale, ox, oy float64, im *sessionIntermission) {
	screen.Fill(color.Black)
	sg.drawIntermissionBackdrop(screen, scale, ox, oy, im.target.mapName)
	sg.drawIntermissionText(screen, fmt.Sprintf("ENTERING %s", im.target.nextMapName), 160, 24, scale, ox, oy, true)
	if im.phase == intermissionPhaseYouAreHere && im.showYouAreHere {
		sg.drawYouAreHerePanel(screen, scale, ox, oy, im.target.mapName, im.target.nextMapName)
	} else {
		sg.drawCurrentIntermissionNode(screen, scale, ox, oy, im.target.mapName)
	}
	if (im.tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO SKIP", 160, 186, scale, ox, oy, true)
	}
}

func (sg *sessionGame) drawFinale(screen *ebiten.Image) {
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
	f := &sg.finale

	screen.Fill(color.Black)
	if strings.TrimSpace(f.screen) != "" {
		_ = sg.drawIntermissionPatch(screen, f.screen, 0, 0, scale, ox, oy, false)
	}
	sg.drawIntermissionText(screen, fmt.Sprintf("EPISODE COMPLETE: %s", f.mapName), 160, 186, scale, ox, oy, true)
	if (f.tic/16)&1 == 0 {
		sg.drawIntermissionText(screen, "PRESS ANY KEY OR CLICK TO CONTINUE", 160, 174, scale, ox, oy, true)
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
	sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, mapCur)
	if mapNext >= 1 && mapNext <= 9 && (sg.intermission.tic/8)&1 == 0 {
		pt := nodes[mapNext-1]
		if !sg.drawIntermissionPatch(screen, "WIURH0", pt.x, pt.y, scale, ox, oy, true) {
			sg.drawIntermissionText(screen, ">", pt.x, pt.y, scale, ox, oy, true)
		}
	}
}

func (sg *sessionGame) drawCurrentIntermissionNode(screen *ebiten.Image, scale, ox, oy float64, current mapdata.MapName) {
	ep, slot, ok := episodeMapSlot(current)
	if !ok {
		return
	}
	nodes := intermissionEpisodeNodePos(ep)
	if len(nodes) != 9 {
		return
	}
	sg.drawIntermissionNodeSplat(screen, scale, ox, oy, nodes, slot)
}

func (sg *sessionGame) drawIntermissionNodeSplat(screen *ebiten.Image, scale, ox, oy float64, nodes []interNodePos, slot int) {
	if slot < 1 || slot > len(nodes) {
		return
	}
	pt := nodes[slot-1]
	if !sg.drawIntermissionPatch(screen, "WISPLAT", pt.x, pt.y, scale, ox, oy, true) {
		sg.drawIntermissionText(screen, "X", pt.x, pt.y, scale, ox, oy, true)
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
	if sg == nil || sg.g == nil {
		return len(text) * 7
	}
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

func (sg *sessionGame) intermissionTextLineHeight() int {
	if sg == nil || sg.g == nil || len(sg.g.opts.MessageFontBank) == 0 {
		return 8
	}
	lineHeight := 0
	for ch := huFontStart; ch <= huFontEnd; ch++ {
		_, _, gh, _, _, ok := sg.g.messageFontGlyph(ch)
		if ok && gh > lineHeight {
			lineHeight = gh
		}
	}
	if lineHeight <= 0 {
		return 8
	}
	return lineHeight
}

func shouldShowYouAreHere(current, next mapdata.MapName) bool {
	epCur, _, okCur := episodeMapSlot(current)
	epNext, _, okNext := episodeMapSlot(next)
	if !okCur || !okNext {
		return false
	}
	return epCur == epNext
}

func shouldShowEnteringScreen(current, next mapdata.MapName) bool {
	_, _, okCur := episodeMapSlot(current)
	if !okCur {
		return false
	}
	_, _, okNext := episodeMapSlot(next)
	return okNext
}

func episodeFinaleScreen(current mapdata.MapName, secret bool) (string, bool) {
	if secret {
		return "", false
	}
	ep, slot, ok := episodeMapSlot(current)
	if !ok || slot != 8 {
		return "", false
	}
	switch ep {
	case 1:
		return "CREDIT", true
	case 2:
		return "VICTORY2", true
	case 3, 4:
		return "ENDPIC", true
	default:
		return "", false
	}
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
		if !thingSpawnsInSession(th, g.opts.SkillLevel, g.opts.GameMode, g.opts.ShowNoSkillItems, g.opts.ShowAllItems) {
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
	out.secretsTotal = g.secretsTotal
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
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton4) {
		return true
	}
	var keys []ebiten.Key
	keys = inpututil.AppendJustPressedKeys(keys)
	return len(keys) > 0
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
		sg.g.setSkyOutputSize(w, h)
		// Sourceport mode renders/presents natively to the current window size,
		// with detail level controlling internal divisor only.
		div := sg.g.sourcePortDetailDivisor()
		if div < 1 {
			div = 1
		}
		rw := max(w/div, 1)
		rh := max(h/div, 1)
		sg.g.Layout(rw, rh)
		return w, h
	}
	// Faithful mode renders game internals at 320x200 and presents at an
	// auto integer-scaled corrected layout (320*n x aspect*n).
	sg.g.Layout(doomLogicalW, doomLogicalH)
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
