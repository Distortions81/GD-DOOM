//go:build raylib && cgo && !js

package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"math"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/render/raymesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Save previews have their own single GPU texture, so overwriting slots cannot
// accumulate stale images in the static WAD patch cache.
type nativeSavePreview struct {
	texture  rl.Texture2D
	slot     int
	modified time.Time
	checked  bool
}

func (p *nativeSavePreview) Close() {
	if p.texture.ID != 0 {
		rl.UnloadTexture(p.texture)
	}
	p.texture = rl.Texture2D{}
	p.checked = false
}
func (p *nativeSavePreview) Draw(slot int, width, height int) {
	if p.checked && p.slot == slot {
		p.drawTexture(width, height)
		return
	}
	p.Close()
	p.slot, p.checked = slot, true
	data, modified, err := doomruntime.NativeSaveThumbnail(slot)
	if err != nil {
		return
	}
	if p.texture.ID == 0 || p.slot != slot || !p.modified.Equal(modified) {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		b := img.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
		native := rl.NewImage(rgba.Pix, int32(b.Dx()), int32(b.Dy()), 1, rl.UncompressedR8g8b8a8)
		p.texture = rl.LoadTextureFromImage(native)
		p.slot, p.modified = slot, modified
	}
	p.drawTexture(width, height)
}
func (p *nativeSavePreview) drawTexture(width, height int) {
	if p.texture.ID == 0 {
		return
	}
	scale, ox, oy := raymesh.MenuTransform(width, height)
	x := ox + 222*scale
	y := oy + 18*scale
	w := 88 * scale
	h := w * float64(p.texture.Height) / float64(p.texture.Width)
	rl.DrawTexturePro(p.texture, rl.NewRectangle(0, 0, float32(p.texture.Width), float32(p.texture.Height)), rl.NewRectangle(float32(x), float32(y), float32(w), float32(h)), rl.Vector2{}, 0, rl.White)
}

func saveNativeThumbnail(slot int) error {
	img, err := raymesh.CaptureWindow()
	if err != nil {
		return err
	}
	defer rl.UnloadImage(img)
	colors := rl.LoadImageColors(img)
	defer rl.UnloadImageColors(colors)
	rgba := image.NewRGBA(image.Rect(0, 0, int(img.Width), int(img.Height)))
	for i, c := range colors {
		copy(rgba.Pix[4*i:4*i+4], []byte{c.R, c.G, c.B, c.A})
	}
	return writeNativeThumbnail(slot, rgba)
}
func writeNativeThumbnail(slot int, src image.Image) error {
	b := src.Bounds()
	scale := math.Min(1, math.Min(320/float64(b.Dx()), 320/float64(b.Dy())))
	w, h := max(1, int(math.Round(float64(b.Dx())*scale))), max(1, int(math.Round(float64(b.Dy())*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, dst); err != nil {
		return err
	}
	return doomruntime.WriteNativeSaveThumbnail(slot, data.Bytes())
}
