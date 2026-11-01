package doomruntime

import "github.com/hajimehoshi/ebiten/v2"

func (g *game) weaponInvisible() bool {
	return g.inventory.InvisTics > 128 || g.inventory.InvisTics&8 != 0
}

// R_DrawFuzzColumn always uses row 6, even with a fixed invulnerability map.
// Read back the already drawn neighbor: negative offsets feed earlier output
// back into the next pixel, including across adjacent original patch posts.
func (g *game) fuzzWeaponPixel(dst []byte, w, x, y int) {
	if len(g.renderColorMap()) < 7*256 || len(g.opts.DoomPaletteRGBA) < 1024 {
		return
	}
	if g.weaponFuzzPalette == nil || g.weaponFuzzGamma != g.gammaLevel {
		g.weaponFuzzGamma = g.gammaLevel
		g.weaponFuzzColors = append(g.weaponFuzzColors[:0], g.opts.DoomPaletteRGBA...)
		for i := 0; i < len(g.weaponFuzzColors); i++ {
			if i%4 != 3 {
				g.weaponFuzzColors[i] = doomGammaTables[clampGamma(g.gammaLevel)][g.weaponFuzzColors[i]]
			}
		}
		g.weaponFuzzPalette = make(map[uint32]byte, 256)
		for i := 0; i < 256; i++ {
			p := g.weaponFuzzColors[i*4:]
			g.weaponFuzzPalette[uint32(p[0])<<16|uint32(p[1])<<8|uint32(p[2])] = byte(i)
		}
	}
	offset := doomFuzzOffsets[g.spectreFuzzPos%len(doomFuzzOffsets)]
	g.spectreFuzzPos = (g.spectreFuzzPos + 1) % len(doomFuzzOffsets)
	source := ((y+offset)*w + x) * 4
	key := uint32(dst[source])<<16 | uint32(dst[source+1])<<8 | uint32(dst[source+2])
	idx, ok := g.weaponFuzzPalette[key]
	if !ok {
		best := int(^uint(0) >> 1)
		for i := 0; i < 256; i++ {
			p := g.weaponFuzzColors[i*4:]
			r, b, c := int(dst[source])-int(p[0]), int(dst[source+1])-int(p[1]), int(dst[source+2])-int(p[2])
			d := r*r + b*b + c*c
			if d < best {
				best = d
				idx = byte(i)
			}
		}
	}
	mapped := int(g.renderColorMap()[6*256+int(idx)])
	copy(dst[(y*w+x)*4:(y*w+x)*4+4], g.weaponFuzzColors[mapped*4:mapped*4+4])
}

func (g *game) drawModernInvisibleWeapon(screen *ebiten.Image) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if g.weaponClassicImage == nil || g.weaponClassicImage.Bounds().Dx() != w || g.weaponClassicImage.Bounds().Dy() != h {
		if g.weaponClassicImage != nil {
			g.weaponClassicImage.Dispose()
		}
		g.weaponClassicImage = ebiten.NewImage(w, h)
	}
	if len(g.weaponClassicPixels) != w*h*4 {
		g.weaponClassicPixels = make([]byte, w*h*4)
		g.weaponFuzzPixels = make([]byte, w*h*4)
	}
	if len(g.weaponFuzzPixels) != w*h*4 {
		g.weaponFuzzPixels = make([]byte, w*h*4)
	}
	g.weaponClassicImage.Clear()
	g.drawWeaponOverlayImage(g.weaponClassicImage)
	g.weaponClassicImage.ReadPixels(g.weaponClassicPixels)
	screen.ReadPixels(g.weaponFuzzPixels)
	rect := g.walkWeaponViewportRect()

	cw, ch := min(320, w), min(200, h)
	if len(g.weaponFuzzLogical) != cw*ch*4 {
		g.weaponFuzzLogical = make([]byte, cw*ch*4)
	}
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			source := (fuzzSamplePixel(y, h, ch)*w + fuzzSamplePixel(x, w, cw)) * 4
			copy(g.weaponFuzzLogical[(y*cw+x)*4:], g.weaponFuzzPixels[source:source+4])
		}
	}
	tex := WallTexture{RGBA: g.weaponClassicPixels, Width: w, Height: h}
	it := cutoutItem{tex: &tex, scale: 1, scaleY: 1, x0: rect.Min.X, x1: rect.Max.X - 1, y0: rect.Min.Y, y1: rect.Max.Y - 1}
	g.walkSpectreFuzzSpans(it, func(span spectreFuzzSpan) {
		// The walker already advanced the frame phase. Replay this column's
		// saved phase for its ordered sampling, then retain the walker's end.
		endPhase := g.spectreFuzzPos
		g.spectreFuzzPos = span.phase
		for cy := span.cy; ; cy++ {
			top, bottom := fuzzCellBounds(cy, h, ch)
			if top > span.y1 {
				break
			}
			g.fuzzWeaponPixel(g.weaponFuzzLogical, cw, span.cx, cy)
			src := (cy*cw + span.cx) * 4
			for yy := max(top, span.y0); yy <= min(bottom, span.y1); yy++ {
				for xx := span.x0; xx <= span.x1; xx++ {
					copy(g.weaponFuzzPixels[(yy*w+xx)*4:], g.weaponFuzzLogical[src:src+4])
				}
			}
		}
		g.spectreFuzzPos = endPhase
	})

	screen.WritePixels(g.weaponFuzzPixels)
}
