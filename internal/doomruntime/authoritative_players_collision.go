package doomruntime

// walkAuthoritativeActorBlockCell is the roster-aware equivalent of
// walkActorBlockCell. Player bodies occupy mutable blockmap links alongside
// things, ordered by playerBlockOrder. Map start markers never become bodies.
// The legacy iterator remains unchanged for demo compatibility.
func (g *game) walkAuthoritativeActorBlockCell(cell int, visitThing func(int), visitPlayer func()) {
	for key := g.authoritativeActorBlockLink(cell, -2); key != -2; {
		next := g.authoritativeActorBlockLink(cell, key)
		if key <= -3 {
			state := g.authoritativePlayerForSlot(-key - 2)
			if state == nil {
				key = next
				continue
			}
			g.withAuthoritativePlayer(state, func() {
				visitPlayer()
				cell = g.thingBlockmapCellFor(g.p.x, g.p.y)
			})
			next = g.authoritativeActorBlockLink(cell, key)
		} else {
			visitThing(key)
			if key < len(g.thingBlockCell) && !(key < len(g.thingCollected) && g.thingCollected[key]) {
				cell = g.thingBlockCell[key]
				next = g.authoritativeActorBlockLink(cell, key)
			}
		}
		key = next
	}
}

// -2 is the head/end sentinel, -(slot+2) is a player, and nonnegative keys
// index map things. The live player context can be ahead of its saved roster
// state while a world thinker runs, so all body reads use the context helper.
func (g *game) authoritativeActorBlockLink(cell, after int) int {
	if cell < 0 || cell >= len(g.thingBlockCells) {
		return -2
	}
	afterOrder := int64(0)
	if after <= -3 {
		if p := g.authoritativePlayerForSlot(-after - 2); p != nil {
			_, _, afterOrder = g.authoritativePlayerBody(p)
		}
	} else if after >= 0 && after < len(g.thingBlockOrder) {
		afterOrder = g.thingBlockOrder[after]
	}
	next := -2
	nextOrder := int64(-1)
	consider := func(key int, order int64) {
		if (after == -2 || order < afterOrder) && order > nextOrder {
			next, nextOrder = key, order
		}
	}
	for _, i := range g.thingBlockCells[cell] {
		if i < 0 || i >= len(g.m.Things) || i >= len(g.thingBlockOrder) ||
			(i < len(g.thingCollected) && g.thingCollected[i]) || isPlayerStart(g.m.Things[i].Type) {
			continue
		}
		consider(i, g.thingBlockOrder[i])
	}
	for _, state := range g.authorityPlayers {
		p, _, order := g.authoritativePlayerBody(state)
		if g.thingBlockmapCellFor(p.x, p.y) == cell {
			consider(-state.localSlot-2, order)
		}
	}
	return next
}
