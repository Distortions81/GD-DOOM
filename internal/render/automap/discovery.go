package automap

import "gddoom/internal/mapdata"

const automapDiscoverRadius = 1024.0

func (g *game) discoverLinesAroundPlayer() {
	px := float64(g.p.x) / fracUnit
	py := float64(g.p.y) / fracUnit
	for i, ld := range g.m.Linedefs {
		if shouldMarkMapped(ld, g.m.Vertexes, px, py, automapDiscoverRadius) {
			g.m.Linedefs[i].Flags |= mlMapped
		}
	}
}

func shouldMarkMapped(ld mapdata.Linedef, vertexes []mapdata.Vertex, px, py, radius float64) bool {
	if ld.Flags&lineNeverSee != 0 {
		return false
	}
	if int(ld.V1) >= len(vertexes) || int(ld.V2) >= len(vertexes) {
		return false
	}
	v1 := vertexes[ld.V1]
	v2 := vertexes[ld.V2]
	x1 := float64(v1.X)
	y1 := float64(v1.Y)
	x2 := float64(v2.X)
	y2 := float64(v2.Y)
	return pointSegmentDistanceSquared(px, py, x1, y1, x2, y2) <= radius*radius
}

func pointSegmentDistanceSquared(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	if dx == 0 && dy == 0 {
		return sq(px-x1) + sq(py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := x1 + t*dx
	cy := y1 + t*dy
	return sq(px-cx) + sq(py-cy)
}

func sq(v float64) float64 { return v * v }
