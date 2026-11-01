package doomruntime

import (
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/scene"
	"math"
	"strings"
)

func (g *game) initBoomTranslucency() {
	g.boomHasTranslucency = false
	g.boomTranMapCount = 0
	g.boomLineTranMap = nil
	found := false
	for _, line := range g.m.Linedefs {
		if line.Special == 260 {
			found = true
			break
		}
	}
	if !found {
		return
	}
	g.boomLineTranMap = make([][]byte, len(g.m.Linedefs))
	var defaultMap []byte
	byTag := make(map[uint16][]int)
	for i, l := range g.m.Linedefs {
		byTag[l.Tag] = append(byTag[l.Tag], i)
	}
	for i, line := range g.m.Linedefs {
		if line.Special != 260 || line.SideNum[0] < 0 || int(line.SideNum[0]) >= len(g.m.Sidedefs) {
			continue
		}
		sideID := int(line.SideNum[0])
		side := *g.renderSidedef(sideID)
		name := strings.ToUpper(g.m.Sidedefs[sideID].Mid)
		table := g.opts.BoomTranMaps[name]
		if name == "TRANMAP" || len(table) == 65536 {
			side.Mid = "-"
			g.boomColormapSides[sideID] = side
		}
		if name == "TRANMAP" || len(table) != 65536 {
			if defaultMap == nil {
				defaultMap = g.opts.BoomTranMaps["TRANMAP"]
				if len(defaultMap) != 65536 {
					defaultMap = doomtex.GenerateBoomTranMap(g.opts.DoomPaletteRGBA)
				}
			}
			table = defaultMap
		}
		if len(table) != 65536 {
			continue
		}
		targets := []int{i}
		if line.Tag != 0 {
			targets = byTag[line.Tag]
		}
		for _, target := range targets {
			g.boomLineTranMap[target] = table
			g.boomHasTranslucency = true
		}
	}
	unique := make(map[*byte]bool)
	for i, table := range g.boomLineTranMap {
		if len(table) == 65536 && g.m.Linedefs[i].SideNum[1] >= 0 {
			unique[&table[0]] = true
		}
	}
	g.boomTranMapCount = len(unique)
}
func boomPaletteIndex(p uint32) byte {
	if index, ok := spectrePaletteIndexByPacked[p|pixelOpaqueA]; ok {
		return index
	}
	index, _ := packedColorPaletteIndex(p)
	return index
}
func (g *game) boomLitIndex(index byte, shade, row int) byte {
	if row >= doomNumColorMaps || doomColormapEnabled {
		cm := g.renderColorMap()
		r := min(max(row, 0), max(doomColormapRowCount()-1, 0))
		if len(cm) > r*256+int(index) {
			return cm[r*256+int(index)]
		}
	}
	packed := maskedWallShadePackedRow(shade, row)
	if len(packed) == 256 {
		return boomPaletteIndex(packed[index])
	}
	return index
}
func (g *game) blendBoomPixel(i int, index byte, shade, row int, table []byte) {
	src, dst := g.boomLitIndex(index, shade, row), boomPaletteIndex(g.wallPix32[i])
	g.wallPix32[i] = wallShadePackedLUT[256][table[int(dst)*256+int(src)]]
}

// Masked columns keep their post clipping and lighting before TRANMAP lookup.
func (g *game) drawBoomTranslucentMid(ms maskedMidSeg, focal float64, shade, row int) {
	var commands *gpuCommands
	if g.gpuFrame != nil {
		commands = g.gpuFrame.startTranslucentCutout(ms.tranmap, ms.X0, ms.X1)
	}
	proj := scene.NewWallProjectionStepper(ms.Projection, ms.X0)
	envelope := newMaskedMidEnvelopeStepper(ms.Projection, ms.X0, ms.WorldHigh, ms.WorldLow, focal, float64(g.viewH)/2)
	tex := ms.tex.from
	for x := ms.X0; x <= ms.X1; x++ {
		depth, u, ok := proj.Sample()
		depth, u = ms.classic.sample(x, depth, u)
		y0, y1, envOK := envelope.Sample()
		y0, y1, envOK = ms.classicClip(x, y0, y1, envOK)
		proj.Next()
		envelope.Next()
		if !ok || !envOK || depth <= 0 || y0 > y1 {
			continue
		}
		sh, light := g.maskedMidColumnShade(ms, depth, shade, row)
		tx := maskedMidTextureColumn(*tex, u+ms.TexUOff)
		y0 = max(0, y0)
		y1 = min(g.viewH-1, y1)
		if y0 > y1 {
			continue
		}
		if !g.opts.SourcePortMode && len(tex.ClassicMasked) == tex.Width {
			scale := int64(max(256, min(64*fracUnit, int(math.Round(focal/depth*fracUnit)))))
			step := int64(uint64(0xffffffff) / uint64(scale))
			mid := floorFixed(ms.TexMid)
			topScreen := int64(g.viewH/2)*fracUnit - fixedMul(mid, scale)
			for _, post := range tex.ClassicMasked[tx] {
				top := topScreen + scale*int64(post.Top)
				bottom := top + scale*int64(post.Length)
				lo, hi := max(y0, int((top+fracUnit-1)>>fracBits)), min(y1, int((bottom-1)>>fracBits))
				if lo > hi {
					continue
				}
				frac := mid - int64(post.Top)*fracUnit + int64(lo-g.viewH/2)*step
				if g.gpuFrame != nil {
					r := g.gpuFrame
					t, ok := r.texture(post.Indexed, nil, 1, 128)
					if ok {
						r.rect(commands, x, lo, x, hi, t, t, 13, -gpuLightRow(sh, light)-1, 0, 0, gpuWrapFixed(frac, 128), 0, gpuWrapFixed(step, 128))
					}
					continue
				}
				for y := lo; y <= hi; y++ {
					g.blendBoomPixel(y*g.viewW+x, post.Indexed[(frac>>fracBits)&127], sh, light, ms.tranmap)
					frac += step
				}
			}
			continue
		}
		tex.EnsureOpaqueMask()
		frac, step := g.wallColumnTextureStep(y0, ms.TexMid, depth, focal)
		if g.gpuFrame != nil {
			r := g.gpuFrame
			t, ok := r.wallTexture(tex)
			if ok {
				for _, vis := range g.maskedColumnVisibleSpans(x, y0, y1, encodeDepthQ(depth)) {
					v := frac + int64(vis.L-y0)*step
					r.rect(commands, x, vis.L, x, vis.R, t, t, 13, -gpuLightRow(sh, light)-1, 0, float32(tx), gpuWrapFixed(v, tex.Height), 0, gpuWrapFixed(step, tex.Height))
				}
			}
			continue
		}
		for _, vis := range g.maskedColumnVisibleSpans(x, y0, y1, encodeDepthQ(depth)) {
			v := frac + int64(vis.L-y0)*step
			for y := vis.L; y <= vis.R; y++ {
				ty := wrapIndex(int(v>>fracBits), tex.Height)
				i := ty*tex.Width + tx
				if len(tex.Indexed) > i && len(tex.OpaqueMask) > i && tex.OpaqueMask[i] != 0 {
					g.blendBoomPixel(y*g.viewW+x, tex.Indexed[i], sh, light, ms.tranmap)
				}
				v += step
			}
		}
	}
}

func (g *game) tranMapForLine(index int) []byte {
	if index >= 0 && index < len(g.boomLineTranMap) {
		return g.boomLineTranMap[index]
	}
	return nil
}
