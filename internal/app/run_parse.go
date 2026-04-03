package app

import (
	"crypto/sha1"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"gddoom/internal/audiofx"
	"gddoom/internal/demo"
	"gddoom/internal/doomsession"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/music"
	"gddoom/internal/platformcfg"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/session"
	"gddoom/internal/sound"
	"gddoom/internal/wad"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func flagProvided(args []string, name string) bool {
	long := "-" + name
	prefix := long + "="
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == long || strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func explicitMapStartInMap(startInMap bool, mapExplicit bool) bool {
	return startInMap || mapExplicit
}

func shouldOpenIWADPicker(render, noExplicitWAD, forceWASMPicker bool, pickerChoiceCount int) bool {
	return render && pickerChoiceCount > 0 && (noExplicitWAD || forceWASMPicker)
}

func resolveForceWASMMode(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-wasm-mode" {
			return true
		}
		if !strings.HasPrefix(a, "-wasm-mode=") {
			continue
		}
		v, err := strconv.ParseBool(strings.TrimPrefix(a, "-wasm-mode="))
		if err != nil {
			return false
		}
		return v
	}
	return false
}

func RunParse(args []string, stdout io.Writer, stderr io.Writer) int {
	const maxCLIAppOPLVolume = 4.0
	normalizedArgs := args
	prevForceWASMMode := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(resolveForceWASMMode(normalizedArgs))
	defer platformcfg.SetForcedWASMMode(prevForceWASMMode)

	configPath, configExplicit := resolveConfigPath(normalizedArgs)
	cfg, err := loadConfig(configPath, configExplicit)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}

	defaultWAD := "DOOM1.WAD"
	defaultMap := ""
	defaultDetails := false
	defaultRender := true
	defaultDebug := false
	defaultDebugEvents := false
	defaultWidth, defaultHeight := doomsession.DefaultCLIWindowSize()
	defaultZoom := 0.0
	defaultDetailLevel := -1
	defaultAutoDetail := true
	defaultGammaLevel := -1
	defaultPlayer := 1
	defaultSkill := 3
	defaultGameMode := "single"
	defaultShowNoSkillItems := false
	defaultShowAllItems := false
	defaultMouseLook := true
	defaultMouseInvert := false
	defaultSmoothCameraYaw := true
	defaultMouseLookSpeed := 0.5
	defaultKeyboardTurnSpeed := 1.0
	defaultMusicVolume := 1.0
	defaultMUSPanMax := 0.8
	defaultOPLVolume := 2.25
	defaultAudioPreEmphasis := false
	defaultMusicBackend := music.DefaultBackend().String()
	defaultOPLBankPath := ""
	defaultSoundFontPath := ""
	if isWASMBuild() {
		defaultSoundFontPath = music.DefaultEmbeddedSoundFontPath()
	}
	defaultSFXVolume := 0.5
	defaultSFXPitchShift := false
	defaultFastMonsters := false
	defaultAlwaysRun := true
	defaultAutoWeaponSwitch := true
	defaultCheatLevel := 0
	defaultInvuln := false
	defaultSourcePortMode := false
	defaultSourcePortThingRenderMode := "sprites"
	defaultSourcePortThingBlendFrames := false
	defaultZombiemanThinkerBlend := true
	defaultDebugMonsterThinkerBlend := false
	defaultSourcePortSectorLighting := true
	defaultDoomLighting := true
	defaultKageShader := false
	defaultGPUSky := false
	defaultSkyUpscaleMode := "sharp"
	defaultCRTEffect := false
	defaultWallOcclusion := false
	defaultWallSpanReject := true
	defaultWallSpanClip := false
	defaultWallSliceOcclusion := true
	defaultBillboardClipping := true
	defaultRendererWorkers := 0
	defaultTextureAnimCrossfadeFrames := 7 // Max effective value is 7 (Doom texture animation cadence is 8 tics).
	defaultAllCheats := false
	defaultStartInMap := false
	defaultImportPCSpeaker := true
	defaultImportTextures := true
	defaultCPUProfile := ""
	defaultMemProfile := ""
	defaultExecTrace := ""
	defaultMemStats := false
	defaultDemo := ""
	defaultRecordDemo := ""
	defaultDemoExitOnDeath := false
	defaultDemoStopAfterTics := 0
	defaultNoVsync := false
	if isWASMBuild() {
		defaultNoVsync = true
	}
	defaultNoFPS := false
	defaultShowTPS := false
	defaultNoAspectCorrection := false
	defaultAniDump := ""
	defaultAniDumpDir := "anidump"
	defaultDumpMusic := false
	defaultDumpMusicDir := "out/music-dump"
	defaultConfigPath := configPath
	if cfg != nil {
		if cfg.Wad != nil {
			defaultWAD = *cfg.Wad
		}
		if cfg.Map != nil {
			defaultMap = *cfg.Map
		}
		if cfg.Details != nil {
			defaultDetails = *cfg.Details
		}
		if cfg.Render != nil {
			defaultRender = *cfg.Render
		}
		if cfg.Debug != nil {
			defaultDebug = *cfg.Debug
		}
		if cfg.DebugEvents != nil {
			defaultDebugEvents = *cfg.DebugEvents
		}
		if cfg.Width != nil {
			defaultWidth = *cfg.Width
		}
		if cfg.Height != nil {
			defaultHeight = *cfg.Height
		}
		if cfg.GammaLevel != nil {
			defaultGammaLevel = *cfg.GammaLevel
		}
		if cfg.Player != nil {
			defaultPlayer = *cfg.Player
		}
		if cfg.Skill != nil {
			defaultSkill = *cfg.Skill
		}
		if cfg.ShowNoSkillItems != nil {
			defaultShowNoSkillItems = *cfg.ShowNoSkillItems
		}
		if cfg.ShowAllItems != nil {
			defaultShowAllItems = *cfg.ShowAllItems
		}
		if cfg.MouseLook != nil {
			defaultMouseLook = *cfg.MouseLook
		}
		if cfg.MouseInvert != nil {
			defaultMouseInvert = *cfg.MouseInvert
		} else if cfg.MouseInvertHorizontal != nil {
			defaultMouseInvert = *cfg.MouseInvertHorizontal
		}
		if cfg.SmoothCameraYaw != nil {
			defaultSmoothCameraYaw = *cfg.SmoothCameraYaw
		}
		if cfg.MouseLookSpeed != nil {
			defaultMouseLookSpeed = *cfg.MouseLookSpeed
		}
		if cfg.KeyboardTurnSpeed != nil {
			defaultKeyboardTurnSpeed = *cfg.KeyboardTurnSpeed
		}
		if cfg.MusicVolume != nil {
			defaultMusicVolume = *cfg.MusicVolume
		}
		if cfg.MUSPanMax != nil {
			defaultMUSPanMax = *cfg.MUSPanMax
		}
		if cfg.MusicBackend != nil {
			defaultMusicBackend = *cfg.MusicBackend
		}
		if cfg.SoundFont != nil {
			defaultSoundFontPath = *cfg.SoundFont
		}
		if cfg.SFXVolume != nil {
			defaultSFXVolume = *cfg.SFXVolume
		}
		if cfg.AlwaysRun != nil {
			defaultAlwaysRun = *cfg.AlwaysRun
		}
		if cfg.AutoWeaponSwitch != nil {
			defaultAutoWeaponSwitch = *cfg.AutoWeaponSwitch
		}
		if cfg.CheatLevel != nil {
			defaultCheatLevel = *cfg.CheatLevel
		}
		if cfg.Invulnerable != nil {
			defaultInvuln = *cfg.Invulnerable
		}
		if cfg.SourcePortMode != nil {
			defaultSourcePortMode = *cfg.SourcePortMode
		}
		if cfg.GPUSky != nil {
			defaultGPUSky = *cfg.GPUSky
		}
		if cfg.SkyUpscaleMode != nil {
			defaultSkyUpscaleMode = *cfg.SkyUpscaleMode
		}
		if cfg.CRTEffect != nil {
			defaultCRTEffect = *cfg.CRTEffect
		}
		if cfg.RendererWorkers != nil {
			defaultRendererWorkers = *cfg.RendererWorkers
		}
		if cfg.TextureAnimCrossfadeFrames != nil {
			defaultTextureAnimCrossfadeFrames = *cfg.TextureAnimCrossfadeFrames
		}
		if cfg.AllCheats != nil {
			defaultAllCheats = *cfg.AllCheats
		}
		if cfg.CPUProfile != nil {
			defaultCPUProfile = *cfg.CPUProfile
		}
		if cfg.MemProfile != nil {
			defaultMemProfile = *cfg.MemProfile
		}
		if cfg.ExecTrace != nil {
			defaultExecTrace = *cfg.ExecTrace
		}
		if cfg.Demo != nil {
			defaultDemo = *cfg.Demo
		}
		if cfg.RecordDemo != nil {
			defaultRecordDemo = *cfg.RecordDemo
		}
		if cfg.DemoStopAfterTics != nil {
			defaultDemoStopAfterTics = *cfg.DemoStopAfterTics
		}
		if cfg.NoVsync != nil {
			defaultNoVsync = *cfg.NoVsync
		}
		if cfg.NoFPS != nil {
			defaultNoFPS = *cfg.NoFPS
		}
		if cfg.NoAspectCorrection != nil {
			defaultNoAspectCorrection = *cfg.NoAspectCorrection
		}
		defaultDetailLevel = configuredDetailLevelForMode(cfg, defaultSourcePortMode)
		if cfg.AutoDetail != nil {
			defaultAutoDetail = *cfg.AutoDetail
		}
	}
	if defaultSourcePortMode {
		if cfg == nil || cfg.GPUSky == nil {
			defaultGPUSky = false
		}
		if cfg == nil || cfg.SkyUpscaleMode == nil {
			defaultSkyUpscaleMode = "sharp"
		}
	}

	fs := flag.NewFlagSet("gddoom", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configFlag := fs.String("config", defaultConfigPath, "path to config toml file (default: config.toml)")
	wadPath := fs.String("wad", defaultWAD, "path to IWAD file")
	filePaths := fs.String("file", "", "comma-separated PWAD overlay paths")
	mapName := fs.String("map", defaultMap, "map name (E#M# or MAP##); empty selects first valid map")
	details := fs.Bool("details", defaultDetails, "print decoded gameplay-relevant map details")
	render := fs.Bool("render", defaultRender, "launch Ebiten automap renderer")
	debug := fs.Bool("debug", defaultDebug, "enable renderer debug HUD text/stats and debug logging")
	debugEvents := fs.Bool("debug-events", defaultDebugEvents, "enable gameplay event logs like teleports and level restarts")
	width := fs.Int("width", defaultWidth, "render window width")
	height := fs.Int("height", defaultHeight, "render window height")
	detailLevel := fs.Int("detail-level", defaultDetailLevel, "startup detail level (-1 keeps mode default)")
	autoDetail := fs.Bool("auto-detail", defaultAutoDetail, "detail level AUTO adjusts to target 60 FPS")
	gammaLevel := fs.Int("gamma-level", defaultGammaLevel, "startup gamma level (-1 keeps mode default)")
	playerSlot := fs.Int("player", defaultPlayer, "player start slot (1-4)")
	skillLevel := fs.Int("skill", defaultSkill, "doom skill level (1-5)")
	showNoSkillItems := fs.Bool("show-no-skill-items", defaultShowNoSkillItems, "show pickup items that have no skill bits set")
	showAllItems := fs.Bool("show-all-items", defaultShowAllItems, "show pickup items regardless of skill/game-mode spawn filters")
	mouseLook := fs.Bool("mouselook", defaultMouseLook, "enable mouse-based turning in walk mode")
	mouseInvert := fs.Bool("mouse-invert", defaultMouseInvert, "invert mouse movement")
	smoothCameraYaw := fs.Bool("smooth-camera-yaw", defaultSmoothCameraYaw, "smooth interpolated player camera yaw between sim ticks")
	mouseLookSpeed := fs.Float64("mouselook-speed", defaultMouseLookSpeed, "mouse turn speed multiplier (>0)")
	keyboardTurnSpeed := fs.Float64("keyboard-turn-speed", defaultKeyboardTurnSpeed, "keyboard turn speed multiplier (>0)")
	musicVolume := fs.Float64("music-volume", defaultMusicVolume, "music output volume (0..1)")
	musPanMax := fs.Float64("mus-pan-max", defaultMUSPanMax, "maximum MUS pan amount (0..1; 0 centers all pan, 1 keeps full range)")
	musicBackend := fs.String("music-backend", defaultMusicBackend, "music synth backend (auto|impsynth|meltysynth)")
	soundFont := fs.String("soundfont", defaultSoundFontPath, "path to external SoundFont (.sf2) used by the meltysynth music backend")
	sfxVolume := fs.Float64("sfx-volume", defaultSFXVolume, "sound-effect output volume (0..1)")
	alwaysRun := fs.Bool("always-run", defaultAlwaysRun, "start with always-run enabled (Shift inverts while held)")
	autoWeaponSwitch := fs.Bool("auto-weapon-switch", defaultAutoWeaponSwitch, "auto-switch to newly picked weapons")
	cheatLevel := fs.Int("cheat-level", defaultCheatLevel, "startup cheats (0=off, 1=automap, 2=idfa-like, 3=idkfa+invuln)")
	invuln := fs.Bool("invuln", defaultInvuln, "start with invulnerability (iddqd-like)")
	sourcePortMode := fs.Bool("sourceport-mode", defaultSourcePortMode, "enable source-port style heading-follow rotation defaults")
	debugMonsterThinkerBlend := fs.Bool("debug-monster-thinker-blend", defaultDebugMonsterThinkerBlend, "overlay raw thinker-position monster sprites in bright red")
	gpuSky := fs.Bool("gpu-sky", defaultGPUSky, "enable experimental GPU sky path in sourceport mode (default off)")
	skyUpscale := fs.String("sky-upscale", defaultSkyUpscaleMode, "GPU sky upscale mode (nearest|sharp)")
	crtEffect := fs.Bool("crt-effect", defaultCRTEffect, "enable CRT postprocess effect")
	rendererWorkers := fs.Int("renderer-workers", defaultRendererWorkers, "renderer worker count (0 uses built-in default policy)")
	textureAnimCrossfadeFrames := fs.Int("texture-anim-crossfade-frames", defaultTextureAnimCrossfadeFrames, "sourceport texture animation crossfade frames (0 disables)")
	allCheats := fs.Bool("all-cheats", defaultAllCheats, "legacy alias for startup full cheats (equivalent to -cheat-level=3 -invuln=true)")
	cpuProfile := fs.String("cpuprofile", defaultCPUProfile, "write Go CPU profile to file")
	memProfile := fs.String("memprofile", defaultMemProfile, "write Go heap profile to file on exit")
	execTrace := fs.String("exectrace", defaultExecTrace, "write Go execution trace to file")
	memStats := fs.Bool("memstats", defaultMemStats, "log Go runtime memory stats at startup and exit")
	demoPath := fs.String("demo", defaultDemo, "path to Doom v1.10 .lmp demo; runs demo benchmark and exits when demo ends")
	recordDemoPath := fs.String("record-demo", defaultRecordDemo, "path to write Doom v1.10 .lmp demo recorded from live input")
	demoExitOnDeath := fs.Bool("demo-exit-on-death", defaultDemoExitOnDeath, "during -demo playback, stop early when the player dies")
	demoStopAfterTics := fs.Int("demo-stop-after-tics", defaultDemoStopAfterTics, "during -demo playback, stop after this many processed tics (0 disables)")
	demoTracePath := fs.String("trace-demo-state", "", "write per-tic GD-DOOM demo state JSONL for -demo playback")
	noVsync := fs.Bool("no-vsync", defaultNoVsync, "disable vsync and uncap draw FPS")
	noFPS := fs.Bool("nofps", defaultNoFPS, "hide FPS/MS overlay")
	noAspectCorrection := fs.Bool("no-aspect-correction", defaultNoAspectCorrection, "disable Doom-style 4:3 aspect correction")
	aniDump := fs.String("anidump", defaultAniDump, "dump animation sprite series for seed (example: SMGTA0)")
	dumpMusic := fs.Bool("dump-music", defaultDumpMusic, "render music WAV exports for detected IWADs or the selected -wad")
	dumpMusicDir := fs.String("dump-music-dir", defaultDumpMusicDir, "output directory for -dump-music WAV exports")
	forceWASMMode := fs.Bool("wasm-mode", platformcfg.ForcedWASMMode(), "force js/wasm runtime behavior on native builds")

	if err := fs.Parse(normalizedArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "flag error: %v\n", err)
		return 2
	}
	mapExplicit := flagProvided(normalizedArgs, "map") && strings.TrimSpace(*mapName) != ""
	skyUpscaleFlagSet := flagProvided(normalizedArgs, "sky-upscale")
	wadFlagSet := flagProvided(normalizedArgs, "wad")
	if *sourcePortMode {
		if !skyUpscaleFlagSet && (cfg == nil || cfg.SkyUpscaleMode == nil) {
			*skyUpscale = "sharp"
		}
	}
	_ = configFlag
	platformcfg.SetForcedWASMMode(*forceWASMMode)
	allCheatsSet := false
	cheatLevelSet := false
	invulnSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "all-cheats" {
			allCheatsSet = true
		}
		if f.Name == "cheat-level" {
			cheatLevelSet = true
		}
		if f.Name == "invuln" {
			invulnSet = true
		}
	})
	detailLevelSet := flagProvided(normalizedArgs, "detail-level")
	if !detailLevelSet {
		*detailLevel = configuredDetailLevelForMode(cfg, *sourcePortMode)
	}
	resolvedCheatLevel := *cheatLevel
	resolvedInvuln := *invuln
	resolvedGameMode := strings.ToLower(strings.TrimSpace(defaultGameMode))
	switch resolvedGameMode {
	case "single", "coop", "deathmatch":
	default:
		resolvedGameMode = "single"
	}
	if *keyboardTurnSpeed <= 0 {
		fmt.Fprintf(stderr, "invalid -keyboard-turn-speed %.3f (must be > 0)\n", *keyboardTurnSpeed)
		return 2
	}
	if *mouseLookSpeed <= 0 {
		fmt.Fprintf(stderr, "invalid -mouselook-speed %.3f (must be > 0)\n", *mouseLookSpeed)
		return 2
	}
	if *musicVolume < 0 || *musicVolume > 1 {
		fmt.Fprintf(stderr, "invalid -music-volume %.3f (must be between 0 and 1)\n", *musicVolume)
		return 2
	}
	if *musPanMax < 0 || *musPanMax > 1 {
		fmt.Fprintf(stderr, "invalid -mus-pan-max %.3f (must be between 0 and 1)\n", *musPanMax)
		return 2
	}
	if *sfxVolume < 0 || *sfxVolume > 1 {
		fmt.Fprintf(stderr, "invalid -sfx-volume %.3f (must be between 0 and 1)\n", *sfxVolume)
		return 2
	}
	if *rendererWorkers < 0 {
		fmt.Fprintf(stderr, "invalid -renderer-workers %d (must be >= 0)\n", *rendererWorkers)
		return 2
	}
	resolvedMusicBackendInput := strings.TrimSpace(*musicBackend)
	resolvedMusicBackend, err := music.ParseBackend(resolvedMusicBackendInput)
	if err != nil {
		fmt.Fprintf(stderr, "invalid music backend %q: %v\n", resolvedMusicBackendInput, err)
		return 2
	}
	if err := music.ValidateBackend(resolvedMusicBackend); err != nil {
		fmt.Fprintf(stderr, "invalid music backend %q: %v\n", resolvedMusicBackendInput, err)
		return 2
	}
	if music.ResolveBackend(resolvedMusicBackend) == music.BackendMeltySynth && strings.TrimSpace(*soundFont) == "" {
		fmt.Fprintln(stderr, "invalid -soundfont \"\": meltysynth backend requires a SoundFont (.sf2)")
		return 2
	}
	if *allCheats && allCheatsSet && !cheatLevelSet {
		resolvedCheatLevel = 3
		if !invulnSet {
			resolvedInvuln = true
		}
	}
	resolvedDemoPath := strings.TrimSpace(*demoPath)
	resolvedRecordDemoPath := strings.TrimSpace(*recordDemoPath)
	resolvedDemoTracePath := strings.TrimSpace(*demoTracePath)
	resolvedFilePaths := resolveWADOverlayPaths(*filePaths)
	resolvedWADPath := resolveIWADAliasPath(*wadPath)
	if resolvedDemoPath != "" && resolvedRecordDemoPath != "" {
		fmt.Fprintln(stderr, "-demo and -record-demo are mutually exclusive")
		return 2
	}
	if resolvedDemoTracePath != "" && resolvedDemoPath == "" {
		fmt.Fprintln(stderr, "-trace-demo-state requires -demo")
		return 2
	}
	if *dumpMusic {
		if err := dumpMusicWAVs(strings.TrimSpace(*dumpMusicDir), resolvedWADPath, wadFlagSet, resolvedFilePaths, strings.TrimSpace(*soundFont), stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "dump music: %v\n", err)
			return 1
		}
		return 0
	}
	writeMemProfile := func() {}
	if strings.TrimSpace(*memProfile) != "" {
		path := strings.TrimSpace(*memProfile)
		writeMemProfile = func() {
			runtime.GC()
			f, perr := os.Create(path)
			if perr != nil {
				fmt.Fprintf(stderr, "open mem profile: %v\n", perr)
				return
			}
			defer f.Close()
			if perr := pprof.WriteHeapProfile(f); perr != nil {
				fmt.Fprintf(stderr, "write mem profile: %v\n", perr)
				return
			}
			fmt.Fprintf(stderr, "mem profile written to %s\n", path)
		}
	}
	defer writeMemProfile()
	stopExecTrace := func() {}
	if strings.TrimSpace(*execTrace) != "" {
		path := strings.TrimSpace(*execTrace)
		f, terr := os.Create(path)
		if terr != nil {
			fmt.Fprintf(stderr, "open exec trace: %v\n", terr)
			return 1
		}
		if terr := trace.Start(f); terr != nil {
			f.Close()
			fmt.Fprintf(stderr, "start exec trace: %v\n", terr)
			return 1
		}
		fmt.Fprintf(stderr, "exec trace recording to %s\n", path)
		stopExecTrace = func() {
			trace.Stop()
			if cerr := f.Close(); cerr != nil {
				fmt.Fprintf(stderr, "close exec trace: %v\n", cerr)
			}
		}
	}
	defer stopExecTrace()
	writeMemStats := func(stage string) {}
	if *memStats {
		writeMemStats = func(stage string) {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			fmt.Fprintf(stderr,
				"memstats[%s]: alloc=%d heap_alloc=%d heap_inuse=%d heap_objects=%d num_gc=%d pause_total_ns=%d\n",
				stage, stats.Alloc, stats.HeapAlloc, stats.HeapInuse, stats.HeapObjects, stats.NumGC, stats.PauseTotalNs,
			)
		}
		writeMemStats("start")
		defer writeMemStats("end")
	}
	stopCPUProfile := func() {}
	if strings.TrimSpace(*cpuProfile) != "" {
		f, perr := os.Create(strings.TrimSpace(*cpuProfile))
		if perr != nil {
			fmt.Fprintf(stderr, "open cpu profile: %v\n", perr)
			return 1
		}
		if perr := pprof.StartCPUProfile(f); perr != nil {
			_ = f.Close()
			fmt.Fprintf(stderr, "start cpu profile: %v\n", perr)
			return 1
		}
		fmt.Fprintf(stderr, "cpu profile recording to %s\n", strings.TrimSpace(*cpuProfile))
		stopCPUProfile = func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}
	}
	defer stopCPUProfile()
	noExplicitWAD := !wadFlagSet && (cfg == nil || cfg.Wad == nil || strings.TrimSpace(*cfg.Wad) == "")
	forceWASMPicker := isWASMBuild() && *render
	choices := detectAvailableIWADChoices(".")
	pickerChoices := choices
	if len(pickerChoices) == 0 && isWASMBuild() {
		if fallback, ok := knownIWADChoiceForPath(defaultWAD); ok {
			pickerChoices = []iwadChoice{fallback}
		}
	}
	if shouldOpenIWADPicker(*render, noExplicitWAD, forceWASMPicker, len(pickerChoices)) {
		buildCfg := renderBuildConfig{
			selectedMap:                strings.ToUpper(strings.TrimSpace(*mapName)),
			mapExplicit:                mapExplicit,
			width:                      *width,
			height:                     *height,
			zoom:                       defaultZoom,
			detailLevel:                *detailLevel,
			detailLevelExplicit:        detailLevelSet,
			autoDetail:                 *autoDetail,
			detailLevelFaithful:        configuredDetailLevelForMode(cfg, false),
			detailLevelSourcePort:      configuredDetailLevelForMode(cfg, true),
			gammaLevel:                 *gammaLevel,
			debug:                      *debug,
			debugEvents:                *debugEvents,
			playerSlot:                 *playerSlot,
			skillLevel:                 *skillLevel,
			gameMode:                   resolvedGameMode,
			showNoSkillItems:           *showNoSkillItems,
			showAllItems:               *showAllItems,
			mouseLook:                  *mouseLook,
			mouseLookSpeed:             *mouseLookSpeed,
			keyboardTurnSpeed:          *keyboardTurnSpeed,
			musicVolume:                *musicVolume,
			musPanMax:                  *musPanMax,
			oplVolume:                  defaultOPLVolume,
			audioPreEmphasis:           defaultAudioPreEmphasis,
			musicBackend:               resolvedMusicBackend,
			oplBankPath:                defaultOPLBankPath,
			soundFontPath:              strings.TrimSpace(*soundFont),
			sfxVolume:                  *sfxVolume,
			sfxPitchShift:              defaultSFXPitchShift,
			fastMonsters:               defaultFastMonsters,
			alwaysRun:                  *alwaysRun,
			autoWeaponSwitch:           *autoWeaponSwitch,
			cheatLevel:                 resolvedCheatLevel,
			invuln:                     resolvedInvuln,
			sourcePortMode:             *sourcePortMode,
			sourcePortThingRenderMode:  defaultSourcePortThingRenderMode,
			sourcePortThingBlendFrames: defaultSourcePortThingBlendFrames,
			sourcePortSectorLighting:   defaultSourcePortSectorLighting,
			doomLighting:               defaultDoomLighting,
			kageShader:                 defaultKageShader,
			gpuSky:                     *gpuSky,
			skyUpscaleMode:             *skyUpscale,
			crtEffect:                  *crtEffect,
			wallOcclusion:              defaultWallOcclusion,
			wallSpanReject:             defaultWallSpanReject,
			wallSpanClip:               defaultWallSpanClip,
			wallSliceOcclusion:         defaultWallSliceOcclusion,
			billboardClipping:          defaultBillboardClipping,
			rendererWorkers:            *rendererWorkers,
			textureAnimCrossfadeFrames: *textureAnimCrossfadeFrames,
			noVsync:                    *noVsync,
			noFPS:                      *noFPS,
			noAspectCorrection:         *noAspectCorrection,
			allCheats:                  *allCheats,
			startInMap:                 defaultStartInMap,
			importPCSpeaker:            defaultImportPCSpeaker,
			importTextures:             defaultImportTextures,
			demoPath:                   resolvedDemoPath,
			recordDemoPath:             resolvedRecordDemoPath,
			demoExitOnDeath:            *demoExitOnDeath,
			demoStopAfterTics:          max(0, *demoStopAfterTics),
			demoTracePath:              resolvedDemoTracePath,
			pwadPaths:                  resolvedFilePaths,
			configPath:                 configPath,
		}
		picker, perr := newIWADPickerGame(pickerChoices, resolvedMusicBackend, func(path string, profile pickerProfile, synthIndex int) (*renderBundle, error) {
			cfg := applyPickerProfile(buildCfg, profile)
			cfg = applyPickerSynth(cfg, synthIndex)
			return buildRenderBundle(resolveIWADAliasPath(path), cfg, stderr)
		})
		if perr != nil {
			fmt.Fprintf(stderr, "iwad picker: %v\n", perr)
			return 1
		}
		if forceWASMPicker {
			picker.stage = pickerStageIWAD
		}
		if err := runGameWithPlatformOptions(picker); err != nil && !errors.Is(err, ebiten.Termination) {
			fmt.Fprintf(stderr, "iwad picker: %v\n", err)
			return 1
		}
		defer picker.Close()
		if picker.Session() == nil {
			return 1
		}
		if p := picker.Session().Options().RecordDemoPath; p != "" {
			rec := picker.Session().EffectiveDemoRecord()
			opts := picker.Session().Options()
			skill := opts.SkillLevel - 1
			if skill < 0 {
				skill = 0
			}
			demoRec, derr := demo.BuildRecorded(picker.Session().StartMapName(), demo.RecordingOptions{
				Skill:           skill,
				Deathmatch:      strings.EqualFold(opts.GameMode, "deathmatch"),
				FastMonsters:    opts.FastMonsters,
				RespawnMonsters: opts.RespawnMonsters,
				NoMonsters:      opts.NoMonsters,
			}, rec)
			if derr != nil {
				fmt.Fprintf(stderr, "build demo recording: %v\n", derr)
				return 1
			}
			if werr := demo.Save(p, demoRec); werr != nil {
				fmt.Fprintf(stderr, "write demo recording: %v\n", werr)
				return 1
			}
			fmt.Fprintf(stdout, "demo-recorded path=%s tics=%d\n", p, len(rec))
		}
		if err := picker.Session().Err(); err != nil {
			fmt.Fprintf(stderr, "render: %v\n", err)
			return 1
		}
		return 0
	}
	if noExplicitWAD && len(choices) > 0 {
		*wadPath = choices[0].Path
	}
	if strings.TrimSpace(*wadPath) == "" {
		fmt.Fprintln(stderr, "-wad is required")
		return 2
	}
	wf, wadPaths, err := openWADStack(resolvedWADPath, resolvedFilePaths)
	if err != nil {
		fmt.Fprintf(stderr, "open wad: %v\n", err)
		return 1
	}
	wadHash := hashWADStackSHA1(wadPaths)
	soundBank := media.SoundBank{}
	dsr := sound.ImportDigitalSounds(wf)
	musicSoundFontChoices := detectAvailableSoundFonts("soundfonts")
	musicPatchBank, err := resolveMusicPatchBank(wf, defaultOPLBankPath, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "music patch import error: %v\n", err)
		return 1
	}
	musicSoundFont, err := resolveMusicSoundFont(resolvedMusicBackend, strings.TrimSpace(*soundFont), stderr)
	if err != nil {
		fmt.Fprintf(stderr, "music soundfont import error: %v\n", err)
		return 1
	}
	if defaultImportPCSpeaker {
		dpr := sound.ImportPCSpeakerSounds(wf)
		fmt.Fprintf(stderr, "sound import: dp(found=%d decoded=%d failed=%d) ds(found=%d decoded=%d failed=%d)\n",
			dpr.Found, dpr.Decoded, dpr.Failed,
			dsr.Found, dsr.Decoded, dsr.Failed,
		)
	} else {
		fmt.Fprintf(stderr, "sound import: ds(found=%d decoded=%d failed=%d)\n",
			dsr.Found, dsr.Decoded, dsr.Failed,
		)
	}
	soundBank = buildAutomapSoundBank(dsr, *sourcePortMode)
	wallTexBank := map[string]media.WallTexture(nil)
	bootSplash := media.WallTexture{}
	doomPaletteRGBA := []byte(nil)
	doomColorMap := []byte(nil)
	doomColorMapRows := 0
	menuPatchBank := map[string]media.WallTexture(nil)
	statusPatchBank := map[string]media.WallTexture(nil)
	messageFontBank := map[rune]media.WallTexture(nil)
	spritePatchBank := map[string]media.WallTexture(nil)
	intermissionPatchBank := map[string]media.WallTexture(nil)
	var texSet *doomtex.Set
	if pal, perr := doomtex.LoadPaletteRGBA(wf, 0); perr != nil {
		fmt.Fprintf(stderr, "palette import failed: %v\n", perr)
	} else {
		doomPaletteRGBA = pal
	}
	if cmLump, ok := wf.LumpByName("COLORMAP"); ok {
		if cmData, err := wf.LumpDataView(cmLump); err == nil && len(cmData) >= 256 {
			doomColorMapRows = len(cmData) / 256
			doomColorMap = cmData[:doomColorMapRows*256]
		}
	}
	if defaultImportTextures {
		ts, terr := doomtex.LoadFromWAD(wf)
		if terr != nil {
			fmt.Fprintf(stderr, "texture import failed: %v\n", terr)
		} else {
			texSet = ts
			fmt.Fprintf(stderr, "texture import: palettes=%d textures=%d\n", ts.PaletteCount(), ts.TextureCount())
			names := ts.TextureNames()
			wallTexBank = make(map[string]media.WallTexture, len(names))
			built := 0
			failed := 0
			for _, name := range names {
				indexed, iw, ih, ierr := ts.BuildTextureIndexed(name)
				rgba, w, h, berr := ts.BuildTextureRGBA(name, 0)
				if berr != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
					failed++
					continue
				}
				rgba32 := []uint32(nil)
				if len(rgba) >= 4 {
					rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
				}
				colMajor := []uint32(nil)
				if len(rgba32) == w*h {
					colMajor = make([]uint32, len(rgba32))
					for tx := 0; tx < w; tx++ {
						colBase := tx * h
						for ty := 0; ty < h; ty++ {
							colMajor[colBase+ty] = rgba32[ty*w+tx]
						}
					}
				}
				indexedColMajor := []byte(nil)
				if ierr == nil && iw == w && ih == h && len(indexed) == w*h {
					indexedColMajor = make([]byte, len(indexed))
					for tx := 0; tx < w; tx++ {
						colBase := tx * h
						for ty := 0; ty < h; ty++ {
							indexedColMajor[colBase+ty] = indexed[ty*w+tx]
						}
					}
				}
				wallTexBank[name] = media.WallTexture{
					RGBA:            rgba,
					RGBA32:          rgba32,
					ColMajor:        colMajor,
					Indexed:         indexed,
					IndexedColMajor: indexedColMajor,
					Width:           w,
					Height:          h,
				}
				built++
			}
			fmt.Fprintf(stderr, "wall texture build: built=%d failed=%d\n", built, failed)
		}
	}
	if texSet != nil {
		bootSplash = buildBootSplashTexture(texSet)
		if bootSplash.Width > 0 && bootSplash.Height > 0 && len(bootSplash.RGBA) == bootSplash.Width*bootSplash.Height*4 {
			fmt.Fprintf(stderr, "boot splash import: %dx%d\n", bootSplash.Width, bootSplash.Height)
		}
		statusPatchBank = buildStatusPatchBank(texSet)
		if len(statusPatchBank) > 0 {
			fmt.Fprintf(stderr, "status patch import: patches=%d\n", len(statusPatchBank))
		}
		menuPatchBank = buildMenuPatchBank(texSet)
		if len(menuPatchBank) > 0 {
			fmt.Fprintf(stderr, "menu patch import: patches=%d\n", len(menuPatchBank))
		}
		messageFontBank = buildMessageFontBank(texSet)
		if len(messageFontBank) > 0 {
			fmt.Fprintf(stderr, "message font import: glyphs=%d\n", len(messageFontBank))
		}
		spritePatchBank = buildMonsterSpriteBank(texSet)
		if len(spritePatchBank) > 0 {
			fmt.Fprintf(stderr, "monster sprite import: patches=%d\n", len(spritePatchBank))
		}
		if strings.TrimSpace(*aniDump) != "" {
			if derr := dumpSpriteAnimationSeries(defaultAniDumpDir, *aniDump, spritePatchBank); derr != nil {
				fmt.Fprintf(stderr, "anidump error: %v\n", derr)
				return 1
			}
			fmt.Fprintf(stderr, "anidump: seed=%s dir=%s\n", strings.ToUpper(strings.TrimSpace(*aniDump)), defaultAniDumpDir)
		}
		intermissionPatchBank = buildIntermissionPatchBank(texSet)
		if len(intermissionPatchBank) > 0 {
			fmt.Fprintf(stderr, "intermission patch import: patches=%d\n", len(intermissionPatchBank))
		}
	}
	flatBank := map[string][]byte(nil)
	flatBankIndexed := map[string][]byte(nil)
	wallTextureAnimSequences := map[string][]string(nil)
	flatTextureAnimSequences := map[string][]string(nil)
	fb, ferr := doomtex.LoadFlatsRGBA(wf, 0)
	if ferr != nil {
		fmt.Fprintf(stderr, "flat import failed: %v\n", ferr)
	} else {
		flatBank = fb
		fmt.Fprintf(stderr, "flat import: flats=%d\n", len(flatBank))
	}
	fbi, ferr := doomtex.LoadFlatsIndexed(wf)
	if ferr == nil {
		flatBankIndexed = fbi
	}
	if texSet != nil {
		wallTextureAnimSequences = doomtex.LoadWallTextureAnimSequences(texSet, doomtex.DoomWallAnimDefs)
	}
	flatTextureAnimSequences = doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)

	selected := mapdata.MapName(strings.ToUpper(strings.TrimSpace(*mapName)))

	if *render {
		opts := doomsession.Options{
			Width:                      *width,
			Height:                     *height,
			StartZoom:                  defaultZoom,
			InitialDetailLevel:         *detailLevel,
			AutoDetail:                 *autoDetail,
			InitialGammaLevel:          *gammaLevel,
			WADHash:                    wadHash,
			Debug:                      *debug,
			DebugEvents:                *debugEvents,
			PlayerSlot:                 *playerSlot,
			SkillLevel:                 *skillLevel,
			GameMode:                   resolvedGameMode,
			ShowNoSkillItems:           *showNoSkillItems,
			ShowAllItems:               *showAllItems,
			MouseLook:                  *mouseLook,
			MouseInvert:                *mouseInvert,
			SmoothCameraYaw:            *smoothCameraYaw,
			MouseLookSpeed:             *mouseLookSpeed,
			KeyboardTurnSpeed:          *keyboardTurnSpeed,
			MusicVolume:                *musicVolume,
			MUSPanMax:                  *musPanMax,
			OPLVolume:                  defaultOPLVolume,
			AudioPreEmphasis:           defaultAudioPreEmphasis,
			MusicBackend:               resolvedMusicBackend,
			OpenMenuOnFrontendStart:    openMenuOnFrontendStart(),
			SFXVolume:                  *sfxVolume,
			SFXPitchShift:              defaultSFXPitchShift,
			FastMonsters:               defaultFastMonsters,
			AlwaysRun:                  *alwaysRun,
			AutoWeaponSwitch:           *autoWeaponSwitch,
			CheatLevel:                 resolvedCheatLevel,
			Invulnerable:               resolvedInvuln,
			SourcePortMode:             *sourcePortMode,
			SourcePortThingRenderMode:  defaultSourcePortThingRenderMode,
			SourcePortThingBlendFrames: defaultSourcePortThingBlendFrames,
			ZombiemanThinkerBlend:      defaultZombiemanThinkerBlend,
			DebugMonsterThinkerBlend:   *debugMonsterThinkerBlend,
			SourcePortSectorLighting:   defaultSourcePortSectorLighting,
			DisableDoomLighting:        !defaultDoomLighting,
			KageShader:                 defaultKageShader,
			GPUSky:                     *gpuSky,
			SkyUpscaleMode:             *skyUpscale,
			CRTEffect:                  *crtEffect,
			DisableWallOcclusion:       !defaultWallOcclusion,
			DisableWallSpanReject:      !defaultWallSpanReject,
			DisableWallSpanClip:        !defaultWallSpanClip,
			DisableWallSliceOcclusion:  !defaultWallSliceOcclusion,
			DisableBillboardClipping:   !defaultBillboardClipping,
			RendererWorkers:            *rendererWorkers,
			TextureAnimCrossfadeFrames: *textureAnimCrossfadeFrames,
			NoVsync:                    *noVsync,
			NoFPS:                      *noFPS,
			ShowTPS:                    defaultShowTPS,
			DisableAspectCorrection:    *noAspectCorrection,
			AllCheats:                  *allCheats,
			StartInMapMode:             explicitMapStartInMap(defaultStartInMap, mapExplicit),
			FlatBank:                   flatBank,
			FlatBankIndexed:            flatBankIndexed,
			WallTexBank:                wallTexBank,
			WallTextureAnimSequences:   wallTextureAnimSequences,
			FlatTextureAnimSequences:   flatTextureAnimSequences,
			BootSplash:                 bootSplash,
			DoomPaletteRGBA:            doomPaletteRGBA,
			DoomColorMap:               doomColorMap,
			DoomColorMapRows:           doomColorMapRows,
			MenuPatchBank:              menuPatchBank,
			StatusPatchBank:            statusPatchBank,
			MessageFontBank:            messageFontBank,
			SpritePatchBank:            spritePatchBank,
			IntermissionPatchBank:      intermissionPatchBank,
			SoundBank:                  soundBank,
			MusicPatchBank:             musicPatchBank,
			MusicSoundFontPath:         strings.TrimSpace(*soundFont),
			MusicSoundFontChoices:      append([]string(nil), musicSoundFontChoices...),
			MusicSoundFont:             musicSoundFont,
			RecordDemoPath:             resolvedRecordDemoPath,
			DemoExitOnDeath:            *demoExitOnDeath,
			DemoStopAfterTics:          max(0, *demoStopAfterTics),
			DemoTracePath:              resolvedDemoTracePath,
			AttractDemos:               builtInAttractDemos(wf),
		}
		opts.TitleMusicLoader = func() ([]byte, error) {
			for _, lump := range []string{"D_DM2TTL", "D_INTRO"} {
				l, ok := wf.LumpByName(lump)
				if !ok {
					continue
				}
				data, err := wf.LumpDataView(l)
				if err != nil {
					return nil, err
				}
				if _, err := music.ParseMUS(data); err != nil {
					return nil, err
				}
				return data, nil
			}
			return nil, nil
		}
		opts.MapMusicLoader = func(mapName string) ([]byte, error) {
			lump, ok := mapMusicLumpName(mapdata.MapName(mapName))
			if !ok {
				return nil, nil
			}
			l, ok := wf.LumpByName(lump)
			if !ok {
				return nil, nil
			}
			data, err := wf.LumpDataView(l)
			if err != nil {
				return nil, err
			}
			// Validate once at load time so runtime playback can assume decodable MUS.
			if _, err := music.ParseMUS(data); err != nil {
				return nil, err
			}
			return data, nil
		}
		opts.MapMusicInfo = mapMusicInfo
		opts.IntermissionMusicLoader = func(commercial bool) ([]byte, error) {
			lump := "D_INTER"
			if commercial {
				lump = "D_DM2INT"
			}
			l, ok := wf.LumpByName(lump)
			if !ok {
				return nil, nil
			}
			data, err := wf.LumpDataView(l)
			if err != nil {
				return nil, err
			}
			if _, err := music.ParseMUS(data); err != nil {
				return nil, err
			}
			return data, nil
		}
		opts.MusicPlayerCatalog, opts.MusicPlayerTrackLoader = buildMusicPlayerCatalog(resolvedWADPath)
		opts.NewGameLoader = func(mapName string) (*mapdata.Map, error) {
			return mapdata.LoadMap(wf, mapdata.MapName(strings.ToUpper(strings.TrimSpace(mapName))))
		}
		opts.DemoMapLoader = func(script *demo.Script) (*mapdata.Map, error) {
			name, err := resolveDemoStartMap(wf, script, "")
			if err != nil {
				return nil, err
			}
			return mapdata.LoadMap(wf, name)
		}
		opts.Episodes = availableEpisodes(wf)
		if strings.TrimSpace(configPath) != "" {
			path := configPath
			opts.OnRuntimeSettingsChanged = func(s doomsession.RuntimeSettings) {
				if err := saveRuntimeSettings(path, s, opts.SourcePortMode); err != nil {
					fmt.Fprintf(stderr, "config save warning: %v\n", err)
				}
			}
		}
		if p := resolvedDemoPath; p != "" {
			demo, derr := demo.Load(p)
			if derr != nil {
				fmt.Fprintf(stderr, "load demo: %v\n", derr)
				return 1
			}
			opts.DemoScript = demo
			opts.DemoQuitOnComplete = true
			selected, err = resolveDemoStartMap(wf, demo, selected)
			if err != nil {
				fmt.Fprintf(stderr, "resolve demo map: %v\n", err)
				return 1
			}
			applyDemoPlaybackHeader(&opts, demo)
			fmt.Fprintf(stderr, "demo loaded: %s tics=%d\n", p, len(demo.Tics))
		}
		if selected == "" {
			selected, err = mapdata.FirstMapName(wf)
			if err != nil {
				fmt.Fprintf(stderr, "resolve first map: %v\n", err)
				return 1
			}
		}
		m, lerr := mapdata.LoadMap(wf, selected)
		if lerr != nil {
			fmt.Fprintf(stderr, "load map %s: %v\n", selected, lerr)
			return 1
		}
		nextMap := func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
			next, nerr := mapdata.NextMapName(wf, current, secret)
			if nerr != nil {
				return nil, "", fmt.Errorf("resolve next map after %s: %w", current, nerr)
			}
			exitKind := "normal"
			if secret {
				exitKind = "secret"
			}
			fmt.Fprintf(stderr, "level exit (%s): %s -> %s\n", exitKind, current, next)
			nm, lerr := mapdata.LoadMap(wf, next)
			if lerr != nil {
				return nil, "", fmt.Errorf("load map %s: %w", next, lerr)
			}
			return nm, next, nil
		}
		rerr := doomsession.Run(m, opts, nextMap)
		if rerr != nil {
			fmt.Fprintf(stderr, "render map %s: %v\n", selected, rerr)
			return 1
		}
		return 0
	}

	if !*render && (resolvedDemoTracePath != "" || resolvedDemoPath != "") {
		buildCfg := renderBuildConfig{
			selectedMap:                strings.ToUpper(strings.TrimSpace(*mapName)),
			mapExplicit:                mapExplicit,
			width:                      *width,
			height:                     *height,
			zoom:                       defaultZoom,
			detailLevel:                *detailLevel,
			detailLevelExplicit:        detailLevelSet,
			autoDetail:                 *autoDetail,
			detailLevelFaithful:        configuredDetailLevelForMode(cfg, false),
			detailLevelSourcePort:      configuredDetailLevelForMode(cfg, true),
			gammaLevel:                 *gammaLevel,
			debug:                      *debug,
			debugEvents:                *debugEvents,
			playerSlot:                 *playerSlot,
			skillLevel:                 *skillLevel,
			gameMode:                   resolvedGameMode,
			showNoSkillItems:           *showNoSkillItems,
			showAllItems:               *showAllItems,
			mouseLook:                  *mouseLook,
			mouseInvert:                *mouseInvert,
			mouseLookSpeed:             *mouseLookSpeed,
			keyboardTurnSpeed:          *keyboardTurnSpeed,
			musicVolume:                0,
			musPanMax:                  *musPanMax,
			oplVolume:                  defaultOPLVolume,
			audioPreEmphasis:           defaultAudioPreEmphasis,
			musicBackend:               resolvedMusicBackend,
			oplBankPath:                defaultOPLBankPath,
			soundFontPath:              strings.TrimSpace(*soundFont),
			sfxVolume:                  0,
			sfxPitchShift:              defaultSFXPitchShift,
			fastMonsters:               defaultFastMonsters,
			alwaysRun:                  *alwaysRun,
			autoWeaponSwitch:           *autoWeaponSwitch,
			cheatLevel:                 resolvedCheatLevel,
			invuln:                     resolvedInvuln,
			sourcePortMode:             *sourcePortMode,
			sourcePortThingRenderMode:  defaultSourcePortThingRenderMode,
			sourcePortThingBlendFrames: defaultSourcePortThingBlendFrames,
			sourcePortSectorLighting:   defaultSourcePortSectorLighting,
			doomLighting:               defaultDoomLighting,
			kageShader:                 defaultKageShader,
			gpuSky:                     *gpuSky,
			skyUpscaleMode:             *skyUpscale,
			crtEffect:                  *crtEffect,
			wallOcclusion:              defaultWallOcclusion,
			wallSpanReject:             defaultWallSpanReject,
			wallSpanClip:               defaultWallSpanClip,
			wallSliceOcclusion:         defaultWallSliceOcclusion,
			billboardClipping:          defaultBillboardClipping,
			rendererWorkers:            *rendererWorkers,
			textureAnimCrossfadeFrames: *textureAnimCrossfadeFrames,
			noVsync:                    *noVsync,
			noFPS:                      *noFPS,
			showTPS:                    defaultShowTPS,
			noAspectCorrection:         *noAspectCorrection,
			allCheats:                  *allCheats,
			startInMap:                 explicitMapStartInMap(defaultStartInMap, mapExplicit),
			importPCSpeaker:            defaultImportPCSpeaker,
			importTextures:             defaultImportTextures,
			demoPath:                   resolvedDemoPath,
			recordDemoPath:             resolvedRecordDemoPath,
			demoExitOnDeath:            *demoExitOnDeath,
			demoStopAfterTics:          max(0, *demoStopAfterTics),
			demoTracePath:              resolvedDemoTracePath,
			pwadPaths:                  resolvedFilePaths,
			configPath:                 configPath,
		}
		bundle, berr := buildRenderBundle(resolvedWADPath, buildCfg, stderr)
		if berr != nil {
			fmt.Fprintf(stderr, "build headless demo run: %v\n", berr)
			return 1
		}
		sess := doomsession.New(bundle.m, bundle.opts, bundle.nextMap)
		defer sess.Close()
		for tic := 0; tic < 1_000_000; tic++ {
			uerr := sess.Update()
			if uerr == nil {
				continue
			}
			if errors.Is(uerr, ebiten.Termination) {
				if err := sess.Err(); err != nil {
					fmt.Fprintf(stderr, "headless demo run: %v\n", err)
					return 1
				}
				return 0
			}
			fmt.Fprintf(stderr, "headless demo run update %d: %v\n", tic, uerr)
			return 1
		}
		fmt.Fprintln(stderr, "headless demo run did not terminate")
		return 1
	}

	if p := resolvedDemoPath; p != "" && !*render {
		demo, derr := demo.Load(p)
		if derr != nil {
			fmt.Fprintf(stderr, "load demo: %v\n", derr)
			return 1
		}
		selected, err = resolveDemoStartMap(wf, demo, selected)
		if err != nil {
			fmt.Fprintf(stderr, "resolve demo map: %v\n", err)
			return 1
		}
	}
	if selected == "" {
		selected, err = defaultStartMap(wf, resolvedFilePaths)
		if err != nil {
			fmt.Fprintf(stderr, "resolve first map: %v\n", err)
			return 1
		}
	}
	m, err := mapdata.LoadMap(wf, selected)
	if err != nil {
		fmt.Fprintf(stderr, "load map %s: %v\n", selected, err)
		return 1
	}

	fmt.Fprintf(stdout, "map=%s things=%d linedefs=%d sidedefs=%d vertexes=%d segs=%d ssectors=%d nodes=%d sectors=%d reject_bytes=%d blockmap_words=%d\n",
		m.Name,
		len(m.Things),
		len(m.Linedefs),
		len(m.Sidedefs),
		len(m.Vertexes),
		len(m.Segs),
		len(m.SubSectors),
		len(m.Nodes),
		len(m.Sectors),
		len(m.Reject),
		len(m.Blockmap),
	)
	if *details {
		ds := m.DoorStats()
		fmt.Fprintf(stdout, "doors total=%d manual=%d use=%d walk=%d shoot=%d repeat=%d one_shot=%d locked_blue=%d locked_red=%d locked_yellow=%d timed_close30=%d timed_raise5m=%d\n",
			ds.Total,
			ds.Manual,
			ds.Use,
			ds.Walk,
			ds.Shoot,
			ds.Repeat,
			ds.OneShot,
			ds.LockedBlue,
			ds.LockedRed,
			ds.LockedYellow,
			ds.TimedCloseIn30,
			ds.TimedRaiseIn5Minute,
		)
		if m.BlockMap != nil {
			fmt.Fprintf(stdout, "blockmap origin=(%d,%d) size=%dx%d cells=%d\n",
				m.BlockMap.OriginX,
				m.BlockMap.OriginY,
				m.BlockMap.Width,
				m.BlockMap.Height,
				len(m.BlockMap.Cells),
			)
		}
		if m.RejectMatrix != nil {
			visible, rerr := m.RejectMatrix.Rejects(0, 0)
			if rerr == nil {
				fmt.Fprintf(stdout, "reject sectors=%d sample_reject_0_0=%t\n", m.RejectMatrix.SectorCount, visible)
			} else {
				fmt.Fprintf(stdout, "reject sectors=%d sample_reject_0_0_error=%q\n", m.RejectMatrix.SectorCount, rerr.Error())
			}
		}
	}
	return 0
}

func runGameWithPlatformOptions(game ebiten.Game) error {
	if !platformcfg.IsWASMBuild() {
		return ebiten.RunGame(game)
	}
	return ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		DisableHiDPI: true,
	})
}

func resolveWADOverlayPaths(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		out = append(out, resolveIWADAliasPath(path))
	}
	return out
}

func openWADStack(basePath string, overlayPaths []string) (*wad.File, []string, error) {
	paths := make([]string, 0, 1+len(overlayPaths))
	basePath = strings.TrimSpace(resolveIWADAliasPath(basePath))
	if basePath == "" {
		return nil, nil, fmt.Errorf("missing base wad path")
	}
	paths = append(paths, basePath)
	for _, path := range overlayPaths {
		path = strings.TrimSpace(resolveIWADAliasPath(path))
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	wf, err := wad.OpenFiles(paths...)
	if err != nil {
		return nil, nil, err
	}
	return wf, paths, nil
}

func defaultStartMap(wf *wad.File, overlayPaths []string) (mapdata.MapName, error) {
	for i := len(overlayPaths) - 1; i >= 0; i-- {
		overlay, err := wad.Open(overlayPaths[i])
		if err != nil {
			return "", err
		}
		if name, err := mapdata.FirstMapName(overlay); err == nil {
			return name, nil
		}
	}
	return mapdata.FirstMapName(wf)
}

func hashWADStackSHA1(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	h := sha1.New()
	for _, path := range paths {
		if data, ok := wad.EmbeddedDataForPath(path); ok {
			if _, err := h.Write(data); err != nil {
				return ""
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return ""
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return ""
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func mapMusicLumpName(name mapdata.MapName) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
	switch s {
	case "E4M1":
		return "D_E3M4", true
	case "E4M2":
		return "D_E3M2", true
	case "E4M3":
		return "D_E3M3", true
	case "E4M4":
		return "D_E1M5", true
	case "E4M5":
		return "D_E2M7", true
	case "E4M6":
		return "D_E2M4", true
	case "E4M7":
		return "D_E2M6", true
	case "E4M8":
		return "D_E2M5", true
	case "E4M9":
		return "D_E1M9", true
	}
	if len(s) == 4 && s[0] == 'E' && s[2] == 'M' &&
		s[1] >= '1' && s[1] <= '9' && s[3] >= '1' && s[3] <= '9' {
		return "D_" + s, true
	}
	if strings.HasPrefix(s, "MAP") && len(s) == 5 && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		n := int(s[3]-'0')*10 + int(s[4]-'0')
		switch n {
		case 1:
			return "D_RUNNIN", true
		case 2:
			return "D_STALKS", true
		case 3:
			return "D_COUNTD", true
		case 4:
			return "D_BETWEE", true
		case 5:
			return "D_DOOM", true
		case 6:
			return "D_THE_DA", true
		case 7:
			return "D_SHAWN", true
		case 8:
			return "D_DDTBLU", true
		case 9:
			return "D_IN_CIT", true
		case 10:
			return "D_DEAD", true
		case 11:
			return "D_STLKS2", true
		case 12:
			return "D_THE_DA2", true
		case 13:
			return "D_DOOM2", true
		case 14:
			return "D_DDTBL2", true
		case 15:
			return "D_RUNNI2", true
		case 16:
			return "D_DEAD2", true
		case 17:
			return "D_STLKS3", true
		case 18:
			return "D_ROMERO", true
		case 19:
			return "D_SHAWN2", true
		case 20:
			return "D_MESSAG", true
		case 21:
			return "D_COUNT2", true
		case 22:
			return "D_DDTBL3", true
		case 23:
			return "D_AMPIE", true
		case 24:
			return "D_THEDA3", true
		case 25:
			return "D_ADRIAN", true
		case 26:
			return "D_MESSG2", true
		case 27:
			return "D_ROMER2", true
		case 28:
			return "D_TENSE", true
		case 29:
			return "D_SHAWN3", true
		case 30:
			return "D_OPENIN", true
		case 31:
			return "D_EVIL", true
		case 32:
			return "D_ULTIMA", true
		}
	}
	return "", false
}

func buildMusicPlayerCatalog(currentWADPath string) ([]runtimecfg.MusicPlayerWAD, func(string, string) ([]byte, error)) {
	currentWADPath = strings.TrimSpace(resolveIWADAliasPath(currentWADPath))
	if currentWADPath == "" {
		return nil, nil
	}
	seen := make(map[string]struct{}, 8)
	choices := make([]iwadChoice, 0, 8)
	appendChoice := func(path, label string) {
		path = strings.TrimSpace(resolveIWADAliasPath(path))
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		choices = append(choices, iwadChoice{Path: path, Label: label})
	}
	appendChoice(currentWADPath, strings.TrimSpace(filepath.Base(currentWADPath)))
	for _, choice := range detectAvailableIWADChoices(filepath.Dir(currentWADPath)) {
		appendChoice(choice.Path, choice.Label)
	}
	catalog := make([]runtimecfg.MusicPlayerWAD, 0, len(choices))
	for _, choice := range choices {
		wf, err := wad.Open(choice.Path)
		if err != nil {
			continue
		}
		episodes := musicPlayerEpisodesForWAD(wf)
		if len(episodes) == 0 {
			continue
		}
		label := strings.TrimSpace(choice.Label)
		if label == "" {
			label = filepath.Base(choice.Path)
		}
		catalog = append(catalog, runtimecfg.MusicPlayerWAD{
			Key:      choice.Path,
			Label:    label,
			Episodes: episodes,
		})
	}
	if len(catalog) == 0 {
		return nil, nil
	}
	loader := func(wadKey string, lumpName string) ([]byte, error) {
		wadKey = strings.TrimSpace(wadKey)
		if wadKey == "" {
			return nil, nil
		}
		lump := strings.ToUpper(strings.TrimSpace(lumpName))
		if lump == "" {
			return nil, nil
		}
		wf, err := wad.Open(wadKey)
		if err != nil {
			return nil, err
		}
		l, ok := wf.LumpByName(lump)
		if !ok {
			return nil, nil
		}
		data, err := wf.LumpDataView(l)
		if err != nil {
			return nil, err
		}
		if _, err := music.ParseMUS(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return catalog, loader
}

func musicPlayerEpisodesForWAD(wf *wad.File) []runtimecfg.MusicPlayerEpisode {
	if wf == nil {
		return nil
	}
	names := mapdata.AvailableMapNames(wf)
	if len(names) == 0 {
		return nil
	}
	type group struct {
		label  string
		tracks []runtimecfg.MusicPlayerTrack
	}
	order := make([]string, 0, 8)
	groups := make(map[string]*group, 8)
	seenLumps := make(map[string]struct{}, 64)
	groupFor := func(label string) *group {
		if g, ok := groups[label]; ok {
			return g
		}
		g := &group{label: label}
		groups[label] = g
		order = append(order, label)
		return g
	}
	for _, name := range names {
		lump, ok := mapMusicLumpName(name)
		if !ok {
			continue
		}
		if _, ok := wf.LumpByName(lump); !ok {
			continue
		}
		mapLabel := strings.ToUpper(strings.TrimSpace(string(name)))
		episodeLabel := "MAPS"
		if len(mapLabel) == 4 && mapLabel[0] == 'E' && mapLabel[2] == 'M' && mapLabel[1] >= '1' && mapLabel[1] <= '9' {
			episodeLabel = fmt.Sprintf("EPISODE %c", mapLabel[1])
		}
		g := groupFor(episodeLabel)
		g.tracks = append(g.tracks, runtimecfg.MusicPlayerTrack{
			MapName:   name,
			Label:     mapDisplayLabel(name),
			LumpName:  lump,
			MusicName: musicTitleForLump(lump),
		})
		seenLumps[lump] = struct{}{}
	}
	const otherMusicLabel = "OTHER MUSIC"
	other := groupFor(otherMusicLabel)
	seenOther := make(map[string]struct{}, 32)
	for _, lump := range wf.Lumps {
		name := strings.ToUpper(strings.TrimSpace(lump.Name))
		if !strings.HasPrefix(name, "D_") {
			continue
		}
		if _, ok := seenLumps[name]; ok {
			continue
		}
		if _, ok := seenOther[name]; ok {
			continue
		}
		other.tracks = append(other.tracks, runtimecfg.MusicPlayerTrack{
			Label:     musicTitleForLump(name),
			LumpName:  name,
			MusicName: musicTitleForLump(name),
		})
		seenOther[name] = struct{}{}
	}
	episodes := make([]runtimecfg.MusicPlayerEpisode, 0, len(order))
	for _, label := range order {
		g := groups[label]
		if g == nil || len(g.tracks) == 0 {
			continue
		}
		episodes = append(episodes, runtimecfg.MusicPlayerEpisode{
			Label:  g.label,
			Tracks: g.tracks,
		})
	}
	return episodes
}

func resolveIWADAliasPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return path
	}
	if resolved, ok := resolvePathCaseInsensitive(trimmed); ok {
		return resolved
	}
	base := strings.ToUpper(filepath.Base(trimmed))
	var aliases []string
	switch base {
	case "DOOM1.WAD":
		aliases = []string{"DOOMU.WAD", "DOOM.WAD", "DOOM2.WAD"}
	case "DOOMU.WAD", "DOOM.WAD":
		aliases = []string{"DOOMU.WAD", "DOOM.WAD"}
	default:
		return path
	}
	dir := filepath.Dir(trimmed)
	for _, candidate := range aliases {
		if alias, ok := resolvePathCaseInsensitive(filepath.Join(dir, candidate)); ok {
			return alias
		}
	}
	return path
}

func resolvePathCaseInsensitive(path string) (string, bool) {
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), name) {
			return filepath.Join(dir, entry.Name()), true
		}
	}
	return "", false
}

func detectAvailableSoundFonts(dir string) []string {
	out := append([]string(nil), music.EmbeddedSoundFontChoices()...)
	out = append(out, music.BrowserSoundFontChoices()...)
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if !strings.HasSuffix(strings.ToLower(name), ".sf2") {
				continue
			}
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi := soundFontDefaultRank(out[i])
		pj := soundFontDefaultRank(out[j])
		if pi != pj {
			return pi < pj
		}
		return strings.ToUpper(out[i]) < strings.ToUpper(out[j])
	})
	if len(out) == 0 {
		return nil
	}
	dedup := out[:0]
	var prev string
	for _, path := range out {
		if prev != "" && strings.EqualFold(prev, path) {
			continue
		}
		dedup = append(dedup, path)
		prev = path
	}
	return dedup
}

func soundFontDefaultRank(path string) int {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	switch base {
	case "sc55.sf2":
		return 0
	case "sgm-hq.sf2":
		return 1
	case "general-midi.sf2":
		return 2
	default:
		return 3
	}
}

type iwadChoice struct {
	Path  string
	Label string
}

type knownIWADChoice struct {
	Label string
	Paths []string
}

func detectAvailableIWADChoices(dir string) []iwadChoice {
	known := knownIWADChoices()
	browserPaths := wad.BrowserLocalWADPaths()
	out := make([]iwadChoice, 0, len(known)+len(browserPaths))
	usedBrowser := make(map[string]struct{}, len(browserPaths))
	for _, k := range known {
		for _, candidate := range k.Paths {
			if p, ok := resolvePathCaseInsensitive(filepath.Join(dir, candidate)); ok {
				out = append(out, iwadChoice{
					Path:  p,
					Label: k.Label,
				})
				goto nextKnownIWAD
			}
		}
		for _, path := range browserPaths {
			if !browserWADMatchesKnownChoice(path, k) {
				continue
			}
			out = append(out, iwadChoice{
				Path:  path,
				Label: k.Label,
			})
			usedBrowser[strings.ToUpper(strings.TrimSpace(path))] = struct{}{}
			goto nextKnownIWAD
		}
		for _, candidate := range k.Paths {
			if _, ok := wad.EmbeddedDataForPath(candidate); ok {
				out = append(out, iwadChoice{
					Path:  candidate,
					Label: k.Label,
				})
				goto nextKnownIWAD
			}
		}
	nextKnownIWAD:
	}
	for _, path := range browserPaths {
		key := strings.ToUpper(strings.TrimSpace(path))
		if _, ok := usedBrowser[key]; ok {
			continue
		}
		label := "LOCAL WAD"
		if wf, err := wad.Open(path); err == nil && strings.EqualFold(wf.Header.Identification, "PWAD") {
			label = "LOCAL PWAD"
		}
		out = append(out, iwadChoice{
			Path:  path,
			Label: label,
		})
	}
	return out
}

func browserWADMatchesKnownChoice(path string, choice knownIWADChoice) bool {
	base := strings.ToUpper(filepath.Base(strings.TrimSpace(path)))
	for _, candidate := range choice.Paths {
		if strings.EqualFold(candidate, base) {
			return true
		}
	}
	return false
}

func knownIWADChoices() []knownIWADChoice {
	return []knownIWADChoice{
		{Label: "The Ultimate DOOM", Paths: []string{"DOOMU.WAD", "DOOM.WAD"}},
		{Label: "DOOM II: Hell on Earth", Paths: []string{"DOOM2.WAD"}},
		{Label: "Final DOOM: TNT", Paths: []string{"TNT.WAD"}},
		{Label: "Final DOOM: Plutonia", Paths: []string{"PLUTONIA.WAD"}},
		{Label: "DOOM Shareware", Paths: []string{"DOOM1.WAD"}},
	}
}

func knownIWADChoiceForPath(path string) (iwadChoice, bool) {
	base := strings.ToUpper(filepath.Base(strings.TrimSpace(path)))
	for _, choice := range knownIWADChoices() {
		for _, candidate := range choice.Paths {
			if strings.EqualFold(candidate, base) {
				return iwadChoice{Path: candidate, Label: choice.Label}, true
			}
		}
	}
	return iwadChoice{}, false
}

type renderBuildConfig struct {
	selectedMap                string
	mapExplicit                bool
	width                      int
	height                     int
	zoom                       float64
	detailLevel                int
	detailLevelExplicit        bool
	autoDetail                 bool
	detailLevelFaithful        int
	detailLevelSourcePort      int
	gammaLevel                 int
	debug                      bool
	debugEvents                bool
	playerSlot                 int
	skillLevel                 int
	gameMode                   string
	showNoSkillItems           bool
	showAllItems               bool
	mouseLook                  bool
	mouseInvert                bool
	mouseLookSpeed             float64
	keyboardTurnSpeed          float64
	musicVolume                float64
	musPanMax                  float64
	oplVolume                  float64
	audioPreEmphasis           bool
	musicBackend               music.Backend
	oplBankPath                string
	soundFontPath              string
	sfxVolume                  float64
	sfxPitchShift              bool
	fastMonsters               bool
	alwaysRun                  bool
	autoWeaponSwitch           bool
	cheatLevel                 int
	invuln                     bool
	sourcePortMode             bool
	sourcePortThingRenderMode  string
	sourcePortThingBlendFrames bool
	sourcePortSectorLighting   bool
	doomLighting               bool
	kageShader                 bool
	gpuSky                     bool
	skyUpscaleMode             string
	crtEffect                  bool
	wallOcclusion              bool
	wallSpanReject             bool
	wallSpanClip               bool
	wallSliceOcclusion         bool
	billboardClipping          bool
	rendererWorkers            int
	textureAnimCrossfadeFrames int
	noVsync                    bool
	noFPS                      bool
	showTPS                    bool
	noAspectCorrection         bool
	allCheats                  bool
	startInMap                 bool
	importPCSpeaker            bool
	importTextures             bool
	demoPath                   string
	recordDemoPath             string
	demoExitOnDeath            bool
	demoStopAfterTics          int
	demoTracePath              string
	pwadPaths                  []string
	configPath                 string
}

type pickerProfile int

const (
	pickerProfileSourcePort pickerProfile = iota
	pickerProfileFaithful
)

type pickerProfileOption struct {
	label          string
	description    string
	sourcePortMode bool
}

var pickerProfiles = [...]pickerProfileOption{
	{label: "MODERN", description: "BETTER GRAPHICS", sourcePortMode: true},
	{label: "FAITHFUL", description: "CLASSIC DOOM - PIXELATED / RETRO", sourcePortMode: false},
}

type pickerSynthOption struct {
	label       string
	description string
	backend     music.Backend
	soundFont   string
}

var pickerSynths = [...]pickerSynthOption{
	{label: "OPL - ADLIB / SB16", backend: music.BackendImpSynth},
	{label: "MIDI - GENERAL MIDI", backend: music.BackendMeltySynth, soundFont: "soundfonts/general-midi.sf2"},
	{label: "MIDI - SC55-HQ", backend: music.BackendMeltySynth, soundFont: "soundfonts/SC55-HQ.sf2"},
	{label: "MIDI - SGM-HQ", backend: music.BackendMeltySynth, soundFont: music.BrowserSGMHQSoundFontPath()},
}

type pickerStage int

const (
	pickerStageIWAD pickerStage = iota
	pickerStageProfile
	pickerStageSynth
)

type renderBundle struct {
	m       *mapdata.Map
	opts    doomsession.Options
	nextMap doomsession.NextMapFunc
}

func resolveMusicPatchBank(wf *wad.File, overridePath string, stderr io.Writer) (music.PatchBank, error) {
	overridePath = strings.TrimSpace(overridePath)
	if overridePath != "" {
		bank, err := music.ParseGENMIDIOP2PatchBankFile(overridePath)
		if err != nil {
			return nil, err
		}
		if stderr != nil {
			fmt.Fprintf(stderr, "music patch import: source=%s instruments=%d\n", overridePath, 128+47)
		}
		return bank, nil
	}

	if wf != nil {
		if genmidiLump, ok := wf.LumpByName("GENMIDI"); ok {
			if genmidiData, gerr := wf.LumpDataView(genmidiLump); gerr != nil {
				if stderr != nil {
					fmt.Fprintf(stderr, "music patch import warning: read GENMIDI: %v\n", gerr)
				}
			} else if bank, gerr := music.ParseGENMIDIOP2PatchBank(genmidiData); gerr != nil {
				if stderr != nil {
					fmt.Fprintf(stderr, "music patch import warning: parse GENMIDI: %v\n", gerr)
				}
			} else {
				if stderr != nil {
					fmt.Fprintf(stderr, "music patch import: source=GENMIDI instruments=%d\n", 128+47)
				}
				return bank, nil
			}
		}
	}

	if bank, err := music.ParseGENMIDIOP2PatchBankFile("GENMIDI.op2"); err == nil {
		if stderr != nil {
			fmt.Fprintf(stderr, "music patch import: source=GENMIDI.op2 instruments=%d\n", 128+47)
		}
		return bank, nil
	}
	return nil, nil
}

func resolveMusicSoundFont(backend music.Backend, path string, stderr io.Writer) (*music.SoundFontBank, error) {
	if music.ResolveBackend(backend) != music.BackendMeltySynth {
		return nil, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("meltysynth backend requires a SoundFont (.sf2)")
	}
	bank, err := music.ParseSoundFontFile(path)
	if err != nil {
		return nil, err
	}
	if stderr != nil {
		fmt.Fprintf(stderr, "music soundfont import: source=%s\n", path)
	}
	return bank, nil
}

func buildRenderBundle(resolvedWADPath string, cfg renderBuildConfig, stderr io.Writer) (*renderBundle, error) {
	wf, wadPaths, err := openWADStack(resolvedWADPath, cfg.pwadPaths)
	if err != nil {
		return nil, fmt.Errorf("open wad: %w", err)
	}
	wadHash := hashWADStackSHA1(wadPaths)
	dsr := sound.ImportDigitalSounds(wf)
	musicSoundFontChoices := detectAvailableSoundFonts("soundfonts")
	musicPatchBank, err := resolveMusicPatchBank(wf, cfg.oplBankPath, stderr)
	if err != nil {
		return nil, err
	}
	musicSoundFont, err := resolveMusicSoundFont(cfg.musicBackend, cfg.soundFontPath, stderr)
	if err != nil {
		return nil, err
	}
	if cfg.importPCSpeaker {
		dpr := sound.ImportPCSpeakerSounds(wf)
		fmt.Fprintf(stderr, "sound import: dp(found=%d decoded=%d failed=%d) ds(found=%d decoded=%d failed=%d)\n",
			dpr.Found, dpr.Decoded, dpr.Failed, dsr.Found, dsr.Decoded, dsr.Failed)
	} else {
		fmt.Fprintf(stderr, "sound import: ds(found=%d decoded=%d failed=%d)\n", dsr.Found, dsr.Decoded, dsr.Failed)
	}
	soundBank := buildAutomapSoundBank(dsr, cfg.sourcePortMode)
	wallTexBank := map[string]media.WallTexture(nil)
	bootSplash := media.WallTexture{}
	doomPaletteRGBA := []byte(nil)
	doomColorMap := []byte(nil)
	doomColorMapRows := 0
	menuPatchBank := map[string]media.WallTexture(nil)
	statusPatchBank := map[string]media.WallTexture(nil)
	messageFontBank := map[rune]media.WallTexture(nil)
	spritePatchBank := map[string]media.WallTexture(nil)
	intermissionPatchBank := map[string]media.WallTexture(nil)
	var texSet *doomtex.Set
	if pal, perr := doomtex.LoadPaletteRGBA(wf, 0); perr == nil {
		doomPaletteRGBA = pal
	}
	if cmLump, ok := wf.LumpByName("COLORMAP"); ok {
		if cmData, err := wf.LumpDataView(cmLump); err == nil && len(cmData) >= 256 {
			doomColorMapRows = len(cmData) / 256
			doomColorMap = cmData[:doomColorMapRows*256]
		}
	}
	if cfg.importTextures {
		ts, terr := doomtex.LoadFromWAD(wf)
		if terr == nil {
			texSet = ts
			fmt.Fprintf(stderr, "texture import: palettes=%d textures=%d\n", ts.PaletteCount(), ts.TextureCount())
			names := ts.TextureNames()
			wallTexBank = make(map[string]media.WallTexture, len(names))
			for _, name := range names {
				indexed, iw, ih, ierr := ts.BuildTextureIndexed(name)
				rgba, w, h, berr := ts.BuildTextureRGBA(name, 0)
				if berr != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
					continue
				}
				rgba32 := []uint32(nil)
				if len(rgba) >= 4 {
					rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
				}
				colMajor := []uint32(nil)
				if len(rgba32) == w*h {
					colMajor = make([]uint32, len(rgba32))
					for tx := 0; tx < w; tx++ {
						colBase := tx * h
						for ty := 0; ty < h; ty++ {
							colMajor[colBase+ty] = rgba32[ty*w+tx]
						}
					}
				}
				indexedColMajor := []byte(nil)
				if ierr == nil && iw == w && ih == h && len(indexed) == w*h {
					indexedColMajor = make([]byte, len(indexed))
					for tx := 0; tx < w; tx++ {
						colBase := tx * h
						for ty := 0; ty < h; ty++ {
							indexedColMajor[colBase+ty] = indexed[ty*w+tx]
						}
					}
				}
				tex := media.WallTexture{RGBA: rgba, RGBA32: rgba32, ColMajor: colMajor, Indexed: indexed, IndexedColMajor: indexedColMajor, Width: w, Height: h}
				tex.EnsureOpaqueMask()
				tex.EnsureOpaqueColumnBounds()
				wallTexBank[name] = tex
			}
		}
	}
	if texSet != nil {
		bootSplash = buildBootSplashTexture(texSet)
		statusPatchBank = buildStatusPatchBank(texSet)
		menuPatchBank = buildMenuPatchBank(texSet)
		messageFontBank = buildMessageFontBank(texSet)
		spritePatchBank = buildMonsterSpriteBank(texSet)
		intermissionPatchBank = buildIntermissionPatchBank(texSet)
	}
	flatBank := map[string][]byte(nil)
	flatBankIndexed := map[string][]byte(nil)
	wallTextureAnimSequences := map[string][]string(nil)
	flatTextureAnimSequences := map[string][]string(nil)
	if fb, ferr := doomtex.LoadFlatsRGBA(wf, 0); ferr == nil {
		flatBank = fb
	}
	if fbi, ferr := doomtex.LoadFlatsIndexed(wf); ferr == nil {
		flatBankIndexed = fbi
	}
	if texSet != nil {
		wallTextureAnimSequences = doomtex.LoadWallTextureAnimSequences(texSet, doomtex.DoomWallAnimDefs)
	}
	flatTextureAnimSequences = doomtex.LoadFlatAnimSequences(wf, doomtex.DoomFlatAnimDefs)
	selected := mapdata.MapName(strings.ToUpper(strings.TrimSpace(cfg.selectedMap)))
	opts := doomsession.Options{
		Width:                      cfg.width,
		Height:                     cfg.height,
		StartZoom:                  cfg.zoom,
		InitialDetailLevel:         cfg.detailLevel,
		AutoDetail:                 cfg.autoDetail,
		InitialGammaLevel:          cfg.gammaLevel,
		WADHash:                    wadHash,
		Debug:                      cfg.debug,
		DebugEvents:                cfg.debugEvents,
		PlayerSlot:                 cfg.playerSlot,
		SkillLevel:                 cfg.skillLevel,
		GameMode:                   cfg.gameMode,
		ShowNoSkillItems:           cfg.showNoSkillItems,
		ShowAllItems:               cfg.showAllItems,
		MouseLook:                  cfg.mouseLook,
		MouseInvert:                cfg.mouseInvert,
		MouseLookSpeed:             cfg.mouseLookSpeed,
		KeyboardTurnSpeed:          cfg.keyboardTurnSpeed,
		MusicVolume:                cfg.musicVolume,
		MUSPanMax:                  cfg.musPanMax,
		OPLVolume:                  cfg.oplVolume,
		AudioPreEmphasis:           cfg.audioPreEmphasis,
		MusicBackend:               cfg.musicBackend,
		OpenMenuOnFrontendStart:    openMenuOnFrontendStart(),
		SFXVolume:                  cfg.sfxVolume,
		SFXPitchShift:              cfg.sfxPitchShift,
		FastMonsters:               cfg.fastMonsters,
		AlwaysRun:                  cfg.alwaysRun,
		AutoWeaponSwitch:           cfg.autoWeaponSwitch,
		CheatLevel:                 cfg.cheatLevel,
		Invulnerable:               cfg.invuln,
		SourcePortMode:             cfg.sourcePortMode,
		SourcePortThingRenderMode:  cfg.sourcePortThingRenderMode,
		SourcePortThingBlendFrames: cfg.sourcePortThingBlendFrames,
		SourcePortSectorLighting:   cfg.sourcePortSectorLighting,
		DisableDoomLighting:        !cfg.doomLighting,
		KageShader:                 cfg.kageShader,
		GPUSky:                     cfg.gpuSky,
		SkyUpscaleMode:             cfg.skyUpscaleMode,
		CRTEffect:                  cfg.crtEffect,
		DisableWallOcclusion:       !cfg.wallOcclusion,
		DisableWallSpanReject:      !cfg.wallSpanReject,
		DisableWallSpanClip:        !cfg.wallSpanClip,
		DisableWallSliceOcclusion:  !cfg.wallSliceOcclusion,
		DisableBillboardClipping:   !cfg.billboardClipping,
		RendererWorkers:            cfg.rendererWorkers,
		TextureAnimCrossfadeFrames: cfg.textureAnimCrossfadeFrames,
		NoVsync:                    cfg.noVsync,
		NoFPS:                      cfg.noFPS,
		ShowTPS:                    cfg.showTPS,
		DisableAspectCorrection:    cfg.noAspectCorrection,
		AllCheats:                  cfg.allCheats,
		StartInMapMode:             explicitMapStartInMap(cfg.startInMap, cfg.mapExplicit),
		FlatBank:                   flatBank,
		FlatBankIndexed:            flatBankIndexed,
		WallTexBank:                wallTexBank,
		WallTextureAnimSequences:   wallTextureAnimSequences,
		FlatTextureAnimSequences:   flatTextureAnimSequences,
		BootSplash:                 bootSplash,
		DoomPaletteRGBA:            doomPaletteRGBA,
		DoomColorMap:               doomColorMap,
		DoomColorMapRows:           doomColorMapRows,
		MenuPatchBank:              menuPatchBank,
		StatusPatchBank:            statusPatchBank,
		MessageFontBank:            messageFontBank,
		SpritePatchBank:            spritePatchBank,
		IntermissionPatchBank:      intermissionPatchBank,
		SoundBank:                  soundBank,
		MusicPatchBank:             musicPatchBank,
		MusicSoundFontPath:         cfg.soundFontPath,
		MusicSoundFontChoices:      append([]string(nil), musicSoundFontChoices...),
		MusicSoundFont:             musicSoundFont,
		RecordDemoPath:             cfg.recordDemoPath,
		DemoExitOnDeath:            cfg.demoExitOnDeath,
		DemoStopAfterTics:          cfg.demoStopAfterTics,
		DemoTracePath:              cfg.demoTracePath,
		AttractDemos:               builtInAttractDemos(wf),
	}
	opts.TitleMusicLoader = func() ([]byte, error) {
		for _, lump := range []string{"D_DM2TTL", "D_INTRO"} {
			if l, ok := wf.LumpByName(lump); ok {
				data, err := wf.LumpDataView(l)
				if err != nil {
					return nil, err
				}
				if _, err := music.ParseMUS(data); err != nil {
					return nil, err
				}
				return data, nil
			}
		}
		return nil, nil
	}
	opts.MapMusicLoader = func(mapName string) ([]byte, error) {
		lump, ok := mapMusicLumpName(mapdata.MapName(mapName))
		if !ok {
			return nil, nil
		}
		l, ok := wf.LumpByName(lump)
		if !ok {
			return nil, nil
		}
		data, err := wf.LumpDataView(l)
		if err != nil {
			return nil, err
		}
		if _, err := music.ParseMUS(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	opts.MapMusicInfo = mapMusicInfo
	opts.IntermissionMusicLoader = func(commercial bool) ([]byte, error) {
		lump := "D_INTER"
		if commercial {
			lump = "D_DM2INT"
		}
		l, ok := wf.LumpByName(lump)
		if !ok {
			return nil, nil
		}
		data, err := wf.LumpDataView(l)
		if err != nil {
			return nil, err
		}
		if _, err := music.ParseMUS(data); err != nil {
			return nil, err
		}
		return data, nil
	}
	opts.MusicPlayerCatalog, opts.MusicPlayerTrackLoader = buildMusicPlayerCatalog(resolvedWADPath)
	opts.NewGameLoader = func(mapName string) (*mapdata.Map, error) {
		return mapdata.LoadMap(wf, mapdata.MapName(strings.ToUpper(strings.TrimSpace(mapName))))
	}
	opts.DemoMapLoader = func(script *demo.Script) (*mapdata.Map, error) {
		name, err := resolveDemoStartMap(wf, script, "")
		if err != nil {
			return nil, err
		}
		return mapdata.LoadMap(wf, name)
	}
	opts.Episodes = availableEpisodes(wf)
	if strings.TrimSpace(cfg.configPath) != "" {
		path := cfg.configPath
		opts.OnRuntimeSettingsChanged = func(s doomsession.RuntimeSettings) {
			_ = saveRuntimeSettings(path, s, opts.SourcePortMode)
		}
	}
	if p := cfg.demoPath; p != "" {
		demo, derr := demo.Load(p)
		if derr != nil {
			return nil, fmt.Errorf("load demo: %w", derr)
		}
		opts.DemoScript = demo
		opts.DemoQuitOnComplete = true
		selected, err = resolveDemoStartMap(wf, demo, selected)
		if err != nil {
			return nil, fmt.Errorf("resolve demo map: %w", err)
		}
		applyDemoPlaybackHeader(&opts, demo)
	}
	if selected == "" {
		selected, err = defaultStartMap(wf, cfg.pwadPaths)
		if err != nil {
			return nil, fmt.Errorf("resolve first map: %w", err)
		}
	}
	m, lerr := mapdata.LoadMap(wf, selected)
	if lerr != nil {
		return nil, fmt.Errorf("load map %s: %w", selected, lerr)
	}
	nextMap := func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
		next, nerr := mapdata.NextMapName(wf, current, secret)
		if nerr != nil {
			return nil, "", fmt.Errorf("resolve next map after %s: %w", current, nerr)
		}
		nm, lerr := mapdata.LoadMap(wf, next)
		if lerr != nil {
			return nil, "", fmt.Errorf("load map %s: %w", next, lerr)
		}
		return nm, next, nil
	}
	return &renderBundle{m: m, opts: opts, nextMap: nextMap}, nil
}

func applyPickerProfile(cfg renderBuildConfig, profile pickerProfile) renderBuildConfig {
	switch profile {
	case pickerProfileSourcePort:
		cfg.sourcePortMode = true
		if !cfg.detailLevelExplicit {
			cfg.detailLevel = cfg.detailLevelSourcePort
		}
	case pickerProfileFaithful:
		cfg.sourcePortMode = false
		if !cfg.detailLevelExplicit {
			cfg.detailLevel = cfg.detailLevelFaithful
		}
	}
	return cfg
}

func pickerSynthIndexForBackend(backend music.Backend) int {
	resolved := music.ResolveBackend(backend)
	for i, option := range pickerSynths {
		if music.ResolveBackend(option.backend) == resolved {
			return i
		}
	}
	return 0
}

func applyPickerSynth(cfg renderBuildConfig, synthIndex int) renderBuildConfig {
	if synthIndex < 0 || synthIndex >= len(pickerSynths) {
		synthIndex = 0
	}
	option := pickerSynths[synthIndex]
	cfg.musicBackend = option.backend
	if music.ResolveBackend(option.backend) == music.BackendMeltySynth {
		if strings.TrimSpace(option.soundFont) != "" {
			cfg.soundFontPath = option.soundFont
		} else if strings.TrimSpace(cfg.soundFontPath) == "" {
			if choices := detectAvailableSoundFonts("soundfonts"); len(choices) > 0 {
				cfg.soundFontPath = choices[0]
			}
		}
	}
	return cfg
}

type iwadPickerGame struct {
	choices      []iwadChoice
	selected     int
	profile      pickerProfile
	synth        int
	stage        pickerStage
	confirmArmed bool
	status       string
	loadingPath  string
	statusUntil  int
	tic          int
	bg           *ebiten.Image
	logo         *ebiten.Image
	menuImg      map[string]*ebiten.Image
	fontBank     map[rune]media.WallTexture
	fontImg      map[rune]*ebiten.Image
	sfx          *audiofx.MenuPlayer
	load         func(string, pickerProfile, int) (*renderBundle, error)
	session      *doomsession.Session
	sessionGame  *session.Game
	err          error
}

func newIWADPickerGame(choices []iwadChoice, initialBackend music.Backend, load func(string, pickerProfile, int) (*renderBundle, error)) (*iwadPickerGame, error) {
	game := &iwadPickerGame{choices: choices, load: load, synth: pickerSynthIndexForBackend(initialBackend)}
	if len(choices) <= 1 {
		game.stage = pickerStageProfile
	}
	_ = audiofx.EnsureSharedAudioContext()
	if assetPath := pickerAssetWADPath(choices); assetPath != "" {
		if wf, err := wad.Open(assetPath); err == nil {
			game.sfx = audiofx.NewMenuPlayer(buildAutomapSoundBank(sound.ImportDigitalSounds(wf), false), 0.5)
			if ts, err := doomtex.LoadFromWAD(wf); err == nil {
				if rgba, w, h, err := ts.BuildTextureRGBA("WALL24_1", 0); err == nil && w > 0 && h > 0 && len(rgba) == w*h*4 {
					img := newDebugImage("picker:bg", w, h)
					img.WritePixels(rgba)
					game.bg = img
				}
				if bank := buildMenuPatchBank(ts); bank != nil {
					game.menuImg = make(map[string]*ebiten.Image, 3)
					if patch, ok := bank["M_DOOM"]; ok && patch.Width > 0 && patch.Height > 0 && len(patch.RGBA) == patch.Width*patch.Height*4 {
						img := newDebugImage("picker:menu:M_DOOM", patch.Width, patch.Height)
						img.WritePixels(patch.RGBA)
						game.logo = img
						game.menuImg["M_DOOM"] = img
					}
					for _, name := range []string{"M_SKULL1", "M_SKULL2"} {
						if patch, ok := bank[name]; ok && patch.Width > 0 && patch.Height > 0 && len(patch.RGBA) == patch.Width*patch.Height*4 {
							img := newDebugImage("picker:menu:"+name, patch.Width, patch.Height)
							img.WritePixels(patch.RGBA)
							game.menuImg[name] = img
						}
					}
				}
				game.fontBank = buildMessageFontBank(ts)
			}
		}
	}
	ebiten.SetWindowTitle("GD-DOOM - Select Game")
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(true)
	return game, nil
}

func pickerConfirmHeld() bool {
	return ebiten.IsKeyPressed(ebiten.KeyEnter) || ebiten.IsKeyPressed(ebiten.KeyKPEnter)
}

func pickerAssetWADPath(choices []iwadChoice) string {
	for _, want := range []string{"DOOM1.WAD", "DOOMU.WAD", "DOOM.WAD", "DOOM2.WAD", "TNT.WAD", "PLUTONIA.WAD"} {
		for _, c := range choices {
			if strings.EqualFold(filepath.Base(c.Path), want) {
				return c.Path
			}
		}
	}
	for _, want := range []string{"DOOM1.WAD", "DOOMU.WAD", "DOOM.WAD", "DOOM2.WAD", "TNT.WAD", "PLUTONIA.WAD"} {
		if p, ok := resolvePathCaseInsensitive(want); ok {
			return p
		}
	}
	if len(choices) > 0 {
		return choices[0].Path
	}
	return ""
}

func (g *iwadPickerGame) Update() error {
	if g.sessionGame != nil {
		return g.sessionGame.Update()
	}
	if len(g.choices) == 0 {
		g.err = fmt.Errorf("no IWADs available")
		return ebiten.Termination
	}
	if strings.TrimSpace(g.loadingPath) != "" {
		if music.BrowserSoundFontLoadPending(g.loadingPath) {
			g.status = pickerSoundFontDownloadStatus(g.loadingPath)
			return nil
		}
		path := g.loadingPath
		g.loadingPath = ""
		if err := music.BrowserSoundFontLoadError(path); err != nil {
			g.status = err.Error()
			return nil
		}
		g.status = "DONE, STARTING."
		g.statusUntil = g.tic + 1
		return nil
	}
	g.tic++
	if strings.TrimSpace(g.status) == "DONE, STARTING." && g.statusUntil > 0 && g.tic >= g.statusUntil {
		if g.load == nil {
			g.err = fmt.Errorf("iwad loader unavailable")
			return ebiten.Termination
		}
		bundle, err := g.load(g.choices[g.selected].Path, g.profile, g.synth)
		if err != nil {
			g.status = err.Error()
			g.statusUntil = 0
			return nil
		}
		g.status = ""
		g.statusUntil = 0
		g.session = doomsession.New(bundle.m, bundle.opts, bundle.nextMap)
		g.sessionGame = session.New(g.session)
		notifyBrowserSessionStarted()
		return nil
	}
	if !g.confirmArmed {
		g.confirmArmed = !pickerConfirmHeld()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.stage == pickerStageSynth {
			g.stage = pickerStageProfile
			g.confirmArmed = false
			g.playPickerBackSound()
		} else if g.stage == pickerStageProfile {
			g.stage = pickerStageIWAD
			g.confirmArmed = false
			g.playPickerBackSound()
		} else {
			g.err = fmt.Errorf("iwad selection cancelled")
			return ebiten.Termination
		}
	}
	switch g.stage {
	case pickerStageProfile:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			prev := g.profile
			g.profile = (g.profile + pickerProfile(len(pickerProfiles)) - 1) % pickerProfile(len(pickerProfiles))
			if g.profile != prev {
				g.playPickerMoveSound()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			prev := g.profile
			g.profile = (g.profile + 1) % pickerProfile(len(pickerProfiles))
			if g.profile != prev {
				g.playPickerMoveSound()
			}
		}
		if g.confirmArmed && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
			g.playPickerConfirmSound()
			g.stage = pickerStageSynth
			g.confirmArmed = false
			return nil
		}
	case pickerStageSynth:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			prev := g.synth
			g.synth = (g.synth + len(pickerSynths) - 1) % len(pickerSynths)
			if g.synth != prev {
				g.playPickerMoveSound()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			prev := g.synth
			g.synth = (g.synth + 1) % len(pickerSynths)
			if g.synth != prev {
				g.playPickerMoveSound()
			}
		}
		if g.confirmArmed && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
			if g.load == nil {
				g.err = fmt.Errorf("iwad loader unavailable")
				return ebiten.Termination
			}
			g.playPickerConfirmSound()
			if soundFontPath := strings.TrimSpace(pickerSynths[g.synth].soundFont); soundFontPath != "" && music.StartBrowserSoundFontLoad(soundFontPath) {
				g.loadingPath = soundFontPath
				g.status = pickerSoundFontDownloadStatus(soundFontPath)
				return nil
			}
			bundle, err := g.load(g.choices[g.selected].Path, g.profile, g.synth)
			if err != nil {
				g.status = err.Error()
				return nil
			}
			g.session = doomsession.New(bundle.m, bundle.opts, bundle.nextMap)
			g.sessionGame = session.New(g.session)
			notifyBrowserSessionStarted()
			return nil
		}
	default:
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			prev := g.selected
			g.selected = (g.selected + len(g.choices) - 1) % len(g.choices)
			if g.selected != prev {
				g.playPickerMoveSound()
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			prev := g.selected
			g.selected = (g.selected + 1) % len(g.choices)
			if g.selected != prev {
				g.playPickerMoveSound()
			}
		}
		if g.confirmArmed && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter)) {
			g.playPickerConfirmSound()
			g.stage = pickerStageProfile
			g.confirmArmed = false
			return nil
		}
	}
	return nil
}

func (g *iwadPickerGame) Draw(screen *ebiten.Image) {
	if g.sessionGame != nil {
		g.sessionGame.Draw(screen)
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	if g.bg != nil {
		drawPickerCoverImage(screen, g.bg)
	} else {
		screen.Fill(color.Black)
	}
	if g.logo != nil {
		drawPickerCenteredIntegerScaledLogo(screen, g.logo)
	} else {
		g.drawPickerTextCentered(screen, "SELECT GAME", sw/2, 20)
	}
	ebitenutil.DrawRect(screen, 0, 0, float64(sw), float64(sh), color.RGBA{R: 8, G: 8, B: 8, A: 128})
	loadingSoundFont := strings.TrimSpace(g.loadingPath) != "" || (strings.TrimSpace(g.status) == "DONE, STARTING." && g.statusUntil > 0)
	switch g.stage {
	case pickerStageProfile:
		if !loadingSoundFont {
			titleWidth := 0
			descWidth := 0
			for _, profile := range pickerProfiles {
				titleWidth = max(titleWidth, g.pickerTextWidthScaled(profile.label, 2))
				descWidth = max(descWidth, g.pickerTextWidth(profile.description))
			}
			contentWidth := max(titleWidth, descWidth)
			titleX := sw/2 - contentWidth/2
			profileY, rowStep := pickerOptionBlockLayout(sh, len(pickerProfiles))
			for i, profile := range pickerProfiles {
				rowY := profileY + i*rowStep
				if i == int(g.profile) {
					g.drawPickerSkull(screen, titleX, rowY+4)
				}
				g.drawPickerTextScaled(screen, profile.label, titleX, rowY, 2)
				g.drawPickerText(screen, profile.description, titleX, rowY+22)
			}
		}
	case pickerStageSynth:
		if !loadingSoundFont {
			titleWidth := 0
			descWidth := 0
			for _, synth := range pickerSynths {
				titleWidth = max(titleWidth, g.pickerTextWidthScaled(synth.label, 2))
				if strings.TrimSpace(synth.description) != "" {
					descWidth = max(descWidth, g.pickerTextWidth(synth.description))
				}
			}
			contentWidth := max(titleWidth, descWidth)
			titleX := sw/2 - contentWidth/2
			synthY, rowStep := pickerOptionBlockLayout(sh, len(pickerSynths))
			for i, synth := range pickerSynths {
				rowY := synthY + i*rowStep
				if i == g.synth {
					g.drawPickerSkull(screen, titleX, rowY+4)
				}
				g.drawPickerTextScaled(screen, synth.label, titleX, rowY, 2)
				if strings.TrimSpace(synth.description) != "" {
					g.drawPickerText(screen, synth.description, titleX, rowY+22)
				}
			}
		}
	default:
		labelTexts := make([]string, len(g.choices))
		fileTexts := make([]string, len(g.choices))
		labelWidth := 0
		fileWidth := 0
		for i, choice := range g.choices {
			labelTexts[i] = strings.ToUpper(choice.Label)
			fileTexts[i] = strings.ToUpper(filepath.Base(choice.Path))
			labelWidth = max(labelWidth, g.pickerTextWidth("> "+labelTexts[i]))
			fileWidth = max(fileWidth, g.pickerTextWidth(fileTexts[i]))
		}
		gap := 16
		blockWidth := labelWidth + gap + fileWidth
		labelX := sw/2 - blockWidth/2
		fileX := labelX + labelWidth + gap
		rowHeight := 18
		blockHeight := max(len(g.choices)*rowHeight, rowHeight)
		y := sh/2 - blockHeight/2
		for i := range g.choices {
			if i == g.selected {
				g.drawPickerSkull(screen, labelX, y+i*rowHeight)
			}
			g.drawPickerText(screen, labelTexts[i], labelX, y+i*rowHeight)
			g.drawPickerText(screen, fileTexts[i], fileX, y+i*rowHeight)
		}
	}
	if strings.TrimSpace(g.status) != "" {
		lines := strings.Split(strings.ToUpper(g.status), "\n")
		lineHeight := 14
		startY := sh - 52 - ((len(lines) - 1) * lineHeight / 2)
		if loadingSoundFont {
			maxWidth := 0
			for _, line := range lines {
				maxWidth = max(maxWidth, g.pickerTextWidth(line))
			}
			panelW := maxWidth + 36
			panelH := len(lines)*lineHeight + 24
			panelX := (sw - panelW) / 2
			textTop := startY
			textBottom := startY + (len(lines)-1)*lineHeight
			panelY := (textTop+textBottom)/2 - panelH/2
			ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.RGBA{A: 192})
		}
		for i, line := range lines {
			g.drawPickerTextCentered(screen, line, sw/2, startY+i*lineHeight)
		}
	}
}

func (g *iwadPickerGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.sessionGame != nil {
		return g.sessionGame.Layout(outsideWidth, outsideHeight)
	}
	return 320, 200
}

func (g *iwadPickerGame) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if g.sessionGame != nil {
		g.sessionGame.DrawFinalScreen(screen, offscreen, geoM)
		return
	}
	if screen == nil || offscreen == nil {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	ow := max(offscreen.Bounds().Dx(), 1)
	oh := max(offscreen.Bounds().Dy(), 1)
	scale := min(sw/ow, sh/oh)
	if scale < 1 {
		scale = 1
	}
	rw := ow * scale
	rh := oh * scale
	ox := (sw - rw) / 2
	oy := (sh - rh) / 2
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(float64(ox), float64(oy))
	screen.DrawImage(offscreen, op)
}

func (g *iwadPickerGame) Close() {
	if g.sfx != nil {
		g.sfx.StopAll()
	}
	if g.sessionGame != nil {
		g.sessionGame.Close()
	} else if g.session != nil {
		g.session.Close()
	}
}

func (g *iwadPickerGame) Session() *doomsession.Session {
	return g.session
}

func drawPickerCenteredIntegerScaledLogo(screen, img *ebiten.Image) {
	if screen == nil || img == nil {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	lw := max(img.Bounds().Dx(), 1)
	lh := max(img.Bounds().Dy(), 1)
	scaleW := int(0.7 * float64(sw) / float64(lw))
	scaleH := int(0.38 * float64(sh) / float64(lh))
	scale := min(max(scaleW, 1), max(scaleH, 1))
	if scale < 1 {
		scale = 1
	}
	dw := lw * scale
	dh := lh * scale
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(float64((sw-dw)/2), float64(max((sh/10)-(dh/6), 8)))
	screen.DrawImage(img, op)
}

func drawPickerCoverImage(screen, img *ebiten.Image) {
	if screen == nil || img == nil {
		return
	}
	sw := max(screen.Bounds().Dx(), 1)
	sh := max(screen.Bounds().Dy(), 1)
	iw := max(img.Bounds().Dx(), 1)
	ih := max(img.Bounds().Dy(), 1)
	scale := max(float64(sw)/float64(iw), float64(sh)/float64(ih))
	dw := float64(iw) * scale
	dh := float64(ih) * scale
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(sw)-dw)*0.5, (float64(sh)-dh)*0.5)
	screen.DrawImage(img, op)
}

func (g *iwadPickerGame) pickerFontGlyph(ch rune) (*ebiten.Image, int, int, int, int, bool) {
	if ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	p, ok := g.fontBank[ch]
	if !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
		return nil, 0, 0, 0, 0, false
	}
	if g.fontImg == nil {
		g.fontImg = make(map[rune]*ebiten.Image, 96)
	}
	if img, ok := g.fontImg[ch]; ok {
		return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
	}
	img := newDebugImage("picker:font:"+string(ch), p.Width, p.Height)
	img.WritePixels(p.RGBA)
	g.fontImg[ch] = img
	return img, p.Width, p.Height, p.OffsetX, p.OffsetY, true
}

func (g *iwadPickerGame) pickerTextWidth(text string) int {
	return g.pickerTextWidthScaled(text, 1)
}

func (g *iwadPickerGame) pickerTextWidthScaled(text string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	if len(g.fontBank) == 0 {
		return len(text) * 7 * scale
	}
	w := 0
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < 33 || uc > 95 {
			w += 4 * scale
			continue
		}
		_, gw, _, _, _, ok := g.pickerFontGlyph(uc)
		if !ok {
			w += 4 * scale
			continue
		}
		w += gw * scale
	}
	return w
}

func (g *iwadPickerGame) drawPickerTextCentered(screen *ebiten.Image, text string, x, y int) {
	g.drawPickerText(screen, text, x-g.pickerTextWidth(text)/2, y)
}

func (g *iwadPickerGame) drawPickerTextCenteredScaled(screen *ebiten.Image, text string, x, y, scale int) {
	g.drawPickerTextScaled(screen, text, x-g.pickerTextWidthScaled(text, scale)/2, y, scale)
}

func (g *iwadPickerGame) drawPickerText(screen *ebiten.Image, text string, x, y int) {
	g.drawPickerTextScaled(screen, text, x, y, 1)
}

func pickerSoundFontDownloadStatus(path string) string {
	received, total := music.BrowserSoundFontLoadProgress(path)
	sizeLine := humanDownloadSize(received)
	if total > 0 {
		sizeLine += " / " + humanDownloadSize(total)
	}
	label := strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))))
	if label == "" {
		label = "SOUNDFONT"
	}
	if sizeLine == "" {
		return "DOWNLOADING\n" + label + " SOUNDFONT"
	}
	return "DOWNLOADING\n" + label + " SOUNDFONT\n" + sizeLine
}

func humanDownloadSize(n int64) string {
	if n <= 0 {
		return ""
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div := int64(unit)
	exp := 0
	for v := n / unit; v >= unit && exp < 5; v /= unit {
		div *= unit
		exp++
	}
	value := float64(n) / float64(div)
	suffix := [...]string{"KB", "MB", "GB", "TB", "PB", "EB"}[exp]
	if value >= 10 {
		return fmt.Sprintf("%.0f %s", value, suffix)
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

func pickerOptionBlockLayout(screenH int, rows int) (startY int, rowStep int) {
	if rows < 1 {
		rows = 1
	}
	const (
		entryH         = 34
		preferredStep  = 52
		minCompactStep = 38
		verticalPad    = 52
	)
	rowStep = preferredStep
	if rows > 1 {
		maxTotalH := screenH - verticalPad
		totalH := entryH + (rows-1)*rowStep
		if totalH > maxTotalH {
			fitStep := (maxTotalH - entryH) / (rows - 1)
			rowStep = max(minCompactStep, fitStep)
		}
	}
	totalH := entryH + (rows-1)*rowStep
	startY = (screenH-totalH)/2 + 8
	if startY < 24 {
		startY = 24
	}
	return startY, rowStep
}

func (g *iwadPickerGame) drawPickerTextScaled(screen *ebiten.Image, text string, x, y, scale int) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if scale < 1 {
		scale = 1
	}
	if len(g.fontBank) == 0 {
		ebitenutil.DebugPrintAt(screen, text, x, y)
		return
	}
	px := float64(x)
	py := float64(y)
	for _, ch := range text {
		uc := ch
		if uc >= 'a' && uc <= 'z' {
			uc -= 'a' - 'A'
		}
		if uc == ' ' || uc < 33 || uc > 95 {
			px += float64(4 * scale)
			continue
		}
		img, w, _, ox, oy, ok := g.pickerFontGlyph(uc)
		if !ok {
			px += float64(4 * scale)
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.Filter = ebiten.FilterNearest
		op.GeoM.Scale(float64(scale), float64(scale))
		op.GeoM.Translate(px-float64(ox*scale), py-float64(oy*scale))
		screen.DrawImage(img, op)
		px += float64(w * scale)
	}
}

func (g *iwadPickerGame) drawPickerSkull(screen *ebiten.Image, textX, textY int) {
	if screen == nil {
		return
	}
	name := "M_SKULL1"
	if (g.tic/8)&1 != 0 {
		name = "M_SKULL2"
	}
	img := g.menuImg[name]
	if img == nil {
		g.drawPickerText(screen, ">", max(textX-10, 0), textY)
		return
	}
	w := max(img.Bounds().Dx(), 1)
	h := max(img.Bounds().Dy(), 1)
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.GeoM.Translate(float64(textX-w-3), float64(textY+6-h/2))
	screen.DrawImage(img, op)
}

func (g *iwadPickerGame) playPickerMoveSound() {
	if g != nil && g.sfx != nil {
		g.sfx.PlayMove()
	}
}

func (g *iwadPickerGame) playPickerConfirmSound() {
	if g != nil && g.sfx != nil {
		g.sfx.PlayConfirm()
	}
}

func (g *iwadPickerGame) playPickerBackSound() {
	if g != nil && g.sfx != nil {
		g.sfx.PlayBack()
	}
}

func dumpSpriteAnimationSeries(outDir, seed string, bank map[string]media.WallTexture) error {
	seed = strings.ToUpper(strings.TrimSpace(seed))
	outDir = strings.TrimSpace(outDir)
	if seed == "" {
		return fmt.Errorf("empty seed")
	}
	if outDir == "" {
		outDir = "anidump"
	}
	if len(bank) == 0 {
		return fmt.Errorf("sprite patch bank is empty")
	}
	series := spriteSeriesFromSeed(seed, bank)
	if len(series) == 0 {
		return fmt.Errorf("no frames found for seed %s", seed)
	}
	targetDir := filepath.Join(outDir, seed)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", targetDir, err)
	}
	for i, name := range series {
		tex := bank[name]
		if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
			continue
		}
		img := image.NewNRGBA(image.Rect(0, 0, tex.Width, tex.Height))
		copy(img.Pix, tex.RGBA)
		p := filepath.Join(targetDir, fmt.Sprintf("%03d_%s.png", i, name))
		f, err := os.Create(p)
		if err != nil {
			return fmt.Errorf("create %s: %w", p, err)
		}
		err = png.Encode(f, img)
		cerr := f.Close()
		if err != nil {
			return fmt.Errorf("encode %s: %w", p, err)
		}
		if cerr != nil {
			return fmt.Errorf("close %s: %w", p, cerr)
		}
	}
	return nil
}

func spriteSeriesFromSeed(seed string, bank map[string]media.WallTexture) []string {
	if len(seed) < 6 {
		return nil
	}
	prefix := seed[:4]
	rot := seed[5]
	addRot := func(names *[]string, seen map[string]struct{}, r byte) {
		for fr := byte('A'); fr <= byte('Z'); fr++ {
			name := fmt.Sprintf("%s%c%c", prefix, fr, r)
			if _, ok := bank[name]; !ok {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			*names = append(*names, name)
		}
	}
	series := make([]string, 0, 32)
	seen := make(map[string]struct{}, 32)
	addRot(&series, seen, '0')
	addRot(&series, seen, '1')
	if rot != '0' && rot != '1' {
		addRot(&series, seen, rot)
	}
	if len(series) == 0 {
		if _, ok := bank[seed]; ok {
			return []string{seed}
		}
		return nil
	}
	sort.Strings(series)
	return series
}

func buildAutomapSoundBank(r sound.DigitalImportReport, sourcePortMode bool) media.SoundBank {
	byName := make(map[string]sound.DigitalSound, len(r.Sounds))
	for _, s := range r.Sounds {
		byName[s.Name] = s
	}
	sample := func(name string) media.PCMSample {
		s, ok := byName[name]
		if !ok {
			return media.PCMSample{}
		}
		data := s.Samples
		if !sourcePortAudioEnabled(sourcePortMode) {
			// Doom pads digital sound payloads to the mixer block size with 128s
			// before playback. Preserve that behavior for the faithful path.
			data = padDoomSoundSamples(data)
		}
		return media.PCMSample{
			// Stock Doom's digital mixer treats SFX as fixed 11025 Hz raw data
			// and ignores the per-lump DMX rate header. Keep the same base pitch
			// semantics in both faithful and source-port playback paths.
			SampleRate: 11025,
			Data:       data,
		}
	}
	bank := media.SoundBank{
		MenuCursor:          firstSample(sample("DSPSTOP"), firstSample(sample("DSSTNMOV"), sample("DSSWTCHN"))),
		DoorOpen:            firstSample(sample("DSDOROPN"), sample("DSBDOPN")),
		DoorClose:           firstSample(sample("DSDORCLS"), sample("DSBDCLS")),
		BlazeOpen:           sample("DSBDOPN"),
		BlazeClose:          sample("DSBDCLS"),
		SwitchOn:            sample("DSSWTCHN"),
		SwitchOff:           sample("DSSWTCHX"),
		NoWay:               firstSample(sample("DSNOWAY"), sample("DSOOF")),
		Tink:                sample("DSTINK"),
		ItemUp:              sample("DSITEMUP"),
		WeaponUp:            sample("DSWPNUP"),
		PowerUp:             sample("DSGETPOW"),
		Teleport:            firstSample(sample("DSTELEPT"), sample("DSNOWAY")),
		BossBrainSpit:       firstSample(firstSample(sample("DSBOSPIT"), sample("DSBOSPITX")), sample("DSFIRSHT")),
		BossBrainCube:       firstSample(sample("DSBOSCUB"), firstSample(sample("DSBOSPITX"), sample("DSFIRSHT"))),
		BossBrainAwake:      firstSample(sample("DSBOSSIT"), firstSample(sample("DSBOSPITX"), sample("DSFIRSHT"))),
		BossBrainPain:       firstSample(sample("DSBOSPN"), firstSample(sample("DSPOPAIN"), sample("DSPLPAIN"))),
		BossBrainDeath:      firstSample(sample("DSBOSDTH"), firstSample(sample("DSBAREXP"), sample("DSBRSDTH"))),
		Oof:                 sample("DSOOF"),
		Pain:                firstSample(sample("DSPLPAIN"), sample("DSOOF")),
		ShootPistol:         sample("DSPISTOL"),
		ShootShotgun:        firstSample(sample("DSSHOTGN"), sample("DSPISTOL")),
		ShootSuperShotgun:   firstSample(sample("DSDSHTGN"), sample("DSSHOTGN")),
		ShootPlasma:         firstSample(sample("DSPLASMA"), sample("DSFIRSHT")),
		ShootBFG:            firstSample(sample("DSBFG"), sample("DSRLAUNC")),
		Punch:               firstSample(sample("DSPUNCH"), sample("DSCLAW")),
		ShootFireball:       firstSample(sample("DSFIRSHT"), sample("DSPISTOL")),
		ShootRocket:         firstSample(sample("DSRLAUNC"), sample("DSSHOTGN")),
		SawUp:               firstSample(sample("DSSAWUP"), sample("DSWPNUP")),
		SawIdle:             firstSample(sample("DSSAWIDL"), sample("DSSAWUP")),
		SawFull:             firstSample(sample("DSSAWFUL"), sample("DSSAWIDL")),
		SawHit:              firstSample(sample("DSSAWHIT"), sample("DSSAWFUL")),
		ShotgunOpen:         firstSample(sample("DSDBOPN"), sample("DSSGCOCK")),
		ShotgunLoad:         firstSample(sample("DSDBLOAD"), sample("DSSGCOCK")),
		ShotgunClose:        firstSample(sample("DSDBCLS"), sample("DSSGCOCK")),
		AttackClaw:          firstSample(sample("DSCLAW"), sample("DSFIRSHT")),
		AttackSgt:           firstSample(sample("DSSGTATK"), sample("DSSHOTGN")),
		AttackSkull:         firstSample(sample("DSSKLATK"), sample("DSFIRSHT")),
		AttackArchvile:      firstSample(sample("DSVILATK"), sample("DSBAREXP")),
		AttackMancubus:      firstSample(sample("DSMANATK"), sample("DSFIRSHT")),
		ImpactFire:          firstSample(sample("DSFIRXPL"), sample("DSBAREXP")),
		ImpactRocket:        firstSample(firstSample(sample("DSRXPLOD"), sample("DSRXPLO")), sample("DSBAREXP")),
		BarrelExplode:       firstSample(sample("DSBAREXP"), sample("DSRXPLOD")),
		SeePosit1:           sample("DSPOSIT1"),
		SeePosit2:           sample("DSPOSIT2"),
		SeePosit3:           sample("DSPOSIT3"),
		SeeBGSit1:           sample("DSBGSIT1"),
		SeeBGSit2:           sample("DSBGSIT2"),
		SeeSgtSit:           sample("DSSGTSIT"),
		SeeCacoSit:          sample("DSCACSIT"),
		SeeBruiserSit:       sample("DSBRSSIT"),
		SeeKnightSit:        sample("DSKNTSIT"),
		SeeSpiderSit:        sample("DSPISIT"),
		SeeBabySit:          sample("DSBSPSIT"),
		SeeCyberSit:         sample("DSCYBSIT"),
		SeePainSit:          sample("DSPESIT"),
		SeeSSSit:            sample("DSSSSIT"),
		SeeVileSit:          sample("DSVILSIT"),
		SeeSkeSit:           sample("DSSKESIT"),
		ActivePosAct:        sample("DSPOSACT"),
		ActiveBGAct:         sample("DSBGACT"),
		ActiveDMAct:         sample("DSDMACT"),
		ActiveBSPAct:        sample("DSBSPACT"),
		ActiveVilAct:        sample("DSVILACT"),
		ActiveSkeAct:        sample("DSSKEACT"),
		MonsterPainHumanoid: firstSample(sample("DSPOPAIN"), firstSample(sample("DSPLPAIN"), sample("DSOOF"))),
		MonsterPainDemon:    firstSample(sample("DSDMPAIN"), firstSample(sample("DSPOPAIN"), sample("DSPLPAIN"))),
		DeathPodth1:         sample("DSPODTH1"),
		DeathPodth2:         sample("DSPODTH2"),
		DeathPodth3:         sample("DSPODTH3"),
		DeathBgdth1:         sample("DSBGDTH1"),
		DeathBgdth2:         sample("DSBGDTH2"),
		DeathSgtDth:         sample("DSSGTDTH"),
		DeathCacoRaw:        sample("DSCACDTH"),
		DeathBaronRaw:       sample("DSBRSDTH"),
		DeathKnightRaw:      sample("DSKNTDTH"),
		DeathCyberRaw:       sample("DSCYBDTH"),
		DeathSpiderRaw:      sample("DSSPIDTH"),
		DeathArachRaw:       sample("DSBSPDTH"),
		DeathLostSoulRaw:    sample("DSFIRXPL"),
		DeathMancubusRaw:    sample("DSMANDTH"),
		DeathRevenantRaw:    sample("DSSKEDTH"),
		DeathPainElemRaw:    sample("DSPEDTH"),
		DeathWolfSSRaw:      sample("DSSDTH"),
		DeathArchvileRaw:    sample("DSVILDTH"),
		DeathZombie:         firstSample(sample("DSPODTH1"), sample("DSBGDTH1")),
		DeathShotgunGuy:     firstSample(sample("DSPODTH2"), sample("DSPODTH1")),
		DeathChaingunner:    firstSample(sample("DSPODTH2"), sample("DSPODTH1")),
		DeathImp:            firstSample(sample("DSBGDTH1"), sample("DSSGTDTH")),
		DeathDemon:          firstSample(sample("DSSGTDTH"), sample("DSBGDTH1")),
		DeathCaco:           firstSample(sample("DSCACDTH"), sample("DSSGTDTH")),
		DeathBaron:          firstSample(sample("DSBRSDTH"), sample("DSSGTDTH")),
		DeathKnight:         firstSample(sample("DSKNTDTH"), sample("DSBRSDTH")),
		DeathCyber:          firstSample(sample("DSCYBDTH"), sample("DSBRSDTH")),
		DeathSpider:         firstSample(sample("DSSPIDTH"), sample("DSBRSDTH")),
		DeathArachnotron:    firstSample(sample("DSBSPDTH"), sample("DSSPIDTH")),
		DeathLostSoul:       firstSample(sample("DSFIRXPL"), sample("DSSGTDTH")),
		DeathMancubus:       firstSample(sample("DSMANDTH"), sample("DSBRSDTH")),
		DeathRevenant:       firstSample(sample("DSSKEDTH"), sample("DSSGTDTH")),
		DeathPainElemental:  firstSample(sample("DSPEDTH"), sample("DSCACDTH")),
		DeathWolfSS:         firstSample(sample("DSSDTH"), sample("DSPODTH1")),
		DeathArchvile:       firstSample(sample("DSVILDTH"), sample("DSBRSDTH")),
		MonsterDeath:        firstSample(firstSample(firstSample(sample("DSBGDTH1"), sample("DSSGTDTH")), sample("DSCACDTH")), sample("DSPODTH1")),
		PlayerDeath:         firstSample(sample("DSPLDETH"), sample("DSPLPAIN")),
		InterTick:           firstSample(sample("DSPISTOL"), sample("DSSWTCHN")),
		InterDone:           firstSample(sample("DSBAREXP"), sample("DSGETPOW")),
	}
	bank = audiofx.PrepareSoundBankForFaithful(bank, music.OutputSampleRate)
	if shouldPrepareSourcePortSoundBank(sourcePortMode) {
		bank = audiofx.PrepareSoundBankForSourcePort(bank, music.OutputSampleRate)
	}
	return bank
}

func sourcePortAudioEnabled(sourcePortMode bool) bool {
	return sourcePortMode && !isWASMBuild()
}

func shouldPrepareSourcePortSoundBank(sourcePortMode bool) bool {
	return sourcePortMode
}

func padDoomSoundSamples(src []byte) []byte {
	const doomSoundMixBlock = 512
	if len(src) == 0 {
		return nil
	}
	paddedLen := ((len(src) + (doomSoundMixBlock - 1)) / doomSoundMixBlock) * doomSoundMixBlock
	if paddedLen == len(src) {
		return src
	}
	out := make([]byte, paddedLen)
	copy(out, src)
	for i := len(src); i < len(out); i++ {
		out[i] = 128
	}
	return out
}

func firstSample(a, b media.PCMSample) media.PCMSample {
	if len(a.Data) > 0 {
		return a
	}
	return b
}

func buildBootSplashTexture(ts *doomtex.Set) media.WallTexture {
	if ts == nil {
		return media.WallTexture{}
	}
	// TITLEPIC is a raw 320x200 indexed image in stock Doom IWADs.
	if rgba, w, h, err := ts.BuildRawPicRGBA("TITLEPIC", 0, 320, 200); err == nil && len(rgba) == w*h*4 {
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		return media.WallTexture{
			RGBA:   rgba,
			RGBA32: rgba32,
			Width:  w,
			Height: h,
		}
	}
	// Fallback if a WAD stores TITLEPIC as a patch lump.
	if rgba, w, h, ox, oy, err := ts.BuildPatchRGBA("TITLEPIC", 0); err == nil && len(rgba) == w*h*4 {
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		return media.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	return media.WallTexture{}
}

func buildStatusPatchBank(ts *doomtex.Set) map[string]media.WallTexture {
	if ts == nil {
		return nil
	}
	names := make([]string, 0, 128)
	add := func(n string) {
		n = strings.ToUpper(strings.TrimSpace(n))
		if n == "" {
			return
		}
		names = append(names, n)
	}
	add("STBAR")
	add("STARMS")
	add("STTPRCNT")
	add("STFGOD0")
	add("STFDEAD0")
	for i := 0; i < 10; i++ {
		add(fmt.Sprintf("STTNUM%d", i))
		add(fmt.Sprintf("STYSNUM%d", i))
	}
	for i := 0; i < 3; i++ {
		add(fmt.Sprintf("STKEYS%d", i))
	}
	for i := 0; i < 6; i++ {
		add(fmt.Sprintf("STGNUM%d", i+2))
	}
	for pain := 0; pain < 5; pain++ {
		for straight := 0; straight < 3; straight++ {
			add(fmt.Sprintf("STFST%d%d", pain, straight))
		}
		add(fmt.Sprintf("STFTR%d0", pain))
		add(fmt.Sprintf("STFTL%d0", pain))
		add(fmt.Sprintf("STFOUCH%d", pain))
		add(fmt.Sprintf("STFEVL%d", pain))
		add(fmt.Sprintf("STFKILL%d", pain))
	}
	out := make(map[string]media.WallTexture, len(names))
	for _, name := range names {
		if _, ok := out[name]; ok {
			continue
		}
		indexed, opaque, iw, ih, _, _, ierr := ts.BuildPatchIndexedView(name)
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		mask := []byte(nil)
		if ierr == nil && iw == w && ih == h && len(indexed) == w*h && len(opaque) == w*h {
			mask = make([]byte, len(opaque))
			for i := range opaque {
				if opaque[i] {
					mask[i] = 1
				}
			}
		}
		out[name] = prepareOpaquePatchTexture(media.WallTexture{
			RGBA:       rgba,
			RGBA32:     rgba32,
			Indexed:    indexed,
			OpaqueMask: mask,
			Width:      w,
			Height:     h,
			OffsetX:    ox,
			OffsetY:    oy,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func prepareOpaquePatchTexture(tex media.WallTexture) media.WallTexture {
	if tex.Width <= 0 || tex.Height <= 0 {
		return tex
	}
	tex.EnsureOpaqueMask()
	tex.EnsureOpaqueColumnBounds()
	return tex
}

func buildMenuPatchBank(ts *doomtex.Set) map[string]media.WallTexture {
	if ts == nil {
		return nil
	}
	names := []string{
		"M_DOOM", "M_NGAME", "M_OPTION", "M_LOADG", "M_SAVEG", "M_RDTHIS", "M_QUITG",
		"M_SKULL1", "M_SKULL2",
		"M_PAUSE",
		"M_NEWG", "M_SKILL", "M_JKILL", "M_ROUGH", "M_HURT", "M_ULTRA", "M_NMARE",
		"M_EPISOD", "M_EPI1", "M_EPI2", "M_EPI3", "M_EPI4",
		"M_OPTTTL", "M_ENDGAM", "M_MESSG", "M_DETAIL", "M_SCRNSZ", "M_MSENS",
		"M_SVOL", "M_SFXVOL", "M_MUSVOL",
		"M_GDHIGH", "M_GDLOW", "M_MSGON", "M_MSGOFF",
		"M_THERML", "M_THERMM", "M_THERMR", "M_THERMO",
	}
	out := make(map[string]media.WallTexture, len(names))
	for _, name := range names {
		if _, ok := out[name]; ok {
			continue
		}
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		out[name] = prepareOpaquePatchTexture(media.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildMessageFontBank(ts *doomtex.Set) map[rune]media.WallTexture {
	if ts == nil {
		return nil
	}
	const (
		fontStart = 33 // '!'
		fontEnd   = 95 // '_'
	)
	out := make(map[rune]media.WallTexture, fontEnd-fontStart+1)
	for c := fontStart; c <= fontEnd; c++ {
		name := fmt.Sprintf("STCFN%03d", c)
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		out[rune(c)] = prepareOpaquePatchTexture(media.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildMonsterSpriteBank(ts *doomtex.Set) map[string]media.WallTexture {
	if ts == nil {
		return nil
	}
	spritePrefixes := []string{
		"POSS", "SPOS", "SSWV", "TROO", "SARG", "SKUL", "HEAD", "BOSS", "BOS2",
		"VILE", "CPOS", "SKEL", "FATT", "BSPI", "CYBR", "SPID", "PAIN",
	}
	frames := make([]byte, 0, 26)
	for fr := byte('A'); fr <= byte('Z'); fr++ {
		frames = append(frames, fr)
	}
	names := make([]string, 0, len(spritePrefixes)*len(frames)*8)
	seen := make(map[string]struct{}, cap(names))
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	addExpandedSeed := func(seed string) {
		if len(seed) < 6 {
			return
		}
		pfx := seed[:4]
		for fr := byte('A'); fr <= byte('Z'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
		}
	}
	for _, pfx := range spritePrefixes {
		for _, fr := range frames {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
			add(fmt.Sprintf("%s%c1%c5", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5%c1", pfx, fr, fr))
			add(fmt.Sprintf("%s%c2%c8", pfx, fr, fr))
			add(fmt.Sprintf("%s%c8%c2", pfx, fr, fr))
			add(fmt.Sprintf("%s%c3%c7", pfx, fr, fr))
			add(fmt.Sprintf("%s%c7%c3", pfx, fr, fr))
			add(fmt.Sprintf("%s%c4%c6", pfx, fr, fr))
			add(fmt.Sprintf("%s%c6%c4", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5", pfx, fr))
		}
	}
	// Some actor families also use non-rotating 0-suffixed frames for corpses or special map things.
	for _, name := range []string{
		"SKULF0", "SKULG0", "SKULH0", "SKULI0", "SKULJ0", "SKULK0",
		"SSWVE0", "SSWVF0", "SSWVG0", "SSWVH0", "SSWVI0", "SSWVJ0", "SSWVK0",
		"SSWVL0", "SSWVM0", "SSWVN0", "SSWVO0", "SSWVP0", "SSWVQ0", "SSWVR0",
		"SSWVS0", "SSWVT0", "SSWVU0", "SSWVV0",
	} {
		add(name)
	}
	// Projectile/effect prefixes used by the billboard renderer.
	for _, pfx := range []string{"MISL", "BAL1", "BAL2", "BAL7", "PLSS", "PLSE", "BFS1", "BFE1", "FATB", "MANF", "FBXP", "BOSF", "FIRE"} {
		for fr := byte('A'); fr <= byte('E'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
			add(fmt.Sprintf("%s%c1%c5", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5%c1", pfx, fr, fr))
		}
	}
	for fr := byte('E'); fr <= byte('F'); fr++ {
		add(fmt.Sprintf("BFE1%c0", fr))
		add(fmt.Sprintf("BFE1%c1", fr))
	}
	for fr := byte('E'); fr <= byte('H'); fr++ {
		add(fmt.Sprintf("FIRE%c0", fr))
		add(fmt.Sprintf("FIRE%c1", fr))
	}
	// First-person weapon psprites use rotation 0 only.
	for _, pfx := range []string{"PUNG", "PISG", "PISF", "SHTG", "SHTF", "SHT2", "CHGG", "CHGF", "MISG", "MISF", "SAWG", "PLSG", "PLSF", "BFGG", "BFGF"} {
		for fr := byte('A'); fr <= byte('Z'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
		}
	}
	// Common pickups, weapons, and decorations (A0 single-frame or animated 0-suffixed sets).
	for _, name := range []string{
		"PLAYN0", "POSSL0", "SPOSL0", "TROOL0", "SARGN0", "HEADL0", "SKULF0",
		"BBRNA0", "BBRNB0",
		"POL1A0", "POL2A0", "POL3A0", "POL4A0", "POL5A0", "POL6A0",
		"COL1A0", "COL2A0", "COL3A0", "COL4A0", "COL5A0", "COLUA0", "TRE1A0", "TRE2A0",
		"CANDA0", "CBRAA0", "CEYEA0", "FSKUA0", "FCANA0", "ELECA0",
		"GOR1A0", "GOR2A0", "GOR3A0", "GOR4A0", "GOR5A0",
		"SMITA0", "SMITB0", "SMITC0", "SMITD0",
		"KEENA0", "KEENB0", "KEENC0", "KEEND0",
		"BKEYA0", "YKEYA0", "RKEYA0",
		"BSKUA0", "YSKUA0", "RSKUA0",
		"STIMA0", "MEDIA0", "BON1A0", "BON2A0",
		"SOULA0", "SOULB0", "SOULC0", "SOULD0",
		"ARM1A0", "ARM2A0", "PINVA0", "PINVB0", "PINVC0", "PINVD0", "PSTRA0", "PINSA0", "PINSB0", "PINSC0", "PINSD0", "SUITA0", "PMAPA0", "PMAPB0", "PMAPC0", "PMAPD0", "PVISA0", "PVISB0", "MEGAA0", "MEGAB0", "MEGAC0", "MEGAD0",
		"CLIPA0", "AMMOA0", "SHELA0", "SBOXA0", "ROCKA0", "BROKA0", "CELLA0", "CELPA0", "BPAKA0",
		"SHOTA0", "SGN2A0", "MGUNA0", "LAUNA0", "PLASA0", "CSAWA0", "BFUGA0",
		"BAR1A0", "BAR1B0", "BAR1C0", "BEXPA0",
		"TBLUA0", "TBLUB0", "TBLUC0", "TBLUD0",
		"TGRNA0", "TGRNB0", "TGRNC0", "TGRND0",
		"TREDA0", "TREDB0", "TREDC0", "TREDD0",
		"SMRTA0", "SMRTB0", "SMRTC0", "SMRTD0",
		"SMGTA0", "SMGTB0", "SMGTC0", "SMGTD0",
		"SMBTA0", "SMBTB0", "SMBTC0", "SMBTD0",
		"TLMPA0", "TLP2A0",
		"PUFFA0", "PUFFB0", "PUFFC0", "PUFFD0",
		"BLUDA0", "BLUDB0", "BLUDC0",
		"TFOGA0", "TFOGB0", "TFOGC0", "TFOGD0", "TFOGE0", "TFOGF0", "TFOGG0", "TFOGH0", "TFOGI0", "TFOGJ0",
	} {
		add(name)
		addExpandedSeed(name)
	}
	out := make(map[string]media.WallTexture, len(names))
	for _, name := range names {
		if _, ok := out[name]; ok {
			continue
		}
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		out[name] = prepareOpaquePatchTexture(media.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildIntermissionPatchBank(ts *doomtex.Set) map[string]media.WallTexture {
	if ts == nil {
		return nil
	}
	names := make([]string, 0, 192)
	add := func(n string) {
		n = strings.ToUpper(strings.TrimSpace(n))
		if n == "" {
			return
		}
		names = append(names, n)
	}
	for i := 0; i < 32; i++ {
		add(fmt.Sprintf("CWILV%02d", i))
	}
	for ep := 0; ep < 4; ep++ {
		for m := 0; m < 9; m++ {
			add(fmt.Sprintf("WILV%d%d", ep, m))
		}
	}
	for i := 0; i < 3; i++ {
		add(fmt.Sprintf("WIMAP%d", i))
	}
	for i := 0; i < 10; i++ {
		add(fmt.Sprintf("WINUM%d", i))
	}
	for _, name := range []string{
		"WIMINUS", "WISUCKS", "WICOLON", "WISCRT2",
	} {
		add(name)
	}
	for j := 0; j < 10; j++ {
		for i := 0; i < 3; i++ {
			add(fmt.Sprintf("WIA0%02d%02d", j, i))
		}
	}
	for j := 0; j < 8; j++ {
		add(fmt.Sprintf("WIA1%02d00", j))
	}
	for i := 0; i < 3; i++ {
		add(fmt.Sprintf("WIA1%02d%02d", 7, i))
	}
	add("WIA10800")
	for j := 0; j < 6; j++ {
		for i := 0; i < 3; i++ {
			add(fmt.Sprintf("WIA2%02d%02d", j, i))
		}
	}
	for _, n := range []string{
		"WIF", "WIENTER", "WISPLAT", "WIURH0", "WIURH1",
		"WIOSTK", "WIOSTI", "WIOSTS", "WITIME", "WIPAR", "WIPCNT",
		"INTERPIC", "CREDIT", "VICTORY2", "ENDPIC", "HELP1", "HELP2",
	} {
		add(n)
	}
	out := make(map[string]media.WallTexture, len(names))
	for _, name := range names {
		if _, ok := out[name]; ok {
			continue
		}
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(name, 0)
		if err != nil || w <= 0 || h <= 0 || len(rgba) != w*h*4 {
			continue
		}
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		out[name] = prepareOpaquePatchTexture(media.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func availableEpisodes(wf *wad.File) []int {
	if wf == nil {
		return nil
	}
	names := mapdata.AvailableMapNames(wf)
	seen := make(map[int]struct{}, 4)
	out := make([]int, 0, 4)
	for _, name := range names {
		s := strings.ToUpper(strings.TrimSpace(string(name)))
		if len(s) == 4 && s[0] == 'E' && s[2] == 'M' && s[1] >= '1' && s[1] <= '9' && s[3] >= '1' && s[3] <= '9' {
			ep := int(s[1] - '0')
			if _, ok := seen[ep]; ok {
				continue
			}
			seen[ep] = struct{}{}
			out = append(out, ep)
		}
	}
	sort.Ints(out)
	return out
}

func resolveDemoStartMap(wf *wad.File, script *demo.Script, fallback mapdata.MapName) (mapdata.MapName, error) {
	if wf == nil || script == nil {
		return "", fmt.Errorf("missing demo")
	}
	candidates := make([]mapdata.MapName, 0, 2)
	if script.Header.Map > 0 {
		candidates = append(candidates, mapdata.MapName(fmt.Sprintf("MAP%02d", script.Header.Map)))
	}
	if script.Header.Episode > 0 && script.Header.Map > 0 && script.Header.Map <= 9 {
		candidates = append(candidates, mapdata.MapName(fmt.Sprintf("E%dM%d", script.Header.Episode, script.Header.Map)))
	}
	for _, candidate := range candidates {
		if _, err := mapdata.LoadMap(wf, candidate); err == nil {
			return candidate, nil
		}
	}
	if fallback != "" {
		return "", fmt.Errorf("demo map episode=%d map=%d not present in wad (requested map %s ignored)", script.Header.Episode, script.Header.Map, fallback)
	}
	return "", fmt.Errorf("demo map episode=%d map=%d not present in wad", script.Header.Episode, script.Header.Map)
}

func loadBuiltInDemos(wf *wad.File) []*demo.Script {
	if wf == nil {
		return nil
	}
	out := make([]*demo.Script, 0, 4)
	for _, name := range []string{"DEMO1", "DEMO2", "DEMO3", "DEMO4"} {
		lump, ok := wf.LumpByName(name)
		if !ok {
			continue
		}
		data, err := wf.LumpDataView(lump)
		if err != nil {
			continue
		}
		demo, err := demo.Parse(data)
		if err != nil {
			continue
		}
		demo.Path = name
		out = append(out, demo)
	}
	return out
}

func applyDemoPlaybackHeader(opts *runtimecfg.Options, script *demo.Script) {
	if opts == nil || script == nil {
		return
	}
	*opts = runtimecfg.PrepareDemoPlaybackOptions(*opts, script)
}
