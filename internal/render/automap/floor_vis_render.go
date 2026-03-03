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
		if ix < minX {
			minX = ix
		}
		if ix > maxX {
			maxX = ix
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
			top := int(math.Ceil(ys[i]))
			bottom := int(math.Floor(ys[i+1]))
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
	for _, pl := range g.floorPlanes {
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
	for _, sp := range g.floorSpans {
		if sp.y < 0 || sp.y >= g.viewH {
			continue
		}
		row := sp.y * g.viewW * 4
		for x := sp.x1; x <= sp.x2; x++ {
			if x < 0 || x >= g.viewW {
				continue
			}
			i := row + x*4
			wx, wy := g.screenToWorld(float64(x)+0.5, float64(sp.y)+0.5)
			if g.floorDbgMode == floorDebugUV {
				u := frac01(wx / 64.0)
				v := frac01(wy / 64.0)
				pix[i+0] = uint8(u * 255)
				pix[i+1] = uint8(v * 255)
				pix[i+2] = 0
				pix[i+3] = 255
				continue
			}
			tex, ok := g.opts.FlatBank[sp.key.flat]
			if !ok || len(tex) != 64*64*4 {
				pix[i+0] = 90
				pix[i+1] = 125
				pix[i+2] = 160
				pix[i+3] = 255
				continue
			}
			u := int(math.Floor(wx)) & 63
			v := int(math.Floor(wy)) & 63
			ti := (v*64 + u) * 4
			pix[i+0] = tex[ti+0]
			pix[i+1] = tex[ti+1]
			pix[i+2] = tex[ti+2]
			pix[i+3] = 255
		}
	}
	g.mapFloorLayer.WritePixels(pix)
	screen.DrawImage(g.mapFloorLayer, &ebiten.DrawImageOptions{})
}
