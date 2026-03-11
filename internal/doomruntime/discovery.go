package doomruntime

import "gddoom/internal/mapdata"

const automapDiscoverPortalDepth = 3

func (g *game) discoverLinesAroundPlayer() {
	sec := g.sectorAt(g.p.x, g.p.y)
	mapped := discoverMappedLinesBySector(g.m, sec, automapDiscoverPortalDepth)
	changed := false
	for i, ok := range mapped {
		if ok {
			if g.m.Linedefs[i].Flags&mlMapped == 0 {
				changed = true
			}
			g.m.Linedefs[i].Flags |= mlMapped
		}
	}
	if changed {
		g.mapLines.Touch()
	}
}

func discoverMappedLinesBySector(m *mapdata.Map, startSector, maxDepth int) []bool {
	out := make([]bool, len(m.Linedefs))
	if startSector < 0 || startSector >= len(m.Sectors) {
		return out
	}
	type qNode struct {
		sec   int
		depth int
	}
	visited := make([]bool, len(m.Sectors))
	queue := []qNode{{sec: startSector, depth: 0}}
	visited[startSector] = true

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]

		for li, ld := range m.Linedefs {
			fsec, bsec, fok, bok := linedefSectorIndices(m, ld)
			touches := (fok && fsec == n.sec) || (bok && bsec == n.sec)
			if !touches {
				continue
			}
			if ld.Flags&lineNeverSee != 0 {
				continue
			}
			out[li] = true

			if n.depth >= maxDepth {
				continue
			}
			other, ok := oppositeSector(n.sec, fsec, bsec, fok, bok)
			if !ok || other < 0 || other >= len(m.Sectors) || visited[other] {
				continue
			}
			if !portalTraversable(m, ld, fsec, bsec, fok, bok) {
				continue
			}
			visited[other] = true
			queue = append(queue, qNode{sec: other, depth: n.depth + 1})
		}
	}

	return out
}

func linedefSectorIndices(m *mapdata.Map, ld mapdata.Linedef) (front, back int, frontOK, backOK bool) {
	if ld.SideNum[0] >= 0 && int(ld.SideNum[0]) < len(m.Sidedefs) {
		s := m.Sidedefs[int(ld.SideNum[0])].Sector
		if int(s) < len(m.Sectors) {
			front, frontOK = int(s), true
		}
	}
	if ld.SideNum[1] >= 0 && int(ld.SideNum[1]) < len(m.Sidedefs) {
		s := m.Sidedefs[int(ld.SideNum[1])].Sector
		if int(s) < len(m.Sectors) {
			back, backOK = int(s), true
		}
	}
	return front, back, frontOK, backOK
}

func oppositeSector(curr, front, back int, frontOK, backOK bool) (int, bool) {
	if frontOK && front == curr && backOK {
		return back, true
	}
	if backOK && back == curr && frontOK {
		return front, true
	}
	return 0, false
}

func portalTraversable(m *mapdata.Map, ld mapdata.Linedef, front, back int, frontOK, backOK bool) bool {
	if !frontOK || !backOK {
		return false
	}
	if ld.SideNum[1] < 0 {
		return false
	}
	if ld.Flags&mlBlocking != 0 {
		return false
	}
	f := m.Sectors[front]
	b := m.Sectors[back]
	openTop := minInt16(f.CeilingHeight, b.CeilingHeight)
	openBottom := maxInt16(f.FloorHeight, b.FloorHeight)
	return openTop > openBottom
}

func minInt16(a, b int16) int16 {
	if a < b {
		return a
	}
	return b
}

func maxInt16(a, b int16) int16 {
	if a > b {
		return a
	}
	return b
}
