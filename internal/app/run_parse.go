package app

import (
	"crypto/sha1"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"unsafe"

	"gddoom/internal/mapdata"
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
	defaultWidth := 2560
	defaultHeight := 1440
	defaultZoom := 0.0
	defaultPlayer := 1
	defaultSkill := 3
	defaultCheatLevel := 0
	defaultInvuln := false
	defaultLineColorMode := "parity"
	defaultSourcePortMode := false
	defaultAllCheats := false
	defaultStartInMap := false
	defaultImportPCSpeaker := true
	defaultImportTextures := true
	defaultCPUProfile := ""
	defaultDemo := ""
	defaultRecordDemo := ""
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
		if cfg.Player != nil {
			defaultPlayer = *cfg.Player
		}
		if cfg.Skill != nil {
			defaultSkill = *cfg.Skill
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
	playerSlot := fs.Int("player", defaultPlayer, "player start slot (1-4)")
	skillLevel := fs.Int("skill", defaultSkill, "doom skill level (1-5)")
	cheatLevel := fs.Int("cheat-level", defaultCheatLevel, "startup cheats (0=off, 1=automap, 2=idfa-like, 3=idkfa+invuln)")
	invuln := fs.Bool("invuln", defaultInvuln, "start with invulnerability (iddqd-like)")
	lineColorMode := fs.String("line-color-mode", defaultLineColorMode, "line color mode for automap")
	sourcePortMode := fs.Bool("sourceport-mode", defaultSourcePortMode, "enable source-port style heading-follow rotation defaults")
	allCheats := fs.Bool("all-cheats", defaultAllCheats, "legacy alias for startup full cheats (equivalent to -cheat-level=3 -invuln=true)")
	startInMap := fs.Bool("start-in-map", defaultStartInMap, "start with automap open")
	importPCSpeaker := fs.Bool("import-pcspeaker", defaultImportPCSpeaker, "import Doom PC speaker sounds (DP* lumps) at startup")
	importTextures := fs.Bool("import-textures", defaultImportTextures, "parse Doom texture data and build wall textures for doom-basic 3D renderer")
	cpuProfile := fs.String("cpuprofile", defaultCPUProfile, "write Go CPU profile to file")
	demoPath := fs.String("demo", defaultDemo, "path to gddoom-demo-v1 script; runs scripted benchmark and exits when demo ends")
	recordDemoPath := fs.String("record-demo", defaultRecordDemo, "path to write gddoom-demo-v1 script recorded from live input")

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

	wf, err := wad.Open(*wadPath)
	if err != nil {
		fmt.Fprintf(stderr, "open wad: %v\n", err)
		return 1
	}
	wadHash := hashFileSHA1(*wadPath)
	soundBank := automap.SoundBank{}
	if *importPCSpeaker {
		dpr := sound.ImportPCSpeakerSounds(wf)
		dsr := sound.ImportDigitalSounds(wf)
		fmt.Fprintf(stderr, "sound import: dp(found=%d decoded=%d failed=%d) ds(found=%d decoded=%d failed=%d)\n",
			dpr.Found, dpr.Decoded, dpr.Failed,
			dsr.Found, dsr.Decoded, dsr.Failed,
		)
		soundBank = buildAutomapSoundBank(dsr)
	}
	wallTexBank := map[string]automap.WallTexture(nil)
	if *importTextures {
		ts, terr := doomtex.LoadFromWAD(wf)
		if terr != nil {
			fmt.Fprintf(stderr, "texture import failed: %v\n", terr)
		} else {
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
	flatBank := map[string][]byte(nil)
	loadFlats := true
	if loadFlats {
		fb, ferr := doomtex.LoadFlatsRGBA(wf, 0)
		if ferr != nil {
			fmt.Fprintf(stderr, "flat import failed: %v\n", ferr)
		} else {
			flatBank = fb
			fmt.Fprintf(stderr, "flat import: flats=%d\n", len(flatBank))
		}
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
		opts := automap.Options{
			Width:          *width,
			Height:         *height,
			StartZoom:      *zoom,
			WADHash:        wadHash,
			Debug:          *debug,
			PlayerSlot:     *playerSlot,
			SkillLevel:     *skillLevel,
			CheatLevel:     resolvedCheatLevel,
			Invulnerable:   resolvedInvuln,
			LineColorMode:  resolvedLineColorMode,
			SourcePortMode: *sourcePortMode,
			AllCheats:      *allCheats,
			StartInMapMode: *startInMap,
			FlatBank:       flatBank,
			WallTexBank:    wallTexBank,
			SoundBank:      soundBank,
			RecordDemoPath: resolvedRecordDemoPath,
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
		DoorOpen:   firstSample(sample("DSDOROPN"), sample("DSBDOPN")),
		DoorClose:  firstSample(sample("DSDORCLS"), sample("DSBDCLS")),
		BlazeOpen:  sample("DSBDOPN"),
		BlazeClose: sample("DSBDCLS"),
		SwitchOn:   sample("DSSWTCHN"),
		SwitchOff:  sample("DSSWTCHX"),
		NoWay:      firstSample(sample("DSNOWAY"), sample("DSOOF")),
		ItemUp:     sample("DSITEMUP"),
		WeaponUp:   sample("DSWPNUP"),
		PowerUp:    sample("DSGETPOW"),
		Oof:        sample("DSOOF"),
	}
}

func firstSample(a, b automap.PCMSample) automap.PCMSample {
	if len(a.Data) > 0 {
		return a
	}
	return b
}
