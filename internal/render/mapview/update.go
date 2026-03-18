package mapview

import (
	"math"
)

type Command struct {
	Forward int64
	Side    int64
	Turn    int
	TurnRaw int64
	Run     bool
}

type InputState struct {
	ToggleFollowPressed bool
	ToggleBigMapPressed bool
	AddMarkPressed      bool
	ClearMarksPressed   bool
	ResetViewPressed    bool
	ZoomInHeld          bool
	ZoomOutHeld         bool
	WheelY              float64
	MoveForwardHeld     bool
	MoveBackwardHeld    bool
	MoveLeftHeld        bool
	MoveRightHeld       bool
	TurnLeftHeld        bool
	TurnRightHeld       bool
	PanUpHeld           bool
	PanDownHeld         bool
	PanLeftHeld         bool
	PanRightHeld        bool
	FireHeld            bool
	CursorX             int
}

type UpdateState struct {
	EdgeInputPass          bool
	IsSourcePort           bool
	FollowMode             bool
	Zoom                   float64
	FitZoom                float64
	RunSpeed               int
	ForwardMove            int64
	SideMove               int64
	PendingUse             bool
	MouseLookEnabled       bool
	MouseLookSpeed         float64
	RenderWidth            int
	LastMouseX             int
	MouseLookSet           bool
	MouseLookSuppressTicks int
}

type UpdateResult struct {
	ToggleFollow           bool
	ToggleBigMap           bool
	AddMark                bool
	ClearMarks             bool
	ResetView              bool
	Zoom                   float64
	ConsumePendingUse      bool
	Command                Command
	FireHeld               bool
	FirePressed            bool
	LastMouseX             int
	MouseLookSet           bool
	MouseLookSuppressTicks int
	SyncCameraToPlayer     bool
	PanDX                  float64
	PanDY                  float64
}

func Update(input InputState, state UpdateState) UpdateResult {
	result := UpdateResult{
		Zoom:                   nextZoom(state.Zoom, state.FitZoom, input),
		LastMouseX:             state.LastMouseX,
		MouseLookSet:           state.MouseLookSet,
		MouseLookSuppressTicks: state.MouseLookSuppressTicks,
	}

	if state.EdgeInputPass && input.ToggleFollowPressed {
		result.ToggleFollow = true
	}
	if state.EdgeInputPass && input.ToggleBigMapPressed {
		result.ToggleBigMap = true
	}
	if state.EdgeInputPass && input.AddMarkPressed {
		result.AddMark = true
	}
	if state.EdgeInputPass && input.ClearMarksPressed {
		result.ClearMarks = true
	}
	if state.EdgeInputPass && state.IsSourcePort && input.ResetViewPressed {
		result.ResetView = true
	}

	cmd := Command{}
	if input.MoveForwardHeld {
		cmd.Forward += state.ForwardMove
	}
	if input.MoveBackwardHeld {
		cmd.Forward -= state.ForwardMove
	}
	if input.MoveLeftHeld {
		cmd.Side -= state.SideMove
	}
	if input.MoveRightHeld {
		cmd.Side += state.SideMove
	}
	// Keep map panning on arrow keys; use Q/E turning in map mode.
	if input.TurnLeftHeld {
		cmd.Turn += 1
	}
	if input.TurnRightHeld {
		cmd.Turn -= 1
	}
	if state.EdgeInputPass && state.PendingUse {
		result.ConsumePendingUse = true
	}
	if state.MouseLookEnabled {
		cmd.TurnRaw, result.LastMouseX, result.MouseLookSet, result.MouseLookSuppressTicks =
			mouseLookTurnRaw(input.CursorX, state.MouseLookSpeed, state.RenderWidth, state.LastMouseX, state.MouseLookSet, state.MouseLookSuppressTicks)
	} else {
		result.MouseLookSet = false
	}
	cmd.Run = state.RunSpeed == 1
	result.Command = cmd
	result.FireHeld = input.FireHeld
	result.FirePressed = input.FireHeld

	followMode := state.FollowMode
	if result.ToggleFollow {
		followMode = !followMode
	}
	result.SyncCameraToPlayer = followMode

	if result.SyncCameraToPlayer {
		return result
	}

	panStep := 32.0 / result.Zoom
	if input.PanUpHeld {
		result.PanDY += panStep
	}
	if input.PanDownHeld {
		result.PanDY -= panStep
	}
	if input.PanLeftHeld {
		result.PanDX -= panStep
	}
	if input.PanRightHeld {
		result.PanDX += panStep
	}
	return result
}

func nextZoom(current, fitZoom float64, input InputState) float64 {
	zoom := current
	zoomStep := 1.03
	if input.ZoomInHeld {
		zoom *= zoomStep
	}
	if input.ZoomOutHeld {
		zoom /= zoomStep
	}
	if input.WheelY > 0 {
		zoom *= 1.1
	}
	if input.WheelY < 0 {
		zoom /= 1.1
	}
	minZoom := fitZoom * 0.05
	maxZoom := fitZoom * 200
	if zoom < minZoom {
		zoom = minZoom
	}
	if zoom > maxZoom {
		zoom = maxZoom
	}
	return zoom
}

func mouseLookTurnRaw(cursorX int, speed float64, renderWidth, lastMouseX int, mouseLookSet bool, suppressTicks int) (turnRaw int64, nextLastMouseX int, nextMouseLookSet bool, nextSuppressTicks int) {
	nextLastMouseX = lastMouseX
	nextMouseLookSet = mouseLookSet
	nextSuppressTicks = suppressTicks
	if nextSuppressTicks > 0 {
		nextSuppressTicks--
		nextLastMouseX = cursorX
		nextMouseLookSet = true
		return 0, nextLastMouseX, nextMouseLookSet, nextSuppressTicks
	}
	if !nextMouseLookSet {
		nextLastMouseX = cursorX
		nextMouseLookSet = true
		return 0, nextLastMouseX, nextMouseLookSet, nextSuppressTicks
	}
	dx := cursorX - nextLastMouseX
	nextLastMouseX = cursorX
	return mouseLookTurnRawWithWidth(dx, speed, renderWidth), nextLastMouseX, nextMouseLookSet, nextSuppressTicks
}

func mouseLookTurnRawWithWidth(dx int, speed float64, renderW int) int64 {
	if dx == 0 {
		return 0
	}
	base := float64(40 << 16)
	scale := mouseLookResolutionScale(renderW)
	raw := int64(math.Round(float64(dx) * scale * base * speed))
	if raw == 0 {
		if dx > 0 {
			raw = 1
		} else {
			raw = -1
		}
	}
	return -raw
}

func mouseLookResolutionScale(renderW int) float64 {
	const doomLogicalW = 320
	refW := doomLogicalW
	if renderW <= 0 {
		renderW = refW
	}
	return float64(refW) / float64(renderW)
}
