package doomruntime

import "gddoom/internal/mapdata"

func (g *game) activateGeneralizedDoor(line int, info mapdata.LineSpecialInfo, isPlayer bool) bool {
	targets, err := g.m.DoorTargetSectors(line)
	if err != nil {
		return false
	}
	gen := info.Door.Generalized
	if info.Trigger == mapdata.TriggerManual {
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
	activated := g.activateDoorSectors(targets, info.Door.Action)
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
