package doomruntime

import "math"

func (g *game) classicMaskedClipBuffers(n int) ([]int16, []int16) {
	start := len(g.maskedClassicClipScratch)
	g.maskedClassicClipScratch = append(g.maskedClassicClipScratch, make([]int16, 2*n)...)
	return g.maskedClassicClipScratch[start : start+n], g.maskedClassicClipScratch[start+n : start+2*n]
}

func (ms maskedMidSeg) classicClip(x, top, bottom int, ok bool) (int, int, bool) {
	i := x - ms.classic.start
	if i >= 0 && i < len(ms.classicTop) {
		top, bottom = int(ms.classicTop[i]), int(ms.classicBottom[i])
		ok = top <= bottom
	}
	return top, bottom, ok
}

// R_DrawMaskedColumn projects each post with the wall's fixed scale, then
// samples relative to that post with the reciprocal scale. Trimming based on
// sampled texture rows instead loses boundary pixels when the first frac is
// just below zero (classic reads source byte 127 in that case).
func (g *game) drawClassicMaskedColumn(x, y0, y1 int, depth, texU, texMid, focal float64, tex *WallTexture, shade, row int) {
	tx := maskedMidTextureColumn(*tex, texU)
	scale := int64(max(256, min(64*fracUnit, int(math.Round(focal/depth*fracUnit)))))
	step := int64(uint64(0xffffffff) / uint64(scale))
	mid := floorFixed(texMid)
	topScreen := int64(g.viewH/2)*fracUnit - fixedMul(mid, scale)
	packed := maskedWallShadePackedRow(shade, row)
	if g.gpuFrame == nil && len(packed) != 256 {
		return
	}
	for _, post := range tex.ClassicMasked[tx] {
		top := topScreen + scale*int64(post.Top)
		bottom := top + scale*int64(post.Length)
		lo := max(y0, int((top+fracUnit-1)>>fracBits))
		hi := min(y1, int((bottom-1)>>fracBits))

		if lo > hi {
			continue
		}
		// y0/y1 already come from classic's saved ceiling/floor clips.
		// Applying the approximate quantized depth mask a second time can
		// erase a boundary pixel that R_DrawMaskedColumn includes.
		frac := mid - int64(post.Top)*fracUnit + int64(lo-g.viewH/2)*step
		if g.gpuFrame != nil {
			r := g.gpuFrame
			texture, ok := r.texture(post.Indexed, nil, 1, 128)
			if !ok {
				continue
			}
			r.rect(r.opaqueCutoutCommands(), x, lo, x, hi, texture, texture, 1, -gpuLightRow(shade, row)-1, 0, 0, gpuWrapFixed(frac, 128), 0, gpuWrapFixed(step, 128))
			continue
		}
		for y := lo; y <= hi; y++ {
			i := y*g.viewW + x
			if !g.cutoutCoveredAtIndex(i) {
				g.wallPix32[i] = packed[post.Indexed[(frac>>fracBits)&127]]
				g.markCutoutCoveredAtIndex(i)
			}
			frac += step
		}
	}
}
