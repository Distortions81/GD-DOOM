//go:build raylib && cgo && !js

package raymesh

import (
	"image/color"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// DrawScreenTint covers the complete presentation, including the status bar,
// as the main host's death and powerup overlays do.
func DrawScreenTint(clr color.RGBA, width, height int) {
	if clr.A != 0 {
		// Go color.RGBA and the shared Ebiten overlay use premultiplied
		// channels. Raylib's default straight-alpha blend would multiply
		// them a second time and also leave a translucent framebuffer.
		rl.BeginBlendMode(rl.BlendAlphaPremultiply)
		rl.DrawRectangle(0, 0, int32(width), int32(height), rl.NewColor(clr.R, clr.G, clr.B, clr.A))
		rl.EndBlendMode()
	}
}
