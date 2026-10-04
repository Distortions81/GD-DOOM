package doomruntime

import (
	"math"

	"gddoom/internal/render/mapview"
)

// The textured automap needs wall crossings, not closed sector polygons.
// Doom linedefs may overlap or meet halfway along another linedef, so tracing
// rings by vertex index can discard an entire room. Keep the original directed
// sides and let the scanline rasterizer determine the interior by winding.
func (g *game) ensureMapFloorBoundarySetsBuilt() {
	if g.mapFloorBoundaryInit {
		return
	}
	g.mapFloorBoundaryInit = true
	if g.m == nil {
		return
	}
	g.mapFloorBoundarySets = make([]mapview.FloorLoopSet, len(g.m.Sectors))
	for i := range g.mapFloorBoundarySets {
		g.mapFloorBoundarySets[i].BBox = mapview.WorldBBox{
			MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1),
		}
	}
	for _, ld := range g.m.Linedefs {
		if int(ld.V1) >= len(g.m.Vertexes) || int(ld.V2) >= len(g.m.Vertexes) {
			continue
		}
		v1, v2 := g.m.Vertexes[ld.V1], g.m.Vertexes[ld.V2]
		a := mapview.WorldPt{X: float64(v1.X), Y: float64(v1.Y)}
		b := mapview.WorldPt{X: float64(v2.X), Y: float64(v2.Y)}
		for side, sn := range ld.SideNum {
			if sn < 0 || int(sn) >= len(g.m.Sidedefs) {
				continue
			}
			sec := int(g.m.Sidedefs[sn].Sector)
			if sec >= len(g.mapFloorBoundarySets) {
				continue
			}
			set := &g.mapFloorBoundarySets[sec]
			e := mapview.WorldEdge{A: a, B: b}
			if side == 1 {
				e.A, e.B = e.B, e.A
			}
			set.Edges = append(set.Edges, e)
			set.BBox.MinX = math.Min(set.BBox.MinX, math.Min(a.X, b.X))
			set.BBox.MinY = math.Min(set.BBox.MinY, math.Min(a.Y, b.Y))
			set.BBox.MaxX = math.Max(set.BBox.MaxX, math.Max(a.X, b.X))
			set.BBox.MaxY = math.Max(set.BBox.MaxY, math.Max(a.Y, b.Y))
		}
	}
}

// mapFloorRasterInput is shared by both native and Ebiten map presentation.
// It uses original directed sector sides, including open or overlapping lines.
func (g *game) mapFloorRasterInput() mapview.FloorRasterInput {
	g.ensureSectorPlaneLevelCacheFresh()
	g.refreshSectorPlaneCacheTextureRefs()
	g.ensureMapFloorBoundarySetsBuilt()
	b := g.screenWorldBBox()
	return mapview.FloorRasterInput{ViewW: g.viewW, ViewH: g.viewH, ViewBBox: mapview.WorldBBox{MinX: b.minX, MinY: b.minY, MaxX: b.maxX, MaxY: b.maxY}, LoopSets: g.mapFloorBoundarySetsForView(), ShadeMuls: g.mapFloorShadeMuls(), Textures: g.mapFloorTextures(), FallbackRGB: [3]byte{wallFloorChange.R, wallFloorChange.G, wallFloorChange.B}, ScreenToWorld: g.screenToWorld, WorldToScreen: g.worldToScreen}
}
