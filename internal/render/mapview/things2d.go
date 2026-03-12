package mapview

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

type ThingDrawItem struct {
	ScreenX      float64
	ScreenY      float64
	Sprite       *ebiten.Image
	SpriteW      int
	SpriteH      int
	SpriteTarget float64
	DrawGlyph    func(screen *ebiten.Image, x, y float64, zoom float64, antiAlias bool)
}

func DrawThings2D(screen *ebiten.Image, items []ThingDrawItem, zoom float64, antiAlias bool) {
	if screen == nil {
		return
	}
	for _, item := range items {
		if item.Sprite != nil && item.SpriteW > 0 && item.SpriteH > 0 {
			target := item.SpriteTarget
			if target < 6 {
				target = 6
			}
			scale := math.Min(target/float64(item.SpriteW), target/float64(item.SpriteH))
			if scale > 0 {
				op := &ebiten.DrawImageOptions{}
				op.Filter = ebiten.FilterNearest
				op.GeoM.Scale(scale, scale)
				op.GeoM.Translate(item.ScreenX-float64(item.SpriteW)*scale*0.5, item.ScreenY-float64(item.SpriteH)*scale*0.5)
				screen.DrawImage(item.Sprite, op)
				continue
			}
		}
		if item.DrawGlyph != nil {
			item.DrawGlyph(screen, item.ScreenX, item.ScreenY, zoom, antiAlias)
		}
	}
}
