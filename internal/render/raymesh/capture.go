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
	if err := CopyWindowToTexture(resolved); err != nil {
		return nil, err
	}
	img := rl.LoadImageFromTexture(resolved.Texture)
	if img == nil || !rl.IsImageValid(img) {
		return nil, fmt.Errorf("could not read screenshot resolve texture")
	}
	rl.ImageFlipVertical(img)
	return img, nil
}

// CopyWindowToTexture resolves the current multisampled framebuffer on the GPU.
// It must run before EndDrawing, and leaves the normal window framebuffer bound.
func CopyWindowToTexture(dst rl.RenderTexture2D) error {
	w, h := int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight())
	return CopyWindowRegionToTexture(dst, w, h)
}

// CopyWindowRegionToTexture resolves a top-left scene region, without scaling
// the multisampled source. The destination remains a cached single-sample image.
func CopyWindowRegionToTexture(dst rl.RenderTexture2D, w, h int32) error {
	rl.DrawRenderBatchActive()
	if !rl.IsRenderTextureValid(dst) || dst.Texture.Width != w || dst.Texture.Height != h {
		return fmt.Errorf("window snapshot texture must match %dx%d", w, h)
	}
	windowH := int32(rl.GetRenderHeight())
	if w < 1 || h < 1 || w > int32(rl.GetRenderWidth()) || h > windowH {
		return fmt.Errorf("scene region exceeds window framebuffer")
	}
	rl.BindFramebuffer(rl.ReadFramebuffer, 0)
	rl.BindFramebuffer(rl.DrawFramebuffer, dst.ID)
	const colorBufferBit = 0x00004000
	rl.BlitFramebuffer(0, windowH-h, w, windowH, 0, 0, w, h, colorBufferBit)
	rl.DisableFramebuffer() // Restore both read/draw bindings before UI or swap.
	return nil
}
