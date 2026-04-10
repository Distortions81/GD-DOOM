package doomruntime

import (
	"gddoom/internal/doomrand"
)

// checkpointIntervalTics is how often the canonical peer emits a checkpoint.
// 175 tics = 5 seconds at 35 tics/sec.
const checkpointIntervalTics = 175

// SimChecksum computes a lightweight hash of the key simulation state that
// must be identical on all peers for a valid lockstep game. It covers:
//   - world tic counter
//   - RNG indices (both game and cosmetic)
//   - local player position, angle, health, armor, ammo, ready weapon
//   - remote player positions and angles
//   - all sector floor and ceiling heights (catches moving platforms/lifts)
//   - all active monsters: position, angle, HP, AI state
func (g *game) SimChecksum() uint32 {
	h := fnv32Init

	// World tic.
	h = fnv32Mix(h, uint32(g.worldTic))

	// RNG state.
	rndIdx, prndIdx := doomrand.State()
	h = fnv32Mix(h, uint32(rndIdx))
	h = fnv32Mix(h, uint32(prndIdx))

	// Local player.
	h = fnv32Mix(h, uint32(g.p.x>>fracBits))
	h = fnv32Mix(h, uint32(g.p.y>>fracBits))
	h = fnv32Mix(h, uint32(g.p.angle>>16))
	h = fnv32Mix(h, uint32(g.stats.Health))
	h = fnv32Mix(h, uint32(g.stats.Armor))
	h = fnv32Mix(h, uint32(g.stats.Bullets))
	h = fnv32Mix(h, uint32(g.stats.Shells))
	h = fnv32Mix(h, uint32(g.stats.Rockets))
	h = fnv32Mix(h, uint32(g.stats.Cells))
	h = fnv32Mix(h, uint32(g.inventory.ReadyWeapon))

	// Remote players (each peer has the same remote player set).
	for _, rp := range g.remotePlayers {
		h = fnv32Mix(h, uint32(rp.slot))
		h = fnv32Mix(h, uint32(rp.p.x>>fracBits))
		h = fnv32Mix(h, uint32(rp.p.y>>fracBits))
		h = fnv32Mix(h, uint32(rp.p.angle>>16))
	}

	// Sector floors and ceilings (moving platforms/lifts change these).
	for i, v := range g.sectorFloor {
		h = fnv32Mix(h, uint32(i))
		h = fnv32Mix(h, uint32(v>>fracBits))
	}
	for i, v := range g.sectorCeil {
		h = fnv32Mix(h, uint32(i+len(g.sectorFloor)))
		h = fnv32Mix(h, uint32(v>>fracBits))
	}

	// All active monsters: position, angle, HP, AI state.
	// "Active" means not dead, not collected, and spawned in this session.
	for i := range g.m.Things {
		if !g.thingActiveInSession(i) {
			continue
		}
		if !isMonster(g.m.Things[i].Type) {
			continue
		}
		h = fnv32Mix(h, uint32(i))
		h = fnv32Mix(h, uint32(g.thingX[i]>>fracBits))
		h = fnv32Mix(h, uint32(g.thingY[i]>>fracBits))
		h = fnv32Mix(h, g.thingAngleState[i]>>16)
		h = fnv32Mix(h, uint32(g.thingHP[i]))
		h = fnv32Mix(h, uint32(g.thingState[i]))
		h = fnv32Mix(h, uint32(g.thingStateTics[i]))
	}

	return h
}

// FNV-1a 32-bit constants.
const (
	fnv32Init  uint32 = 2166136261
	fnv32Prime uint32 = 16777619
)

func fnv32Mix(h, v uint32) uint32 {
	h ^= v
	h *= fnv32Prime
	return h
}
