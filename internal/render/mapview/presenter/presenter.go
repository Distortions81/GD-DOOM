package presenter

import (
	"gddoom/internal/render/mapview"

	"github.com/hajimehoshi/ebiten/v2"
)

type Inputs struct {
	DrawFloorTextures2D bool
	DrawGrid            bool
	IsSourcePort        bool
	DrawThings          bool
	ShowLegend          bool
	HUDMessage          string
	ShowHUDMessage      bool
	IsDead              bool
	Paused              bool
	ShowPerf            bool
}

type Hooks struct {
	PrepareRenderState  func()
	DrawFloorTextures2D func(screen *ebiten.Image)
	DrawGrid            func(screen *ebiten.Image)
	DrawLines           func(screen *ebiten.Image)
	DrawUseOverlays     func(screen *ebiten.Image)
	DrawThings          func(screen *ebiten.Image)
	DrawActorOverlays   func(screen *ebiten.Image)
	DrawOverlays        func(screen *ebiten.Image, state mapview.RenderState)
}

func BuildRenderState(in Inputs) mapview.RenderState {
	return mapview.RenderState{
		DrawFloorTextures2D: in.DrawFloorTextures2D,
		DrawGrid:            in.DrawGrid,
		IsSourcePort:        in.IsSourcePort,
		DrawThings:          in.DrawThings,
		ShowLegend:          in.ShowLegend,
		HUDMessage:          in.HUDMessage,
		ShowHUDMessage:      in.ShowHUDMessage,
		IsDead:              in.IsDead,
		Paused:              in.Paused,
		ShowPerf:            in.ShowPerf,
	}
}

func Draw(screen *ebiten.Image, in Inputs, hooks Hooks) {
	mapview.Draw(screen, BuildRenderState(in), backend{hooks: hooks})
}

type backend struct {
	hooks Hooks
}

func (b backend) MapViewPrepareRenderState() {
	if b.hooks.PrepareRenderState != nil {
		b.hooks.PrepareRenderState()
	}
}

func (b backend) MapViewDrawFloorTextures2D(screen *ebiten.Image) {
	if b.hooks.DrawFloorTextures2D != nil {
		b.hooks.DrawFloorTextures2D(screen)
	}
}

func (b backend) MapViewDrawGrid(screen *ebiten.Image) {
	if b.hooks.DrawGrid != nil {
		b.hooks.DrawGrid(screen)
	}
}

func (b backend) MapViewDrawLines(screen *ebiten.Image) {
	if b.hooks.DrawLines != nil {
		b.hooks.DrawLines(screen)
	}
}

func (b backend) MapViewDrawUseOverlays(screen *ebiten.Image) {
	if b.hooks.DrawUseOverlays != nil {
		b.hooks.DrawUseOverlays(screen)
	}
}

func (b backend) MapViewDrawThings(screen *ebiten.Image) {
	if b.hooks.DrawThings != nil {
		b.hooks.DrawThings(screen)
	}
}

func (b backend) MapViewDrawActorOverlays(screen *ebiten.Image) {
	if b.hooks.DrawActorOverlays != nil {
		b.hooks.DrawActorOverlays(screen)
	}
}

func (b backend) MapViewDrawOverlays(screen *ebiten.Image, state mapview.RenderState) {
	if b.hooks.DrawOverlays != nil {
		b.hooks.DrawOverlays(screen, state)
	}
}
