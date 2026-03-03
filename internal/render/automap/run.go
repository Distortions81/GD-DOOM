package automap

import (
	"errors"
	"fmt"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

func RunAutomap(m *mapdata.Map, opts Options) error {
	g := newGame(m, opts)
	ebiten.SetWindowSize(g.opts.Width, g.opts.Height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle(fmt.Sprintf("GD-DOOM Automap - %s", m.Name))
	if err := ebiten.RunGame(g); err != nil {
		if errors.Is(err, ebiten.Termination) {
			return nil
		}
		return fmt.Errorf("run ebiten automap: %w", err)
	}
	return nil
}
