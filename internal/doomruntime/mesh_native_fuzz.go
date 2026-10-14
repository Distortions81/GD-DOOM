package doomruntime

import (
	"gddoom/internal/render/levelmesh"
	"math"
)

type nativeFuzzPalette struct {
	palette, lookup levelmesh.Texture
	probes          int
}

// SpectreFuzz retains the main frontend's mask sampling and column-first phase
// progression. Only logical posts are returned; hardware depth clips the final
// card against native walls and other objects.
func (n *NativeMeshGame) SpectreFuzz(s levelmesh.Sprite, c levelmesh.Camera, width, height int) []levelmesh.FuzzSpan {
	n.fuzzSpans = n.fuzzSpans[:0]
	x, y, sx, sy, _, visible := levelmesh.ProjectSprite(s, c, width, height)
	if !s.Shadow || !visible {
		return n.fuzzSpans
	}
	tex := WallTexture{RGBA: s.Texture.RGBA, Indexed: s.Texture.Indexed, Width: s.Texture.Width, Height: s.Texture.Height}
	it := cutoutItem{tex: &tex, scale: sx, scaleY: sy, dstX: x, dstY: y, flip: s.Flip,
		x0: max(0, int(math.Floor(x))), x1: min(width-1, int(math.Ceil(x+float64(tex.Width)*sx))-1),
		y0: max(0, int(math.Floor(y))), y1: min(height-1, int(math.Ceil(y+float64(tex.Height)*sy))-1)}
	g := n.g
	w, h := g.viewW, g.viewH
	g.viewW, g.viewH = width, height
	defer func() { g.viewW, g.viewH = w, h }()
	g.walkSpectreFuzzSpans(it, func(span spectreFuzzSpan) {
		n.fuzzSpans = append(n.fuzzSpans, levelmesh.FuzzSpan{X: span.cx, Y0: span.cy, Y1: span.y1 * min(height, 200) / height, Phase: span.phase})
	})
	return n.fuzzSpans
}

// SpectreFuzzColors shares Ebiten's exact active-palette hash, quantized
// fallback and raw COLORMAP feedback. The small immutable images upload once.
func (n *NativeMeshGame) SpectreFuzzColors() levelmesh.FuzzColors {
	data := levelmesh.FuzzColors{Offsets: gpuFuzzOffsets, Shade: 1}
	if doomLightingEnabled {
		data.Shade = float32(doomShadeMulFromRow(6)) / 256
	}
	row := 6
	if fixed, active := n.g.playerFixedColormapRow(); active {
		row, data.RemapEnabled = fixed, true
	} else {
		data.RemapEnabled = doomColormapEnabled || (doomLightingEnabled && n.g.opts.DoomColorMapRows > 6)
	}
	if !data.RemapEnabled || len(n.g.opts.DoomColorMap) < (row+1)*256 || len(doomPalIndexLUT32) != 32768 {
		data.RemapEnabled = false
		return data
	}
	bank, ok := n.fuzzColors[activeGammaLevel]
	if !ok {
		palette := make([]byte, 256*4)
		for i, p := range wallShadePackedLUT[256] {
			putGPUPackedPixel(palette, i*4, p)
		}
		lookup := make([]byte, 256*256*4)
		for i, index := range doomPalIndexLUT32 {
			lookup[(gpuPaletteLookupOffset+i)*4+2], lookup[(gpuPaletteLookupOffset+i)*4+3] = index, 255
		}
		bank = nativeFuzzPalette{palette: levelmesh.Texture{RGBA: palette, Width: 256, Height: 1}, lookup: levelmesh.Texture{RGBA: lookup, Width: 256, Height: 256}, probes: putGPUFuzzPalette(lookup, wallShadePackedLUT[256][:])}
		if n.fuzzColors == nil {
			n.fuzzColors = make(map[int]nativeFuzzPalette)
		}
		n.fuzzColors[activeGammaLevel] = bank
	}
	remap, ok := n.fuzzRemaps[row]
	if !ok {
		pixels := make([]byte, 256*4)
		for i, index := range n.g.opts.DoomColorMap[row*256 : (row+1)*256] {
			pixels[i*4], pixels[i*4+3] = index, 255
		}
		remap = levelmesh.Texture{RGBA: pixels, Width: 256, Height: 1}
		if n.fuzzRemaps == nil {
			n.fuzzRemaps = make(map[int]levelmesh.Texture)
		}
		n.fuzzRemaps[row] = remap
	}
	data.Palette, data.Lookup, data.Remap, data.Probes = bank.palette, bank.lookup, remap, bank.probes
	return data
}
