package doomruntime

import "github.com/hajimehoshi/ebiten/v2"

// Match R_DrawPSprite/R_DrawMaskedColumn: horizontal edges floor, post starts
// ceil, and column samples use fixed-point texturemid relative to each post.
// A normal scaled image uses pixel centers and disagrees at fractional poses.
func (g *game) drawClassicWeaponOverlay(screen *ebiten.Image) bool {
	if len(g.opts.DoomPaletteRGBA) < 256*4 || len(g.renderColorMap()) < 32*256 {
		return false
	}
	rect := g.walkWeaponViewportRect()
	w, h := rect.Dx(), rect.Dy()
	if w <= 0 || h <= 0 {
		return true
	}
	if g.weaponClassicImage == nil || g.weaponClassicImage.Bounds().Dx() != w || g.weaponClassicImage.Bounds().Dy() != h {
		if g.weaponClassicImage != nil {
			g.weaponClassicImage.Dispose()
		}
		g.weaponClassicImage = ebiten.NewImage(w, h)
		g.weaponClassicPixels = make([]byte, w*h*4)
	}
	clear(g.weaponClassicPixels)
	if g.weaponInvisible() {
		screen.SubImage(rect).(*ebiten.Image).ReadPixels(g.weaponClassicPixels)
	}
	x, y := g.weaponPresentationPosition()
	for _, name := range []string{g.weaponSpriteName(), g.weaponFlashSpriteName()} {
		tex, ok := g.opts.SpritePatchBank[name]
		if !ok {
			continue
		}
		g.rasterClassicWeapon(g.weaponClassicPixels, w, h, tex, int64(x*fracUnit), int64(y*fracUnit), g.weaponLightRow(weaponPatchFullbright(name)))
	}
	g.weaponClassicImage.WritePixels(g.weaponClassicPixels)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.DrawImage(g.weaponClassicImage, op)
	return true
}

func (g *game) rasterClassicWeapon(dst []byte, w, h int, tex WallTexture, sx, sy int64, row int) {
	if tex.Width <= 0 || tex.Height <= 0 || len(tex.Indexed) != tex.Width*tex.Height {
		return
	}
	scale := int64(w) * fracUnit / 320
	if scale <= 0 {
		return
	}
	inv := int64(320) * fracUnit / int64(w)
	tx := sx - 160*fracUnit - int64(tex.OffsetX)*fracUnit
	x1 := int((int64(w/2)*fracUnit + fixedMul(tx, scale)) >> fracBits)
	x2 := int((int64(w/2)*fracUnit+fixedMul(tx+int64(tex.Width)*fracUnit, scale))>>fracBits) - 1
	texturemid := int64(100)*fracUnit + fracUnit/2 - (sy - int64(tex.OffsetY)*fracUnit)
	topscreen := int64(h/2)*fracUnit - fixedMul(texturemid, scale)
	opaque := func(x, y int) bool {
		i := y*tex.Width + x
		if len(tex.OpaqueMask) == tex.Width*tex.Height {
			return tex.OpaqueMask[i] != 0
		}
		return len(tex.RGBA) == tex.Width*tex.Height*4 && tex.RGBA[i*4+3] != 0
	}
	for x := max(0, x1); x <= min(w-1, x2); x++ {
		u := int((int64(x-x1) * inv) >> fracBits)
		if u < 0 || u >= tex.Width {
			continue
		}
		drawPost := func(top, end int, source []byte) {
			yl := max(0, int((topscreen+scale*int64(top)+fracUnit-1)>>fracBits))
			yh := min(h-1, int((topscreen+scale*int64(end)-1)>>fracBits))
			if g.weaponInvisible() {
				for y := max(1, yl); y <= min(h-2, yh); y++ {
					g.fuzzWeaponPixel(dst, w, x, y)
				}
				return
			}
			frac := texturemid - int64(top)*fracUnit + int64(yl-h/2)*inv
			for y := yl; y <= yh; y++ {
				v := int((frac >> fracBits) & 127)
				frac += inv
				var idx byte
				if source != nil {
					if v >= len(source) {
						continue
					}
					idx = source[v]
				} else {
					if top+v >= end {
						continue
					}
					idx = tex.Indexed[(top+v)*tex.Width+u]
				}
				mapped := g.renderColorMap()[row*256+int(idx)]
				copy(dst[(y*w+x)*4:(y*w+x)*4+4], g.opts.DoomPaletteRGBA[int(mapped)*4:int(mapped)*4+4])
			}
		}
		if len(tex.ClassicMasked) == tex.Width {
			for _, post := range tex.ClassicMasked[u] {
				drawPost(post.Top, post.Top+post.Length, post.Indexed)
			}
			continue
		}
		for top := 0; top < tex.Height; {
			if !opaque(u, top) {
				top++
				continue
			}
			end := top + 1
			for end < tex.Height && opaque(u, end) {
				end++
			}
			drawPost(top, end, nil)
			top = end
		}
	}
}
