//go:build !cgo

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
