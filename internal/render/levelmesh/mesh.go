// Package levelmesh builds and rasterizes experimental Doom level triangles.
// Coordinates and UVs are in Doom map units, with Z up and V down.
package levelmesh

import (
	"math"
	"strings"

	"gddoom/internal/mapdata"
)

type Point2 struct{ X, Y float64 }
type PlaneTriangle [3]Point2
type Vertex struct{ X, Y, Z, U, V float64 }
type Kind uint8

const (
	Floor Kind = iota
	Ceiling
	Middle
	Upper
	Lower
	Billboard
	EmissiveBillboard
	ShadowBillboard
)

type Triangle struct {
	Vertices        [3]Vertex
	Sector, Sidedef int
	Kind            Kind
	Texture         string
	Masked, Sky     bool
}

type Heights struct{ Floor, Ceiling float64 }

// Build reuses dst's capacity. The caller supplies cached triangulated planes
// and current render heights. Wall quads use the
// original linedefs so BSP splitting cannot introduce cracks or UV seams.
// height returns the current wall texture height, including animated frames.
func Build(dst []Triangle, m *mapdata.Map, planes [][]PlaneTriangle, heights []Heights, height func(string, int, Kind) float64, scroll func(uint16) float64) []Triangle {
	dst = dst[:0]
	for sector, tris := range planes {
		if sector >= len(m.Sectors) || sector >= len(heights) {
			continue
		}
		s, h := m.Sectors[sector], heights[sector]
		if h.Ceiling <= h.Floor {
			continue
		}
		for _, tri := range tris {
			area := (tri[1].X-tri[0].X)*(tri[2].Y-tri[0].Y) - (tri[1].Y-tri[0].Y)*(tri[2].X-tri[0].X)
			if math.Abs(area) < 1e-8 {
				continue
			}
			if area < 0 {
				tri[1], tri[2] = tri[2], tri[1]
			}
			for _, kind := range []Kind{Floor, Ceiling} {
				z, tex := h.Floor, s.FloorPic
				if kind == Ceiling {
					z, tex = h.Ceiling, s.CeilingPic
				}
				t := Triangle{Sector: sector, Sidedef: -1, Kind: kind, Texture: tex, Sky: sky(tex)}
				for i, p := range tri {
					t.Vertices[i] = Vertex{p.X, p.Y, z, p.X, p.Y}
				}
				if kind == Ceiling {
					t.Vertices[1], t.Vertices[2] = t.Vertices[2], t.Vertices[1]
				}
				dst = append(dst, t)
			}
		}
	}
	for _, line := range m.Linedefs {
		if int(line.V1) >= len(m.Vertexes) || int(line.V2) >= len(m.Vertexes) {
			continue
		}
		for side, idx := range line.SideNum {
			if idx < 0 || int(idx) >= len(m.Sidedefs) {
				continue
			}
			sd := m.Sidedefs[idx]
			sector := int(sd.Sector)
			if sector >= len(heights) {
				continue
			}
			front := heights[sector]
			a, b := m.Vertexes[line.V1], m.Vertexes[line.V2]
			if side == 1 {
				a, b = b, a
			}
			length := math.Hypot(float64(b.X)-float64(a.X), float64(b.Y)-float64(a.Y))
			if length == 0 {
				continue
			}
			u := float64(sd.TextureOffset)
			if scroll != nil {
				u += scroll(line.Special)
			}
			texHeight := func(tex string, kind Kind) float64 {
				if height != nil {
					if h := height(tex, int(idx), kind); h > 0 {
						return h
					}
				}
				return 128
			}
			quad := func(kind Kind, tex string, bottom, top, anchor float64, masked bool) {
				if top <= bottom {
					return
				}
				anchor += float64(sd.RowOffset)
				// Masked mids use the whole portal envelope. The project's
				// column renderer wraps V before alpha testing; truncating the
				// mesh at one texture height leaves gaps (e.g. E1M3 BRNSMALC).
				v := [4]Vertex{
					{float64(a.X), float64(a.Y), bottom, u, anchor - bottom},
					{float64(b.X), float64(b.Y), bottom, u + length, anchor - bottom},
					{float64(b.X), float64(b.Y), top, u + length, anchor - top},
					{float64(a.X), float64(a.Y), top, u, anchor - top},
				}
				for _, ids := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					dst = append(dst, Triangle{Vertices: [3]Vertex{v[ids[0]], v[ids[1]], v[ids[2]]}, Sector: sector, Sidedef: int(idx), Kind: kind, Texture: tex, Masked: masked})
				}
			}
			other := line.SideNum[1-side]
			backExists := line.Flags&4 != 0 && other >= 0 && int(other) < len(m.Sidedefs) && int(m.Sidedefs[other].Sector) < len(heights)
			if !backExists {
				anchor := front.Ceiling
				if line.Flags&16 != 0 {
					anchor = front.Floor + texHeight(sd.Mid, Middle)
				}
				quad(Middle, sd.Mid, front.Floor, front.Ceiling, anchor, false)
				continue
			}
			backSector := int(m.Sidedefs[other].Sector)
			back := heights[backSector]
			bothSky := sky(m.Sectors[sector].CeilingPic) && sky(m.Sectors[backSector].CeilingPic)
			if back.Ceiling < front.Ceiling && !bothSky {
				anchor := back.Ceiling + texHeight(sd.Top, Upper)
				if line.Flags&8 != 0 {
					anchor = front.Ceiling
				}
				quad(Upper, sd.Top, math.Max(front.Floor, back.Ceiling), front.Ceiling, anchor, false)
			}
			if back.Floor > front.Floor {
				anchor := back.Floor
				if line.Flags&16 != 0 {
					anchor = front.Ceiling
				}
				quad(Lower, sd.Bottom, front.Floor, math.Min(front.Ceiling, back.Floor), anchor, false)
			}
			if sd.Mid != "" && sd.Mid != "-" {
				anchor := math.Min(front.Ceiling, back.Ceiling)
				if line.Flags&16 != 0 {
					anchor = math.Max(front.Floor, back.Floor) + texHeight(sd.Mid, Middle)
				}
				quad(Middle, sd.Mid, math.Max(front.Floor, back.Floor), math.Min(front.Ceiling, back.Ceiling), anchor, true)
			}
		}
	}
	return dst
}

func sky(name string) bool { return strings.EqualFold(name, "F_SKY1") }
