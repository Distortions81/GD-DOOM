package doomruntime

// These are the original mobjinfo MF_COUNTKILL/MF_COUNTITEM flags, rather
// than the broader sets of damageable monsters and collectible things.
func thingCountsKill(typ int16) bool {
	return isMonster(typ) && typ != 3006
}

func thingCountsItem(typ int16) bool {
	switch typ {
	case 2014, 2015, 2013, 2022, 2023, 2024, 2026, 2045, 83:
		return true
	default:
		return false
	}
}

func (g *game) initLevelStats() {
	g.levelKillsTotal, g.levelItemsTotal = 0, 0
	g.playerKillCount, g.playerItemCount = 0, 0
	// P_SpawnMapThing sets the totals once. Resurrections and runtime spawns
	// must not enlarge the denominator when intermission begins.
	for i, th := range g.m.Things {
		if i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		if thingCountsKill(th.Type) {
			g.levelKillsTotal++
		}
		if thingCountsItem(th.Type) {
			g.levelItemsTotal++
		}
	}
}
