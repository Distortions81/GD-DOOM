package doomruntime

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

// remotePlayer holds the simulated state for a peer co-op player.
type remotePlayer struct {
	slot int
	p    player
}

// spawnRemotePlayer creates a player struct for a remote peer at their map start.
func spawnRemotePlayer(m *mapdata.Map, slot int) player {
	starts := collectPlayerStarts(m)
	if s, ok := chooseSpawnStart(starts, slot); ok {
		return player{
			x: s.x, y: s.y, z: 0,
			floorz: 0, ceilz: 128 * fracUnit,
			subsector: -1, sector: -1,
			angle:      s.angle,
			viewHeight: playerViewHeight,
		}
	}
	b := mapBounds(m)
	return player{
		x: int64(((b.minX + b.maxX) / 2) * fracUnit),
		y: int64(((b.minY + b.maxY) / 2) * fracUnit),
		ceilz: 128 * fracUnit, subsector: -1, sector: -1,
		viewHeight: playerViewHeight,
	}
}

// stepRemotePlayer advances a remote player one tic using a received demo tic.
func (g *game) stepRemotePlayer(rp *remotePlayer, tc demo.Tic) {
	saved := g.p
	savedSlot := g.localSlot
	g.p = rp.p
	g.localSlot = rp.slot

	cmd, usePressed, fireHeld := demoTicCommand(DemoTic(tc))
	g.runGameplayTic(cmd, usePressed, fireHeld)

	rp.p = g.p
	g.p = saved
	g.localSlot = savedSlot
}

type playerStart struct {
	index int
	slot  int
	x     int64
	y     int64
	angle uint32
}

func collectPlayerStarts(m *mapdata.Map) []playerStart {
	starts := make([]playerStart, 0, 4)
	for i, t := range m.Things {
		slot := playerSlotFromThingType(t.Type)
		if slot == 0 {
			continue
		}
		starts = append(starts, playerStart{
			index: i,
			slot:  slot,
			x:     int64(t.X) << fracBits,
			y:     int64(t.Y) << fracBits,
			angle: thingDegToWorldAngle(t.Angle),
		})
	}
	return starts
}

func playerSlotFromThingType(typ int16) int {
	switch typ {
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 3
	case 4:
		return 4
	default:
		return 0
	}
}

func chooseSpawnStart(starts []playerStart, requestedSlot int) (playerStart, bool) {
	if requestedSlot >= 1 && requestedSlot <= 4 {
		for _, s := range starts {
			if s.slot == requestedSlot {
				return s, true
			}
		}
	}
	for _, s := range starts {
		if s.slot == 1 {
			return s, true
		}
	}
	if len(starts) > 0 {
		return starts[0], true
	}
	return playerStart{}, false
}

func nonLocalStarts(starts []playerStart, localSlot int) []playerStart {
	out := make([]playerStart, 0, len(starts))
	for _, s := range starts {
		if s.slot == localSlot {
			continue
		}
		out = append(out, s)
	}
	return out
}
