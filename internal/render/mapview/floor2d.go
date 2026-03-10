package mapview

import (
	"math"
	"sort"
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

type FloorLoopSet struct {
	Rings [][]WorldPt
	BBox  WorldBBox
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
		if len(set.Rings) == 0 {
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

		screenRings := make([][]ScreenPt, 0, len(set.Rings))
		minSX := math.Inf(1)
		minSY := math.Inf(1)
		maxSX := math.Inf(-1)
		maxSY := math.Inf(-1)
		for _, ring := range set.Rings {
			sring := make([]ScreenPt, 0, len(ring))
			for _, p := range ring {
				sx, sy := in.WorldToScreen(p.X, p.Y)
				sring = append(sring, ScreenPt{X: sx, Y: sy})
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
			}
			if len(sring) >= 3 {
				screenRings = append(screenRings, sring)
			}
		}
		if len(screenRings) == 0 || !isFinite(minSX) || !isFinite(minSY) || !isFinite(maxSX) || !isFinite(maxSY) {
			continue
		}

		x0 := max(0, int(math.Floor(minSX)))
		y0 := max(0, int(math.Floor(minSY)))
		x1 := min(w-1, int(math.Ceil(maxSX)))
		y1 := min(h-1, int(math.Ceil(maxSY)))
		if x0 > x1 || y0 > y1 {
			continue
		}

		xHits := make([]float64, 0, 64)
		for py := y0; py <= y1; py++ {
			xHits = xHits[:0]
			row := py * w * 4
			fy := float64(py) + 0.5
			for _, ring := range screenRings {
				for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
					a := ring[j]
					b := ring[i]
					if (a.Y > fy) == (b.Y > fy) {
						continue
					}
					x := a.X + (fy-a.Y)*(b.X-a.X)/(b.Y-a.Y)
					xHits = append(xHits, x)
				}
			}
			if len(xHits) < 2 {
				continue
			}
			sort.Float64s(xHits)
			rowWX0, rowWY0 := in.ScreenToWorld(0.5, fy)
			rowWX1, rowWY1 := in.ScreenToWorld(1.5, fy)
			stepWX := rowWX1 - rowWX0
			stepWY := rowWY1 - rowWY0
			for i := 0; i+1 < len(xHits); i += 2 {
				start := int(math.Ceil(xHits[i] - 0.5))
				end := int(math.Ceil(xHits[i+1]-0.5) - 1)
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
