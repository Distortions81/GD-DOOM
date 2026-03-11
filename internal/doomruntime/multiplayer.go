package doomruntime

import "gddoom/internal/mapdata"

type playerStart struct {
	slot  int
	x     int64
	y     int64
	angle uint32
}

func collectPlayerStarts(m *mapdata.Map) []playerStart {
	starts := make([]playerStart, 0, 4)
	for _, t := range m.Things {
		slot := playerSlotFromThingType(t.Type)
		if slot == 0 {
			continue
		}
		starts = append(starts, playerStart{
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
