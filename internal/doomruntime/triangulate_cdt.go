package doomruntime

func triangulateWorldPolygonCDT(_ []worldPt) ([][3]int, bool) {
	return nil, false
}

func cdtTriangulationAvailable() bool {
	return false
}

func triangulateSectorLoopsCDT(_ sectorLoopSet) ([]worldTri, bool) {
	return nil, false
}

func cdtSegmentsValid(pts [][2]float64, segs [][2]int32) bool {
	if len(pts) < 3 || len(segs) < 3 {
		return false
	}
	worldPts := make([]worldPt, len(pts))
	for i, p := range pts {
		worldPts[i] = worldPt{x: p[0], y: p[1]}
	}
	for i, seg := range segs {
		a := int(seg[0])
		b := int(seg[1])
		if a < 0 || b < 0 || a >= len(worldPts) || b >= len(worldPts) || a == b {
			return false
		}
		a1 := worldPts[a]
		a2 := worldPts[b]
		if nearlyEqualWorldPt(a1, a2, 1e-9) {
			return false
		}
		for j := i + 1; j < len(segs); j++ {
			other := segs[j]
			c := int(other[0])
			d := int(other[1])
			if c < 0 || d < 0 || c >= len(worldPts) || d >= len(worldPts) || c == d {
				return false
			}
			if a == c || a == d || b == c || b == d {
				continue
			}
			if segmentsIntersectStrict(a1, a2, worldPts[c], worldPts[d]) {
				return false
			}
		}
	}
	return true
}
