package automap

import (
	"errors"
	"fmt"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

func RunAutomap(m *mapdata.Map, opts Options) (RunResult, error) {
	g := newGame(m, opts)
	ebiten.SetTPS(doomTicsPerSecond)
	ebiten.SetWindowSize(g.opts.Width, g.opts.Height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	if err := ebiten.RunGame(g); err != nil {
		if errors.Is(err, ebiten.Termination) {
			return RunResult{
				LevelExited: g.levelExitRequested,
				SecretExit:  g.secretLevelExit,
			}, nil
		}
		return RunResult{}, fmt.Errorf("run ebiten automap: %w", err)
	}
	return RunResult{
		LevelExited: g.levelExitRequested,
		SecretExit:  g.secretLevelExit,
	}, nil
}
