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
			if g.activeDoorThinker(sec) == nil && g.ceilings[sec] == nil {
				filtered = append(filtered, sec)
			}
		}
		targets = filtered
	}
	if info.Trigger == mapdata.TriggerManual && !gen.Locked {
		for _, sec := range targets {
			if d := g.activeDoorThinker(sec); d != nil {
				if gen.Kind != 0 || d.typ != doorNormal {
					return false
				}
				if d.direction < 0 {
					d.direction = 1
				} else if isPlayer {
					d.direction = -1
				} else {
					return false
				}
				g.emitDoorSectorSound(sec, doorMoveEvent(d.typ, d.direction))
				return true
			}
		}
	}
	firstOrder := g.nextThinkerOrder
	activated := g.activateDoorSectorsWithFloorSlot(targets, info.Door.Action, true)
	for _, sec := range targets {
		if d := g.activeDoorThinker(sec); d != nil && d.order >= firstOrder {
			if gen.Kind == 2 {
				d.typ = doorGenCloseWaitOpen
			}
			d.speed = vDoorSpeed << gen.Speed
			d.topWait = [...]int{1, 4, 9, 30}[gen.Delay] * doomTicsPerSecond
		}
	}
	return activated
}
