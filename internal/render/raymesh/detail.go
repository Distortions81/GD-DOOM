//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Detail resolves a reduced viewport rendered directly into the window's MSAA
// framebuffer, then presents it with the same nearest scaling as Ebiten. The
// full-resolution path allocates nothing. Geometry and material caches stay live.
type Detail struct{ scene rl.RenderTexture2D }

func (d *Detail) Present(sceneW, sceneH, windowW, windowH int) error {
	if sceneW == windowW && sceneH == windowH {
		return nil
	}
	w, h := int32(sceneW), int32(sceneH)
	if d.scene.ID == 0 || d.scene.Texture.Width != w || d.scene.Texture.Height != h {
		d.Close()
		d.scene = rl.LoadRenderTexture(w, h)
		if !rl.IsRenderTextureValid(d.scene) {
			return fmt.Errorf("could not allocate detail scene %dx%d", w, h)
		}
		rl.SetTextureFilter(d.scene.Texture, rl.FilterPoint)
		rl.SetTextureWrap(d.scene.Texture, rl.WrapClamp)
	}
	if err := CopyWindowRegionToTexture(d.scene, w, h); err != nil {
		return err
	}
	rl.Viewport(0, 0, int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight()))
	// This is a framebuffer copy. Map sprites can leave fractional alpha in
	// the resolved scene; blending it again would mix in the previous pixels.
	const glOne, glZero, glFuncAdd = 1, 0, 0x8006
	rl.SetBlendFactors(glOne, glZero, glFuncAdd)
	rl.BeginBlendMode(rl.BlendCustom)
	rl.DrawTexturePro(d.scene.Texture, rl.NewRectangle(0, 0, float32(w), -float32(h)), rl.NewRectangle(0, 0, float32(windowW), float32(windowH)), rl.Vector2{}, 0, rl.White)
	rl.EndBlendMode()
	return nil
}

func (d *Detail) Close() {
	if d.scene.ID != 0 {
		rl.UnloadRenderTexture(d.scene)
	}
	d.scene = rl.RenderTexture2D{}
}
