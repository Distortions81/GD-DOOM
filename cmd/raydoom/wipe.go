//go:build raylib && cgo && !js

package main

import (
	"fmt"
	"gddoom/internal/doomruntime"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/sessiontransition"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// nativeWipe retains two framebuffer snapshots. Its only per-tic state is the
// shared melt positions; drawing batches slices of the old GPU texture.
type nativeWipe struct {
	from, to      rl.RenderTexture2D
	positions     []int
	active, ready bool
}

func (w *nativeWipe) Close() {
	if w.from.ID != 0 {
		rl.UnloadRenderTexture(w.from)
	}
	if w.to.ID != 0 {
		rl.UnloadRenderTexture(w.to)
	}
	*w = nativeWipe{}
}
func (w *nativeWipe) Clear()       { w.active, w.ready = false, false; w.positions = nil }
func (w *nativeWipe) Active() bool { return w.active }
func (w *nativeWipe) Ready() bool  { return w.ready }
func (w *nativeWipe) Queue() {
	w.active = w.from.ID != 0
	w.ready = false
	w.positions = nil
}
func (w *nativeWipe) NeedsResize() bool {
	return w.ready && (w.to.Texture.Width != int32(rl.GetRenderWidth()) || w.to.Texture.Height != int32(rl.GetRenderHeight()))
}
func ensureNativeSnapshot(target *rl.RenderTexture2D) error {
	width, height := int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight())
	if target.ID != 0 && target.Texture.Width == width && target.Texture.Height == height {
		return nil
	}
	if target.ID != 0 {
		rl.UnloadRenderTexture(*target)
		*target = rl.RenderTexture2D{}
	}
	*target = rl.LoadRenderTexture(width, height)
	if !rl.IsRenderTextureValid(*target) {
		return fmt.Errorf("could not allocate wipe snapshot %dx%d", width, height)
	}
	rl.SetTextureFilter(target.Texture, rl.FilterPoint)
	rl.SetTextureWrap(target.Texture, rl.WrapClamp)
	return nil
}
func (w *nativeWipe) CaptureLastFrame() error {
	if w.active {
		return nil
	}
	if err := ensureNativeSnapshot(&w.from); err != nil {
		return err
	}
	return raymesh.CopyWindowToTexture(w.from)
}
func (w *nativeWipe) Prepare() error {
	if !w.active || (w.ready && !w.NeedsResize()) {
		return nil
	}
	if err := ensureNativeSnapshot(&w.to); err != nil {
		return err
	}
	if err := raymesh.CopyWindowToTexture(w.to); err != nil {
		return err
	}
	w.positions = sessiontransition.InitMeltColumnsScaled(sessiontransition.SourcePortMeltColumns, sessiontransition.SourcePortMeltRNGScale(int(w.to.Texture.Height)))
	w.ready = true
	return nil
}
func (w *nativeWipe) Tick() {
	if !w.active || !w.ready {
		return
	}
	if sessiontransition.AdvanceMeltPositions(w.positions, sessiontransition.MeltVirtualH, sessiontransition.SourcePortMeltColumns) {
		w.active = false
	}
}
func (w *nativeWipe) Draw(width, height int) {
	if !w.active || !w.ready {
		return
	}
	source := rl.NewRectangle(0, 0, float32(w.to.Texture.Width), -float32(w.to.Texture.Height))
	rl.DrawTexturePro(w.to.Texture, source, rl.NewRectangle(0, 0, float32(width), float32(height)), rl.Vector2{}, 0, rl.White)
	cols := sessiontransition.SourcePortMeltColumns
	tw, th := int(w.to.Texture.Width), int(w.to.Texture.Height)
	sx, sy := float32(w.from.Texture.Width)/float32(tw), float32(w.from.Texture.Height)/float32(th)
	dx, dy := float32(width)/float32(tw), float32(height)/float32(th)
	for i := 0; i < cols; i++ {
		x0, x1 := i*tw/cols, (i+1)*tw/cols
		cut := max(0, min(sessiontransition.MeltVirtualH, w.positions[i])) * th / sessiontransition.MeltVirtualH
		if x1 <= x0 || cut >= th {
			continue
		}
		// Framebuffer textures are upside down. Negative source height selects
		// the old frame's top rows, sliding down behind the newly revealed frame.
		source := rl.NewRectangle(float32(x0)*sx, float32(cut)*sy, float32(x1-x0)*sx, -float32(th-cut)*sy)
		dest := rl.NewRectangle(float32(x0)*dx, float32(cut)*dy, float32(x1-x0)*dx, float32(th-cut)*dy)
		rl.DrawTexturePro(w.from.Texture, source, dest, rl.Vector2{}, 0, rl.White)
	}
}

// Advance the transition instead of the campaign while a wipe is pending or
// moving. The finishing tic remains a transition tic, as in the main host.
func advanceNativeCampaign(c *doomruntime.NativeCampaign, w *nativeWipe, in doomruntime.NativeMeshInput, skip bool) error {
	if w.Active() {
		w.Tick()
		return nil
	}
	previous := c.Game
	if err := c.Tick(in, skip); err != nil {
		return err
	}
	if c.Game != previous && !c.DemoStatus().Active {
		w.Queue()
	}
	return nil
}
