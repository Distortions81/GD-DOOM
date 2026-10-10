package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
)

func TestAuthorityAdvanceMapCarriesCoopLoadoutsAndResetsLevel(t *testing.T) {
	a := testAuthority(t)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	a.g.withAuthoritativePlayer(a.players[1], func() {
		a.g.stats.Health, a.g.playerMobjHealth, a.g.stats.Armor = 67, 67, 20
		a.g.stats.ArmorType, a.g.stats.Bullets, a.g.stats.Shells = 1, 17, 8
		a.g.inventory.Weapons[2001] = true
		a.g.inventory.ReadyWeapon = weaponShotgun
		a.g.inventory.BlueKey, a.g.inventory.InvulnTics = true, 200
	})
	killAuthorityTestPlayer(a, 2, 0)
	a.g.worldTic = 9
	a.g.levelExitRequested, a.g.secretLevelExit = true, true
	a.finishAuthoritativeRulesTic()
	if !a.MatchState().Ended || !a.SecretExit() {
		t.Fatal("campaign did not expose the secret exit")
	}
	next := cloneMapForRestart(a.g.restartTemplate)
	next.Name = "E1M2"
	oldWorld := a.g
	if err := a.AdvanceMap(next); err != nil {
		t.Fatal(err)
	}
	if a.g == oldWorld || a.MapName() != "E1M2" || a.Tic() != 0 || a.MatchState().Ended || a.SecretExit() {
		t.Fatal("level transition did not establish a fresh level")
	}
	p := a.players[1]
	if p.stats.Health != 67 || p.stats.Armor != 20 || p.stats.Bullets != 17 || p.stats.Shells != 8 || !p.inventory.Weapons[2001] || p.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("living co-op loadout not retained: %+v %+v", p.stats, p.inventory)
	}
	if p.inventory.BlueKey || p.inventory.InvulnTics != 0 || a.g.authorityRules.TeamKeys.Blue {
		t.Fatal("map-specific keys or powers leaked into the next map")
	}
	if dead := a.players[2]; dead.isDead || dead.stats.Health != 100 || dead.stats.Bullets != 50 || dead.inventory.ReadyWeapon != weaponPistol {
		t.Fatal("dead co-op player did not start the next map with a fresh loadout")
	}
	for _, score := range a.MatchState().Scores {
		if score.Generation != 2 {
			t.Fatal("new map reused an old player incarnation")
		}
	}
	if err := a.Step(nil); err != nil || a.Tic() != 1 {
		t.Fatalf("next map did not run: %v", err)
	}
}

func TestAuthorityAdvanceMapRejectsWithoutMutatingOldWorld(t *testing.T) {
	a := testAuthority(t)
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	next := cloneMapForRestart(a.g.restartTemplate)
	next.Name = "E1M2"
	if err := a.AdvanceMap(next); err == nil {
		t.Fatal("unfinished map was silently replaced")
	}
	a.g.levelExitRequested = true
	a.finishAuthoritativeRulesTic()
	before, oldWorld := a.PlayerStates(), a.g
	rnd, prnd := doomrand.State()
	if err := a.AdvanceMap(nil); err == nil {
		t.Fatal("nil map was accepted")
	}
	// Candidate construction consumes RNG; failing its spawn phase must
	// roll that back as well as retaining the old level/player state.
	next.Things = nil
	if err := a.AdvanceMap(next); err == nil {
		t.Fatal("map with no usable player starts was accepted")
	}
	if a.g != oldWorld || !reflect.DeepEqual(before, a.PlayerStates()) || !a.MatchState().Ended {
		t.Fatal("failed transition mutated the current completed level")
	}
	if afterRnd, afterPRnd := doomrand.State(); afterRnd != rnd || afterPRnd != prnd {
		t.Fatalf("failed map setup changed RNG: (%d,%d) -> (%d,%d)", rnd, prnd, afterRnd, afterPRnd)
	}
}

func TestAuthorityDeathmatchRotationResetsRoundScoresAndLoadouts(t *testing.T) {
	a := testAuthority(t)
	a.g.opts.GameMode = gameModeDeathmatch
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.SetRules(AuthorityRules{FragLimit: 1, TimeLimitTics: 35}); err != nil {
		t.Fatal(err)
	}
	a.g.withAuthoritativePlayer(a.players[1], func() { a.g.stats.Bullets = 2 })
	killAuthorityTestPlayer(a, 2, 1)
	a.finishAuthoritativeRulesTic()
	next := cloneMapForRestart(a.g.restartTemplate)
	next.Name = "E1M2"
	if err := a.AdvanceMap(next); err != nil {
		t.Fatal(err)
	}
	for _, score := range a.MatchState().Scores {
		if score.Frags != 0 || score.Deaths != 0 || score.Generation != 2 {
			t.Fatalf("old round score survived rotation: %+v", score)
		}
	}
	if a.players[1].stats.Bullets != 50 || !a.players[1].inventory.BlueKey || a.g.authorityRules.Config.FragLimit != 1 || a.g.authorityRules.Config.TimeLimitTics != 35 {
		t.Fatal("round loadout or configured limits were not initialized correctly")
	}
}
