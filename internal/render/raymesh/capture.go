//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// CaptureWindow reads the current window before EndDrawing, on its locked GL
// thread. Resolve explicitly: direct screen reads can return a single MSAA
// sample on some drivers instead of the antialiased image shown on screen.
// The caller owns the returned image and must call rl.UnloadImage.
func CaptureWindow() (*rl.Image, error) {
	rl.DrawRenderBatchActive()
	w, h := int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight())
	resolved := rl.LoadRenderTexture(w, h)
	if !rl.IsRenderTextureValid(resolved) {
		return nil, fmt.Errorf("could not create screenshot resolve framebuffer")
	}
	defer rl.UnloadRenderTexture(resolved)
	rl.BindFramebuffer(rl.ReadFramebuffer, 0)
	rl.BindFramebuffer(rl.DrawFramebuffer, resolved.ID)
	const colorBufferBit = 0x00004000
	rl.BlitFramebuffer(0, 0, w, h, 0, 0, w, h, colorBufferBit)
	rl.DisableFramebuffer() // Restore both read/draw bindings before UI or swap.
	img := rl.LoadImageFromTexture(resolved.Texture)
	if img == nil || !rl.IsImageValid(img) {
		return nil, fmt.Errorf("could not read screenshot resolve texture")
	}
	rl.ImageFlipVertical(img)
	return img, nil
}
