//go:build raylib && cgo && !js

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gddoom/internal/audiofx"
	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	configPath := flag.String("config", "config.toml", "preferences path (empty disables loading and saving)")
	mouseSpeed := flag.Float64("mouse-speed", runtimecfg.DefaultMouseLookSpeed, "mouse sensitivity (positive multiplier)")
	keyboardSpeed := flag.Float64("keyboard-speed", runtimecfg.DefaultKeyboardTurnSpeed, "keyboard turn speed (positive multiplier)")
	smoothCameraYaw := flag.Bool("smooth-camera-yaw", runtimecfg.DefaultSmoothCameraYaw, "smooth interpolated player camera yaw between sim ticks")
	gammaLevel := flag.Int("gamma-level", -1, "startup gamma level (-1 keeps mode default)")
	detailLevel := flag.Int("detail-level", 0, "source-port resolution divisor: 0=1x, 1=1/2, 2=1/3, 3=1/4")
	autoDetail := flag.Bool("auto-detail", false, "adjust detail using the shared 60 FPS AUTO policy")
	crtEffect := flag.Bool("crt-effect", false, "enable CRT postprocess effect")
	noAspectCorrection := flag.Bool("no-aspect-correction", false, "disable faithful-mode presentation aspect correction (no effect in source-port mode)")
	mouseLook := flag.Bool("mouselook", true, "enable horizontal mouse aiming")
	mouseInvert := flag.Bool("mouse-invert", false, "reverse horizontal mouse aiming")
	alwaysRun := flag.Bool("always-run", runtimecfg.DefaultAlwaysRun, "run by default (run modifier walks)")
	autoWeaponSwitch := flag.Bool("auto-weapon-switch", true, "switch to collected weapons")
	fullscreen := flag.Bool("fullscreen", false, "start fullscreen")
	noVsync := flag.Bool("no-vsync", false, "disable vsync and default draw cap (an explicit or saved -fps still applies)")
	wadPath := flag.String("wad", "DOOM1.WAD", "IWAD path")
	filePaths := flag.String("file", "", "comma-separated PWAD overlay paths, in load order")
	mapName := flag.String("map", "", "level name for direct launch (default: start at the menu)")
	width := flag.Int("width", 1280, "window width")
	height := flag.Int("height", 800, "window height")
	fps := flag.Int("fps", 144, "frame limit (0 = uncapped)")
	frames := flag.Int("frames", 0, "exit after this many frames (0 = interactive)")
	capture := flag.String("capture", "", "PNG screenshot path, taken on final frame")
	pose := flag.String("camera", "", "inspection pose x,y,eye-z,yaw-degrees")
	modeFlag := flag.String("mode", "textured", "textured, sectors, or wireframe")
	textureScale := flag.Int("texture-scale", 2, "texture upload scale (1 or 2; nearest-neighbor enlargement)")
	textureFilter := flag.String("texture-filter", "anisotropic", "nearest, trilinear, or anisotropic (8x)")
	lightingFlag := flag.String("lighting", "doom", "doom, sector, or fullbright")
	msaa := flag.Bool("msaa", true, "request 4x multisample antialiasing for geometry edges and seams")
	noMonsters := flag.Bool("nomonsters", false, "disable monster spawns")
	god := flag.Bool("god", false, "invulnerable inspection player")
	skill := flag.Int("skill", 3, "Doom skill level (1 through 5)")
	debug := flag.Bool("debug", false, "show renderer diagnostics (Shift+F10 toggles)")
	soundEnabled := flag.Bool("sound", true, "enable native sound effects (capture runs are silent)")
	sfxVolume := flag.Float64("sfx-volume", runtimecfg.DefaultSFXVolume, "sound volume (0 through 1)")
	musicEnabled := flag.Bool("music", true, "enable native MUS music (capture runs are silent)")
	musicBackend := flag.String("music-backend", music.DefaultBackend().String(), "auto, impsynth, pcspeaker, or meltysynth")
	soundFont := flag.String("soundfont", "", "SoundFont (.sf2) for MeltySynth")
	musicPan := flag.Float64("mus-pan-max", .8, "maximum MUS stereo pan (0 through 1)")
	musicCompression := flag.Float64("mus-volume-compression", music.DefaultMUSVolumeCompression, "compress MUS note/controller volume toward max (1 = off, 2 = moderate, 3 = stronger)")
	speakerVolume := flag.Float64("pc-speaker-volume", 1, "PC-speaker volume (0 through 1)")
	speakerEffects := flag.Bool("pc-speaker", false, "use DP PC-speaker effects in place of digital effects")
	speakerInterleave := flag.Float64("pc-speaker-interleave-hz", 280, "shared PC-speaker effect/music interleave rate (10 through 1000 Hz)")
	speakerVariant := flag.String("pc-speaker-variant", "paper-speaker", "passthrough, paper-speaker, or small-buzzer")
	speakerOutput := flag.String("pc-speaker-output", "emulated", "PC-speaker output backend (emulated or linux)")
	musicVolume := flag.Float64("music-volume", runtimecfg.DefaultMusicVolume, "music volume (0 through 1)")
	startAutomap := flag.Bool("automap", false, "start direct gameplay in the textured automap (Tab toggles)")
	demoPath := flag.String("demo", "", "Doom v1.9/v1.10 LMP path or DEMO lump; play and exit at completion")
	recordDemoPath := flag.String("record-demo", "", "write a Doom v1.10 LMP recorded from live commands")
	demoExitOnDeath := flag.Bool("demo-exit-on-death", false, "stop demo playback when the player dies")
	demoStopAfterTics := flag.Int("demo-stop-after-tics", 0, "stop after this many demo commands (0 disables)")
	demoTracePath := flag.String("trace-demo-state", "", "write shared per-tic state JSONL for demo playback")
	startMenu := flag.Bool("menu", true, "enable the title loop; an explicit -menu opens its menu, -menu=false skips it")
	flag.String("broadcast", "", "publish to a GDSF relay (bare flag uses 127.0.0.1:6670)")
	flag.String("watch", "", "watch a relay session (bare flag uses 127.0.0.1:6670)")
	watchSession := flag.Uint64("watch-session", 0, "relay session ID to watch")
	lowLatency := flag.Bool("low-latency", false, "flush every broadcast tic immediately")
	gameplayFlags := registerNativeGameplayFlags(flag.CommandLine)
	registerNativeFlagAliases(flag.CommandLine)
	if err := flag.CommandLine.Parse(nativeNetworkArgs(os.Args[1:])); err != nil {
		return err
	}
	cfg, err := applyNativeConfig(flag.CommandLine)
	if err != nil {
		return err
	}
	broadcast, watch, err := nativeNetworkFlags(flag.CommandLine)
	if err != nil {
		return err
	}
	backend, err := music.ParseBackend(*musicBackend)
	if err != nil {
		return err
	}
	if *musicPan < 0 || *musicPan > 1 || math.IsNaN(*musicPan) || *speakerVolume < 0 || *speakerVolume > 1 || math.IsNaN(*speakerVolume) {
		return fmt.Errorf("invalid music pan or PC-speaker volume")
	}
	if *speakerInterleave < 10 || *speakerInterleave > 1000 || math.IsNaN(*speakerInterleave) {
		return fmt.Errorf("PC-speaker interleave rate must be between 10 and 1000 Hz")
	}
	audiofx.SetPCSpeakerInterleaveHz(*speakerInterleave)
	if music.ResolveBackend(backend) == music.BackendMeltySynth && strings.TrimSpace(*soundFont) == "" {
		return fmt.Errorf("meltysynth backend requires a SoundFont (.sf2)")
	}
	if !validNativeControlSpeed(*mouseSpeed) || !validNativeControlSpeed(*keyboardSpeed) {
		return fmt.Errorf("control speeds must be finite positive multipliers")
	}
	if err := validateNativeDemoFlags(*demoPath, *recordDemoPath, *demoTracePath, *demoStopAfterTics, *pose); err != nil {
		return err
	}
	bindings := runtimecfg.DefaultInputBindings()
	if cfg.Keybinds != nil {
		bindings = runtimecfg.NormalizeInputBindings(*cfg.Keybinds)
	}
	lightingMode := raymesh.LightingMode(*lightingFlag)
	if err := lightingMode.Validate(); err != nil {
		return err
	}
	textureOptions := raymesh.TextureOptions{Scale: *textureScale, Filter: raymesh.TextureFilter(*textureFilter)}
	if err := textureOptions.Validate(); err != nil {
		return err
	}
	if *width < 320 || *height < 200 || *fps < 0 || *frames < 0 || *skill < 1 || *skill > 5 {
		return fmt.Errorf("invalid window size, frame limit, or frame count")
	}
	mode := levelmesh.Mode(*modeFlag)
	if mode != levelmesh.Textured && mode != levelmesh.Sectors && mode != levelmesh.Wireframe {
		return fmt.Errorf("invalid mesh mode %q", mode)
	}
	if *capture != "" && *frames == 0 {
		return fmt.Errorf("-capture requires -frames")
	}
	if *sfxVolume < 0 || *sfxVolume > 1 || math.IsNaN(*sfxVolume) || *musicVolume < 0 || *musicVolume > 1 || math.IsNaN(*musicVolume) {
		return fmt.Errorf("invalid sound volume")
	}
	overlayPaths := launchcatalog.ResolveWADOverlayPaths(*filePaths)
	wf, wadPaths, err := launchcatalog.OpenWADStack(*wadPath, overlayPaths)
	if err != nil {
		return err
	}
	maps := mapdata.AvailableMapNames(wf)
	menuSpecified := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "menu" {
			menuSpecified = true
		}
	})
	launch, err := chooseNativeLaunch(*mapName, *startMenu, menuSpecified, maps)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*mapName) == "" {
		launch.mapName, err = launchcatalog.DefaultStartMap(wf, overlayPaths)
		if err != nil {
			return err
		}
	}
	script, err := loadNativeDemo(wf, *demoPath)
	if err != nil {
		return err
	}
	if script != nil {
		launch.mapName, err = launchcatalog.ResolveDemoStartMap(wf, script, launch.mapName)
		if err != nil {
			return err
		}
		launch.frontend, launch.showMenu = false, false
	} else if strings.TrimSpace(*recordDemoPath) != "" {
		launch.frontend, launch.showMenu = false, false
	}
	m, err := mapdata.LoadMap(wf, launch.mapName)
	if err != nil {
		return err
	}
	opts, err := loadAssets(wf)
	if err != nil {
		return err
	}
	opts.WADSources = launchcatalog.BuildWADSources(wadPaths)
	opts.WADHash = launchcatalog.HashWADStackSHA1(wadPaths)
	opts.MusicPlayerCatalog, opts.MusicPlayerTrackLoader = launchcatalog.BuildMusicPlayerCatalog(wadPaths[0])
	opts.MusicSoundFontChoices = launchcatalog.DetectAvailableSoundFonts("soundfonts")
	if strings.TrimSpace(*soundFont) != "" {
		found := false
		for _, path := range opts.MusicSoundFontChoices {
			if path == *soundFont {
				found = true
				break
			}
		}
		if !found {
			opts.MusicSoundFontChoices = append(opts.MusicSoundFontChoices, *soundFont)
		}
	}
	opts.Width, opts.Height = *width, *height
	opts.NoMonsters, opts.Invulnerable, opts.SkillLevel = *noMonsters, *god, *skill
	if err := gameplayFlags.Apply(&opts); err != nil {
		return err
	}
	opts.DemoScript, opts.DemoQuitOnComplete = script, script != nil
	opts.DemoExitOnDeath, opts.DemoStopAfterTics, opts.DemoTracePath = *demoExitOnDeath, *demoStopAfterTics, strings.TrimSpace(*demoTracePath)
	opts.RecordDemoPath = strings.TrimSpace(*recordDemoPath)
	opts.InputBindings = bindings
	opts.AlwaysRun, opts.AutoWeaponSwitch = *alwaysRun, *autoWeaponSwitch
	opts.MouseLook, opts.MouseInvert, opts.MouseLookSpeed, opts.KeyboardTurnSpeed = *mouseLook, *mouseInvert, *mouseSpeed, *keyboardSpeed
	opts.SmoothCameraYaw = *smoothCameraYaw
	opts.InitialGammaLevel = *gammaLevel
	opts.InitialDetailLevel, opts.AutoDetail = *detailLevel, *autoDetail
	opts.CRTEffect, opts.DisableAspectCorrection = *crtEffect, *noAspectCorrection
	opts.NoVsync = *noVsync
	opts.ZombiemanThinkerBlend = true
	opts.MUSVolumeCompression = music.NormalizeMUSVolumeCompression(*musicCompression)
	var musicPlayer *nativeMusic
	opts.NewGameLoader = func(name string) (*mapdata.Map, error) { return mapdata.LoadMap(wf, mapdata.MapName(name)) }
	opts.AttractDemos = launchcatalog.LoadBuiltInDemos(wf)
	opts.DemoMapLoader = func(script *doomruntime.DemoScript) (*mapdata.Map, error) {
		name, err := launchcatalog.ResolveDemoStartMap(wf, script, launch.mapName)
		if err != nil {
			return nil, err
		}
		return mapdata.LoadMap(wf, name)
	}
	for episode := 1; episode <= 4; episode++ {
		for _, name := range maps {
			if strings.HasPrefix(string(name), fmt.Sprintf("E%dM", episode)) {
				opts.Episodes = append(opts.Episodes, episode)
				break
			}
		}
	}
	opts.PlayCheatMusic = func(current, code string) (bool, error) { return musicPlayer.PlayCheat(current, code) }
	network := &nativeNetwork{}
	defer network.Close()
	if broadcast != "" || watch != "" {
		name, err := network.Connect(broadcast, watch, *watchSession, *lowLatency, &opts, m.Name)
		if err != nil {
			return err
		}
		if name != m.Name {
			m, err = mapdata.LoadMap(wf, name)
			if err != nil {
				return err
			}
		}
		launch.frontend, launch.showMenu = false, false
		launch.mapName = m.Name
	}
	settings := nativeSettings{pcSpeaker: *speakerEffects, musicBackend: backend, soundFont: *soundFont, musicPan: *musicPan, speakerVolume: *speakerVolume, speakerVariant: *speakerVariant, sfxVolume: *sfxVolume, musicVolume: *musicVolume, mouseSensitivity: *mouseSpeed, keyboardSpeed: *keyboardSpeed, mouseLook: *mouseLook, mouseInvert: *mouseInvert, alwaysRun: *alwaysRun, autoWeaponSwitch: *autoWeaponSwitch, fullscreen: *fullscreen, bindings: bindings, textureFilter: textureOptions.Filter, lighting: lightingMode, fps: *fps, debug: *debug}
	settings.messages, settings.showFPS = true, !opts.NoFPS
	settings.noAspectCorrection = opts.DisableAspectCorrection
	settings.noVsync = opts.NoVsync
	settings.detailLevel, settings.autoDetail = opts.InitialDetailLevel, opts.AutoDetail
	settings.musicCompression = opts.MUSVolumeCompression
	settings.smoothCameraYaw = *smoothCameraYaw
	settings.autoWeaponSwitch = opts.AutoWeaponSwitch
	settings.screenBlocks, settings.hudScale = doomruntime.NativeDefaultMenuHUD(opts)
	if cfg.Raylib != nil {
		r := cfg.Raylib
		if r.Messages != nil {
			settings.messages = *r.Messages
		}
		if r.ScreenBlocks != nil {
			settings.screenBlocks = max(1, min(2, *r.ScreenBlocks))
		}
		if r.HUDScale != nil {
			settings.hudScale = max(0, min(7, *r.HUDScale))
		}
	}
	if !*soundEnabled {
		settings.sfxVolume = 0
	}
	if !*musicEnabled {
		settings.musicVolume, settings.musicMuted = 0, true
	}
	menu := newNativeMenu(opts, maps, m.Name)
	menu.frontend = launch.frontend
	if launch.showMenu {
		menu.open(menuMain)
	}
	nextMap := func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
		name, err := mapdata.NextMapName(wf, current, secret)
		if err != nil {
			return nil, "", err
		}
		next, err := mapdata.LoadMap(wf, name)
		return next, name, err
	}
	game := doomruntime.NewNativeMeshGame(m, opts)
	campaign := doomruntime.NewNativeCampaign(game, opts, nextMap)
	if *gammaLevel >= 0 {
		campaign.SetGammaLevel(*gammaLevel)
	}
	if len(network.initial) > 0 {
		if err := campaign.LoadKeyframe(network.initial); err != nil {
			return fmt.Errorf("watch initial keyframe: %w", err)
		}
		game, m = campaign.Game, campaign.Map()
		fmt.Printf("watch: keyframe loaded tic=%d\n", game.Frame(1).WorldTic)
	}
	settings.gammaLevel = game.GammaLevel()
	settings.crtEffect = game.CRTEnabled()
	if launch.frontend {
		campaign.StartFrontend()
	}
	defer func() {
		campaign.CloseDemoTrace()
		if count, err := campaign.FlushDemoRecording(); err != nil {
			if runErr == nil {
				runErr = err
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		} else if count > 0 {
			fmt.Printf("demo-recorded tics=%d\n", count)
		}
	}()
	lastDemoFlush := time.Now()
	if *pose != "" {
		var x, y, z, yaw float64
		if n, err := fmt.Sscanf(*pose, "%f,%f,%f,%f", &x, &y, &z, &yaw); err != nil || n != 4 {
			return fmt.Errorf("-camera needs x,y,eye-z,yaw-degrees")
		}
		if err := game.SetPose(x, y, z, yaw*math.Pi/180); err != nil {
			return err
		}
	}
	if err := campaign.BroadcastInitialKeyframe(); err != nil {
		return err
	}
	rl.SetTraceLogLevel(rl.LogWarning)
	windowFlags := uint32(rl.FlagWindowResizable)
	if !*noVsync {
		windowFlags |= rl.FlagVsyncHint
	}
	if *fullscreen {
		windowFlags |= rl.FlagFullscreenMode
	}
	aaLabel := "off"
	if *msaa {
		// Raylib's MSAA hint must precede InitWindow. Draw directly to its
		// multisampled framebuffer; ordinary render textures are single-sample.
		windowFlags |= rl.FlagMsaa4xHint
		aaLabel = "4x MSAA requested"
	}
	rl.SetConfigFlags(windowFlags)
	rl.InitWindow(int32(*width), int32(*height), "GD-DOOM | Raylib GPU mesh experiment")
	if !rl.IsWindowReady() {
		return fmt.Errorf("Raylib window could not initialize")
	}
	defer rl.CloseWindow()
	rl.SetExitKey(rl.KeyNull)
	rl.SetTargetFPS(int32(*fps))
	if *frames == 0 && menu.page == menuClosed && !menu.frontend {
		rl.DisableCursor()
	}
	renderer, err := raymesh.NewRendererWithTextureOptions(textureOptions)
	if err != nil {
		return err
	}
	crt := &raymesh.CRT{}
	defer crt.Close()
	detail := &raymesh.Detail{}
	defer detail.Close()
	defer renderer.Close()
	presentation, err := raymesh.NewPresentation(textureOptions)
	if err != nil {
		return err
	}
	defer presentation.Close()
	automapRenderer := &nativeAutomap{}
	defer automapRenderer.Close()
	savePreview := &nativeSavePreview{}
	defer savePreview.Close()
	wipe := &nativeWipe{}
	defer wipe.Close()
	pendingMusicLump := ""
	var pendingMusicConfig *nativeMusicConfig
	thumbnailSlot := -1
	automap := *startAutomap
	var pendingMap doomruntime.NativeMapInput
	game.SetMapActive(automap)
	var audio *nativeAudio
	if *frames == 0 {
		audio = newNativeAudioWithOutput(wf, float32(settings.sfxVolume), audiofx.ParsePCSpeakerOutput(strings.TrimSpace(*speakerOutput)))
		if audio == nil {
			fmt.Fprintln(os.Stderr, "Native audio device unavailable; continuing silently")
		}
	}
	defer audio.Close()
	if audio != nil {
		if err := audio.ConfigureSpeaker(settings.pcSpeaker, settings.speakerVolume, settings.speakerVariant); err != nil {
			return err
		}
		if music.ResolveBackend(settings.musicBackend) == music.BackendPCSpeaker && audio.speakerError != nil {
			return audio.speakerError
		}
		musicPlayer, err = newNativeMusicWithConfig(wf, float32(settings.musicPlaybackVolume()), settings.musicConfig())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			settings.musicVolume, settings.speakerVolume = 0, 0
		}
		musicPlayer.UseSharedSpeaker(audio.speaker)
		if musicPlayer != nil && settings.musicPlaybackVolume() > 0 {
			var err error
			if menu.frontend {
				err = musicPlayer.PlayTitle(string(m.Name))
			} else {
				err = musicPlayer.PlayLump(campaign.MusicLump())
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	defer musicPlayer.Close()
	attachAudio := func() {
		audio.Attach(game)
	}
	attachAudio()
	if err := renderer.SetLightingMode(lightingMode); err != nil {
		return err
	}
	renderer.SetLightRamp(game.LightRamp())
	renderer.SetLightRows(game.LightRows())
	presentation.Sprites.SetLightRamp(game.LightRamp())
	presentation.Sprites.SetLightRows(game.LightRows())
	const tic = 1.0 / 35
	accumulator := 0.0
	last := time.Now()
	use, fire := false, false
	weapon, weaponCycle := 0, 0
	yawPending := 0.0
	paused := false
	blockedBefore := menu.page != menuClosed
	releaseButtons := false
	quit := false
	skipPending := false
	reload := func(fresh *mapdata.Map, levelSkill int) error {
		if err := campaign.StopDemoRecording(); err != nil {
			game.Notify(err.Error())
			fmt.Fprintln(os.Stderr, err)
		}
		campaign.CloseDemoTrace()
		opts.RecordDemoPath, opts.DemoTracePath, opts.DemoScript, opts.DemoQuitOnComplete = "", "", nil, false
		wipe.Queue()
		audio.Stop()
		opts.SkillLevel = levelSkill
		opts.AlwaysRun, opts.MouseLookSpeed = settings.alwaysRun, settings.mouseSensitivity
		opts.InputBindings = settings.bindings
		opts.MouseLook, opts.MouseInvert, opts.KeyboardTurnSpeed, opts.AutoWeaponSwitch = settings.mouseLook, settings.mouseInvert, settings.keyboardSpeed, settings.autoWeaponSwitch
		opts.SmoothCameraYaw = settings.smoothCameraYaw
		opts.InitialDetailLevel, opts.AutoDetail = settings.detailLevel, settings.autoDetail
		m = fresh
		game = doomruntime.NewNativeMeshGame(fresh, opts)
		campaign = doomruntime.NewNativeCampaign(game, opts, nextMap)
		campaign.SetGammaLevel(settings.gammaLevel)
		campaign.SetCRTEnabled(settings.crtEffect)
		automap = false
		pendingMap = doomruntime.NativeMapInput{}
		attachAudio()
		accumulator, yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, 0, false, false
		releaseButtons = true
		renderer.SetLightRamp(game.LightRamp())
		renderer.SetLightRows(game.LightRows())
		presentation.Sprites.SetLightRamp(game.LightRamp())
		presentation.Sprites.SetLightRows(game.LightRows())
		musicPlayer.Stop()
		pendingMusicLump = campaign.MusicLump()
		for i, name := range menu.maps {
			if name == m.Name {
				menu.mapIndex = i
			}
		}
		return campaign.BroadcastMandatoryKeyframe()
	}
	adoptCampaignGame := func(transition bool) {
		if campaign.Game != game {
			if transition && !campaign.DemoStatus().Active {
				wipe.Queue()
			}
			use, fire = false, false
			game = campaign.Game
			settings.gammaLevel = game.GammaLevel()
			settings.crtEffect = game.CRTEnabled()
			settings.detailLevel, settings.autoDetail = campaign.DetailSettings()
			m = campaign.Map()
			attachAudio()
			automap = game.MapActive()
			if !menu.frontend {
				settings.alwaysRun = game.AlwaysRun()
				settings.autoWeaponSwitch = game.AutoWeaponSwitch()
				opts.SkillLevel, menu.skill = game.SkillLevel(), game.SkillLevel()
				for i, name := range menu.maps {
					if name == m.Name {
						menu.mapIndex = i
					}
				}
			}
			pendingMap = doomruntime.NativeMapInput{}
			releaseButtons = true
			renderer.SetLightRamp(game.LightRamp())
			renderer.SetLightRows(game.LightRows())
			presentation.Sprites.SetLightRamp(game.LightRamp())
			presentation.Sprites.SetLightRows(game.LightRows())
			rl.SetWindowTitle("GD-DOOM | " + string(m.Name) + " | Raylib GPU mesh experiment")
		}
	}
	openSlots := func(page menuPage) {
		if campaign.Watching() {
			game.Notify("WATCH MODE")
			return
		}
		menu.open(page)
		menu.saveSlots = campaign.SaveSlots(page == menuSave)
		savePreview.Close()
	}
	saveSlot := func(slot int) {
		if campaign.Watching() {
			game.Notify("WATCH MODE")
			return
		}
		if menu.frontend || campaign.DemoStatus().Active || campaign.Phase() != doomruntime.NativeCampaignPlaying {
			menu.setStatus("START A GAME TO SAVE", 70)
			game.Notify(menu.status)
			return
		}
		campaign.SetControls(settings.alwaysRun, settings.mouseSensitivity)
		campaign.SetCameraSmoothing(settings.smoothCameraYaw)
		if err := campaign.SaveSlot(slot); err != nil {
			menu.setStatus("SAVE FAILED", 70)
			game.Notify("SAVE FAILED: " + err.Error())
			fmt.Fprintln(os.Stderr, err)
			return
		}
		thumbnailSlot = slot
		accumulator, yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, 0, false, false
		menu.open(menuClosed)
		releaseButtons = true
	}
	loadSlot := func(slot int) {
		if campaign.Watching() {
			game.Notify("WATCH MODE")
			return
		}
		if campaign.DemoStatus().Active && !menu.frontend {
			menu.setStatus("START A GAME TO LOAD", 70)
			return
		}
		if _, err := campaign.FlushDemoRecording(); err != nil {
			game.Notify(err.Error())
			return
		}
		if err := campaign.LoadSlot(slot); err != nil {
			menu.setStatus("LOAD FAILED", 70)
			game.Notify("LOAD FAILED: " + err.Error())
			fmt.Fprintln(os.Stderr, err)
			return
		}
		audio.Stop()
		wipe.Clear()
		menu.frontend, paused = false, false
		adoptCampaignGame(false)
		menu.open(menuClosed)
		accumulator, yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, 0, false, false
		pendingMap = doomruntime.NativeMapInput{}
		skipPending = false
		releaseButtons = true
	}
	windowWidth, windowHeight := *width, *height
	resizeChanged := time.Time{}
	persist := func() {
		if *frames > 0 {
			return
		}
		if err := saveNativePreferences(*configPath, settings, settings.bindings, windowWidth, windowHeight, *textureScale, *msaa, string(mode)); err != nil {
			game.Notify("SETTINGS SAVE FAILED: " + err.Error())
			fmt.Fprintln(os.Stderr, err)
		}
	}
	menuClock, menuTic := 0.0, 0
	for frame := 0; !quit && !rl.WindowShouldClose(); frame++ {
		now := time.Now()
		accumulator += math.Min(now.Sub(last).Seconds(), .25)
		menuClock += math.Min(now.Sub(last).Seconds(), .25)
		for menuClock >= tic {
			menuTic++
			if rl.IsWindowFocused() || *frames > 0 {
				quit = quit || menu.advanceFrame() == menuExit
			}
			menuClock -= tic
		}
		last = now
		focused := rl.IsWindowFocused() || *frames > 0
		if err := campaign.PollNetwork(); err != nil {
			return err
		}
		adoptCampaignGame(false)
		if *frames > 0 && (menu.frontend || campaign.DemoStatus().Active || campaign.RecordingActive()) {
			accumulator = tic
		}
		chars := []rune(nil)
		for r := rl.GetCharPressed(); r != 0; r = rl.GetCharPressed() {
			chars = append(chars, rune(r))
		}
		cheatTyping := false
		playing := campaign.Phase() == doomruntime.NativeCampaignPlaying
		chatHandled := false
		if focused && !paused && !wipe.Active() && menu.page == menuClosed && playing && !menu.frontend {
			chatHandled = game.ChatInput(doomruntime.NativeChatInput{Open: nativeBindingPressed(settings.bindings.Chat), Cancel: rl.IsKeyPressed(rl.KeyEscape), Backspace: rl.IsKeyPressed(rl.KeyBackspace), Send: rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter), Runes: chars})
			if chatHandled {
				yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, false, false
			}
		}
		if !chatHandled && !campaign.Watching() && focused && !paused && !wipe.Active() && menu.page == menuClosed && playing && !menu.frontend && !campaign.DemoStatus().Active {
			cheatTyping, err = campaign.TypeCheats(chars)
			if err != nil {
				return err
			}
		}
		menuWasOpen := menu.page != menuClosed
		menuEffect := ""
		if focused && menu.frontend && !menuWasOpen && (rl.GetKeyPressed() != 0 || rl.IsMouseButtonPressed(rl.MouseButtonLeft)) {
			menu.open(menuMain)
		}
		captureWasWaiting := menu.capture || chatHandled
		if focused && !wipe.Active() && !captureWasWaiting && rl.IsKeyPressed(rl.KeyEscape) {
			if menuWasOpen {
				previousPage := menu.page
				menu.back()
				if previousPage != menuQuit || menu.page != menuQuit {
					menuEffect = "DSSWTCHX"
				}
			} else {
				menu.open(menuMain)
				menuEffect = "DSSWTCHN"
				game.ClearCheatInput()
			}
		}
		if focused && !wipe.Active() && !captureWasWaiting && menu.page != menuQuit && rl.IsKeyPressed(rl.KeyF10) && !rl.IsKeyDown(rl.KeyLeftShift) && !rl.IsKeyDown(rl.KeyRightShift) {
			menu.open(menuQuit)
			game.ClearCheatInput()
		}
		if focused && !wipe.Active() && !captureWasWaiting && rl.IsKeyPressed(rl.KeyF1) {
			menu.open(menuHelp)
			game.ClearCheatInput()
		}
		if focused && !wipe.Active() && !captureWasWaiting && rl.IsKeyPressed(rl.KeyF4) {
			menu.open(menuSound)
			game.ClearCheatInput()
		}
		if focused && !wipe.Active() && !captureWasWaiting && rl.IsKeyPressed(rl.KeyF2) {
			openSlots(menuSave)
			game.ClearCheatInput()
		}
		if focused && !wipe.Active() && !captureWasWaiting && rl.IsKeyPressed(rl.KeyF3) {
			openSlots(menuLoad)
			game.ClearCheatInput()
		}
		oldSettings, oldMode := settings, mode
		if pendingMusicConfig != nil {
			path := pendingMusicConfig.soundFont
			if music.BrowserSoundFontLoadPending(path) {
				menu.setStatus(nativeSoundFontDownloadStatus(path), 0)
			} else {
				if err := music.BrowserSoundFontLoadError(path); err != nil {
					menu.setStatus("SOUNDFONT DOWNLOAD FAILED", 70)
					game.Notify(err.Error())
				} else {
					settings.setMusicConfig(*pendingMusicConfig)
				}
				pendingMusicConfig = nil
			}
		}
		if focused && !wipe.Active() && captureWasWaiting {
			input := menuInput{mouseRow: -1, back: rl.IsKeyPressed(rl.KeyEscape)}
			if !input.back {
				for _, name := range nativeBindingNames {
					if nativeBindingNamePressed(name) {
						input.captured = name
						break
					}
				}
			}
			menu.update(input, &settings)
			releaseButtons = true
		}
		if focused && !wipe.Active() && !captureWasWaiting && menuWasOpen && menu.page != menuClosed && !rl.IsKeyPressed(rl.KeyEscape) {
			mouseRow := -1
			mouse := rl.GetMousePosition()
			if delta := rl.GetMouseDelta(); delta.X != 0 || delta.Y != 0 || rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				mouseRow = menu.rowAt(float64(mouse.X), float64(mouse.Y), rl.GetScreenWidth(), rl.GetScreenHeight())
			}
			mouseRowValid := mouseRow >= 0 && mouseRow < len(menu.rows(settings))
			input := menuInput{up: menuKey(rl.KeyUp) || rl.GetMouseWheelMove() > 0, down: menuKey(rl.KeyDown) || rl.GetMouseWheelMove() < 0, left: menuKey(rl.KeyLeft), right: menuKey(rl.KeyRight), confirm: rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) || (mouseRowValid && rl.IsMouseButtonPressed(rl.MouseButtonLeft)), back: menu.page != menuBindings && rl.IsKeyPressed(rl.KeyBackspace), mouseRow: mouseRow, mouseSlot: menu.bindingSlotAt(float64(mouse.X), rl.GetScreenWidth(), rl.GetScreenHeight()), clear: rl.IsKeyPressed(rl.KeyBackspace), defaults: rl.IsKeyPressed(rl.KeyF5), quitYes: rl.IsKeyPressed(rl.KeyY), quitNo: rl.IsKeyPressed(rl.KeyN) || rl.IsKeyPressed(rl.KeySpace), anyKey: len(chars) > 0 || rl.GetKeyPressed() != 0}
			changed := input.up || input.down || input.left || input.right || input.confirm || input.back
			previousPage := menu.page
			command := menu.update(input, &settings)
			if previousPage != menu.page && menu.page == menuMusicPlayer && musicPlayer != nil {
				menu.syncMusicSelection(musicPlayer.trackKey, musicPlayer.track)
			}
			if previousPage != menu.page && (menu.page == menuSave || menu.page == menuLoad) {
				menu.saveSlots = campaign.SaveSlots(menu.page == menuSave)
				savePreview.Close()
			}
			if previousPage == menuQuit {
				if menu.page != menuQuit {
					menuEffect = "DSSWTCHX"
				}
			} else if changed {
				menuEffect = "DSPSTOP"
				if input.confirm {
					menuEffect = "DSPISTOL"
				}
				if input.back {
					menuEffect = "DSSWTCHX"
				}
			}
			switch command {
			case menuPlayMusic:
				track, selected := menu.selectedMusicTrack(), menu.selectedMusicWAD()
				if musicPlayer == nil {
					menu.setStatus("AUDIO DEVICE UNAVAILABLE", 70)
				} else if track == nil || selected == nil || opts.MusicPlayerTrackLoader == nil {
					menu.setStatus("SONG NOT FOUND", 70)
				} else {
					data, err := opts.MusicPlayerTrackLoader(selected.Key, track.LumpName)
					if err == nil && len(data) > 0 {
						err = musicPlayer.PlayData(data, track.LumpName)
					} else if err == nil {
						err = fmt.Errorf("song not found")
					}
					if err != nil {
						menu.setStatus("MUSIC LOAD FAILED", 70)
					} else {
						musicPlayer.trackKey = selected.Key
						menu.setStatus("PLAYING: "+track.MusicName, 70)
					}
				}
			case menuStopMusic:
				musicPlayer.Stop()
				menu.setStatus("MUSIC STOPPED", 70)
			case menuConfirmQuit:
				audio.PlayQuit(game, len(menu.opts.Episodes) == 0, menu.quitSequence-1)
			case menuExit:
				quit = true
			case menuSaveGame:
				saveSlot(menu.saveSlot)
			case menuLoadGame:
				loadSlot(menu.saveSlot)
			case menuStart:
				if campaign.Watching() {
					menu.open(menuMain)
					game.Notify("WATCH MODE")
					break
				}
				fresh, err := mapdata.LoadMap(wf, menu.maps[menu.mapIndex])
				if err != nil {
					return err
				}
				paused = false
				if err := reload(fresh, menu.skill); err != nil {
					return err
				}
			}
		}
		if focused && !wipe.Active() && !captureWasWaiting && !menuWasOpen && menu.page == menuClosed && !menu.frontend {
			if playing && !chatHandled {
				if rl.IsKeyPressed(rl.KeyF5) {
					campaign.CycleDetail()
					settings.detailLevel, settings.autoDetail = campaign.DetailSettings()
				}
				if !automap && !cheatTyping {
					shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
					game.ApplyMapPresentation(doomruntime.NativeMapInput{
						ToggleRotate: rl.IsKeyPressed(rl.KeyR) && !shift,
						ToggleGrid:   rl.IsKeyPressed(rl.KeyG), ToggleReveal: rl.IsKeyPressed(rl.KeyO),
						CycleIDDT: rl.IsKeyPressed(rl.KeyI), CycleThings: rl.IsKeyPressed(rl.KeyT), ToggleLegend: rl.IsKeyPressed(rl.KeyV),
					})
				}
				if !automap {
					blocks, scale := 0, 0
					if rl.IsKeyPressed(rl.KeyEqual) || rl.IsKeyPressed(rl.KeyKpAdd) {
						blocks++
					}
					if rl.IsKeyPressed(rl.KeyMinus) || rl.IsKeyPressed(rl.KeyKpSubtract) {
						blocks--
					}
					if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
						if rl.IsKeyPressed(rl.KeyRightBracket) {
							scale++
						}
						if rl.IsKeyPressed(rl.KeyLeftBracket) {
							scale--
						}
					}
					if blocks != 0 || scale != 0 {
						game.SetMenuHUDSettings(settings.messages, settings.screenBlocks, settings.hudScale)
						settings.screenBlocks, settings.hudScale = game.AdjustMenuHUDSettings(blocks, scale)
					}
				}
				if rl.IsKeyPressed(rl.KeyComma) {
					campaign.SetSimulationSpeed(campaign.SimulationSpeed() - 0.1)
				}
				if rl.IsKeyPressed(rl.KeyPeriod) {
					campaign.SetSimulationSpeed(campaign.SimulationSpeed() + 0.1)
				}
				if rl.IsKeyPressed(rl.KeySlash) {
					campaign.SetSimulationSpeed(1)
				}
				if rl.IsKeyPressed(rl.KeyCapsLock) {
					settings.alwaysRun = !settings.alwaysRun
					message := "Always Run OFF"
					if settings.alwaysRun {
						message = "Always Run ON"
					}
					game.Notify(message)
				}
				if rl.IsKeyPressed(rl.KeyF12) {
					settings.autoWeaponSwitch = !settings.autoWeaponSwitch
					message := "Auto Weapon Switch OFF"
					if settings.autoWeaponSwitch {
						message = "Auto Weapon Switch ON"
					}
					game.Notify(message)
				}
				if rl.IsKeyPressed(rl.KeyBackSlash) {
					settings.mouseLook = !settings.mouseLook
					// Discard any motion accumulated before changing the baseline.
					yawPending = 0
					message := "Mouse Look OFF"
					if settings.mouseLook {
						message = "Mouse Look ON"
					}
					game.Notify(message)
				}
			}
			if rl.IsKeyPressed(rl.KeyF11) {
				game.CycleGammaLevel()
				settings.gammaLevel = game.GammaLevel()
			}
			if nativeBindingPressed(settings.bindings.Automap) && playing && !cheatTyping {
				automap = !automap
				pendingMap = doomruntime.NativeMapInput{}
				game.SetMapActive(automap)
			}
			if (rl.IsKeyPressed(rl.KeyP) && !cheatTyping && !nativeKeyBound(settings.bindings, "P")) || rl.IsKeyPressed(rl.KeyPause) {
				paused = !paused
			}
			if rl.IsKeyPressed(rl.KeyF10) && (rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)) {
				settings.debug = !settings.debug
			}
			if rl.IsKeyPressed(rl.KeyF7) {
				switch mode {
				case levelmesh.Textured:
					mode = levelmesh.Sectors
				case levelmesh.Sectors:
					mode = levelmesh.Wireframe
				default:
					mode = levelmesh.Textured
				}
			}
			if rl.IsKeyPressed(rl.KeyF8) {
				if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
					values := []raymesh.TextureFilter{raymesh.Nearest, raymesh.Trilinear, raymesh.Anisotropic}
					for i, v := range values {
						if v == settings.textureFilter {
							settings.textureFilter = values[(i+1)%len(values)]
							break
						}
					}
				} else {
					game.ToggleCRT()
					settings.crtEffect = game.CRTEnabled()
				}
			}
			if rl.IsKeyPressed(rl.KeyF6) && playing {
				saveSlot(0)
			}
			shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
			if rl.IsKeyPressed(rl.KeyF9) && !shift {
				loadSlot(0)
			}
			if rl.IsKeyPressed(rl.KeyF9) && shift {
				values := []raymesh.LightingMode{raymesh.DoomLighting, raymesh.SectorLighting, raymesh.FullbrightLighting}
				for i, v := range values {
					if v == settings.lighting {
						settings.lighting = values[(i+1)%len(values)]
						break
					}
				}
			}
			if !campaign.Watching() && playing && !campaign.DemoStatus().Active && ((rl.IsKeyPressed(rl.KeyR) && (rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)) && !cheatTyping && !automap && !nativeKeyBound(settings.bindings, "R")) || (game.IsDead() && (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter)))) {
				fresh, err := mapdata.LoadMap(wf, m.Name)
				if err != nil {
					return err
				}
				if err := campaign.Restart(fresh); err != nil {
					return err
				}
				audio.Stop()
				paused = false
				accumulator, yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, 0, false, false
				adoptCampaignGame(true)
			}
		}
		if fresh, levelSkill, ok := game.TakeNewGameRequest(); ok {
			if err := reload(fresh, levelSkill); err != nil {
				return err
			}
			game.Notify("IDCLEV " + string(m.Name))
		}
		campaign.SetControls(settings.alwaysRun, settings.mouseSensitivity)
		campaign.SetCameraSmoothing(settings.smoothCameraYaw)
		campaign.SetInputPreferences(settings.bindings, settings.mouseLook, settings.mouseInvert, settings.autoWeaponSwitch, settings.keyboardSpeed)
		campaign.SetGammaLevel(settings.gammaLevel)
		campaign.SetCRTEnabled(settings.crtEffect)
		if focused && menu.page == menuClosed && !captureWasWaiting && (rl.IsKeyDown(rl.KeyLeftAlt) || rl.IsKeyDown(rl.KeyRightAlt)) && rl.IsKeyPressed(rl.KeyEnter) {
			settings.fullscreen = !settings.fullscreen
		}
		if settings.textureFilter != oldSettings.textureFilter {
			if err := renderer.SetTextureFilter(settings.textureFilter); err != nil {
				return err
			}
			if err := presentation.Sprites.SetTextureFilter(settings.textureFilter); err != nil {
				return err
			}
		}
		if settings.fps != oldSettings.fps {
			rl.SetTargetFPS(int32(settings.fps))
		}
		if settings.fullscreen != oldSettings.fullscreen {
			rl.ToggleFullscreen()
		}
		if settings.musicConfig() != oldSettings.musicConfig() {
			requested := settings.musicConfig()
			pendingMusicConfig = nil
			if musicPlayer != nil && music.ResolveBackend(requested.backend) == music.BackendMeltySynth && music.StartBrowserSoundFontLoad(requested.soundFont) {
				pendingMusicConfig = &requested
				menu.setStatus(nativeSoundFontDownloadStatus(requested.soundFont), 0)
				settings.setMusicConfig(oldSettings.musicConfig())
			} else if err := musicPlayer.Configure(requested); err != nil {
				menu.setStatus("MUSIC CHANGE FAILED", 70)
				game.Notify(err.Error())
				settings.setMusicConfig(oldSettings.musicConfig())
			} else {
				menu.setStatus("", 0)
			}
		}
		if *frames == 0 && !settings.fullscreen {
			w, h := int(rl.GetScreenWidth()), int(rl.GetScreenHeight())
			if w != windowWidth || h != windowHeight {
				windowWidth, windowHeight = w, h
				resizeChanged = now
			}
			if !resizeChanged.IsZero() && now.Sub(resizeChanged) > time.Second/2 {
				persist()
				resizeChanged = time.Time{}
			}
		}
		if audio != nil {
			audio.volume = float32(settings.sfxVolume)
			if err := audio.ConfigureSpeaker(settings.pcSpeaker, settings.speakerVolume, settings.speakerVariant); err != nil {
				settings.pcSpeaker = oldSettings.pcSpeaker
				game.Notify(err.Error())
			}
			audio.Attach(game)
			if audio.speaker != nil {
				audio.SetPaused(paused || !focused)
			}
		}
		if settings != oldSettings || mode != oldMode {
			persist()
		}
		if musicPlayer != nil {
			volume := settings.musicPlaybackVolume()
			if music.ResolveBackend(settings.musicBackend) == music.BackendPCSpeaker {
				volume = settings.speakerVolume
			}
			musicPlayer.SetVolume(float32(volume))
			if settings.musicPlaybackVolume() > 0 && oldSettings.musicPlaybackVolume() == 0 && musicPlayer.parsed == nil {
				err := musicPlayer.PlayLump(campaign.MusicLump())
				if err != nil {
					game.Notify(err.Error())
				}
			}
		}
		menu.nowPlaying = ""
		if musicPlayer != nil && musicPlayer.parsed != nil {
			menu.nowPlaying = "SONG: " + launchcatalog.MusicTitleForLump(musicPlayer.track)
		}
		playing = campaign.Phase() == doomruntime.NativeCampaignPlaying
		blocked := paused || menu.page != menuClosed || !focused
		if blocked != blockedBefore {
			if blocked {
				audio.Stop()
				rl.EnableCursor()
			} else if *frames == 0 && !menu.frontend {
				rl.DisableCursor()
			}
			releaseButtons = true
		}
		musicPlayer.SetPaused(paused || !focused)
		if err := musicPlayer.Update(); err != nil {
			game.Notify(err.Error())
		}
		if menuEffect != "" {
			audio.Play(game, doomruntime.NativeSound{Name: menuEffect, Pitch: 1})
		}
		if menu.frontend && focused && !paused {
			for accumulator >= tic {
				if err := campaign.TickFrontend(); err != nil {
					return err
				}
				adoptCampaignGame(false)
				accumulator -= tic
			}
			yawPending, weapon, weaponCycle, use, fire, skipPending = 0, 0, 0, false, false, false
			pendingMap = doomruntime.NativeMapInput{}
		} else if blocked || blockedBefore {
			accumulator, yawPending, weapon, weaponCycle, use, fire = 0, 0, 0, 0, false, false
			pendingMap = doomruntime.NativeMapInput{}
			skipPending = false
		} else {
			if !playing {
				for key := rl.GetKeyPressed(); key != 0; key = rl.GetKeyPressed() {
					skipPending = true
				}
				skipPending = skipPending || rl.IsMouseButtonPressed(rl.MouseButtonLeft)
			}
			if automap && playing && !chatHandled {
				sw, sh := campaign.SceneSize(rl.GetScreenWidth(), rl.GetScreenHeight())
				game.MapViewport(sw, nativeViewHeight(sw, sh, game.HUDMode()))
				sampleNativeMapInput(&pendingMap, cheatTyping)
			}
			sampled := sampleNativeMovement(settings.bindings, settings.alwaysRun, automap, cheatTyping, nativeBindingNameHeld, nativeBindingNamePressed)
			if chatHandled || campaign.Watching() {
				sampled = doomruntime.NativeMeshInput{}
			}
			if !automap && !cheatTyping && !chatHandled && !campaign.Watching() {
				wheel := rl.GetMouseWheelMove()
				if wheel < 0 {
					sampled.WeaponCycle = 1
				}
				if wheel > 0 {
					sampled.WeaponCycle = -1
				}
			}
			useDown, fireDown := sampled.Use, sampled.Fire
			if releaseButtons && !useDown && !fireDown {
				releaseButtons = false
			}
			if !releaseButtons {
				use = use || useDown
				fire = fireDown
			}
			if sampled.WeaponSlot != 0 {
				weapon = sampled.WeaponSlot
			}
			if sampled.WeaponCycle != 0 {
				weaponCycle = sampled.WeaponCycle
			}
			if *frames == 0 && settings.mouseLook && !chatHandled && !campaign.Watching() {
				yawPending += game.MouseTurn(int(math.Round(float64(rl.GetMouseDelta().X))))
			}
		updateLoop:
			for accumulator >= tic {
				if wipe.Active() {
					if err := advanceNativeCampaign(campaign, wipe, doomruntime.NativeMeshInput{}, false); err != nil {
						return err
					}
					accumulator -= tic
					weapon, weaponCycle, yawPending, use, fire, skipPending = 0, 0, 0, false, false, false
					pendingMap = doomruntime.NativeMapInput{}
					continue
				}
				accumulator -= tic
				for step, steps := 0, campaign.ConsumeSimulationTicks(); step < steps; step++ {
					input := sampled
					input.Use, input.Fire, input.WeaponSlot, input.WeaponCycle, input.YawDelta = use, fire, weapon, weaponCycle, yawPending
					if automap {
						input.Map = &pendingMap
					}
					oldPhase := campaign.Phase()
					if err := advanceNativeCampaign(campaign, wipe, input, skipPending); err != nil {
						return err
					}
					skipPending = false
					if campaign.Phase() != oldPhase {
						audio.Stop()
						yawPending, weapon, weaponCycle = 0, 0, 0
					}
					adoptCampaignGame(false)
					if campaign.DemoStatus().Done {
						quit = true
						accumulator = 0
						break updateLoop
					}
					if wipe.Active() {
						accumulator = 0
						break updateLoop
					}
					if campaign.Phase() == doomruntime.NativeCampaignComplete {
						menu.frontend = true
						campaign.StartFrontend()
						menu.open(menuClosed)
						rl.EnableCursor()
						paused = false
						accumulator = 0
						break updateLoop
					}
					consumeNativeMapEdges(&pendingMap)
					weapon, weaponCycle, yawPending = 0, 0, 0
					use = useDown && !releaseButtons
					if campaign.Phase() != oldPhase {
						break
					}
				}
			}
		}
		if campaign.RecordingActive() && now.Sub(lastDemoFlush) >= 10*time.Second {
			if _, err := campaign.FlushDemoRecording(); err != nil {
				game.Notify(err.Error())
				fmt.Fprintln(os.Stderr, err)
			}
			lastDemoFlush = now
		}
		if !campaign.RecordingActive() && strings.TrimSpace(*recordDemoPath) != "" {
			if _, err := campaign.FlushDemoRecording(); err != nil {
				game.Notify(err.Error())
				fmt.Fprintln(os.Stderr, err)
			}
		}
		if lump := campaign.TakeMusicRequest(); lump != "" {
			pendingMusicLump = lump
		}
		if !wipe.Active() && pendingMusicLump != "" {
			musicPlayer.Stop()
			if musicPlayer != nil && settings.musicPlaybackVolume() > 0 {
				if err := musicPlayer.PlayLump(pendingMusicLump); err != nil {
					game.Notify(err.Error())
				}
			}
			pendingMusicLump = ""
		}
		blockedBefore = blocked
		lightingMode, textureOptions.Filter = settings.lighting, settings.textureFilter
		if err := renderer.SetLightingMode(lightingMode); err != nil {
			return err
		}
		alpha := accumulator / tic
		if *frames > 0 || blocked || thumbnailSlot >= 0 || wipe.Active() {
			alpha = 1
		} else { // Slow live play interpolates over the longer simulation step.
			alpha = campaign.SimulationRenderAlpha(alpha)
		}
		needScene := !wipe.Active() || !wipe.Ready() || wipe.NeedsResize()
		frontendPage := campaign.FrontendStatus().Page
		showScene := !menu.frontend || frontendPage == ""
		game.SetMenuHUDSettings(settings.messages, settings.screenBlocks, settings.hudScale)
		snapshot := game.Frame(alpha)
		audio.Update(game)
		if needScene && !automap && showScene && campaign.Phase() == doomruntime.NativeCampaignPlaying {
			renderer.Sync(snapshot.Triangles, game.Texture, game.Light, mode)
		}
		renderer.SetFullbright(snapshot.Fullbright)
		renderer.SetFixedColormap(snapshot.FixedColormap)
		renderer.SetGammaTable(game.GammaTable())
		w, h := rl.GetScreenWidth(), rl.GetScreenHeight()
		sceneW, sceneH := campaign.SceneSize(w, h)
		sceneViewH := nativeViewHeight(sceneW, sceneH, snapshot.HUDMode)
		viewH := nativeViewHeight(w, h, snapshot.HUDMode)
		presentation.SetSceneSize(sceneW, sceneH)
		if needScene && !automap && showScene && campaign.Phase() == doomruntime.NativeCampaignPlaying {
			presentation.SyncSpritesViewport(snapshot.Sprites, snapshot.Camera, sceneW, sceneViewH)
		}
		presentation.Sprites.SetFullbright(snapshot.Fullbright)
		presentation.Sprites.SetFixedColormap(snapshot.FixedColormap)
		presentation.Sprites.SetGammaTable(game.GammaTable())
		if err := presentation.Sprites.SetLightingMode(lightingMode); err != nil {
			return err
		}
		drawStarted := time.Now()
		rl.BeginDrawing()
		stats := renderer.Stats()
		if needScene {
			if menu.frontend && frontendPage != "" {
				rl.ClearBackground(rl.Black)
				title, ok := opts.MenuPatchBank[frontendPage]
				if !ok {
					title, ok = opts.IntermissionPatchBank[frontendPage]
				}
				if !ok {
					title, ok = opts.MenuPatchBank["TITLEPIC"]
				}
				if ok {
					presentation.DrawTitle(levelmesh.Texture{RGBA: title.RGBA, Width: title.Width, Height: title.Height}, w, h)
				}
			} else if campaign.Phase() != doomruntime.NativeCampaignPlaying {
				rl.ClearBackground(rl.Black)
				presentation.DrawUI(campaign.Patches(), w, h)
				if paused {
					presentation.DrawScreenPatches(campaign.PausePatches(w, h))
				}
			} else {
				rl.ClearBackground(rl.NewColor(42, 49, 65, 255))
				if automap {
					w, h, viewH = sceneW, sceneH, sceneViewH
				}
				if automap {
					automapRenderer.draw(game.MapFrame(int(w), int(viewH)), presentation)
				} else {
					presentation.DrawSky(snapshot.Sky, snapshot.Camera, sceneW, sceneViewH)
					rl.DrawRenderBatchActive()
					rl.Viewport(0, int32(h-sceneViewH), int32(sceneW), int32(sceneViewH))
					renderer.Draw(snapshot.Camera, sceneW, sceneViewH, mode)
					if err := presentation.DrawSprites(snapshot.Camera, sceneW, sceneViewH, func(index, width, height int) ([]levelmesh.FuzzSpan, levelmesh.FuzzColors) {
						return game.SpectreFuzz(snapshot.Sprites[index], snapshot.Camera, width, height), game.SpectreFuzzColors()
					}); err != nil {
						return err
					}
					rl.Viewport(0, 0, int32(w), int32(h))
					if game.CRTEnabled() {
						if err := crt.ApplyRegion(snapshot.WorldTic, sceneW, sceneH); err != nil {
							return err
						}
					}
					if err := detail.Present(sceneW, sceneH, w, h); err != nil {
						return err
					}
					presentation.DrawWeapon(snapshot.WeaponPatches, w, viewH)
				}
				presentation.DrawHUDLayout(snapshot.HUD, w, h, snapshot.HUDMode, snapshot.HUDScale)
				if automap {
					presentation.DrawScreenPatches(campaign.MessagePatches(w, h))
					presentation.DrawScreenPatches(campaign.ChatPatches(w, viewH))
				}
				death := campaign.DeathOverlay(w, h)
				raymesh.DrawScreenTint(death.Tint, w, h)
				presentation.DrawScreenPatches(death.Patches)
				raymesh.DrawScreenTint(snapshot.FlashOverlay, w, h)
				if !automap {
					presentation.DrawScreenPatches(campaign.MessagePatches(w, h))
					presentation.DrawScreenPatches(campaign.ChatPatches(w, viewH))
				}
				if settings.showFPS {
					presentation.DrawScreenPatches(campaign.PerfPatches(w, h))
				}
				if settings.debug {
					rl.DrawText(fmt.Sprintf("Raylib GPU | %s | %d tris | %d batches | F7 cycle", mode, stats.Triangles, stats.DrawCalls), 8, 8, 18, rl.White)
					rl.DrawText(fmt.Sprintf("Resident meshes: %d | uploads: %d | buffer updates: %d", stats.ResidentMeshes, stats.MeshUploads, stats.BufferUpdates), 8, 30, 16, rl.White)
					rl.DrawText(fmt.Sprintf("Textures: %dx | %s | Shift+F8 filter | F8 CRT", textureOptions.Scale, textureOptions.Filter), 8, 50, 16, rl.White)
					rl.DrawText(fmt.Sprintf("Lighting: %s | Shift+F9 cycle | AA: %s", lightingMode, aaLabel), 8, 70, 16, rl.White)
				}
				if paused {
					presentation.DrawScreenPatches(campaign.PausePatches(w, h))
				}
				if !automap {
					recording := campaign.RecordingOverlay(w, h)
					if recording.Radius > 0 {
						rl.DrawCircleV(rl.NewVector2(recording.X, recording.Y), recording.Radius, recording.Color)
					}
					presentation.DrawScreenPatches(recording.Patches)
				}
				if snapshot.Exited {
					rl.DrawText("LEVEL COMPLETE", int32(w/2-120), int32(h/2), 28, rl.White)
				}
				if automap && game.CRTEnabled() {
					if err := crt.ApplyRegion(snapshot.WorldTic, sceneW, sceneH); err != nil {
						return err
					}
				}
				if automap {
					w, h = rl.GetScreenWidth(), rl.GetScreenHeight()
					if err := detail.Present(sceneW, sceneH, w, h); err != nil {
						return err
					}
				}

			}
		}
		if wipe.Active() {
			if err := wipe.Prepare(); err != nil {
				return err
			}
			wipe.Draw(w, h)
		}
		if thumbnailSlot >= 0 {
			if err := saveNativeThumbnail(thumbnailSlot); err != nil {
				game.Notify("SAVE PREVIEW FAILED: " + err.Error())
				fmt.Fprintln(os.Stderr, err)
			} else {
				game.Notify("GAME SAVED")
			}
			thumbnailSlot = -1
		}
		if menu.page != menuClosed {
			rl.DrawRectangle(0, 0, int32(w), int32(h), rl.NewColor(0, 0, 0, 128))
			presentation.DrawUI(menu.draw(settings, menuTic), w, h)
			if (menu.page == menuSave || menu.page == menuLoad) && menu.row < len(menu.saveSlots) && menu.saveSlots[menu.row].Present {
				savePreview.Draw(menu.saveSlots[menu.row].Slot, w, h)
			}
		}
		if menu.frontend && menu.page == menuClosed {
			presentation.DrawUI(menu.drawBeginPrompt(), w, h)
		}
		if err := wipe.CaptureLastFrame(); err != nil {
			return err
		}
		var captureErr error
		if *frames > 0 && (frame+1 >= *frames || quit) && *capture != "" {
			captureErr = captureScreen(*capture)
		}
		renderDuration := time.Since(drawStarted)
		rl.EndDrawing()
		campaign.RecordRenderFrame(renderDuration)
		previousDetail, previousAuto := settings.detailLevel, settings.autoDetail
		settings.detailLevel, settings.autoDetail = campaign.DetailSettings()
		if settings.detailLevel != previousDetail || settings.autoDetail != previousAuto {
			persist()
		}
		if captureErr != nil {
			return captureErr
		}
		if *frames > 0 && (frame+1 >= *frames || quit) {
			fmt.Printf("raylib-mesh map=%s frames=%d world-tic=%d triangles=%d batches=%d resident=%d uploads=%d updates=%d texture-scale=%d texture-filter=%s lighting=%s msaa=%t crt=%t scene=%dx%d detail=%d auto-detail=%t\n", m.Name, frame+1, campaign.Game.Frame(1).WorldTic, stats.Triangles, stats.DrawCalls, stats.ResidentMeshes, stats.MeshUploads, stats.BufferUpdates, textureOptions.Scale, textureOptions.Filter, lightingMode, *msaa, game.CRTEnabled(), sceneW, sceneH, settings.detailLevel, settings.autoDetail)
			break
		}
	}
	if !resizeChanged.IsZero() {
		persist()
	}
	return nil
}

func axis(positive, negative int32) int {
	v := 0
	if rl.IsKeyDown(positive) {
		v++
	}
	if rl.IsKeyDown(negative) {
		v--
	}
	return v
}

func loadAssets(wf *wad.File) (doomruntime.Options, error) {
	var colorMap []byte
	if lump, ok := wf.LumpByName("COLORMAP"); ok {
		var err error
		colorMap, err = wf.LumpData(lump)
		if err != nil {
			return doomruntime.Options{}, err
		}
	}
	set, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		return doomruntime.Options{}, err
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		return doomruntime.Options{}, err
	}
	flatIndices, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		return doomruntime.Options{}, err
	}
	palette, err := doomtex.LoadPaletteRGBA(wf, 0)
	if err != nil {
		return doomruntime.Options{}, err
	}
	walls := make(map[string]doomruntime.WallTexture)
	for _, name := range set.TextureNames() {
		pixels, w, h, err := set.BuildTextureRGBA(name, 0)
		if err != nil {
			return doomruntime.Options{}, err
		}
		indexed, _, _, err := set.BuildTextureIndexed(name)
		if err != nil {
			return doomruntime.Options{}, err
		}
		walls[name] = doomruntime.WallTexture{RGBA: pixels, Indexed: indexed, Width: w, Height: h}
	}
	status, sprites := make(map[string]doomruntime.WallTexture), make(map[string]doomruntime.WallTexture)
	menus := make(map[string]doomruntime.WallTexture)
	intermission := make(map[string]doomruntime.WallTexture)
	font := make(map[rune]doomruntime.WallTexture)
	for _, lump := range wf.Lumps {
		isMenu := strings.HasPrefix(lump.Name, "M_") || lump.Name == "TITLEPIC"
		isIntermission := strings.HasPrefix(lump.Name, "WI") || strings.HasPrefix(lump.Name, "CWILV") || lump.Name == "INTERPIC" || lump.Name == "CREDIT" || lump.Name == "VICTORY2" || lump.Name == "ENDPIC" || lump.Name == "BOSSBACK" || lump.Name == "HELP" || lump.Name == "HELP1" || lump.Name == "HELP2"
		fontCode := 0
		if len(lump.Name) == 8 && strings.HasPrefix(lump.Name, "STCFN") {
			fontCode, _ = strconv.Atoi(lump.Name[5:])
		}
		isFont := fontCode >= 33 && fontCode <= 95
		isSprite := !isFont && (len(lump.Name) == 6 || len(lump.Name) == 8) && lump.Name[4] >= 'A' && lump.Name[4] <= 'Z' && lump.Name[5] >= '0' && lump.Name[5] <= '8'
		// STIMA0 is a stimpack sprite, despite sharing the HUD's ST prefix.
		isStatus := strings.HasPrefix(lump.Name, "ST") && !isSprite && !isFont
		if !isStatus && !isSprite && !isMenu && !isFont && !isIntermission {
			continue
		}
		rgba, w, h, ox, oy, err := set.BuildPatchRGBA(lump.Name, 0)
		if err != nil {
			continue
		}
		indexed, _, _, _, _, _, err := set.BuildPatchIndexedView(lump.Name)
		if err != nil {
			return doomruntime.Options{}, err
		}
		tex := doomruntime.WallTexture{RGBA: rgba, Indexed: indexed, Width: w, Height: h, OffsetX: ox, OffsetY: oy}
		if isFont {
			font[rune(fontCode)] = tex
		} else if isIntermission {
			intermission[lump.Name] = tex
		} else if isMenu {
			menus[lump.Name] = tex
		} else if isStatus {
			status[lump.Name] = tex
		} else {
			sprites[lump.Name] = tex
		}
	}
	return doomruntime.Options{SourcePortThingRenderMode: "sprites", SourcePortSectorLighting: true, MouseLookSpeed: 1, KeyboardTurnSpeed: 1, AutoWeaponSwitch: true, FlatBank: flats, FlatBankIndexed: flatIndices, WallTexBank: walls, StatusPatchBank: status, SpritePatchBank: sprites, MenuPatchBank: menus, IntermissionPatchBank: intermission, MessageFontBank: font, DoomPaletteRGBA: palette, DoomColorMap: colorMap, DoomColorMapRows: len(colorMap) / 256, WallTextureAnimSequences: doomtex.LoadWallTextureAnimSequences(set, doomtex.DoomWallAnimDefs), FlatTextureAnimSequences: doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)}, nil
}

func captureScreen(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	img, err := raymesh.CaptureWindow()
	if err != nil {
		return err
	}
	defer rl.UnloadImage(img)
	if !rl.ExportImage(*img, path) {
		return fmt.Errorf("could not save screenshot %q", path)
	}
	return nil
}

func menuKey(key int32) bool { return rl.IsKeyPressed(key) || rl.IsKeyPressedRepeat(key) }
