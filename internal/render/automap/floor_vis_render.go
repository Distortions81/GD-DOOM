package automap

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

func (g *game) markScreenPolygonColumns(pl *floorVisplane, poly []screenPt) {
	if pl == nil || len(poly) < 3 {
		return
	}
	minX := g.viewW - 1
	maxX := 0
	for _, p := range poly {
		ix := int(math.Floor(p.x))
		ax := int(math.Ceil(p.x)) - 1
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
		}
		if ax > maxX {
			maxX = ax
		}
	}
	if minX < 0 {
		minX = 0
	}
	if maxX >= g.viewW {
		maxX = g.viewW - 1
	}
	if minX > maxX {
		return
	}

	ys := make([]float64, 0, len(poly))
	for x := minX; x <= maxX; x++ {
		ys = ys[:0]
		sx := float64(x) + 0.5
		for i := 0; i < len(poly); i++ {
			a := poly[i]
			b := poly[(i+1)%len(poly)]
			if (a.x <= sx && b.x > sx) || (b.x <= sx && a.x > sx) {
				t := (sx - a.x) / (b.x - a.x)
				y := a.y + (b.y-a.y)*t
				ys = append(ys, y)
			}
		}
		if len(ys) < 2 {
			continue
		}
		sort.Float64s(ys)
		for i := 0; i+1 < len(ys); i += 2 {
			// Slight epsilon expansion avoids 1px cracks between adjacent polygons.
			const eps = 1e-4
			top := int(math.Ceil(ys[i] - eps))
			bottom := int(math.Floor(ys[i+1] + eps))
			if markFloorColumnRange(pl, x, top, bottom, g.floorClip, g.ceilingClip) {
				g.floorFrame.markedCols++
			} else {
				g.floorFrame.rejectedSpan++
			}
		}
	}
}

func (g *game) buildFloorVisplaneMarks() {
	for ss := range g.m.SubSectors {
		worldVerts, cx, cy, ok := g.subSectorWorldVertices(ss)
		if !ok {
			worldVerts, cx, cy, ok = g.subSectorVerticesFromSegList(ss)
		}
		if !ok {
			worldVerts, cx, cy, ok = g.subSectorConvexVertices(ss)
		}
		if !ok {
			continue
		}
		poly := make([]screenPt, 0, len(worldVerts))
		for _, v := range worldVerts {
			sx, sy := g.worldToScreen(v.x, v.y)
			poly = append(poly, screenPt{x: sx, y: sy})
		}
		secIdx, ok := g.subSectorSectorIndex(ss)
		if !ok || secIdx < 0 || secIdx >= len(g.m.Sectors) {
			secIdx = g.sectorAt(int64(cx*fracUnit), int64(cy*fracUnit))
			if secIdx < 0 || secIdx >= len(g.m.Sectors) {
				continue
			}
		}
		sec := g.m.Sectors[secIdx]
		key := floorPlaneKey{
			flat:   normalizeFlatName(sec.FloorPic),
			floorH: sec.FloorHeight,
			light:  sec.Light,
		}
		pl := g.floorVisplaneForKey(key)
		g.markScreenPolygonColumns(pl, poly)
	}
}

func (g *game) buildFloorVisplaneSpans() {
	g.floorSpans = g.floorSpans[:0]
	keys := make([]floorPlaneKey, 0, len(g.floorPlanes))
	for k := range g.floorPlanes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].flat != keys[j].flat {
			return keys[i].flat < keys[j].flat
		}
		if keys[i].floorH != keys[j].floorH {
			return keys[i].floorH < keys[j].floorH
		}
		return keys[i].light < keys[j].light
	})
	for _, k := range keys {
		pl := g.floorPlanes[k]
		if pl.minX > pl.maxX {
			continue
		}
		g.floorSpans = makePlaneSpans(pl, g.viewH, g.floorSpans)
	}
	g.floorFrame.emittedSpans = len(g.floorSpans)
}

func (g *game) renderFloorVisplaneSpans(screen *ebiten.Image) {
	if g.floorDbgMode == floorDebugSolid {
		clr := color.RGBA{R: 95, G: 145, B: 215, A: 255}
		for _, sp := range g.floorSpans {
			ebitenutil.DrawRect(screen, float64(sp.x1), float64(sp.y), float64(sp.x2-sp.x1+1), 1, clr)
		}
		g.drawFloorVisplaneDiagnostics(screen)
		return
	}
	g.ensureMapFloorLayer()
	pix := g.mapFloorPix
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = 0
		pix[i+1] = 0
		pix[i+2] = 0
		pix[i+3] = 0
	}
	rowWX0 := make([]float64, g.viewH)
	rowWY0 := make([]float64, g.viewH)
	rowStepWX := make([]float64, g.viewH)
	rowStepWY := make([]float64, g.viewH)
	for y := 0; y < g.viewH; y++ {
		wx0, wy0 := g.screenToWorld(0.5, float64(y)+0.5)
		wx1, wy1 := g.screenToWorld(1.5, float64(y)+0.5)
		rowWX0[y] = wx0
		rowWY0[y] = wy0
		rowStepWX[y] = wx1 - wx0
		rowStepWY[y] = wy1 - wy0
	}
	flatCache := make(map[string][]byte, 64)
	for _, sp := range g.floorSpans {
		if sp.y < 0 || sp.y >= g.viewH {
			continue
		}
		if sp.x1 < 0 {
			sp.x1 = 0
		}
		if sp.x2 >= g.viewW {
			sp.x2 = g.viewW - 1
		}
		if sp.x1 > sp.x2 {
			continue
		}
		row := sp.y * g.viewW * 4
		wx := rowWX0[sp.y] + rowStepWX[sp.y]*float64(sp.x1)
		wy := rowWY0[sp.y] + rowStepWY[sp.y]*float64(sp.x1)
		stepWX := rowStepWX[sp.y]
		stepWY := rowStepWY[sp.y]
		tex := flatCache[sp.key.flat]
		if tex == nil {
			tex = g.opts.FlatBank[sp.key.flat]
			flatCache[sp.key.flat] = tex
		}
		for x := sp.x1; x <= sp.x2; x++ {
			if x < 0 || x >= g.viewW {
				wx += stepWX
				wy += stepWY
				continue
			}
			i := row + x*4
			if g.floorDbgMode == floorDebugUV {
				u := frac01(wx / 64.0)
				v := frac01(wy / 64.0)
				pix[i+0] = uint8(u * 255)
				pix[i+1] = uint8(v * 255)
				pix[i+2] = 0
				pix[i+3] = 255
				wx += stepWX
				wy += stepWY
				continue
			}
			if len(tex) != 64*64*4 {
				pix[i+0] = 90
				pix[i+1] = 125
				pix[i+2] = 160
				pix[i+3] = 255
				wx += stepWX
				wy += stepWY
				continue
			}
			u := int(math.Floor(wx)) & 63
			v := int(math.Floor(wy)) & 63
			ti := (v*64 + u) * 4
			pix[i+0] = tex[ti+0]
			pix[i+1] = tex[ti+1]
			pix[i+2] = tex[ti+2]
			pix[i+3] = 255
			wx += stepWX
			wy += stepWY
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, &ebiten.DrawImageOptions{})
	g.drawFloorVisplaneDiagnostics(screen)
}

func (g *game) drawFloorVisplaneDiagnostics(screen *ebiten.Image) {
	drawClip := g.floorVisDiag == floorVisDiagClip || g.floorVisDiag == floorVisDiagBoth
	drawSpan := g.floorVisDiag == floorVisDiagSpan || g.floorVisDiag == floorVisDiagBoth
	if drawSpan {
		clr := color.RGBA{R: 255, G: 64, B: 180, A: 255}
		for _, sp := range g.floorSpans {
			ebitenutil.DrawRect(screen, float64(sp.x1), float64(sp.y), float64(sp.x2-sp.x1+1), 1, clr)
		}
	}
	if !drawClip {
		return
	}
	topClr := color.RGBA{R: 60, G: 255, B: 120, A: 255}
	botClr := color.RGBA{R: 255, G: 180, B: 40, A: 255}
	for _, pl := range g.floorPlanes {
		if pl.minX > pl.maxX {
			continue
		}
		for x := pl.minX; x <= pl.maxX; x++ {
			ix := x + 1
			if ix < 0 || ix >= len(pl.top) {
				continue
			}
			t := int(pl.top[ix])
			b := int(pl.bottom[ix])
			if t != int(floorUnset) && t >= 0 && t < g.viewH {
				ebitenutil.DrawRect(screen, float64(x), float64(t), 1, 1, topClr)
			}
			if b != int(floorUnset) && b >= 0 && b < g.viewH {
				ebitenutil.DrawRect(screen, float64(x), float64(b), 1, 1, botClr)
			}
		}
	}
}
