package doomruntime

var spectrePaletteIndexByPacked map[uint32]uint8

// A fuzz span is a post on the original 320x200 grid. Its phase advances once
// per opaque logical pixel, column first, independently of output resolution.
type spectreFuzzSpan struct {
	x0, x1, y0, y1 int
	cx, cy, phase  int
}

func fuzzCellBounds(cell, pixels, logical int) (int, int) {
	return (cell*pixels + logical - 1) / logical, ((cell+1)*pixels+logical-1)/logical - 1
}

func fuzzSamplePixel(cell, pixels, logical int) int {
	return min(max((cell*pixels+pixels/2)/logical, 0), pixels-1)
}

// Only inspect at most 320x200 mask samples. Magnifying a spectre therefore
// increases its grain size without increasing mask work or upload volume.
func (g *game) walkSpectreFuzzSpans(it cutoutItem, emit func(spectreFuzzSpan)) {
	if g == nil || it.tex == nil || it.scale <= 0 || g.viewW <= 0 || g.viewH <= 0 {
		return
	}
	t := it.tex
	if t.Width <= 0 || t.Height <= 0 {
		return
	}
	t.EnsureOpaqueMask()
	hasMask := len(t.OpaqueMask) == t.Width*t.Height
	hasRGBA32 := len(t.RGBA32) == t.Width*t.Height
	if !hasMask && !hasRGBA32 && len(t.Indexed) != t.Width*t.Height {
		return
	}
	x0, x1 := max(it.x0, 0), min(it.x1, g.viewW-1)
	y0, y1 := max(it.y0, 0), min(it.y1, g.viewH-1)
	if x0 > x1 || y0 > y1 {
		return
	}
	cw, ch := min(doomLogicalW, g.viewW), min(doomLogicalH, g.viewH)
	sy := it.scale
	if it.scaleY > 0 {
		sy = it.scaleY
	}
	for cx := x0 * cw / g.viewW; cx <= x1*cw/g.viewW; cx++ {
		left, right := fuzzCellBounds(cx, g.viewW, cw)
		left, right = max(left, x0), min(right, x1)
		if left > right {
			continue
		}
		tx := min(max(int((float64((left+right)/2)+0.5-it.dstX)/it.scale), 0), t.Width-1)
		if it.flip {
			tx = t.Width - 1 - tx
		}
		start, phase := -1, 0
		flush := func(end int) {
			if start < 0 {
				return
			}
			top, _ := fuzzCellBounds(start, g.viewH, ch)
			_, bottom := fuzzCellBounds(end, g.viewH, ch)
			emit(spectreFuzzSpan{left, right, max(top, y0), min(bottom, y1), cx, start, phase})
			start = -1
		}
		for cy := y0 * ch / g.viewH; cy <= y1*ch/g.viewH; cy++ {
			// Classic Doom leaves the first and last logical rows untouched.
			if cy == 0 || cy == ch-1 {
				flush(cy - 1)
				continue
			}
			top, bottom := fuzzCellBounds(cy, g.viewH, ch)
			top, bottom = max(top, y0), min(bottom, y1)
			ty := min(max(int((float64((top+bottom)/2)+0.5-it.dstY)/sy), 0), t.Height-1)
			i := ty*t.Width + tx
			opaque := true
			if hasMask {
				opaque = t.OpaqueMask[i] != 0
			} else if hasRGBA32 {
				opaque = t.RGBA32[i]&pixelOpaqueA != 0
			}
			if !opaque {
				flush(cy - 1)
				continue
			}
			if start < 0 {
				start, phase = cy, g.spectreFuzzPos
			}
			g.nextSourcePortFuzzOffset()
		}
		flush(y1 * ch / g.viewH)
	}
}

// Preserve full-resolution wall/portal clipping around the coarse silhouette.
func (g *game) clipSpectreFuzzSpan(it cutoutItem, span spectreFuzzSpan, emit func(x, y0, y1 int)) {
	for x := span.x0; x <= span.x1; x++ {
		if !xInSolidSpans(x, it.clipSpans) {
			continue
		}
		for _, visible := range g.maskedColumnVisibleSpans(x, span.y0, span.y1, it.depthQ) {
			emit(x, visible.L, visible.R)
		}
	}
}

func (g *game) drawShadowSpriteCutout(it cutoutItem) {
	cw, ch := min(doomLogicalW, g.viewW), min(doomLogicalH, g.viewH)
	g.walkSpectreFuzzSpans(it, func(span spectreFuzzSpan) {
		// Column order and in-place sampling retain classic fuzz feedback: a
		// negative offset can pick up the pixel just darkened in the row above.
		for cy := span.cy; ; cy++ {
			top, bottom := fuzzCellBounds(cy, g.viewH, ch)
			if top > span.y1 {
				break
			}
			direction := doomFuzzOffsets[(span.phase+cy-span.cy)%len(doomFuzzOffsets)]
			sx := fuzzSamplePixel(span.cx, g.viewW, cw)
			sy := fuzzSamplePixel(cy+direction, g.viewH, ch)
			p := g.wallPix32[sy*g.viewW+sx]
			if p == 0 {
				p = pixelOpaqueA
			}
			p = g.shadePackedSpectreFuzz(p)
			cell := span
			cell.y0, cell.y1 = max(top, span.y0), min(bottom, span.y1)
			g.clipSpectreFuzzSpan(it, cell, func(x, y0, y1 int) {
				for y := y0; y <= y1; y++ {
					g.writeWallPixel(y*g.viewW+x, p)
				}
			})
		}
	})
}
