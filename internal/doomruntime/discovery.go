package doomruntime

import "gddoom/internal/mapdata"

const automapDiscoverPortalDepth = 3

func (g *game) discoverLinesAroundPlayer() {
	sec := g.sectorAt(g.p.x, g.p.y)
	mapped := g.discoverMappedLinesBySectorScratch(sec, automapDiscoverPortalDepth)
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

func buildAutomapSectorLineAdj(m *mapdata.Map) [][]automapSectorLine {
	if m == nil || len(m.Sectors) == 0 {
		return nil
	}
	out := make([][]automapSectorLine, len(m.Sectors))
	for li, ld := range m.Linedefs {
		front, back, frontOK, backOK := linedefSectorIndices(m, ld)
		entry := automapSectorLine{
			line:    li,
			front:   front,
			back:    back,
			frontOK: frontOK,
			backOK:  backOK,
		}
		if frontOK {
			out[front] = append(out[front], entry)
		}
		if backOK && (!frontOK || back != front) {
			out[back] = append(out[back], entry)
		}
	}
	return out
}

func (g *game) discoverMappedLinesBySectorScratch(startSector, maxDepth int) []bool {
	if g == nil || g.m == nil {
		return nil
	}
	out := resizeSliceLen(g.automapMappedScratch, len(g.m.Linedefs))
	g.automapMappedScratch = out
	if startSector < 0 || startSector >= len(g.m.Sectors) {
		return out
	}
	visited := resizeSliceLen(g.automapVisitedScratch, len(g.m.Sectors))
	g.automapVisitedScratch = visited
	queue := reserveSliceCap(g.automapQueueScratch, max(len(g.m.Sectors), 1))
	queue = append(queue, automapQueueNode{sec: startSector, depth: 0})
	visited[startSector] = true
	for head := 0; head < len(queue); head++ {
		n := queue[head]
		for _, entry := range g.sectorLineAdj[n.sec] {
			ld := g.m.Linedefs[entry.line]
			if ld.Flags&lineNeverSee != 0 {
				continue
			}
			out[entry.line] = true
			if n.depth >= maxDepth {
				continue
			}
			other, ok := oppositeSector(n.sec, entry.front, entry.back, entry.frontOK, entry.backOK)
			if !ok || other < 0 || other >= len(g.m.Sectors) || visited[other] {
				continue
			}
			if !portalTraversable(g.m, ld, entry.front, entry.back, entry.frontOK, entry.backOK) {
				continue
			}
			visited[other] = true
			queue = append(queue, automapQueueNode{sec: other, depth: n.depth + 1})
		}
	}
	g.automapQueueScratch = queue[:0]
	return out
}

func discoverMappedLinesBySector(m *mapdata.Map, startSector, maxDepth int) []bool {
	out := make([]bool, len(m.Linedefs))
	if startSector < 0 || startSector >= len(m.Sectors) {
		return out
	}
	visited := make([]bool, len(m.Sectors))
	queue := []automapQueueNode{{sec: startSector, depth: 0}}
	adj := buildAutomapSectorLineAdj(m)
	visited[startSector] = true

	for head := 0; head < len(queue); head++ {
		n := queue[head]
		for _, entry := range adj[n.sec] {
			ld := m.Linedefs[entry.line]
			if ld.Flags&lineNeverSee != 0 {
				continue
			}
			out[entry.line] = true

			if n.depth >= maxDepth {
				continue
			}
			other, ok := oppositeSector(n.sec, entry.front, entry.back, entry.frontOK, entry.backOK)
			if !ok || other < 0 || other >= len(m.Sectors) || visited[other] {
				continue
			}
			if !portalTraversable(m, ld, entry.front, entry.back, entry.frontOK, entry.backOK) {
				continue
			}
			visited[other] = true
			queue = append(queue, automapQueueNode{sec: other, depth: n.depth + 1})
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
