package automap

import (
	"math"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/mapview/linepolicy"
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
			occ = addSolidSpanInPlace(occ, l, r)
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
	bb := n.BBoxR
	if childSide != 0 {
		bb = n.BBoxL
	}
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	minX := math.Min(left, right)
	maxX := math.Max(left, right)
	minY := math.Min(bottom, top)
	maxY := math.Max(bottom, top)
	if minX > maxX || minY > maxY {
		return true
	}

	pxw := float64(px) / fracUnit
	pyw := float64(py) / fracUnit
	if pxw >= minX && pxw <= maxX && pyw >= minY && pyw <= maxY {
		return true
	}

	// Test bbox corners against the camera frustum half-planes in camera space.
	// Reject only when all corners are outside the same plane.
	corners := [4][2]float64{
		{minX, minY},
		{maxX, minY},
		{maxX, maxY},
		{minX, maxY},
	}
	outNear := 0
	outLeft := 0
	outRight := 0
	for _, c := range corners {
		dx := c[0] - pxw
		dy := c[1] - pyw
		f := dx*ca + dy*sa
		s := -dx*sa + dy*ca
		if f < near {
			outNear++
		}
		if s+tanHalfFOV*f < 0 {
			outLeft++
		}
		if -s+tanHalfFOV*f < 0 {
			outRight++
		}
	}
	if outNear == len(corners) || outLeft == len(corners) || outRight == len(corners) {
		return false
	}
	return true
}

func (g *game) nodeChildScreenRange(n mapdata.Node, childSide int, px, py int64, ca, sa, near, focal float64) (int, int, bool) {
	bb := n.BBoxR
	if childSide != 0 {
		bb = n.BBoxL
	}
	top := float64(bb[0])
	bottom := float64(bb[1])
	left := float64(bb[2])
	right := float64(bb[3])
	minX := math.Min(left, right)
	maxX := math.Max(left, right)
	minY := math.Min(bottom, top)
	maxY := math.Max(bottom, top)
	if minX > maxX || minY > maxY || g.viewW <= 0 {
		return 0, 0, false
	}
	pxw := float64(px) / fracUnit
	pyw := float64(py) / fracUnit
	if pxw >= minX && pxw <= maxX && pyw >= minY && pyw <= maxY {
		return 0, g.viewW - 1, true
	}
	corners := [4][2]float64{
		{minX, minY},
		{maxX, minY},
		{maxX, maxY},
		{minX, maxY},
	}
	minSX := float64(g.viewW - 1)
	maxSX := 0.0
	any := false
	for _, c := range corners {
		dx := c[0] - pxw
		dy := c[1] - pyw
		f := dx*ca + dy*sa
		s := -dx*sa + dy*ca
		if f < near {
			f = near
		}
		if f <= 0 {
			continue
		}
		sx := float64(g.viewW)/2 - (s/f)*focal
		if sx < minSX {
			minSX = sx
		}
		if sx > maxSX {
			maxSX = sx
		}
		any = true
	}
	if !any {
		return 0, 0, false
	}
	// Conservative screen bounds for BSP child culling.
	// BBox-corner projection can slightly underestimate the true covered span
	// while turning near partition planes; pad a little to avoid false culls.
	const childCullPad = 1
	l := int(math.Floor(minSX)) - childCullPad
	r := int(math.Ceil(maxSX)) + childCullPad
	if l < 0 {
		l = 0
	}
	if r >= g.viewW {
		r = g.viewW - 1
	}
	if l > r {
		return 0, 0, false
	}
	return l, r, true
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
	pxw := float64(px) / fracUnit
	pyw := float64(py) / fracUnit
	x1 := x1w - pxw
	y1 := y1w - pyw
	x2 := x2w - pxw
	y2 := y2w - pyw
	f1 := x1*ca + y1*sa
	s1 := -x1*sa + y1*ca
	f2 := x2*ca + y2*sa
	s2 := -x2*sa + y2*ca
	f1, s1, f2, s2, ok = clipSegmentToNear(f1, s1, f2, s2, near)
	if !ok {
		return 0, 0, false
	}
	if f1*s2-s1*f2 >= 0 {
		return 0, 0, false
	}
	sx1 := float64(g.viewW)/2 - (s1/f1)*focal
	sx2 := float64(g.viewW)/2 - (s2/f2)*focal
	if !isFinite(sx1) || !isFinite(sx2) {
		return 0, 0, false
	}
	l := int(math.Floor(math.Min(sx1, sx2)))
	r := int(math.Ceil(math.Max(sx1, sx2)))
	if l < 0 {
		l = 0
	}
	if r >= g.viewW {
		r = g.viewW - 1
	}
	if l > r {
		return 0, 0, false
	}
	return l, r, true
}

func addSolidSpanInPlace(spans []solidSpan, l, r int) []solidSpan {
	if l > r {
		return spans
	}
	n := len(spans)
	if n == 0 {
		return append(spans, solidSpan{l: l, r: r})
	}
	i := 0
	for i < n && spans[i].r+1 < l {
		i++
	}
	if i == n {
		return append(spans, solidSpan{l: l, r: r})
	}
	if r+1 < spans[i].l {
		spans = append(spans, solidSpan{})
		copy(spans[i+1:], spans[i:n])
		spans[i] = solidSpan{l: l, r: r}
		return spans
	}
	if spans[i].l < l {
		l = spans[i].l
	}
	if spans[i].r > r {
		r = spans[i].r
	}
	j := i + 1
	for j < n && spans[j].l-1 <= r {
		if spans[j].r > r {
			r = spans[j].r
		}
		j++
	}
	spans[i] = solidSpan{l: l, r: r}
	if j > i+1 {
		copy(spans[i+1:], spans[j:n])
		spans = spans[:n-(j-(i+1))]
	}
	return spans
}

func (g *game) linedefDecisionPseudo3D(ld mapdata.Linedef) linepolicy.Decision {
	front, back := g.lineSectors(ld)
	st := linepolicy.Pseudo3DStateFromAutomap(g.parity.reveal == revealAllMap, g.parity.iddt)
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
