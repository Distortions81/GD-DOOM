//go:build raylib && cgo && !js

package raymesh

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Ebitengine v2.10.4's ebitenutil/text.png, generated from bitmapfont. Keep the
// glyphs, one-pixel x offset, shadow and 6x16 metrics identical to DebugPrintAt.
// Upstream license and font notices are included beside the unchanged asset.
//
//go:embed assets/debug-font.png
var debugFontPNG []byte

type DebugText struct{ texture rl.Texture2D }

func (f *DebugText) Draw(text string, x, y int) {
	if f.texture.ID == 0 {
		img, err := png.Decode(bytes.NewReader(debugFontPNG))
		if err != nil {
			panic(err)
		}
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		f.texture = rl.LoadTextureFromImage(rl.NewImage(rgba.Pix, int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()), 1, rl.UncompressedR8g8b8a8))
		rl.SetTextureFilter(f.texture, rl.FilterPoint)
		rl.SetTextureWrap(f.texture, rl.WrapClamp)
	}
	px, py := x+1, y
	rl.BeginBlendMode(rl.BlendAlphaPremultiply)
	for _, ch := range text {
		if ch == '\n' {
			px, py = x+1, py+16
			continue
		}
		if ch <= 255 {
			cols := int(f.texture.Width) / 6
			source := rl.NewRectangle(float32(int(ch)%cols*6), float32(int(ch)/cols*16), 6, 16)
			rl.DrawTextureRec(f.texture, source, rl.NewVector2(float32(px), float32(py)), rl.White)
		}
		px += 6
	}
	rl.EndBlendMode()
}

func (f *DebugText) Close() {
	if f.texture.ID != 0 {
		rl.UnloadTexture(f.texture)
		f.texture = rl.Texture2D{}
	}
}
