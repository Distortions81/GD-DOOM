package runtimehost

import (
	"fmt"

	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	doomTicsPerSecond = 35
	hostTPS           = doomTicsPerSecond * 4
)

func WindowTitle(name mapdata.MapName) string {
	return fmt.Sprintf("GD-DOOM - %s", name)
}

func ConfigureInitialHost(opts runtimecfg.Options, windowW, windowH int, name mapdata.MapName) {
	vsyncEnabled := !opts.NoVsync
	ebiten.SetVsyncEnabled(vsyncEnabled)
	ebiten.SetTPS(hostTPS)
	ebiten.SetWindowDecorated(true)

	if opts.SourcePortMode {
		ebiten.SetWindowSize(opts.Width, opts.Height)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	} else {
		ebiten.SetWindowSize(windowW, windowH)
		ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	}
	ebiten.SetWindowTitle(WindowTitle(name))
	ebiten.SetScreenClearedEveryFrame(false)
}
