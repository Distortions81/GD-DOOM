package doomruntime

import (
	"gddoom/internal/render/mapview"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (g *game) buildMapViewInputState() mapview.InputState {
	mx, _ := ebiten.CursorPosition()
	_, wheelY := ebiten.Wheel()
	return mapview.InputState{
		ToggleFollowPressed: inpututil.IsKeyJustPressed(ebiten.KeyF),
		ToggleBigMapPressed: (g.opts.SourcePortMode && inpututil.IsKeyJustPressed(ebiten.KeyB)) ||
			inpututil.IsKeyJustPressed(ebiten.Key0) ||
			inpututil.IsKeyJustPressed(ebiten.KeyKP0),
		AddMarkPressed:    inpututil.IsKeyJustPressed(ebiten.KeyM),
		ClearMarksPressed: inpututil.IsKeyJustPressed(ebiten.KeyC),
		ResetViewPressed:  inpututil.IsKeyJustPressed(ebiten.KeyHome),
		ZoomInHeld:        ebiten.IsKeyPressed(ebiten.KeyEqual) || ebiten.IsKeyPressed(ebiten.KeyKPAdd),
		ZoomOutHeld:       ebiten.IsKeyPressed(ebiten.KeyMinus) || ebiten.IsKeyPressed(ebiten.KeyKPSubtract),
		WheelY:            wheelY,
		MoveForwardHeld:   ebiten.IsKeyPressed(ebiten.KeyW),
		MoveBackwardHeld:  ebiten.IsKeyPressed(ebiten.KeyS),
		MoveLeftHeld:      ebiten.IsKeyPressed(ebiten.KeyA),
		MoveRightHeld:     ebiten.IsKeyPressed(ebiten.KeyD),
		TurnLeftHeld:      ebiten.IsKeyPressed(ebiten.KeyQ),
		TurnRightHeld:     ebiten.IsKeyPressed(ebiten.KeyE),
		PanUpHeld:         ebiten.IsKeyPressed(ebiten.KeyArrowUp),
		PanDownHeld:       ebiten.IsKeyPressed(ebiten.KeyArrowDown),
		PanLeftHeld:       ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		PanRightHeld:      ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		FireHeld: ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
			ebiten.IsKeyPressed(ebiten.KeyControlRight) ||
			ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		CursorX: mx,
	}
}

func (g *game) buildMapViewUpdateState() mapview.UpdateState {
	speed := g.currentRunSpeed()
	view := g.State.Snapshot()
	return mapview.UpdateState{
		EdgeInputPass:          g.edgeInputPass,
		IsSourcePort:           g.opts.SourcePortMode,
		FollowMode:             view.FollowEnabled(),
		Zoom:                   view.ZoomLevel(),
		FitZoom:                view.FitZoomLevel(),
		RunSpeed:               speed,
		ForwardMove:            forwardMove[speed],
		SideMove:               sideMove[speed],
		PendingUse:             g.pendingUse,
		MouseLookEnabled:       g.opts.SourcePortMode && g.opts.MouseLook,
		MouseLookSpeed:         g.opts.MouseLookSpeed,
		RenderWidth:            g.viewW,
		LastMouseX:             g.lastMouseX,
		MouseLookSet:           g.mouseLookSet,
		MouseLookSuppressTicks: g.mouseLookSuppressTicks,
	}
}

func (g *game) applyMapViewUpdateResult(result mapview.UpdateResult) {
	g.State.SetZoom(result.Zoom)
	if result.ToggleFollow {
		if g.State.ToggleFollowMode() {
			g.setHUDMessage("Follow ON", 70)
		} else {
			g.setHUDMessage("Follow OFF", 70)
		}
	}
	if result.ToggleBigMap {
		g.toggleBigMap()
	}
	if result.AddMark {
		g.addMark()
	}
	if result.ClearMarks {
		g.clearMarks()
	}
	if result.ResetView {
		g.resetView()
	}
	if result.ConsumePendingUse {
		g.pendingUse = false
	}
	g.lastMouseX = result.LastMouseX
	g.mouseLookSet = result.MouseLookSet
	g.mouseLookSuppressTicks = result.MouseLookSuppressTicks
	g.runGameplayTic(moveCmd{
		forward: result.Command.Forward,
		side:    result.Command.Side,
		turn:    result.Command.Turn,
		turnRaw: result.Command.TurnRaw,
		run:     result.Command.Run,
	}, result.ConsumePendingUse, result.FireHeld)
	g.recordDemoTic(moveCmd{
		forward: result.Command.Forward,
		side:    result.Command.Side,
		turn:    result.Command.Turn,
		turnRaw: result.Command.TurnRaw,
		run:     result.Command.Run,
	}, result.ConsumePendingUse, result.FirePressed)
	g.discoverLinesAroundPlayer()
	if result.SyncCameraToPlayer {
		g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
		return
	}
	g.State.Pan(result.PanDX, result.PanDY)
}
