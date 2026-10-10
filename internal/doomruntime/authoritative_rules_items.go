package doomruntime

import "slices"

// Deathmatch has a fixed 30-second item cycle, with no weapons-stay mode.
// Original map pickups return except invulnerability and partial invisibility;
// drops and runtime-created pickups never return. This policy is part of the
// simulation version, not a client-selected rule.
const authorityItemRespawnTics = 30 * doomTicsPerSecond

func (g *game) scheduleAuthoritativeItemRespawn(i int) {
	if g.authorityRules == nil || g.opts.GameMode != gameModeDeathmatch ||
		i < 0 || i >= len(g.thingSpawnPoint) ||
		(i < len(g.thingDropped) && g.thingDropped[i]) {
		return
	}
	typ := g.thingSpawnPoint[i].Type
	if !isPickupType(typ) || typ == 2022 || typ == 2024 {
		return
	}
	if g.authorityItemRespawns == nil {
		g.authorityItemRespawns = make(map[int]uint64)
	}
	g.authorityItemRespawns[i] = uint64(g.worldTic) + authorityItemRespawnTics
}

func (g *game) tickAuthoritativeItemRespawns() {
	if g.authorityRules == nil || g.opts.GameMode != gameModeDeathmatch || len(g.authorityItemRespawns) == 0 {
		return
	}
	// Reappearance changes blockmap/thinker ordering. Resolve simultaneous
	// deadlines in map order rather than Go's randomized map iteration order.
	due := make([]int, 0)
	for i, tic := range g.authorityItemRespawns {
		if uint64(g.worldTic) >= tic {
			due = append(due, i)
		}
	}
	slices.Sort(due)
	for _, i := range due {
		delete(g.authorityItemRespawns, i)
		if i < 0 || i >= len(g.m.Things) || i >= len(g.thingCollected) ||
			i >= len(g.thingSpawnPoint) || !g.thingCollected[i] {
			continue
		}
		spawn := g.thingSpawnPoint[i]
		x, y := int64(spawn.X)<<fracBits, int64(spawn.Y)<<fracBits
		floor, ceil, _ := g.subsectorFloorCeilAt(x, y)
		g.m.Things[i] = spawn
		g.thingCollected[i] = false
		g.setThingPosFixed(i, x, y)
		g.setThingSupportState(i, floor, floor, ceil)
		g.setThingMomentum(i, 0, 0, 0)
		if i < len(g.thingThinkerOrder) {
			g.thingThinkerOrder[i] = g.allocThinkerOrder()
		}
		g.snapThingRenderState(i)
	}
}
