package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/automap"
	"gddoom/internal/wad"
)

func RunParse(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("gddoom", flag.ContinueOnError)
	fs.SetOutput(stderr)

	wadPath := fs.String("wad", "DOOM1.WAD", "path to IWAD file")
	mapName := fs.String("map", "E1M1", "map name (E#M# or MAP##)")
	details := fs.Bool("details", false, "print decoded gameplay-relevant map details")
	render := fs.Bool("render", true, "launch Ebiten automap renderer")
	width := fs.Int("width", 1280, "render window width")
	height := fs.Int("height", 800, "render window height")
	zoom := fs.Float64("zoom", 0, "starting zoom (<=0 means auto-fit)")
	lineColorMode := fs.String("line-color-mode", "doom", "line color mode for automap")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "flag error: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*wadPath) == "" {
		fmt.Fprintln(stderr, "-wad is required")
		return 2
	}

	wf, err := wad.Open(*wadPath)
	if err != nil {
		fmt.Fprintf(stderr, "open wad: %v\n", err)
		return 1
	}

	selected := mapdata.MapName(strings.ToUpper(strings.TrimSpace(*mapName)))
	if selected == "" {
		selected, err = mapdata.FirstMapName(wf)
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
	if *render {
		err = automap.RunAutomap(m, automap.Options{
			Width:         *width,
			Height:        *height,
			StartZoom:     *zoom,
			LineColorMode: *lineColorMode,
		})
		if err != nil {
			fmt.Fprintf(stderr, "render map %s: %v\n", selected, err)
			return 1
		}
		return 0
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
