package automap

import (
	"errors"
	"fmt"
	"image/color"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

type NextMapFunc func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error)

type sessionGame struct {
	g               *game
	current         mapdata.MapName
	opts            Options
	nextMap         NextMapFunc
	err             error
	faithfulSurface *ebiten.Image
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
	ebiten.SetVsyncEnabled(!opts.NoVsync)
	if opts.SourcePortMode {
		ebiten.SetWindowSize(opts.Width, opts.Height)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	} else {
		ebiten.SetWindowSize(doomWindowW, doomWindowH)
		// Keep faithful mode's internal framebuffer fixed and let Ebiten scale it
		// to the current window size for CRT-style aspect correction.
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	}
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	ebiten.SetScreenClearedEveryFrame(false)
	if err := ebiten.RunGame(sg); err != nil {
		if errors.Is(err, ebiten.Termination) {
			if p := sg.opts.RecordDemoPath; p != "" {
				if werr := SaveDemoScript(p, sg.g.demoRecord); werr != nil {
					return fmt.Errorf("write demo recording: %w", werr)
				}
				fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(sg.g.demoRecord))
			}
			if sg.err != nil {
				return sg.err
			}
			return nil
		}
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	if p := sg.opts.RecordDemoPath; p != "" {
		if werr := SaveDemoScript(p, sg.g.demoRecord); werr != nil {
			return fmt.Errorf("write demo recording: %w", werr)
		}
		fmt.Printf("demo-recorded path=%s tics=%d\n", p, len(sg.g.demoRecord))
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
	if !sg.opts.SourcePortMode {
		vw := max(sg.g.viewW, 1)
		vh := max(sg.g.viewH, 1)
		if sg.faithfulSurface == nil || sg.faithfulSurface.Bounds().Dx() != vw || sg.faithfulSurface.Bounds().Dy() != vh {
			sg.faithfulSurface = ebiten.NewImage(vw, vh)
		}
		sg.g.Draw(sg.faithfulSurface)

		sw := screen.Bounds().Dx()
		sh := screen.Bounds().Dy()
		dstW := sw
		dstH := (dstW * 3) / 4 // 8:5 render shown as 4:3 display -> 20% taller
		if dstH > sh {
			dstH = sh
			dstW = (dstH * 4) / 3
		}
		if dstW < 1 {
			dstW = 1
		}
		if dstH < 1 {
			dstH = 1
		}
		offX := (sw - dstW) / 2
		offY := (sh - dstH) / 2

		screen.Fill(color.Black)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(dstW)/float64(vw), float64(dstH)/float64(vh))
		op.GeoM.Translate(float64(offX), float64(offY))
		screen.DrawImage(sg.faithfulSurface, op)
		return
	}
	sg.g.Draw(screen)
}

func (sg *sessionGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	if !sg.opts.SourcePortMode {
		// Faithful mode keeps internal render size fixed and handles display
		// aspect correction in Draw via a 4:3 presentation transform.
		return max(outsideWidth, 1), max(outsideHeight, 1)
	}
	return sg.g.Layout(outsideWidth, outsideHeight)
}
