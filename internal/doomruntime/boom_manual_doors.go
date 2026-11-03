package doomruntime

// Most sectors infer ownership from their single mover. A Boom D1 door can
// overwrite ceilingdata while the previous thinker stays in the list. For
// those sectors, retain the actual pointer identity (including a null owner).
func (g *game) trackCeilingSlot(sec int) {
	if _, ok := g.ceilingOwners[sec]; ok {
		return
	}
	if g.ceilingOwners == nil {
		g.ceilingOwners = make(map[int]int64)
	}
	owner := int64(0)
	if d := g.doors[sec]; d != nil && !d.pendingRemove {
		owner = d.order
	} else if c := g.ceilings[sec]; c != nil {
		owner = c.order
	} else if f := g.floors[sec]; f != nil && f.elevator {
		owner = f.order
	}
	g.ceilingOwners[sec] = owner
}
func (g *game) assignTrackedCeilingSlot(sec int, order int64) {
	if _, ok := g.ceilingOwners[sec]; ok {
		g.ceilingOwners[sec] = order
	}
}
func (g *game) clearTrackedCeilingSlot(sec int) {
	if g == nil {
		return
	}
	if _, ok := g.ceilingOwners[sec]; ok {
		g.ceilingOwners[sec] = 0
	}
}
func (g *game) installDoorThinker(sec int, d *doorThinker) {
	if old := g.doors[sec]; g.boomMomentumDropoff && old != nil && !old.pendingRemove && old != d {
		g.extraDoors = append(g.extraDoors, old)
	}
	if g.doors == nil {
		g.doors = make(map[int]*doorThinker)
	}
	g.doors[sec] = d
	g.assignTrackedCeilingSlot(sec, d.order)
}
func (g *game) installCeilingThinker(sec int, c *ceilingThinker) {
	if old := g.ceilings[sec]; old != nil && old != c {
		g.ceilings[-int(old.order)-1] = old
	}
	if g.ceilings == nil {
		g.ceilings = make(map[int]*ceilingThinker)
	}
	g.ceilings[sec] = c
	g.assignTrackedCeilingSlot(sec, c.order)
}
func (g *game) ceilingThinkerOwner(sec int) *ceilingThinker {
	c := g.ceilings[sec]
	if c == nil {
		return nil
	}
	if owner, tracked := g.ceilingOwners[sec]; tracked && owner != c.order {
		return nil
	}
	return c
}
func (g *game) resumeBoomCeilings(tag uint16) bool {
	active := false
	for _, c := range g.ceilings {
		if c != nil && g.ceilingTag(c) == tag && c.direction == 0 {
			c.direction = c.oldDirection
			active = true
		}
	}
	return active
}
func cloneCeilingOwners(src map[int]int64) map[int]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]int64, len(src))
	for sec, order := range src {
		dst[sec] = order
	}
	return dst
}
func captureExtraDoors(src []*doorThinker) map[int]doorThinkerSaveState {
	items := make(map[int]*doorThinker, len(src))
	for _, d := range src {
		if d == nil || d.pendingRemove {
			continue
		}
		items[len(items)] = d
	}
	return captureDoorThinkers(items)
}
func restoreExtraDoors(src map[int]doorThinkerSaveState) []*doorThinker {
	if len(src) == 0 {
		return nil
	}
	items := restoreDoorThinkers(src)
	keys := sortedSaveIntKeys(items)
	dst := make([]*doorThinker, 0, len(keys))
	for _, i := range keys {
		dst = append(dst, items[i])
	}
	return dst
}

func validManualDoorState(s gameSaveState, n int) bool {
	for _, d := range s.ExtraDoors {
		if d.Sector < 0 || d.Sector >= n || d.Order <= 0 {
			return false
		}
	}
	for key, c := range s.Ceilings {
		if key < 0 && (c.Sector < 0 || c.Sector >= n || c.Order <= 0 || key != -int(c.Order)-1) {
			return false
		}
	}
	for sec, order := range s.CeilingOwners {
		if sec < 0 || sec >= n || order < 0 {
			return false
		}
		if order == 0 {
			continue
		}
		found := false
		for _, d := range s.Doors {
			if d.Sector == sec && d.Order == order {
				found = true
			}
		}
		for _, d := range s.ExtraDoors {
			if d.Sector == sec && d.Order == order {
				found = true
			}
		}
		for _, c := range s.Ceilings {
			if c.Sector == sec && c.Order == order {
				found = true
			}
		}
		for _, f := range s.Floors {
			if f.Sector == sec && f.Order == order && f.Elevator {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}
