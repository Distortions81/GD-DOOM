package automap

import (
	"time"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"

	"github.com/hajimehoshi/ebiten/v2"
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
	attractSeq       int
	attractPage      string
	attractPageTic   int
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
	rt              sessionRuntime
	gameFactory     gameplay.RuntimeFactory[Options, *game]
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

type sessionRuntime interface {
	Update() error
	Draw(*ebiten.Image)
	Layout(int, int) (int, int)
	sessionSignals() gameplay.SessionSignals
	clearPendingSoundState()
	clearSpritePatchCache()
	initSkyLayerShader()
	setSkyOutputSize(int, int)
	sessionSetQuitPromptActive(bool)
	sessionAcknowledgeNewGameRequest()
	sessionAcknowledgeQuitPrompt()
	sessionAcknowledgeReadThis()
	sessionToggleHUDMessages() bool
	sessionCycleDetail() int
	sessionMouseLookSpeed() float64
	sessionSetMouseLookSpeed(float64)
	sessionMusicVolume() float64
	sessionSetMusicVolume(float64)
	sessionSFXVolume() float64
	sessionSetSFXVolume(float64)
	sessionPublishRuntimeSettings()
	sessionDrawHUTextAt(*ebiten.Image, string, float64, float64, float64, float64)
	sessionPlaySoundEvent(soundEvent)
	sessionTickSound()
}

const quitPromptExitDelayTics = 53

const (
	attractPageTitleNonCommercial = 170
	attractPageTitleCommercial    = 35 * 11
	attractPageInfo               = 200
)

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

func cloneMapForRestart(src *mapdata.Map) *mapdata.Map { return gameplay.CloneMapForRestart(src) }

type sessionPersistentSettings struct {
	detailLevel        int
	rotateView         bool
	mouseLook          bool
	mouseLookSpeed     float64
	musicVolume        float64
	oplVolume          float64
	sfxVolume          float64
	hudMessagesEnabled bool
	alwaysRun          bool
	autoWeaponSwitch   bool
	lineColorMode      string
	thingRenderMode    string
	showLegend         bool
	paletteLUT         bool
	gammaLevel         int
	crtEnabled         bool
	reveal             revealMode
	iddt               int
}

func (s sessionPersistentSettings) gameplay() gameplay.PersistentSettings {
	return gameplay.PersistentSettings{
		DetailLevel:      s.detailLevel,
		RotateView:       s.rotateView,
		MouseLook:        s.mouseLook,
		MouseLookSpeed:   s.mouseLookSpeed,
		MusicVolume:      s.musicVolume,
		OPLVolume:        s.oplVolume,
		SFXVolume:        s.sfxVolume,
		HUDMessages:      s.hudMessagesEnabled,
		AlwaysRun:        s.alwaysRun,
		AutoWeaponSwitch: s.autoWeaponSwitch,
		LineColorMode:    s.lineColorMode,
		ThingRenderMode:  s.thingRenderMode,
		ShowLegend:       s.showLegend,
		PaletteLUT:       s.paletteLUT,
		GammaLevel:       s.gammaLevel,
		CRTEnabled:       s.crtEnabled,
		Reveal:           int(s.reveal),
		IDDT:             s.iddt,
	}
}

func clampDetailLevelForMode(level int, sourcePort bool) int {
	return gameplay.ClampDetailLevel(level, sourcePort, len(detailPresets), len(sourcePortDetailDivisors))
}

func normalizeRevealForMode(mode revealMode, sourcePort bool) revealMode {
	return revealMode(gameplay.NormalizeReveal(int(mode), sourcePort, int(revealNormal), int(revealAllMap)))
}

func clampIDDT(v int) int {
	return gameplay.ClampIDDT(v)
}

func clampGamma(level int) int {
	return gameplay.ClampGamma(level, len(gammaTargets))
}

func clampVolume(v float64) float64 {
	return gameplay.ClampVolume(v)
}

func clampOPLVolume(v float64) float64 {
	return gameplay.ClampOPLVolume(v, music.MaxOutputGain)
}

func (sg *sessionGame) capturePersistentSettings() {
	if sg == nil || sg.g == nil {
		return
	}
	g := sg.g
	sg.settings = sessionPersistentSettings{
		detailLevel:        g.detailLevel,
		rotateView:         g.rotateView,
		mouseLook:          g.opts.MouseLook,
		mouseLookSpeed:     g.opts.MouseLookSpeed,
		musicVolume:        g.opts.MusicVolume,
		oplVolume:          g.opts.OPLVolume,
		sfxVolume:          g.opts.SFXVolume,
		hudMessagesEnabled: g.hudMessagesEnabled,
		alwaysRun:          g.alwaysRun,
		autoWeaponSwitch:   g.autoWeaponSwitch,
		lineColorMode:      g.opts.LineColorMode,
		thingRenderMode:    g.opts.SourcePortThingRenderMode,
		showLegend:         g.showLegend,
		paletteLUT:         g.paletteLUTEnabled,
		gammaLevel:         g.gammaLevel,
		crtEnabled:         g.crtEnabled,
		reveal:             g.parity.reveal,
		iddt:               g.parity.iddt,
	}
}

func (sg *sessionGame) applyPersistentSettingsToOptions() {
	next := gameplay.ApplyPersistentSettingsToOptions(gameplay.OptionState{
		MouseLook:        sg.opts.MouseLook,
		MouseLookSpeed:   sg.opts.MouseLookSpeed,
		MusicVolume:      sg.opts.MusicVolume,
		OPLVolume:        sg.opts.OPLVolume,
		SFXVolume:        sg.opts.SFXVolume,
		AlwaysRun:        sg.opts.AlwaysRun,
		AutoWeaponSwitch: sg.opts.AutoWeaponSwitch,
		LineColorMode:    sg.opts.LineColorMode,
		ThingRenderMode:  sg.opts.SourcePortThingRenderMode,
	}, sg.settings.gameplay(), music.MaxOutputGain)
	sg.opts.MouseLook = next.MouseLook
	sg.opts.MouseLookSpeed = next.MouseLookSpeed
	sg.opts.MusicVolume = next.MusicVolume
	sg.opts.OPLVolume = next.OPLVolume
	sg.opts.SFXVolume = next.SFXVolume
	sg.opts.AlwaysRun = next.AlwaysRun
	sg.opts.AutoWeaponSwitch = next.AutoWeaponSwitch
	sg.opts.LineColorMode = next.LineColorMode
	sg.opts.SourcePortThingRenderMode = next.ThingRenderMode
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
	next := gameplay.ApplyRuntimeSettings(
		sg.settings.gameplay(),
		s,
		sg.opts.SourcePortMode,
		len(detailPresets),
		len(sourcePortDetailDivisors),
		len(gammaTargets),
		music.MaxOutputGain,
	)
	sg.settings.detailLevel = next.DetailLevel
	sg.settings.mouseLook = next.MouseLook
	sg.settings.musicVolume = next.MusicVolume
	sg.settings.oplVolume = next.OPLVolume
	sg.settings.sfxVolume = next.SFXVolume
	sg.settings.alwaysRun = next.AlwaysRun
	sg.settings.autoWeaponSwitch = next.AutoWeaponSwitch
	sg.settings.lineColorMode = next.LineColorMode
	sg.settings.thingRenderMode = normalizeSourcePortThingRenderMode(next.ThingRenderMode, sg.opts.SourcePortMode)
	sg.settings.gammaLevel = next.GammaLevel
	sg.settings.crtEnabled = next.CRTEnabled && sg.opts.KageShader
	sg.opts.MusicVolume = sg.settings.musicVolume
	sg.opts.OPLVolume = sg.settings.oplVolume
	sg.opts.SFXVolume = sg.settings.sfxVolume
	if sg.menuSfx != nil {
		sg.menuSfx.SetVolume(sg.settings.sfxVolume)
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
	var pending []DemoTic
	if sg.g != nil {
		pending = sg.g.demoRecord
	}
	if sg.g != nil {
		sg.g.clearSpritePatchCache()
	}
	sg.capturePersistentSettings()
	state, remaining := gameplay.PrepareRebuild(
		sg.demoRecord,
		pending,
		gameplay.OptionState{
			MouseLook:        sg.opts.MouseLook,
			MouseLookSpeed:   sg.opts.MouseLookSpeed,
			MusicVolume:      sg.opts.MusicVolume,
			OPLVolume:        sg.opts.OPLVolume,
			SFXVolume:        sg.opts.SFXVolume,
			AlwaysRun:        sg.opts.AlwaysRun,
			AutoWeaponSwitch: sg.opts.AutoWeaponSwitch,
			LineColorMode:    sg.opts.LineColorMode,
			ThingRenderMode:  sg.opts.SourcePortThingRenderMode,
		},
		sg.settings.gameplay(),
		music.MaxOutputGain,
	)
	sg.demoRecord = state.DemoRecord
	if sg.g != nil {
		sg.g.demoRecord = remaining
	}
	sg.opts.MouseLook = state.Options.MouseLook
	sg.opts.MouseLookSpeed = state.Options.MouseLookSpeed
	sg.opts.MusicVolume = state.Options.MusicVolume
	sg.opts.OPLVolume = state.Options.OPLVolume
	sg.opts.SFXVolume = state.Options.SFXVolume
	sg.opts.AlwaysRun = state.Options.AlwaysRun
	sg.opts.AutoWeaponSwitch = state.Options.AutoWeaponSwitch
	sg.opts.LineColorMode = state.Options.LineColorMode
	sg.opts.SourcePortThingRenderMode = state.Options.ThingRenderMode
	ng := sg.buildGame(next, sg.opts)
	sg.applyPersistentSettingsToGame(ng)
	sg.g = ng
	sg.rt = ng
}

func (sg *sessionGame) buildGame(m *mapdata.Map, opts Options) *game {
	if sg == nil {
		return nil
	}
	factory := sg.gameFactory
	if factory == nil {
		factory = newGame
	}
	return gameplay.BuildRuntime(factory, m, opts)
}

func (sg *sessionGame) collectDemoRecord() {
	if sg == nil || sg.g == nil || len(sg.g.demoRecord) == 0 {
		return
	}
	sg.demoRecord, sg.g.demoRecord = gameplay.CollectDemoRecord(sg.demoRecord, sg.g.demoRecord)
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
	return gameplay.RestartMapForRespawn(sg.g.m, sg.currentTemplate, normalizeGameMode(sg.opts.GameMode) == gameModeSingle)
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
	sg.g = sg.buildGame(sg.bootMap, sg.opts)
	sg.rt = sg.g
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
