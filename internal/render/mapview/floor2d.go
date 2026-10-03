package mapview

import (
	"math"
	"slices"
)

type FloorFrameStats struct {
	MarkedCols       int
	EmittedSpans     int
	RejectedSpan     int
	RejectNoSector   int
	RejectNoPoly     int
	RejectDegenerate int
	RejectSpanClip   int
}

type WorldBBox struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

type WorldPt struct {
	X float64
	Y float64
}

type WorldEdge struct {
	A, B WorldPt
}

type FloorLoopSet struct {
	Rings [][]WorldPt
	BBox  WorldBBox
	// Edges are directed sector sides, which need not form closed rings.
	// When present, their nonzero winding defines the fill; Rings retain the
	// even-odd fill used by callers that already have polygon boundaries.
	Edges []WorldEdge
}

type ScreenPt struct {
	X float64
	Y float64
}

type FloorRasterInput struct {
	ViewW         int
	ViewH         int
	ViewBBox      WorldBBox
	LoopSets      []FloorLoopSet
	ShadeMuls     []uint32
	Textures      []([]byte)
	FallbackRGB   [3]byte
	ScreenToWorld func(float64, float64) (float64, float64)
	WorldToScreen func(float64, float64) (float64, float64)
}

func RasterizeFloor2D(pix []byte, in FloorRasterInput) FloorFrameStats {
	stats := FloorFrameStats{}
	if len(in.LoopSets) == 0 || in.ViewW <= 0 || in.ViewH <= 0 || len(pix) != in.ViewW*in.ViewH*4 || in.ScreenToWorld == nil || in.WorldToScreen == nil {
		stats.RejectedSpan++
		stats.RejectNoPoly++
		return stats
	}

	w := in.ViewW
	h := in.ViewH

	for sec := range in.LoopSets {
		set := in.LoopSets[sec]
		if len(set.Rings) == 0 && len(set.Edges) == 0 {
			continue
		}
		if set.BBox.MaxX < in.ViewBBox.MinX || set.BBox.MinX > in.ViewBBox.MaxX || set.BBox.MaxY < in.ViewBBox.MinY || set.BBox.MinY > in.ViewBBox.MaxY {
			continue
		}

		texOK := sec >= 0 && sec < len(in.Textures) && len(in.Textures[sec]) == 64*64*4
		var tex []byte
		if texOK {
			tex = in.Textures[sec]
		}
		shadeMul := uint32(256)
		if sec >= 0 && sec < len(in.ShadeMuls) {
			shadeMul = in.ShadeMuls[sec]
		}

		type screenEdge struct{ a, b ScreenPt }
		screenEdges := make([]screenEdge, 0, len(set.Edges))
		minSX := math.Inf(1)
		minSY := math.Inf(1)
		maxSX := math.Inf(-1)
		maxSY := math.Inf(-1)
		toScreen := func(p WorldPt) ScreenPt {
			sx, sy := in.WorldToScreen(p.X, p.Y)
			if sx < minSX {
				minSX = sx
			}
			if sy < minSY {
				minSY = sy
			}
			if sx > maxSX {
				maxSX = sx
			}
			if sy > maxSY {
				maxSY = sy
			}
			return ScreenPt{X: sx, Y: sy}
		}
		if len(set.Edges) > 0 {
			for _, e := range set.Edges {
				screenEdges = append(screenEdges, screenEdge{toScreen(e.A), toScreen(e.B)})
			}
		} else {
			for _, ring := range set.Rings {
				if len(ring) < 3 {
					continue
				}
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					screenEdges = append(screenEdges, screenEdge{toScreen(a), toScreen(b)})
				}
			}
		}
		if len(screenEdges) == 0 || !isFinite(minSX) || !isFinite(minSY) || !isFinite(maxSX) || !isFinite(maxSY) {
			continue
		}

		x0 := max(0, int(math.Floor(minSX)))
		y0 := max(0, int(math.Floor(minSY)))
		x1 := min(w-1, int(math.Ceil(maxSX)))
		y1 := min(h-1, int(math.Ceil(maxSY)))
		if x0 > x1 || y0 > y1 {
			continue
		}

		type crossing struct {
			x     float64
			delta int
		}
		xHits := make([]crossing, 0, 64)
		for py := y0; py <= y1; py++ {
			xHits = xHits[:0]
			row := py * w * 4
			fy := float64(py) + 0.5
			for _, e := range screenEdges {
				a, b := e.a, e.b
				if (a.Y > fy) == (b.Y > fy) {
					continue
				}
				x := a.X + (fy-a.Y)*(b.X-a.X)/(b.Y-a.Y)
				delta := 1
				if b.Y < a.Y {
					delta = -1
				}
				xHits = append(xHits, crossing{x, delta})
			}
			if len(xHits) < 2 {
				continue
			}
			slices.SortFunc(xHits, func(a, b crossing) int {
				if a.x < b.x {
					return -1
				}
				if a.x > b.x {
					return 1
				}
				return 0
			})
			rowWX0, rowWY0 := in.ScreenToWorld(0.5, fy)
			rowWX1, rowWY1 := in.ScreenToWorld(1.5, fy)
			stepWX := rowWX1 - rowWX0
			stepWY := rowWY1 - rowWY0
			winding := 0
			for i := 0; i+1 < len(xHits); i++ {
				if len(set.Edges) > 0 {
					winding += xHits[i].delta
				} else {
					winding ^= 1
				}
				if winding == 0 {
					continue
				}
				start := int(math.Ceil(xHits[i].x - 0.5))
				end := int(math.Ceil(xHits[i+1].x-0.5) - 1)
				if start < x0 {
					start = x0
				}
				if end > x1 {
					end = x1
				}
				if start > end {
					continue
				}
				wx := rowWX0 + float64(start)*stepWX
				wy := rowWY0 + float64(start)*stepWY
				for px := start; px <= end; px++ {
					iPix := row + px*4
					if texOK {
						u := int(math.Floor(wx)) & 63
						v := int(math.Floor(wy)) & 63
						ti := (v*64 + u) * 4
						r, g, b := shadeRGBByMul(tex[ti+0], tex[ti+1], tex[ti+2], shadeMul)
						pix[iPix+0] = r
						pix[iPix+1] = g
						pix[iPix+2] = b
						pix[iPix+3] = 255
						stats.MarkedCols++
					} else {
						r, g, b := shadeRGBByMul(in.FallbackRGB[0], in.FallbackRGB[1], in.FallbackRGB[2], shadeMul)
						pix[iPix+0] = r
						pix[iPix+1] = g
						pix[iPix+2] = b
						pix[iPix+3] = 255
						stats.RejectedSpan++
						stats.RejectNoSector++
					}
					wx += stepWX
					wy += stepWY
				}
				stats.EmittedSpans++
			}
		}
	}

	return stats
}

func shadeRGBByMul(r, g, b byte, mul uint32) (byte, byte, byte) {
	if mul >= 256 {
		return r, g, b
	}
	return byte((uint32(r) * mul) >> 8), byte((uint32(g) * mul) >> 8), byte((uint32(b) * mul) >> 8)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
