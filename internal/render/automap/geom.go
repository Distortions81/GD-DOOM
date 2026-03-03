package automap

import "gddoom/internal/mapdata"

type bounds struct {
	minX float64
	minY float64
	maxX float64
	maxY float64
}

func mapBounds(m *mapdata.Map) bounds {
	if len(m.Vertexes) == 0 {
		return bounds{}
	}
	b := bounds{
		minX: float64(m.Vertexes[0].X),
		maxX: float64(m.Vertexes[0].X),
		minY: float64(m.Vertexes[0].Y),
		maxY: float64(m.Vertexes[0].Y),
	}
	for i := 1; i < len(m.Vertexes); i++ {
		vx := float64(m.Vertexes[i].X)
		vy := float64(m.Vertexes[i].Y)
		if vx < b.minX {
			b.minX = vx
		}
		if vx > b.maxX {
			b.maxX = vx
		}
		if vy < b.minY {
			b.minY = vy
		}
		if vy > b.maxY {
			b.maxY = vy
		}
	}
	return b
}
