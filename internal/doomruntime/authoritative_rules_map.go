package doomruntime

import (
	"fmt"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func (a *Authority) MapName() mapdata.MapName { return a.g.m.Name }

// SecretExit tells a campaign owner which progression edge completed the map.
// mapdata.NextMapName already resolves Doom's secret exits and return maps.
func (a *Authority) SecretExit() bool { return a.g.secretLevelExit }

// AdvanceMap atomically replaces a completed level while preserving active
// player slots. The caller owns campaign/rotation selection and must establish
// a NEW network epoch, discard pending inputs and acknowledgements, and send a
// reliable map-change message before sending the new baseline or stepping.
// Tic deliberately resets to zero: Doom's damage/mover/respawn timing is local
// to a level. This method never runs automatically on an individual death.
//
// Living co-op players keep health, armor, ammo, and weapons. Dead players get a
// fresh loadout. Keys and temporary powers clear for the next map. Deathmatch
// starts a fresh round with reset scores and loadouts. Every active slot gets a
// new incarnation so old projectiles/targets cannot bind a replacement body.
// Failure leaves the current completed match and RNG unchanged.
func (a *Authority) AdvanceMap(next *mapdata.Map) error {
	if !a.g.authorityRules.Ended {
		return fmt.Errorf("cannot advance an unfinished authoritative match")
	}
	if next == nil || len(next.Sectors) == 0 {
		return fmt.Errorf("next authority map requires sectors")
	}
	menuRNG, playRNG := doomrand.State()
	succeeded := false
	defer func() {
		if !succeeded {
			doomrand.SetState(menuRNG, playRNG)
		}
	}()
	oldRules := a.g.authorityRules
	// A normal level transition preserves the RNG streams while map setup
	// consumes the next values. A failed tentative setup restores them above.
	candidate := &Authority{g: newGameWithRNG(cloneMapForRestart(next), a.g.opts, false)}
	candidate.g.authorityEvents = &authorityEventLog{}
	candidate.g.authorityRules = &authorityRulesState{Config: oldRules.Config}
	for id := range oldRules.Scores {
		candidate.g.authorityRules.Scores[id].Generation = oldRules.Scores[id].Generation
	}
	candidate.selectAnchor()
	for id := byte(1); id <= 4; id++ {
		old := a.players[id]
		if old == nil {
			continue
		}
		if err := candidate.AddPlayer(id); err != nil {
			return fmt.Errorf("spawn player %d on next map: %w", id, err)
		}
		if a.g.opts.GameMode == gameModeCoop {
			candidate.g.authorityRules.Scores[id].Deaths = oldRules.Scores[id].Deaths
			if !old.isDead && old.stats.Health > 0 {
				candidate.g.withAuthoritativePlayer(candidate.players[id], func() {
					candidate.g.applyLevelCarryover(playerLevelCarryover{Inventory: old.inventory, Stats: old.stats})
					candidate.g.bringUpWeapon()
				})
			}
		}
	}
	candidate.selectAnchor()
	a.g, a.players = candidate.g, candidate.players
	succeeded = true
	return nil
}
