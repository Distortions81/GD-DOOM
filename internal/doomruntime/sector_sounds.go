package doomruntime

import "fmt"

func soundEventLoopTics(ev soundEvent) int {
	if ev == soundEventSectorMove {
		return 8
	}
	return 0
}
func soundOriginGroup(ev soundEvent, origin queuedSoundOrigin) string {
	if origin.sectorPlusOne > 0 {
		return fmt.Sprintf("sector:%d", origin.sectorPlusOne-1)
	}
	return singularSoundGroup(ev)
}
func (g *game) emitSectorSound(sec int, ev soundEvent) {
	if x, y, ok := g.sectorSoundOrigin(sec); ok {
		previousID := uint64(0)
		if g.authorityEvents != nil {
			previousID = g.authorityEvents.lastID
		}
		g.emitSoundEventAt(ev, x, y)
		i := len(g.soundQueueOrigin) - 1
		g.soundQueueOrigin[i].sectorPlusOne = sec + 1
		g.soundQueueOrigin[i].tick = g.worldTic
		// Attach source identity to the event just appended by emitSoundEventAt.
		if log := g.authorityEvents; log != nil && log.lastID != previousID && log.count > 0 {
			index := (log.start + log.count - 1) % len(log.events)
			log.events[index].SectorPlusOne = sec + 1
		}
	} else {
		g.emitSoundEvent(ev)
	}
}
func (g *game) emitSectorMoveLoop(sec int) {
	if max(g.worldTic-1, 0)&7 == 0 {
		g.emitSectorSound(sec, soundEventSectorMove)
	}
}
func doorThinkerMoveEvent(d *doorThinker, direction int) soundEvent {
	if d.speed >= 4*vDoorSpeed {
		if direction < 0 {
			return soundEventBlazeClose
		}
		return soundEventBlazeOpen
	}
	return doorMoveEvent(d.typ, direction)
}
