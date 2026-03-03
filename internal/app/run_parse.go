package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/automap"
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
	defaultWidth := 1280
	defaultHeight := 800
	defaultZoom := 0.0
	defaultPlayer := 1
	defaultLineColorMode := "parity"
	defaultSourcePortMode := false
	defaultAllCheats := false
	defaultStartInMap := true
	defaultImportPCSpeaker := true
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
	}

	fs := flag.NewFlagSet("gddoom", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configFlag := fs.String("config", defaultConfigPath, "path to config toml file (default: config.toml)")
	wadPath := fs.String("wad", defaultWAD, "path to IWAD file")
	mapName := fs.String("map", defaultMap, "map name (E#M# or MAP##); empty selects first valid map")
	details := fs.Bool("details", defaultDetails, "print decoded gameplay-relevant map details")
	render := fs.Bool("render", defaultRender, "launch Ebiten automap renderer")
	width := fs.Int("width", defaultWidth, "render window width")
	height := fs.Int("height", defaultHeight, "render window height")
	zoom := fs.Float64("zoom", defaultZoom, "starting zoom (>0 overrides Doom-style startup zoom)")
	playerSlot := fs.Int("player", defaultPlayer, "player start slot (1-4)")
	lineColorMode := fs.String("line-color-mode", defaultLineColorMode, "line color mode for automap")
	sourcePortMode := fs.Bool("sourceport-mode", defaultSourcePortMode, "enable source-port style heading-follow rotation defaults")
	allCheats := fs.Bool("all-cheats", defaultAllCheats, "enable automap cheats at startup (allmap + iddt2)")
	startInMap := fs.Bool("start-in-map", defaultStartInMap, "start with automap open")
	importPCSpeaker := fs.Bool("import-pcspeaker", defaultImportPCSpeaker, "import Doom PC speaker sounds (DP* lumps) at startup")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "flag error: %v\n", err)
		return 2
	}
	_ = configFlag
	lineColorModeSet := configLineColorSet
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "line-color-mode" {
			lineColorModeSet = true
		}
	})
	if strings.TrimSpace(*wadPath) == "" {
		fmt.Fprintln(stderr, "-wad is required")
		return 2
	}

	wf, err := wad.Open(*wadPath)
	if err != nil {
		fmt.Fprintf(stderr, "open wad: %v\n", err)
		return 1
	}
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

	selected := mapdata.MapName(strings.ToUpper(strings.TrimSpace(*mapName)))
	if selected == "" {
		selected, err = mapdata.FirstMapName(wf)
		if err != nil {
			fmt.Fprintf(stderr, "resolve first map: %v\n", err)
			return 1
		}
	}

	if *render {
		resolvedLineColorMode := *lineColorMode
		// Source-port defaults unless user explicitly chose a color mode.
		if *sourcePortMode && !lineColorModeSet {
			resolvedLineColorMode = "doom"
		}
		opts := automap.Options{
			Width:          *width,
			Height:         *height,
			StartZoom:      *zoom,
			PlayerSlot:     *playerSlot,
			LineColorMode:  resolvedLineColorMode,
			SourcePortMode: *sourcePortMode,
			AllCheats:      *allCheats,
			StartInMapMode: *startInMap,
			SoundBank:      soundBank,
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
