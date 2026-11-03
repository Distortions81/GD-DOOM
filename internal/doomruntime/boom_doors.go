package doomruntime

import "gddoom/internal/mapdata"

func (g *game) activateGeneralizedDoor(line int, info mapdata.LineSpecialInfo, isPlayer bool) bool {
	targets, err := g.m.DoorTargetSectors(line)
	if err != nil {
		return false
	}
	gen := info.Door.Generalized
	if gen.Locked {
		if !isPlayer || !info.Door.CanActivate(g.inventory.keys()) {
			return false
		}
		filtered := targets[:0]
		for _, sec := range targets {
			if !g.ceilingSlotBusy(sec) {
				filtered = append(filtered, sec)
			}
		}
		targets = filtered
	}
	return g.activateDoorSectorsConfigured(targets, info.Door.Action, true, gen)
}
