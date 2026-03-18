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
	root := uint16(len(g.m.Nodes) - 1)
	if cap(g.bspOccBuf) < 64 {
		g.bspOccBuf = make([]solidSpan, 0, 64)
	}
	occ := g.bspOccBuf[:0]
	occ = g.traverseBSPSegs(root, px, py, ca, sa, near, focal, tanHalfFOV, occ)
	g.bspOccBuf = occ
	return g.visibleBuf
}

func (g *game) traverseBSPSegs(child uint16, px, py int64, ca, sa, near, focal, tanHalfFOV float64, occ []solidSpan) []solidSpan {
	if child&0x8000 != 0 {
		ss := int(child & 0x7fff)
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
	return scene.NodeChildBBoxMaybeVisible(n, childSide, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, tanHalfFOV)
}

func (g *game) nodeChildScreenRange(n mapdata.Node, childSide int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	return scene.NodeChildScreenRange(n, childSide, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, focal, g.viewW)
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
	front, back := g.segSectors(si)
	if front == nil {
		return false
	}
	if back == nil {
		return true
	}
	return back.CeilingHeight <= front.FloorHeight || back.FloorHeight >= front.CeilingHeight
}

func (g *game) segScreenRange(si int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	x1w, y1w, x2w, y2w, ok := g.segWorldEndpoints(si)
	if !ok || g.viewW <= 0 {
		return 0, 0, false
	}
	return scene.SegScreenRangeFromWorld(x1w, y1w, x2w, y2w, float64(px)/fracUnit, float64(py)/fracUnit, ca, sa, near, focal, g.viewW)
}

func (g *game) linedefDecisionPseudo3D(ld mapdata.Linedef) linepolicy.Decision {
	front, back := g.lineSectors(ld)
	st := linepolicy.Pseudo3DStateFromAutomap(g.automapRevealAll(), g.parity.iddt)
	return linepolicy.ParityDecision(ld, front, back, st, "doom")
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
	front := &g.m.Sectors[frontSectorIdx]
	back := &g.m.Sectors[backSectorIdx]
	return front.FloorHeight != back.FloorHeight ||
		front.CeilingHeight != back.CeilingHeight ||
		normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
		normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
		(front.Light != back.Light && doomSectorLighting)
}
