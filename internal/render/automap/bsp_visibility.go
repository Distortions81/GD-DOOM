package automap

import (
	"math"

	"gddoom/internal/mapdata"
)

func (g *game) visibleSegIndicesPseudo3D() []int {
	if len(g.m.Nodes) == 0 {
		g.visibleBuf = g.visibleBuf[:0]
		for i := range g.m.Segs {
			g.visibleBuf = append(g.visibleBuf, i)
		}
		return g.visibleBuf
	}
	g.visibleBuf = g.visibleBuf[:0]

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
	g.traverseBSPSegs(root, px, py, ca, sa, near, tanHalfFOV)
	return g.visibleBuf
}

func (g *game) traverseBSPSegs(child uint16, px, py int64, ca, sa, near, tanHalfFOV float64) {
	if child&0x8000 != 0 {
		ss := int(child & 0x7fff)
		if ss < 0 || ss >= len(g.m.SubSectors) {
			return
		}
		sub := g.m.SubSectors[ss]
		for i := 0; i < int(sub.SegCount); i++ {
			si := int(sub.FirstSeg) + i
			if si < 0 || si >= len(g.m.Segs) {
				continue
			}
			g.visibleBuf = append(g.visibleBuf, si)
		}
		return
	}
	ni := int(child)
	if ni < 0 || ni >= len(g.m.Nodes) {
		return
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
	g.traverseBSPSegs(front, px, py, ca, sa, near, tanHalfFOV)
	if g.nodeChildBBoxMaybeVisible(n, side^1, px, py, ca, sa, near, tanHalfFOV) {
		g.traverseBSPSegs(back, px, py, ca, sa, near, tanHalfFOV)
	}
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

func (g *game) linedefDecisionPseudo3D(ld mapdata.Linedef) lineDecision {
	front, back := g.lineSectors(ld)
	st := g.parity
	// Pseudo-3D should not depend on automap exploration/mapped status.
	st.reveal = revealAllMap
	return parityLineDecision(ld, front, back, st, "doom")
}
