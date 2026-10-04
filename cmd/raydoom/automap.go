//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/render/mapview"
	"gddoom/internal/render/mapview/presenter"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
	"image/color"
)

type nativeAutomap struct{ texture rl.Texture2D }

func (a *nativeAutomap) Close() {
	if a.texture.ID != 0 {
		rl.UnloadTexture(a.texture)
		a.texture = rl.Texture2D{}
	}
}
func (a *nativeAutomap) draw(f doomruntime.NativeMapFrame, p *raymesh.Presentation) {
	rl.BeginScissorMode(0, 0, int32(f.Width), int32(f.Height))
	rl.DrawRectangle(0, 0, int32(f.Width), int32(f.Height), rl.NewColor(12, 16, 20, 255))
	if a.texture.ID == 0 || int(a.texture.Width) != f.Width || int(a.texture.Height) != f.Height {
		a.Close()
		a.texture = rl.LoadTextureFromImage(rl.NewImage(f.Pixels, int32(f.Width), int32(f.Height), 1, rl.UncompressedR8g8b8a8))
		rl.SetTextureFilter(a.texture, rl.FilterPoint)
		rl.SetTextureWrap(a.texture, rl.WrapClamp)
	} else {
		rl.UpdateTexture(a.texture, f.Pixels)
	}
	rl.DrawTexture(a.texture, 0, 0, rl.White)
	segments := func(items []mapview.Segment) {
		for _, s := range items {
			c := color.RGBAModel.Convert(s.Color).(color.RGBA)
			rl.DrawLineEx(rl.Vector2{X: float32(s.X1), Y: float32(s.Y1)}, rl.Vector2{X: float32(s.X2), Y: float32(s.Y2)}, s.Width, c)
		}
	}
	segments(f.Segments[:f.GridSegments])
	for _, s := range f.Lines {
		rl.DrawLineEx(rl.Vector2{X: s.X1, Y: s.Y1}, rl.Vector2{X: s.X2, Y: s.Y2}, s.W, s.Clr)
	}
	segments(f.Segments[f.GridSegments:f.ActorSegments])
	p.DrawScreenPatches(f.Patches)
	segments(f.Segments[f.ActorSegments:])
	for _, l := range f.Labels {
		rl.DrawText(l.Text, int32(l.X), int32(l.Y), 12, rl.NewColor(120, 210, 255, 255))
	}
	if f.Legend {
		x := float64(max(8, f.Width-235))
		y := float64(28)
		rl.DrawRectangle(int32(x-8), int32(y-5), 235, 235, rl.NewColor(0, 0, 0, 200))
		label := func(text string) { rl.DrawText(text, int32(x+18), int32(y), 12, rl.White) }
		label("THING LEGEND")
		y += 16
		for _, e := range presenter.ThingLegendEntries(presenter.LegendInputs{SourcePortMode: true, SourcePortThingLabel: f.ThingMode}, f.LegendColors) {
			segments(presenter.AppendThingGlyph(nil, presenter.ThingStyle{Glyph: e.Glyph, Color: e.Color}, x+8, y+5, 0, 4.6))
			label(e.Label)
			y += 14
		}
		y += 8
		label("LINE COLORS")
		y += 16
		for _, e := range presenter.LineLegendEntries(f.LegendColors) {
			c := color.RGBAModel.Convert(e.Color).(color.RGBA)
			rl.DrawLineEx(rl.Vector2{X: float32(x + 2), Y: float32(y + 5)}, rl.Vector2{X: float32(x + 14), Y: float32(y + 5)}, 2.4, c)
			label(e.Label)
			y += 14
		}
	}

	rl.EndScissorMode()
}

// Edge inputs wait for the next simulation tic; held zoom/pan repeats at 35 Hz.
func sampleNativeMapInput(pending *doomruntime.NativeMapInput, cheatTyping bool) {
	pending.ResetViewPressed = pending.ResetViewPressed || rl.IsKeyPressed(rl.KeyHome)
	pending.ToggleRotate = pending.ToggleRotate || (!cheatTyping && rl.IsKeyPressed(rl.KeyR))
	if !cheatTyping {
		pending.ToggleFollowPressed = pending.ToggleFollowPressed || rl.IsKeyPressed(rl.KeyF)
		pending.ToggleBigMapPressed = pending.ToggleBigMapPressed || rl.IsKeyPressed(rl.KeyB) || rl.IsKeyPressed(rl.KeyZero) || rl.IsKeyPressed(rl.KeyKp0)
		pending.AddMarkPressed = pending.AddMarkPressed || rl.IsKeyPressed(rl.KeyM)
		pending.ClearMarksPressed = pending.ClearMarksPressed || rl.IsKeyPressed(rl.KeyC)
		pending.ToggleGrid = pending.ToggleGrid || rl.IsKeyPressed(rl.KeyG)
		pending.ToggleReveal = pending.ToggleReveal || rl.IsKeyPressed(rl.KeyO)
		pending.CycleIDDT = pending.CycleIDDT || rl.IsKeyPressed(rl.KeyI)
		pending.CycleThings = pending.CycleThings || rl.IsKeyPressed(rl.KeyT)
		pending.ToggleLegend = pending.ToggleLegend || rl.IsKeyPressed(rl.KeyV)
	}
	pending.WheelY += float64(rl.GetMouseWheelMove())
	pending.ZoomInHeld = rl.IsKeyDown(rl.KeyEqual) || rl.IsKeyDown(rl.KeyKpAdd)
	pending.ZoomOutHeld = rl.IsKeyDown(rl.KeyMinus) || rl.IsKeyDown(rl.KeyKpSubtract)
	pending.PanUpHeld = rl.IsKeyDown(rl.KeyUp)
	pending.PanDownHeld = rl.IsKeyDown(rl.KeyDown)
	pending.PanLeftHeld = rl.IsKeyDown(rl.KeyLeft)
	pending.PanRightHeld = rl.IsKeyDown(rl.KeyRight)
}
func consumeNativeMapEdges(pending *doomruntime.NativeMapInput) {
	*pending = doomruntime.NativeMapInput{InputState: mapview.InputState{ZoomInHeld: pending.ZoomInHeld, ZoomOutHeld: pending.ZoomOutHeld, PanUpHeld: pending.PanUpHeld, PanDownHeld: pending.PanDownHeld, PanLeftHeld: pending.PanLeftHeld, PanRightHeld: pending.PanRightHeld}}
}
