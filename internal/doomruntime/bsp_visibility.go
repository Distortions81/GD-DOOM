package doomruntime

import (
	"math"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview/linepolicy"
	"gddoom/internal/render/scene"
)

func (g *game) visibleSegIndicesPseudo3D() []int {
	if len(g.m.Nodes) == 0 {
		g.visibleBuf = g.visibleBuf[:0]
		for i := range g.m.Segs {
			g.visibleBuf = append(g.visibleBuf, i)
		}
		if len(g.visibleSectorSeen) > 0 {
			g.visibleEpoch++
			if g.visibleEpoch == 0 {
				g.visibleEpoch = 1
			}
			for i := range g.visibleSectorSeen {
				g.visibleSectorSeen[i] = g.visibleEpoch
			}
		}
		return g.visibleBuf
	}
	g.visibleBuf = g.visibleBuf[:0]
	g.visibleEpoch++
	if g.visibleEpoch == 0 {
		g.visibleEpoch = 1
	}

	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	near := 2.0
	focal := doomFocalLength(g.viewW)
	tanHalfFOV := 1.0
	if focal > 0 {
		tanHalfFOV = (float64(g.viewW) * 0.5) / focal
	}
	px := floatToFixed(g.renderPX)
	py := floatToFixed(g.renderPY)
	root := uint32(len(g.m.Nodes) - 1)
	if cap(g.bspOccBuf) < 64 {
		g.bspOccBuf = make([]solidSpan, 0, 64)
	}
	occ := g.bspOccBuf[:0]
	occ = g.traverseBSPSegs(root, px, py, ca, sa, near, focal, tanHalfFOV, occ)
	g.bspOccBuf = occ
	return g.visibleBuf
}

func (g *game) traverseBSPSegs(child uint32, px, py int64, ca, sa, near, focal, tanHalfFOV float64, occ []solidSpan) []solidSpan {
	if child&mapdata.SubsectorFlag != 0 {
		ss := int(child & ^mapdata.SubsectorFlag)
		if ss < 0 || ss >= len(g.m.SubSectors) {
			return occ
		}
		if ss >= 0 && ss < len(g.visibleSubSectorSeen) && g.visibleSubSectorSeen[ss] == g.visibleEpoch {
			return occ
		}
		if ss >= 0 && ss < len(g.visibleSubSectorSeen) {
			g.visibleSubSectorSeen[ss] = g.visibleEpoch
		}
		if sec := g.sectorForSubSector(ss); sec >= 0 && sec < len(g.visibleSectorSeen) {
			g.visibleSectorSeen[sec] = g.visibleEpoch
		}
		sub := g.m.SubSectors[ss]
		for i := 0; i < int(sub.SegCount); i++ {
			si := int(sub.FirstSeg) + i
			if si < 0 || si >= len(g.m.Segs) {
				continue
			}
			sg := g.m.Segs[si]
			li := int(sg.Linedef)
			if li < 0 || li >= len(g.m.Linedefs) {
				continue
			}
			if !g.linedefDecisionPseudo3D(g.m.Linedefs[li]).Visible &&
				!g.segHasTwoSidedMidTexture(si) &&
				!g.segPortalSplitPseudo3D(si) {
				continue
			}
			g.visibleBuf = append(g.visibleBuf, si)
			if !g.segCoarseOpaque(si) {
				continue
			}
			l, r, ok := g.segScreenRange(si, px, py, ca, sa, near, focal)
			if !ok {
				continue
			}
			occ = scene.AddSpanInPlace(occ, l, r)
		}
		return occ
	}
	ni := int(child)
	if ni < 0 || ni >= len(g.m.Nodes) {
		return occ
	}
	n := g.m.Nodes[ni]
	dl := divline{
		x:  int64(n.X) << fracBits,
		y:  int64(n.Y) << fracBits,
		dx: int64(n.DX) << fracBits,
		dy: int64(n.DY) << fracBits,
	}
	side := pointOnDivlineSide(px, py, dl)
	front := n.ChildID[side]
	back := n.ChildID[side^1]
	// Doom visibility order: traverse nearer BSP side first.
	occ = g.traverseBSPSegs(front, px, py, ca, sa, near, focal, tanHalfFOV, occ)
	if g.nodeChildBBoxMaybeVisible(n, side^1, px, py, ca, sa, near, tanHalfFOV) {
		if l, r, ok := g.nodeChildScreenRangeCached(ni, n, side^1, px, py, ca, sa, near, focal); ok && solidFullyCovered(occ, l, r) {
			return occ
		}
		occ = g.traverseBSPSegs(back, px, py, ca, sa, near, focal, tanHalfFOV, occ)
	}
	return occ
}

func (g *game) nodeChildBBoxMaybeVisible(n mapdata.Node, childSide int, px, py int64, ca, sa, near, tanHalfFOV float64) bool {
	if !g.opts.SourcePortMode {
		_, _, ok := g.classicBBoxRange(n, childSide, px, py, ca, sa)
		return ok
	}
	return scene.NodeChildBBoxMaybeVisible(n, childSide, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, tanHalfFOV)
}

func (g *game) nodeChildScreenRange(n mapdata.Node, childSide int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	if !g.opts.SourcePortMode {
		return g.classicBBoxRange(n, childSide, px, py, ca, sa)
	}
	return scene.NodeChildScreenRange(n, childSide, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, focal, g.viewW)
}

// R_CheckBBox uses the same quantized angle table as walls. Float frustum
// tests can reject a box exactly on the left edge even when column zero draws.
func (g *game) classicBBoxRange(n mapdata.Node, side int, px, py int64, ca, sa float64) (int, int, bool) {
	bb := n.BBoxR
	if side != 0 {
		bb = n.BBoxL
	}
	x, y := float64(px)/fracUnit, float64(py)/fracUnit
	bx, by := 2, 2
	if x <= float64(bb[2]) {
		bx = 0
	} else if x < float64(bb[3]) {
		bx = 1
	}
	if y >= float64(bb[0]) {
		by = 0
	} else if y > float64(bb[1]) {
		by = 1
	}
	pos := by*4 + bx
	if pos == 5 {
		return 0, g.viewW - 1, true
	}
	coords := [12][4]int{{3, 0, 2, 1}, {3, 0, 2, 0}, {3, 1, 2, 0}, {}, {2, 0, 2, 1}, {}, {3, 1, 3, 0}, {}, {2, 0, 3, 1}, {2, 1, 3, 1}, {2, 1, 3, 0}, {}}
	c := coords[pos]
	in := scene.WallPrepassWorldInput{X1W: float64(bb[c[0]]), Y1W: float64(bb[c[1]]), X2W: float64(bb[c[2]]), Y2W: float64(bb[c[3]])}
	a := doomPointToAngle2(px, py, floorFixed(in.X1W), floorFixed(in.Y1W))
	b := doomPointToAngle2(px, py, floorFixed(in.X2W), floorFixed(in.Y2W))
	if a-b >= doomAng180 {
		return 0, g.viewW - 1, true
	}
	return classicProjectionForWidth(g.viewW).wallBounds(in, x, y, radiansToDoomAngle(math.Atan2(sa, ca)))
}

func (g *game) nodeChildScreenRangeCached(nodeIdx int, n mapdata.Node, childSide int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	slot := nodeIdx*2 + childSide
	if slot >= 0 && slot < len(g.nodeChildRangeEpoch) && g.nodeChildRangeEpoch[slot] == g.visibleEpoch {
		return g.nodeChildRangeL[slot], g.nodeChildRangeR[slot], g.nodeChildRangeOK[slot] != 0
	}
	l, r, ok := g.nodeChildScreenRange(n, childSide, px, py, ca, sa, near, focal)
	if slot >= 0 && slot < len(g.nodeChildRangeEpoch) {
		g.nodeChildRangeEpoch[slot] = g.visibleEpoch
		g.nodeChildRangeL[slot] = l
		g.nodeChildRangeR[slot] = r
		if ok {
			g.nodeChildRangeOK[slot] = 1
		} else {
			g.nodeChildRangeOK[slot] = 0
		}
	}
	return l, r, ok
}

func (g *game) segCoarseOpaque(si int) bool {
	frontIdx, backIdx := g.wallRenderSectorIndices(si)
	if frontIdx < 0 || frontIdx >= len(g.m.Sectors) {
		return false
	}
	front := g.renderSector(frontIdx, false)
	var back *mapdata.Sector
	if backIdx >= 0 && backIdx < len(g.m.Sectors) {
		back = g.renderSector(backIdx, true)
	}
	if front == nil {
		return false
	}
	if back == nil {
		return true
	}
	if g.authorityRender != nil || g.clientPrediction != nil || g.boomHeightFrameValid {
		frontFloor, frontCeil, frontOK := g.sectorHeightRenderSnapshot(frontIdx)
		backFloor, backCeil, backOK := g.sectorBackRenderHeights(backIdx)
		if frontOK && backOK {
			// The most recent collision state can already be closed while the
			// buffered door is still visibly open. Cull on the rendered opening.
			return backCeil <= frontFloor || backFloor >= frontCeil
		}
	}
	return back.CeilingHeight <= front.FloorHeight || back.FloorHeight >= front.CeilingHeight
}

func (g *game) segScreenRange(si int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	x1w, y1w, x2w, y2w, ok := g.segWorldEndpoints(si)
	if !ok || g.viewW <= 0 {
		return 0, 0, false
	}
	if !g.opts.SourcePortMode {
		// Occlusion must cover exactly the columns the wall will draw. The
		// floating-point floor/ceil range can hide a neighboring column and
		// discard a subsector that classic still visits, changing plane spans.
		input := scene.WallPrepassWorldInput{X1W: x1w, Y1W: y1w, X2W: x2w, Y2W: y2w}
		return classicProjectionForWidth(g.viewW).wallBounds(input, float64(px)/fracUnit, float64(py)/fracUnit, radiansToDoomAngle(math.Atan2(sa, ca)))
	}
	return scene.SegScreenRangeFromWorld(x1w, y1w, x2w, y2w, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, focal, g.viewW)
}

func (g *game) linedefDecisionPseudo3D(ld mapdata.Linedef) linepolicy.Decision {
	front, back := g.lineSectors(ld)
	st := linepolicy.Pseudo3DStateFromAutomap(g.automapRevealAll(), g.parity.iddt)
	return linepolicy.ParityDecision(ld, front, back, st)
}

func (g *game) segPortalSplitPseudo3D(segIdx int) bool {
	if segIdx < 0 || segIdx >= len(g.m.Segs) {
		return false
	}
	sg := g.m.Segs[segIdx]
	li := int(sg.Linedef)
	if li < 0 || li >= len(g.m.Linedefs) {
		return false
	}
	ld := g.m.Linedefs[li]
	frontSide := int(sg.Direction)
	if frontSide < 0 || frontSide > 1 {
		frontSide = 0
	}
	backSide := frontSide ^ 1
	if ld.SideNum[frontSide] < 0 || ld.SideNum[backSide] < 0 {
		return false
	}
	frontSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[frontSide])
	backSectorIdx := g.sectorIndexFromSideNum(ld.SideNum[backSide])
	if frontSectorIdx < 0 || backSectorIdx < 0 ||
		frontSectorIdx >= len(g.m.Sectors) || backSectorIdx >= len(g.m.Sectors) {
		return false
	}
	front := g.renderSector(frontSectorIdx, false)
	back := g.renderSector(backSectorIdx, true)
	heightSplit := front.FloorHeight != back.FloorHeight || front.CeilingHeight != back.CeilingHeight
	if g.authorityRender != nil || g.clientPrediction != nil || g.boomHeightFrameValid {
		frontFloor, frontCeil, frontOK := g.sectorHeightRenderSnapshot(frontSectorIdx)
		backFloor, backCeil, backOK := g.sectorBackRenderHeights(backSectorIdx)
		if frontOK && backOK {
			heightSplit = frontFloor != backFloor || frontCeil != backCeil
		}
	}
	return heightSplit ||
		normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
		normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
		g.sectorsLightDifferForRender(frontSectorIdx, backSectorIdx, front, back)
}
