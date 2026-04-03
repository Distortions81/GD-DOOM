package doomruntime

import (
	"strings"

	"gddoom/internal/render/mapview"

	"github.com/hajimehoshi/ebiten/v2"
)

func (g *game) buildMapViewInputState() mapview.InputState {
	return mapview.InputState{
		ToggleFollowPressed: g.keyJustPressed(ebiten.KeyF),
		ToggleBigMapPressed: (g.opts.SourcePortMode && g.keyJustPressed(ebiten.KeyB)) ||
			g.keyJustPressed(ebiten.Key0) ||
			g.keyJustPressed(ebiten.KeyKP0),
		AddMarkPressed:    g.keyJustPressed(ebiten.KeyM),
		ClearMarksPressed: g.keyJustPressed(ebiten.KeyC),
		ResetViewPressed:  g.keyJustPressed(ebiten.KeyHome),
		ZoomInHeld:        g.keyHeld(ebiten.KeyEqual) || g.keyHeld(ebiten.KeyKPAdd),
		ZoomOutHeld:       g.keyHeld(ebiten.KeyMinus) || g.keyHeld(ebiten.KeyKPSubtract),
		WheelY:            g.input.wheelY,
		MoveForwardHeld:   g.keyHeld(ebiten.KeyW),
		MoveBackwardHeld:  g.keyHeld(ebiten.KeyS),
		MoveLeftHeld:      g.keyHeld(ebiten.KeyA),
		MoveRightHeld:     g.keyHeld(ebiten.KeyD),
		TurnLeftHeld:      g.keyHeld(ebiten.KeyQ),
		TurnRightHeld:     g.keyHeld(ebiten.KeyE),
		PanUpHeld:         g.keyHeld(ebiten.KeyArrowUp),
		PanDownHeld:       g.keyHeld(ebiten.KeyArrowDown),
		PanLeftHeld:       g.keyHeld(ebiten.KeyArrowLeft),
		PanRightHeld:      g.keyHeld(ebiten.KeyArrowRight),
		FireHeld: g.keyHeld(ebiten.KeyControlLeft) ||
			g.keyHeld(ebiten.KeyControlRight) ||
			g.mouseHeld(ebiten.MouseButtonLeft),
		CursorX: g.input.cursorX,
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
		MouseInvert:            g.opts.MouseInvert,
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
	cmd := moveCmd{
		forward: result.Command.Forward,
		side:    result.Command.Side,
		turn:    result.Command.Turn,
		turnRaw: result.Command.TurnRaw,
		run:     result.Command.Run,
	}
	if strings.TrimSpace(g.opts.RecordDemoPath) != "" {
		cmd.forward = int64(int8(clampDemoMove(cmd.forward)))
		cmd.side = int64(int8(clampDemoMove(cmd.side)))
		angleturn16 := int16(cmd.turnRaw >> 16)
		cmd.turnRaw = int64(int16(((int32(angleturn16)+128)>>8)<<8)) << 16
	}
	g.runGameplayTic(cmd, result.ConsumePendingUse, result.FireHeld)
	g.recordDemoTic(cmd, result.ConsumePendingUse, result.FireHeld)
	g.discoverLinesAroundPlayer()
	if result.SyncCameraToPlayer {
		g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
		return
	}
	g.State.Pan(result.PanDX, result.PanDY)
}
