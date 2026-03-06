package app

import (
	"crypto/sha1"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"unsafe"

	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/automap"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

func RunParse(args []string, stdout io.Writer, stderr io.Writer) int {
	configPath, configExplicit := resolveConfigPath(args)
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
	defaultMultiCore := true
	defaultWidth, defaultHeight := automap.DefaultCLIWindowSize()
	defaultZoom := 0.0
	defaultDetailLevel := -1
	defaultGammaLevel := -1
	defaultPlayer := 1
	defaultSkill := 3
	defaultGameMode := "single"
	defaultMouseLook := true
	defaultMouseLookSpeed := 2.0
	defaultKeyboardTurnSpeed := 1.0
	defaultMusicVolume := 1.0
	defaultMUSPanMax := 0.8
	defaultOPLVolume := 2.5
	defaultSFXVolume := 1.0
	defaultFastMonsters := false
	defaultAlwaysRun := true
	defaultAutoWeaponSwitch := true
	defaultCheatLevel := 0
	defaultInvuln := false
	defaultLineColorMode := "parity"
	defaultSourcePortMode := false
	defaultWalkRenderer := ""
	defaultSourcePortSectorLighting := true
	defaultDoomLighting := true
	defaultKageShader := false
	defaultGPUSky := false
	defaultCRTEffect := false
	defaultDepthBufferView := false
	defaultDepthOcclusion := false
	defaultWallOcclusion := true
	defaultWallSpanReject := true
	defaultWallSpanClip := false
	defaultWallSliceOcclusion := false
	defaultBillboardClipping := true
	defaultOverdrawDebug := false
	defaultTextureAnimCrossfadeFrames := 7 // Max effective value is 7 (Doom texture animation cadence is 8 tics).
	defaultAllCheats := false
	defaultStartInMap := false
	defaultImportPCSpeaker := true
	defaultImportTextures := true
	defaultCPUProfile := ""
	defaultDemo := ""
	defaultRecordDemo := ""
	defaultNoVsync := false
	defaultNoFPS := false
	defaultNoAspectCorrection := false
	defaultAniDump := ""
	defaultAniDumpDir := "anidump"
	defaultConfigPath := configPath
	configLineColorSet := false
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
		if cfg.MultiCore != nil {
			defaultMultiCore = *cfg.MultiCore
		}
		if cfg.Width != nil {
			defaultWidth = *cfg.Width
		}
		if cfg.Height != nil {
			defaultHeight = *cfg.Height
		}
		if cfg.Zoom != nil {
			defaultZoom = *cfg.Zoom
		}
		if cfg.DetailLevel != nil {
			defaultDetailLevel = *cfg.DetailLevel
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
		if cfg.GameMode != nil {
			defaultGameMode = *cfg.GameMode
		}
		if cfg.MouseLook != nil {
			defaultMouseLook = *cfg.MouseLook
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
		if cfg.OPLVolume != nil {
			defaultOPLVolume = *cfg.OPLVolume
		}
		if cfg.SFXVolume != nil {
			defaultSFXVolume = *cfg.SFXVolume
		}
		if cfg.FastMonsters != nil {
			defaultFastMonsters = *cfg.FastMonsters
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
		if cfg.LineColorMode != nil {
			defaultLineColorMode = *cfg.LineColorMode
			configLineColorSet = true
		}
		if cfg.SourcePortMode != nil {
			defaultSourcePortMode = *cfg.SourcePortMode
		}
		if cfg.WalkRenderer != nil {
			defaultWalkRenderer = *cfg.WalkRenderer
		}
		if cfg.SourcePortSectorLighting != nil {
			defaultSourcePortSectorLighting = *cfg.SourcePortSectorLighting
		}
		if cfg.DoomLighting != nil {
			defaultDoomLighting = *cfg.DoomLighting
		}
		if cfg.KageShader != nil {
			defaultKageShader = *cfg.KageShader
		}
		if cfg.GPUSky != nil {
			defaultGPUSky = *cfg.GPUSky
		}
		if cfg.CRTEffect != nil {
			defaultCRTEffect = *cfg.CRTEffect
		}
		if cfg.DepthBufferView != nil {
			defaultDepthBufferView = *cfg.DepthBufferView
		}
		if cfg.DepthOcclusion != nil {
			defaultDepthOcclusion = *cfg.DepthOcclusion
		}
		if cfg.WallOcclusion != nil {
			defaultWallOcclusion = *cfg.WallOcclusion
		}
		if cfg.WallSpanReject != nil {
			defaultWallSpanReject = *cfg.WallSpanReject
		}
		if cfg.WallSpanClip != nil {
			defaultWallSpanClip = *cfg.WallSpanClip
		}
		if cfg.WallSliceOcclusion != nil {
			defaultWallSliceOcclusion = *cfg.WallSliceOcclusion
		}
		if cfg.BillboardClipping != nil {
			defaultBillboardClipping = *cfg.BillboardClipping
		}
		if cfg.OverdrawDebug != nil {
			defaultOverdrawDebug = *cfg.OverdrawDebug
		}
		if cfg.TextureAnimCrossfadeFrames != nil {
			defaultTextureAnimCrossfadeFrames = *cfg.TextureAnimCrossfadeFrames
		}
		if cfg.AllCheats != nil {
			defaultAllCheats = *cfg.AllCheats
		}
		if cfg.StartInMap != nil {
			defaultStartInMap = *cfg.StartInMap
		}
		if cfg.ImportPCSpeaker != nil {
			defaultImportPCSpeaker = *cfg.ImportPCSpeaker
		}
		if cfg.ImportTextures != nil {
			defaultImportTextures = *cfg.ImportTextures
		}
		if cfg.CPUProfile != nil {
			defaultCPUProfile = *cfg.CPUProfile
		}
		if cfg.Demo != nil {
			defaultDemo = *cfg.Demo
		}
		if cfg.RecordDemo != nil {
			defaultRecordDemo = *cfg.RecordDemo
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
	}

	fs := flag.NewFlagSet("gddoom", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configFlag := fs.String("config", defaultConfigPath, "path to config toml file (default: config.toml)")
	wadPath := fs.String("wad", defaultWAD, "path to IWAD file")
	mapName := fs.String("map", defaultMap, "map name (E#M# or MAP##); empty selects first valid map")
	details := fs.Bool("details", defaultDetails, "print decoded gameplay-relevant map details")
	render := fs.Bool("render", defaultRender, "launch Ebiten automap renderer")
	debug := fs.Bool("debug", defaultDebug, "enable renderer debug HUD text/stats and debug logging")
	multiCore := fs.Bool("multi-core", defaultMultiCore, "use multiple CPU cores (GOMAXPROCS=NumCPU when true, 1 when false)")
	width := fs.Int("width", defaultWidth, "render window width")
	height := fs.Int("height", defaultHeight, "render window height")
	zoom := fs.Float64("zoom", defaultZoom, "starting zoom (>0 overrides Doom-style startup zoom)")
	detailLevel := fs.Int("detail-level", defaultDetailLevel, "startup detail level (-1 keeps mode default)")
	gammaLevel := fs.Int("gamma-level", defaultGammaLevel, "startup gamma level (-1 keeps mode default)")
	playerSlot := fs.Int("player", defaultPlayer, "player start slot (1-4)")
	skillLevel := fs.Int("skill", defaultSkill, "doom skill level (1-5)")
	gameMode := fs.String("game-mode", defaultGameMode, "thing spawn game mode (single|coop|deathmatch)")
	mouseLook := fs.Bool("mouselook", defaultMouseLook, "enable mouse-based turning in walk mode")
	mouseLookSpeed := fs.Float64("mouselook-speed", defaultMouseLookSpeed, "mouse turn speed multiplier (>0)")
	keyboardTurnSpeed := fs.Float64("keyboard-turn-speed", defaultKeyboardTurnSpeed, "keyboard turn speed multiplier (>0)")
	musicVolume := fs.Float64("music-volume", defaultMusicVolume, "music output volume (0..1)")
	musPanMax := fs.Float64("mus-pan-max", defaultMUSPanMax, "maximum MUS pan amount (0..1; 0 centers all pan, 1 keeps full range)")
	oplVolume := fs.Float64("opl-volume", defaultOPLVolume, "OPL synth output gain (0..4; default 2.0)")
	sfxVolume := fs.Float64("sfx-volume", defaultSFXVolume, "sound-effect output volume (0..1)")
	fastMonsters := fs.Bool("fastmonsters", defaultFastMonsters, "enable fast monsters (-fast style)")
	alwaysRun := fs.Bool("always-run", defaultAlwaysRun, "start with always-run enabled (Shift inverts while held)")
	autoWeaponSwitch := fs.Bool("auto-weapon-switch", defaultAutoWeaponSwitch, "auto-switch to newly picked weapons")
	cheatLevel := fs.Int("cheat-level", defaultCheatLevel, "startup cheats (0=off, 1=automap, 2=idfa-like, 3=idkfa+invuln)")
	invuln := fs.Bool("invuln", defaultInvuln, "start with invulnerability (iddqd-like)")
	lineColorMode := fs.String("line-color-mode", defaultLineColorMode, "line color mode for automap")
	sourcePortMode := fs.Bool("sourceport-mode", defaultSourcePortMode, "enable source-port style heading-follow rotation defaults")
	walkRenderer := fs.String("walk-renderer", defaultWalkRenderer, "startup walk renderer (doom-basic|unified-bsp|wireframe)")
	sourcePortSectorLighting := fs.Bool("sourceport-sector-lighting", defaultSourcePortSectorLighting, "show classic sector lighting while in sourceport mode")
	doomLighting := fs.Bool("doom-lighting", defaultDoomLighting, "enable Doom lighting math/colormap shading")
	kageShader := fs.Bool("kage-shader", defaultKageShader, "enable Kage postprocess shaders (palette/gamma/crt)")
	gpuSky := fs.Bool("gpu-sky", defaultGPUSky, "enable experimental GPU sky path in sourceport mode (default off)")
	crtEffect := fs.Bool("crt-effect", defaultCRTEffect, "enable CRT postprocess effect")
	depthBufferView := fs.Bool("depth-buffer-view", defaultDepthBufferView, "replace 3D viewport with grayscale depth-buffer visualization")
	depthOcclusion := fs.Bool("depth-occlusion", defaultDepthOcclusion, "enable software depth occlusion buffer for sprites/planes")
	wallOcclusion := fs.Bool("wall-occlusion", defaultWallOcclusion, "enable coarse wall-span occlusion for wall traversal")
	wallSpanReject := fs.Bool("wall-span-reject", defaultWallSpanReject, "enable early solid-span wall rejection")
	wallSpanClip := fs.Bool("wall-span-clip", defaultWallSpanClip, "clip solid wall x-ranges against coarse wall spans")
	wallSliceOcclusion := fs.Bool("wall-slice-occlusion", defaultWallSliceOcclusion, "enable wall-slice triangle/bbox occlusion checks")
	billboardClipping := fs.Bool("billboard-clipping", defaultBillboardClipping, "enable sprite/thing/projectile/puff clipping and occlusion")
	noCullClipping := fs.Bool("no-cull-clipping", false, "disable wall occlusion, depth occlusion, and billboard clipping together")
	overdrawDebug := fs.Bool("overdraw-debug", defaultOverdrawDebug, "debug: paint repeated 3D software-buffer writes red")
	textureAnimCrossfadeFrames := fs.Int("texture-anim-crossfade-frames", defaultTextureAnimCrossfadeFrames, "sourceport texture animation crossfade frames (0 disables)")
	allCheats := fs.Bool("all-cheats", defaultAllCheats, "legacy alias for startup full cheats (equivalent to -cheat-level=3 -invuln=true)")
	startInMap := fs.Bool("start-in-map", defaultStartInMap, "start with automap open")
	importPCSpeaker := fs.Bool("import-pcspeaker", defaultImportPCSpeaker, "import Doom PC speaker sounds (DP* lumps) at startup")
	importTextures := fs.Bool("import-textures", defaultImportTextures, "parse Doom texture data and build wall textures for doom-basic 3D renderer")
	cpuProfile := fs.String("cpuprofile", defaultCPUProfile, "write Go CPU profile to file")
	demoPath := fs.String("demo", defaultDemo, "path to gddoom-demo-v1 script; runs scripted benchmark and exits when demo ends")
	recordDemoPath := fs.String("record-demo", defaultRecordDemo, "path to write gddoom-demo-v1 script recorded from live input")
	noVsync := fs.Bool("no-vsync", defaultNoVsync, "disable vsync and uncap draw FPS")
	noFPS := fs.Bool("nofps", defaultNoFPS, "hide FPS/MS overlay")
	noAspectCorrection := fs.Bool("no-aspect-correction", defaultNoAspectCorrection, "disable Doom-style 4:3 aspect correction")
	aniDump := fs.String("anidump", defaultAniDump, "dump animation sprite series for seed (example: SMGTA0)")
	aniDumpDir := fs.String("anidump-dir", defaultAniDumpDir, "output directory for -anidump PNG dumps")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "flag error: %v\n", err)
		return 2
	}
	_ = configFlag
	lineColorModeSet := configLineColorSet
	allCheatsSet := false
	cheatLevelSet := false
	invulnSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "line-color-mode" {
			lineColorModeSet = true
		}
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
	resolvedCheatLevel := *cheatLevel
	resolvedInvuln := *invuln
	resolvedGameMode := strings.ToLower(strings.TrimSpace(*gameMode))
	switch resolvedGameMode {
	case "single", "coop", "deathmatch":
	default:
		fmt.Fprintf(stderr, "invalid -game-mode %q (want single|coop|deathmatch)\n", *gameMode)
		return 2
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
	switch strings.ToLower(strings.TrimSpace(*walkRenderer)) {
	case "", "doom-basic", "doom_basic", "basic", "unified-bsp", "unified_bsp", "unified", "wireframe", "pseudo":
	default:
		fmt.Fprintf(stderr, "invalid -walk-renderer %q (want doom-basic|unified-bsp|wireframe)\n", *walkRenderer)
		return 2
	}
	if *oplVolume < 0 || *oplVolume > music.MaxOutputGain {
		fmt.Fprintf(stderr, "invalid -opl-volume %.3f (must be between 0 and %.1f)\n", *oplVolume, music.MaxOutputGain)
		return 2
	}
	if *sfxVolume < 0 || *sfxVolume > 1 {
		fmt.Fprintf(stderr, "invalid -sfx-volume %.3f (must be between 0 and 1)\n", *sfxVolume)
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
	if resolvedDemoPath != "" && resolvedRecordDemoPath != "" {
		fmt.Fprintln(stderr, "-demo and -record-demo are mutually exclusive")
		return 2
	}
	if strings.TrimSpace(*wadPath) == "" {
		fmt.Fprintln(stderr, "-wad is required")
		return 2
	}
	if *multiCore {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(1)
	}

	resolvedWADPath := resolveIWADAliasPath(*wadPath)
	wf, err := wad.Open(resolvedWADPath)
	if err != nil {
		fmt.Fprintf(stderr, "open wad: %v\n", err)
		return 1
	}
	wadHash := hashFileSHA1(resolvedWADPath)
	soundBank := automap.SoundBank{}
	dsr := sound.ImportDigitalSounds(wf)
	var musicPatchBank music.PatchBank
	if genmidiLump, ok := wf.LumpByName("GENMIDI"); ok {
		if genmidiData, gerr := wf.LumpData(genmidiLump); gerr != nil {
			fmt.Fprintf(stderr, "music patch import warning: read GENMIDI: %v\n", gerr)
		} else if bank, gerr := music.ParseGENMIDIOP2PatchBank(genmidiData); gerr != nil {
			fmt.Fprintf(stderr, "music patch import warning: parse GENMIDI: %v\n", gerr)
		} else {
			musicPatchBank = bank
			fmt.Fprintf(stderr, "music patch import: source=GENMIDI instruments=%d\n", 128+47)
		}
	} else if genmidiData, gerr := os.ReadFile("GENMIDI.op2"); gerr == nil {
		if bank, perr := music.ParseGENMIDIOP2PatchBank(genmidiData); perr == nil {
			musicPatchBank = bank
			fmt.Fprintf(stderr, "music patch import: source=GENMIDI.op2 instruments=%d\n", 128+47)
		}
	}
	if *importPCSpeaker {
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
	soundBank = buildAutomapSoundBank(dsr)
	wallTexBank := map[string]automap.WallTexture(nil)
	bootSplash := automap.WallTexture{}
	doomPaletteRGBA := []byte(nil)
	doomColorMap := []byte(nil)
	doomColorMapRows := 0
	statusPatchBank := map[string]automap.WallTexture(nil)
	messageFontBank := map[rune]automap.WallTexture(nil)
	spritePatchBank := map[string]automap.WallTexture(nil)
	intermissionPatchBank := map[string]automap.WallTexture(nil)
	var texSet *doomtex.Set
	if pal, perr := doomtex.LoadPaletteRGBA(wf, 0); perr != nil {
		fmt.Fprintf(stderr, "palette import failed: %v\n", perr)
	} else {
		doomPaletteRGBA = pal
	}
	if cmLump, ok := wf.LumpByName("COLORMAP"); ok {
		if cmData, err := wf.LumpData(cmLump); err == nil && len(cmData) >= 256 {
			doomColorMapRows = len(cmData) / 256
			doomColorMap = cmData[:doomColorMapRows*256]
		}
	}
	if *importTextures {
		ts, terr := doomtex.LoadFromWAD(wf)
		if terr != nil {
			fmt.Fprintf(stderr, "texture import failed: %v\n", terr)
		} else {
			texSet = ts
			fmt.Fprintf(stderr, "texture import: palettes=%d textures=%d\n", ts.PaletteCount(), ts.TextureCount())
			names := ts.TextureNames()
			wallTexBank = make(map[string]automap.WallTexture, len(names))
			built := 0
			failed := 0
			for _, name := range names {
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
				wallTexBank[name] = automap.WallTexture{
					RGBA:     rgba,
					RGBA32:   rgba32,
					ColMajor: colMajor,
					Width:    w,
					Height:   h,
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
		messageFontBank = buildMessageFontBank(texSet)
		if len(messageFontBank) > 0 {
			fmt.Fprintf(stderr, "message font import: glyphs=%d\n", len(messageFontBank))
		}
		spritePatchBank = buildMonsterSpriteBank(texSet)
		if len(spritePatchBank) > 0 {
			fmt.Fprintf(stderr, "monster sprite import: patches=%d\n", len(spritePatchBank))
		}
		if strings.TrimSpace(*aniDump) != "" {
			if derr := dumpSpriteAnimationSeries(*aniDumpDir, *aniDump, spritePatchBank); derr != nil {
				fmt.Fprintf(stderr, "anidump error: %v\n", derr)
				return 1
			}
			fmt.Fprintf(stderr, "anidump: seed=%s dir=%s\n", strings.ToUpper(strings.TrimSpace(*aniDump)), strings.TrimSpace(*aniDumpDir))
		}
		intermissionPatchBank = buildIntermissionPatchBank(texSet)
		if len(intermissionPatchBank) > 0 {
			fmt.Fprintf(stderr, "intermission patch import: patches=%d\n", len(intermissionPatchBank))
		}
	}
	flatBank := map[string][]byte(nil)
	fb, ferr := doomtex.LoadFlatsRGBA(wf, 0)
	if ferr != nil {
		fmt.Fprintf(stderr, "flat import failed: %v\n", ferr)
	} else {
		flatBank = fb
		fmt.Fprintf(stderr, "flat import: flats=%d\n", len(flatBank))
	}

	selected := mapdata.MapName(strings.ToUpper(strings.TrimSpace(*mapName)))
	if selected == "" {
		selected, err = mapdata.FirstMapName(wf)
		if err != nil {
			fmt.Fprintf(stderr, "resolve first map: %v\n", err)
			return 1
		}
	}

	if *render {
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

		resolvedLineColorMode := *lineColorMode
		// Source-port defaults unless user explicitly chose a color mode.
		if *sourcePortMode && !lineColorModeSet {
			resolvedLineColorMode = "doom"
		}
		if *noCullClipping {
			*depthOcclusion = false
			*wallOcclusion = false
			*wallSpanReject = false
			*wallSpanClip = false
			*wallSliceOcclusion = false
			*billboardClipping = false
		}
		opts := automap.Options{
			Width:                      *width,
			Height:                     *height,
			StartZoom:                  *zoom,
			InitialDetailLevel:         *detailLevel,
			InitialGammaLevel:          *gammaLevel,
			WADHash:                    wadHash,
			Debug:                      *debug,
			PlayerSlot:                 *playerSlot,
			SkillLevel:                 *skillLevel,
			GameMode:                   resolvedGameMode,
			MouseLook:                  *mouseLook,
			MouseLookSpeed:             *mouseLookSpeed,
			KeyboardTurnSpeed:          *keyboardTurnSpeed,
			MusicVolume:                *musicVolume,
			MUSPanMax:                  *musPanMax,
			OPLVolume:                  *oplVolume,
			SFXVolume:                  *sfxVolume,
			FastMonsters:               *fastMonsters,
			AlwaysRun:                  *alwaysRun,
			AutoWeaponSwitch:           *autoWeaponSwitch,
			CheatLevel:                 resolvedCheatLevel,
			Invulnerable:               resolvedInvuln,
			LineColorMode:              resolvedLineColorMode,
			SourcePortMode:             *sourcePortMode,
			InitialWalkRenderer:        *walkRenderer,
			SourcePortSectorLighting:   *sourcePortSectorLighting,
			DisableDoomLighting:        !*doomLighting,
			KageShader:                 *kageShader,
			GPUSky:                     *gpuSky,
			CRTEffect:                  *crtEffect,
			DepthBufferView:            *depthBufferView,
			DisableDepthOcclusion:      !*depthOcclusion,
			DisableWallOcclusion:       !*wallOcclusion,
			DisableWallSpanReject:      !*wallSpanReject,
			DisableWallSpanClip:        !*wallSpanClip,
			DisableWallSliceOcclusion:  !*wallSliceOcclusion,
			DisableBillboardClipping:   !*billboardClipping,
			OverdrawDebug:              *overdrawDebug,
			TextureAnimCrossfadeFrames: *textureAnimCrossfadeFrames,
			NoVsync:                    *noVsync,
			NoFPS:                      *noFPS,
			DisableAspectCorrection:    *noAspectCorrection,
			AllCheats:                  *allCheats,
			StartInMapMode:             *startInMap,
			FlatBank:                   flatBank,
			WallTexBank:                wallTexBank,
			BootSplash:                 bootSplash,
			DoomPaletteRGBA:            doomPaletteRGBA,
			DoomColorMap:               doomColorMap,
			DoomColorMapRows:           doomColorMapRows,
			StatusPatchBank:            statusPatchBank,
			MessageFontBank:            messageFontBank,
			SpritePatchBank:            spritePatchBank,
			IntermissionPatchBank:      intermissionPatchBank,
			SoundBank:                  soundBank,
			MusicPatchBank:             musicPatchBank,
			RecordDemoPath:             resolvedRecordDemoPath,
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
			data, err := wf.LumpData(l)
			if err != nil {
				return nil, err
			}
			// Validate once at load time so runtime playback can assume decodable MUS.
			if _, err := music.ParseMUS(data); err != nil {
				return nil, err
			}
			return data, nil
		}
		if strings.TrimSpace(configPath) != "" {
			path := configPath
			opts.OnRuntimeSettingsChanged = func(s automap.RuntimeSettings) {
				if err := saveRuntimeSettings(path, s); err != nil {
					fmt.Fprintf(stderr, "config save warning: %v\n", err)
				}
			}
		}
		if p := resolvedDemoPath; p != "" {
			demo, derr := automap.LoadDemoScript(p)
			if derr != nil {
				fmt.Fprintf(stderr, "load demo: %v\n", derr)
				return 1
			}
			opts.DemoScript = demo
			fmt.Fprintf(stderr, "demo loaded: %s tics=%d\n", p, len(demo.Tics))
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
		rerr := automap.RunAutomap(m, opts, nextMap)
		if rerr != nil {
			fmt.Fprintf(stderr, "render map %s: %v\n", selected, rerr)
			return 1
		}
		return 0
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

func hashFileSHA1(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func mapMusicLumpName(name mapdata.MapName) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(string(name)))
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

func resolveIWADAliasPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return path
	}
	if resolved, ok := resolvePathCaseInsensitive(trimmed); ok {
		return resolved
	}
	base := strings.ToUpper(filepath.Base(trimmed))
	if base != "DOOM1.WAD" {
		return path
	}
	dir := filepath.Dir(trimmed)
	if alias, ok := resolvePathCaseInsensitive(filepath.Join(dir, "DOOM.WAD")); ok {
		return alias
	}
	if alias, ok := resolvePathCaseInsensitive(filepath.Join(dir, "DOOM2.WAD")); ok {
		return alias
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

func dumpSpriteAnimationSeries(outDir, seed string, bank map[string]automap.WallTexture) error {
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

func spriteSeriesFromSeed(seed string, bank map[string]automap.WallTexture) []string {
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

func buildAutomapSoundBank(r sound.DigitalImportReport) automap.SoundBank {
	byName := make(map[string]sound.DigitalSound, len(r.Sounds))
	for _, s := range r.Sounds {
		byName[s.Name] = s
	}
	sample := func(name string) automap.PCMSample {
		s, ok := byName[name]
		if !ok {
			return automap.PCMSample{}
		}
		return automap.PCMSample{
			SampleRate: int(s.SampleRate),
			Data:       s.Samples,
		}
	}
	return automap.SoundBank{
		DoorOpen:            firstSample(sample("DSDOROPN"), sample("DSBDOPN")),
		DoorClose:           firstSample(sample("DSDORCLS"), sample("DSBDCLS")),
		BlazeOpen:           sample("DSBDOPN"),
		BlazeClose:          sample("DSBDCLS"),
		SwitchOn:            sample("DSSWTCHN"),
		SwitchOff:           sample("DSSWTCHX"),
		NoWay:               firstSample(sample("DSNOWAY"), sample("DSOOF")),
		ItemUp:              sample("DSITEMUP"),
		WeaponUp:            sample("DSWPNUP"),
		PowerUp:             sample("DSGETPOW"),
		Oof:                 sample("DSOOF"),
		Pain:                firstSample(sample("DSPLPAIN"), sample("DSOOF")),
		ShootPistol:         sample("DSPISTOL"),
		ShootShotgun:        firstSample(sample("DSSHOTGN"), sample("DSPISTOL")),
		ShootFireball:       firstSample(sample("DSFIRSHT"), sample("DSPISTOL")),
		ShootRocket:         firstSample(sample("DSRLAUNC"), sample("DSSHOTGN")),
		ImpactFire:          firstSample(sample("DSFIRXPL"), sample("DSBAREXP")),
		ImpactRocket:        firstSample(firstSample(sample("DSRXPLOD"), sample("DSRXPLO")), sample("DSBAREXP")),
		MonsterPainHumanoid: firstSample(sample("DSPOPAIN"), firstSample(sample("DSPLPAIN"), sample("DSOOF"))),
		MonsterPainDemon:    firstSample(sample("DSDMPAIN"), firstSample(sample("DSPOPAIN"), sample("DSPLPAIN"))),
		DeathZombie:         firstSample(sample("DSPODTH1"), sample("DSBGDTH1")),
		DeathShotgunGuy:     firstSample(sample("DSPODTH2"), sample("DSPODTH1")),
		DeathImp:            firstSample(sample("DSBGDTH1"), sample("DSSGTDTH")),
		DeathDemon:          firstSample(sample("DSSGTDTH"), sample("DSBGDTH1")),
		DeathCaco:           firstSample(sample("DSCACDTH"), sample("DSSGTDTH")),
		DeathBaron:          firstSample(sample("DSBRSDTH"), sample("DSSGTDTH")),
		DeathCyber:          firstSample(sample("DSCYBDTH"), sample("DSBRSDTH")),
		DeathSpider:         firstSample(sample("DSSPIDTH"), sample("DSBRSDTH")),
		DeathLostSoul:       firstSample(sample("DSFIRXPL"), sample("DSSGTDTH")),
		MonsterDeath:        firstSample(firstSample(firstSample(sample("DSBGDTH1"), sample("DSSGTDTH")), sample("DSCACDTH")), sample("DSPODTH1")),
		PlayerDeath:         firstSample(sample("DSPLDETH"), sample("DSPLPAIN")),
		InterTick:           firstSample(sample("DSPISTOL"), sample("DSSWTCHN")),
		InterDone:           firstSample(sample("DSBAREXP"), sample("DSGETPOW")),
	}
}

func firstSample(a, b automap.PCMSample) automap.PCMSample {
	if len(a.Data) > 0 {
		return a
	}
	return b
}

func buildBootSplashTexture(ts *doomtex.Set) automap.WallTexture {
	if ts == nil {
		return automap.WallTexture{}
	}
	// TITLEPIC is a raw 320x200 indexed image in stock Doom IWADs.
	if rgba, w, h, err := ts.BuildRawPicRGBA("TITLEPIC", 0, 320, 200); err == nil && len(rgba) == w*h*4 {
		rgba32 := []uint32(nil)
		if len(rgba) >= 4 {
			rgba32 = unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(rgba))), len(rgba)/4)
		}
		return automap.WallTexture{
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
		return automap.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	return automap.WallTexture{}
}

func buildStatusPatchBank(ts *doomtex.Set) map[string]automap.WallTexture {
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
	out := make(map[string]automap.WallTexture, len(names))
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
		out[name] = automap.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildMessageFontBank(ts *doomtex.Set) map[rune]automap.WallTexture {
	if ts == nil {
		return nil
	}
	const (
		fontStart = 33 // '!'
		fontEnd   = 95 // '_'
	)
	out := make(map[rune]automap.WallTexture, fontEnd-fontStart+1)
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
		out[rune(c)] = automap.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildMonsterSpriteBank(ts *doomtex.Set) map[string]automap.WallTexture {
	if ts == nil {
		return nil
	}
	spritePrefixes := []string{
		"POSS", "SPOS", "TROO", "SARG", "SKUL", "HEAD", "BOSS", "CYBR", "SPID",
	}
	frames := make([]byte, 0, 26)
	for fr := byte('A'); fr <= byte('Z'); fr++ {
		frames = append(frames, fr)
	}
	names := make([]string, 0, len(spritePrefixes)*len(frames)*8)
	add := func(name string) {
		for _, ex := range names {
			if ex == name {
				return
			}
		}
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
			add(fmt.Sprintf("%s%c1", pfx, fr))
			add(fmt.Sprintf("%s%c2%c8", pfx, fr, fr))
			add(fmt.Sprintf("%s%c8%c2", pfx, fr, fr))
			add(fmt.Sprintf("%s%c3%c7", pfx, fr, fr))
			add(fmt.Sprintf("%s%c7%c3", pfx, fr, fr))
			add(fmt.Sprintf("%s%c4%c6", pfx, fr, fr))
			add(fmt.Sprintf("%s%c6%c4", pfx, fr, fr))
			add(fmt.Sprintf("%s%c5", pfx, fr))
		}
	}
	// Projectile prefixes (flight frames are usually A/B or A-D in Doom).
	for _, pfx := range []string{"MISL", "BAL1", "BAL2", "BAL7", "PLSS"} {
		for fr := byte('A'); fr <= byte('D'); fr++ {
			add(fmt.Sprintf("%s%c0", pfx, fr))
			add(fmt.Sprintf("%s%c1", pfx, fr))
		}
	}
	// Common pickups, weapons, and decorations (A0 single-frame or animated 0-suffixed sets).
	for _, name := range []string{
		"PLAYN0", "POSSL0", "SPOSL0", "TROOL0", "SARGN0", "HEADL0", "SKULL0",
		"POL1A0", "POL2A0", "POL3A0", "POL4A0", "POL5A0", "POL6A0",
		"COL1A0", "COL2A0", "COL3A0", "COL4A0", "COL5A0", "TRE1A0", "TRE2A0",
		"CANDA0", "CBRAA0", "CEYEA0", "FSKUA0", "FCANA0", "ELECA0",
		"GOR1A0", "GOR2A0", "GOR3A0", "GOR4A0", "GOR5A0",
		"SMITA0", "SMITB0", "SMITC0", "SMITD0",
		"KEENA0", "KEENB0", "KEENC0", "KEEND0",
		"BKEYA0", "YKEYA0", "RKEYA0",
		"BSKUA0", "YSKUA0", "RSKUA0",
		"STIMA0", "MEDIA0", "BON1A0", "BON2A0",
		"ARM1A0", "ARM2A0", "SUITA0",
		"CLIPA0", "AMMOA0", "SHELA0", "SBOXA0", "ROCKA0", "BROKA0", "CELLA0", "CELPA0", "BPAKA0",
		"SHOTA0", "MGUNA0", "LAUNA0", "PLASA0", "CSAWA0", "BFUGA0",
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
	} {
		add(name)
		addExpandedSeed(name)
	}
	out := make(map[string]automap.WallTexture, len(names))
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
		out[name] = automap.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildIntermissionPatchBank(ts *doomtex.Set) map[string]automap.WallTexture {
	if ts == nil {
		return nil
	}
	names := make([]string, 0, 96)
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
	for i := 0; i < 3; i++ {
		add(fmt.Sprintf("WIMAP%d", i))
	}
	for _, n := range []string{
		"WIF", "WIENTER", "WISPLAT", "WIURH0", "WIURH1",
		"WIOSTK", "WIOSTI", "WIOSTS", "WITIME", "WIPAR", "WIPCNT",
		"INTERPIC", "CREDIT", "VICTORY2", "ENDPIC", "HELP1", "HELP2",
	} {
		add(n)
	}
	out := make(map[string]automap.WallTexture, len(names))
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
		out[name] = automap.WallTexture{
			RGBA:    rgba,
			RGBA32:  rgba32,
			Width:   w,
			Height:  h,
			OffsetX: ox,
			OffsetY: oy,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
