package session

import (
	"errors"

	"github.com/hajimehoshi/ebiten/v2"

	"gddoom/internal/platformcfg"
)

type Runtime interface {
	Update() error
	Draw(screen *ebiten.Image)
	Layout(outsideWidth, outsideHeight int) (int, int)
}

type hostFrameSampler interface {
	SampleInput()
}

type finalScreenDrawer interface {
	DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM)
}

type closer interface {
	Close()
}

type errProvider interface {
	Err() error
}

type Game struct {
	runtime             Runtime
	hostUpdateRemainder int
}

func New(runtime Runtime) *Game {
	return &Game{
		runtime:             runtime,
		hostUpdateRemainder: hostUpdatesPerTick - 1,
	}
}

func (g *Game) Update() error {
	if g == nil || g.runtime == nil {
		return ebiten.Termination
	}
	if s, ok := g.runtime.(hostFrameSampler); ok {
		s.SampleInput()
	}
	g.hostUpdateRemainder++
	if g.hostUpdateRemainder < hostUpdatesPerTick {
		return nil
	}
	g.hostUpdateRemainder = 0
	return g.runtime.Update()
}

const hostUpdatesPerTick = 4

func (g *Game) Draw(screen *ebiten.Image) {
	if g == nil || g.runtime == nil {
		return
	}
	g.runtime.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g == nil || g.runtime == nil {
		if outsideWidth < 1 {
			outsideWidth = 1
		}
		if outsideHeight < 1 {
			outsideHeight = 1
		}
		return outsideWidth, outsideHeight
	}
	return g.runtime.Layout(outsideWidth, outsideHeight)
}

func (g *Game) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if g == nil || g.runtime == nil {
		return
	}
	drawer, ok := g.runtime.(finalScreenDrawer)
	if !ok {
		return
	}
	drawer.DrawFinalScreen(screen, offscreen, geoM)
}

func (g *Game) Close() {
	if g == nil || g.runtime == nil {
		return
	}
	c, ok := g.runtime.(closer)
	if !ok {
		return
	}
	c.Close()
}

func (g *Game) Err() error {
	if g == nil || g.runtime == nil {
		return nil
	}
	p, ok := g.runtime.(errProvider)
	if !ok {
		return nil
	}
	return p.Err()
}

func Run(g *Game) error {
	if err := runGame(g); err != nil {
		if errors.Is(err, ebiten.Termination) {
			return g.Err()
		}
		return err
	}
	return g.Err()
}

func runGame(game ebiten.Game) error {
	if !platformcfg.IsWASMBuild() {
		return ebiten.RunGame(game)
	}
	return ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		DisableHiDPI: false,
	})
}
