package doomruntime

// clipSubSectorPolyByLinedefBounds bounds a BSP cell by its oriented walls.
// Vanilla SEGs omit implicit BSP edges and can have rounded split vertices, so
// their endpoints are neither a closed polygon nor exact wall half-planes.
func (g *game) clipSubSectorPolyByLinedefBounds(ss int, poly []worldPt) []worldPt {
	if g == nil || g.m == nil || ss < 0 || ss >= len(g.m.SubSectors) || len(poly) < 3 {
		return nil
	}
	sub := g.m.SubSectors[ss]
	if sub.SegCount == 0 {
		return nil
	}
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		if si >= len(g.m.Segs) {
			return nil
		}
		seg := g.m.Segs[si]
		if int(seg.Linedef) >= len(g.m.Linedefs) || seg.Direction > 1 {
			return nil
		}
		ld := g.m.Linedefs[seg.Linedef]
		if int(ld.V1) >= len(g.m.Vertexes) || int(ld.V2) >= len(g.m.Vertexes) {
			return nil
		}
		a, b := g.m.Vertexes[ld.V1], g.m.Vertexes[ld.V2]
		if a == b {
			return nil
		}
		if seg.Direction == 1 {
			a, b = b, a
		}
		// The SEG's front sector is on the right of the directed linedef.
		poly = clipWorldPolyByDivline(poly,
			worldPt{x: float64(a.X), y: float64(a.Y)},
			worldPt{x: float64(b.X), y: float64(b.Y)}, 0)
		if len(poly) < 3 {
			return nil
		}
	}
	return poly
}
