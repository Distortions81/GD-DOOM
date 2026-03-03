package mapdata

import "fmt"

func Validate(m *Map) error {
	for i, ld := range m.Linedefs {
		if int(ld.V1) >= len(m.Vertexes) || int(ld.V2) >= len(m.Vertexes) {
			return fmt.Errorf("linedef[%d] has out-of-range vertex index (%d,%d)", i, ld.V1, ld.V2)
		}
		for si, side := range ld.SideNum {
			if side < -1 {
				return fmt.Errorf("linedef[%d] side %d has invalid negative index %d", i, si, side)
			}
			if side >= 0 && int(side) >= len(m.Sidedefs) {
				return fmt.Errorf("linedef[%d] side %d out of range: %d", i, si, side)
			}
		}
	}

	for i, sd := range m.Sidedefs {
		if int(sd.Sector) >= len(m.Sectors) {
			return fmt.Errorf("sidedef[%d] has out-of-range sector index %d", i, sd.Sector)
		}
	}

	for i, seg := range m.Segs {
		if int(seg.StartVertex) >= len(m.Vertexes) || int(seg.EndVertex) >= len(m.Vertexes) {
			return fmt.Errorf("seg[%d] has out-of-range vertex index (%d,%d)", i, seg.StartVertex, seg.EndVertex)
		}
		if int(seg.Linedef) >= len(m.Linedefs) {
			return fmt.Errorf("seg[%d] has out-of-range linedef index %d", i, seg.Linedef)
		}
	}

	for i, ss := range m.SubSectors {
		start := int(ss.FirstSeg)
		end := start + int(ss.SegCount)
		if start > len(m.Segs) || end > len(m.Segs) || end < start {
			return fmt.Errorf("subsector[%d] has invalid seg range [%d,%d)", i, start, end)
		}
	}

	for i, n := range m.Nodes {
		for ci, child := range n.ChildID {
			if child&0x8000 != 0 {
				ss := int(child & 0x7fff)
				if ss >= len(m.SubSectors) {
					return fmt.Errorf("node[%d] child %d has out-of-range subsector index %d", i, ci, ss)
				}
				continue
			}
			ni := int(child)
			if ni >= len(m.Nodes) {
				return fmt.Errorf("node[%d] child %d has out-of-range node index %d", i, ci, ni)
			}
		}
	}

	return nil
}
