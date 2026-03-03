package automap

import "gddoom/internal/mapdata"

func (g *game) visibleLineIndicesPseudo3D() []int {
	if len(g.m.Nodes) == 0 {
		return g.visibleLineIndices()
	}
	g.visibleBuf = g.visibleBuf[:0]
	g.renderEpoch++
	if g.renderEpoch == 0 {
		for i := range g.renderSeen {
			g.renderSeen[i] = 0
		}
		g.renderEpoch = 1
	}

	root := uint16(len(g.m.Nodes) - 1)
	g.traverseBSPNode(root, g.p.x, g.p.y)
	return g.visibleBuf
}

func (g *game) traverseBSPNode(child uint16, px, py int64) {
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
			li := int(g.m.Segs[si].Linedef)
			if li < 0 || li >= len(g.m.Linedefs) {
				continue
			}
			if g.renderSeen[li] == g.renderEpoch {
				continue
			}
			g.renderSeen[li] = g.renderEpoch
			g.visibleBuf = append(g.visibleBuf, li)
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
	g.traverseBSPNode(front, px, py)
	g.traverseBSPNode(back, px, py)
}

func (g *game) linedefDecisionPseudo3D(ld mapdata.Linedef) lineDecision {
	front, back := g.lineSectors(ld)
	st := g.parity
	// Pseudo-3D should not depend on automap exploration/mapped status.
	st.reveal = revealAllMap
	return parityLineDecision(ld, front, back, st, "doom")
}
