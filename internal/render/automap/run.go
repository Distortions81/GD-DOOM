package automap

import (
	"errors"
	"fmt"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

type sessionGame struct {
	g       *game
	current mapdata.MapName
	opts    Options
	nextMap NextMapFunc
	err     error
}

func RunAutomap(m *mapdata.Map, opts Options, nextMap NextMapFunc) error {
	const (
		doomLogicalW = 640
		doomLogicalH = 400
		doomWindowW  = 1280
		doomWindowH  = 960
	)
	if opts.SourcePortMode {
		if opts.Width <= 0 {
			opts.Width = 1280
		}
		if opts.Height <= 0 {
			opts.Height = 800
		}
	} else {
		opts.Width = doomLogicalW
		opts.Height = doomLogicalH
	}
	sg := &sessionGame{
		g:       newGame(m, opts),
		current: m.Name,
		opts:    opts,
		nextMap: nextMap,
	}
	ebiten.SetTPS(doomTicsPerSecond)
	if opts.SourcePortMode {
		ebiten.SetWindowSize(opts.Width, opts.Height)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	} else {
		ebiten.SetWindowSize(doomWindowW, doomWindowH)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	}
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	if err := ebiten.RunGame(sg); err != nil {
		if errors.Is(err, ebiten.Termination) {
			if sg.err != nil {
				return sg.err
			}
			return nil
		}
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	return sg.err
}

func (sg *sessionGame) Update() error {
	err := sg.g.Update()
	if err == nil {
		if sg.g.levelRestartRequested {
			sg.g = newGame(sg.g.m, sg.opts)
			ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", sg.current))
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
	if sg.nextMap == nil {
		return ebiten.Termination
	}
	next, nextName, nerr := sg.nextMap(sg.current, sg.g.secretLevelExit)
	if nerr != nil {
		sg.err = nerr
		return ebiten.Termination
	}
	sg.current = nextName
	sg.g = newGame(next, sg.opts)
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", next.Name))
	return nil
}

func (sg *sessionGame) Draw(screen *ebiten.Image) {
	sg.g.Draw(screen)
}

func (sg *sessionGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return sg.g.Layout(outsideWidth, outsideHeight)
}
