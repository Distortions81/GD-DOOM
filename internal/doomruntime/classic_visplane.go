package doomruntime

// R_FindPlane returns the first plane with this material, height and light.
// R_CheckPlane may split that plane, but later subsectors start from the first
// one again. Searching every compatible split changes span starts and UVs.
func (g *game) findClassicPlane(key plane3DKey, order *[]*plane3DVisplane) *plane3DVisplane {
	identity := key
	identity.floor = false // Not part of classic's height/picnum/light identity.
	b := g.plane3DVisBuckets[identity]
	if b.gen != g.plane3DVisGen {
		b.gen = g.plane3DVisGen
		b.list = b.list[:0]
	}
	if len(b.list) != 0 {
		return b.list[0]
	}
	pl := g.acquirePlane3DVisplane(key, g.viewW, -1, g.viewW)
	b.list = append(b.list, pl)
	g.plane3DVisBuckets[identity] = b
	*order = append(*order, pl)
	return pl
}

// Retain the returned plane across wall fragments in the same subsector,
// exactly as R_StoreWallRange updates floorplane/ceilingplane.
func (g *game) checkClassicPlane(pl *plane3DVisplane, start, stop int, order *[]*plane3DVisplane) *plane3DVisplane {
	if pl == nil || start > stop {
		return pl
	}
	for x := max(start, pl.minX); x <= min(stop, pl.maxX); x++ {
		if pl.top[x+1] != plane3DUnset {
			split := g.acquirePlane3DVisplane(pl.key, start, stop, g.viewW)
			*order = append(*order, split)
			return split
		}
	}
	pl.minX = min(pl.minX, start)
	pl.maxX = max(pl.maxX, stop)
	return pl
}

func (g *game) segSubsectorIndex(si int) int {
	if si >= 0 && si < len(g.wallSegStaticCache) && g.wallSegStaticCache[si].valid {
		return g.wallSegStaticCache[si].subSectorIdx
	}
	for i, ss := range g.m.SubSectors {
		if si >= int(ss.FirstSeg) && si < int(ss.FirstSeg)+int(ss.SegCount) {
			return i
		}
	}
	// Synthetic maps without SSECTORS treat each segment as its own subsector.
	return -si - 1
}

// R_Subsector selects frontsector from its first SEG, including subsectors
// whose later SEG sidedefs name a different sector (e.g. Doom II MAP21).
func (g *game) wallRenderSectorIndices(si int) (int, int) {
	front, back := g.segSectorIndices(si)
	if !g.opts.SourcePortMode {
		if sector, ok := g.subSectorSectorIndex(g.segSubsectorIndex(si)); ok {
			front = sector
		}
	}
	return front, back
}
