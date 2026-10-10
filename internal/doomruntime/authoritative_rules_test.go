package doomruntime

import (
	"errors"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

func newAuthorityRulesTest(t *testing.T, mode string) *Authority {
	t.Helper()
	g := newAuthoritativePlayersTestGame()
	g.m.Things = []mapdata.Thing{
		{Type: 1}, {Type: 2, X: 128},
		{Type: 11, X: 256}, {Type: 11, X: 512}, {Type: 11, X: 768},
	}
	g.opts.GameMode = mode
	g.nextThinkerOrder = int64(len(g.m.Things) + 1)
	g.authorityRules = &authorityRulesState{Config: AuthorityRules{RespawnDelayTics: 2}}
	a := &Authority{g: g}
	a.selectAnchor()
	return a
}

func killAuthorityTestPlayer(a *Authority, id byte, source int) {
	a.g.withAuthoritativePlayer(a.players[id], func() {
		old := a.g.authorityDamageSource
		a.g.authorityDamageSource = source
		a.g.damagePlayer(10000, "test damage")
		a.g.authorityDamageSource = old
	})
}

func TestAuthorityCoopRespawnPreservesSharedWorld(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeCoop)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	world := a.g.m
	a.g.withAuthoritativePlayer(a.players[1], func() { a.g.inventory.BlueKey = true })
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	if !a.players[2].inventory.BlueKey {
		t.Fatal("co-op keys were not shared")
	}
	a.g.doors[0] = &doorThinker{order: 100, sector: 0, typ: doorOpen, direction: 1, speed: fracUnit, topHeight: 200 * fracUnit}
	startCeiling := a.g.sectorCeil[0]
	killAuthorityTestPlayer(a, 1, 0)
	for step := range 3 {
		if err := a.Step(map[byte]demo.Tic{1: {Buttons: demo.ButtonUse}, 2: {Forward: 25}}); err != nil {
			t.Fatal(err)
		}
		if step < 2 && !a.players[1].isDead {
			t.Fatal("respawn bypassed the server delay")
		}
	}
	if a.players[1].isDead || a.players[1].stats.Health != 100 || !a.players[1].inventory.BlueKey {
		t.Fatalf("respawn did not restore player with retained co-op keys: %+v", a.players[1])
	}
	if a.g.m != world || a.Tic() != 4 || a.g.sectorCeil[0] != startCeiling+3*fracUnit || a.players[2].p.x <= 128*fracUnit {
		t.Fatal("a death/respawn reset or stalled the shared world")
	}
	if score := a.MatchState().Scores[0]; score.Generation != 2 || score.Deaths != 1 {
		t.Fatalf("respawn incarnation/scoring=%+v", score)
	}
	if a.players[1].authorityPlayerThinkerOrder == 0 {
		t.Fatal("respawn reused the original map thinker position")
	}
}

func TestAuthorityRespawnWaitsUntilSpawnIsFree(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeCoop)
	// One player start with two slots verifies blocked joins and delayed
	// respawns without telefragging or spawning bodies inside each other.
	a.g.m.Things = a.g.m.Things[:1]
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(2); !errors.Is(err, ErrAuthoritySpawnBlocked) {
		t.Fatalf("occupied join spawn err=%v", err)
	}
	a.g.withAuthoritativePlayer(a.players[1], func() { a.g.p.x = 128 * fracUnit })
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	killAuthorityTestPlayer(a, 1, 0)
	for range 4 {
		if err := a.Step(map[byte]demo.Tic{1: {Buttons: demo.ButtonUse}}); err != nil {
			t.Fatal(err)
		}
	}
	if !a.players[1].isDead || a.players[2].stats.Health != 100 || a.Tic() != 4 {
		t.Fatal("blocked respawn overlapped/killed another player or stalled the world")
	}
	a.g.withAuthoritativePlayer(a.players[2], func() { a.g.p.x = 256 * fracUnit })
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	if a.players[1].isDead || a.players[1].p.x != 0 {
		t.Fatal("queued respawn did not use the freed spawn")
	}
}

func TestAuthorityDeathmatchUsesFreeDistantSpawnsAndKeys(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeDeathmatch)
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	if a.players[1].p.x != 256*fracUnit || a.players[2].p.x != 768*fracUnit {
		t.Fatalf("deathmatch spawn positions=(%d,%d)", a.players[1].p.x, a.players[2].p.x)
	}
	for _, id := range []byte{1, 2} {
		p := a.players[id]
		if !p.inventory.BlueKey || !p.inventory.RedKey || !p.inventory.YellowKey {
			t.Fatal("deathmatch player lacks door keys")
		}
	}
	a.g.m.Things = a.g.m.Things[:2]
	if err := a.AddPlayer(3); err == nil {
		t.Fatal("deathmatch silently fell back to a co-op start")
	}
}

func TestAuthorityDeathmatchScoresAndFragLimit(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeDeathmatch)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.SetRules(AuthorityRules{FragLimit: 1}); err != nil {
		t.Fatal(err)
	}
	killAuthorityTestPlayer(a, 2, 1)
	// A duplicate event must not add another frag or death.
	a.g.authorityPlayerKilled(2, 1)
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	state := a.MatchState()
	if !state.Ended || state.Reason != "frag_limit" || state.WinnerID != 1 || state.Scores[0].Frags != 1 || state.Scores[1].Deaths != 1 {
		t.Fatalf("incorrect frag result: %+v", state)
	}
	if err := a.Step(nil); !errors.Is(err, ErrAuthorityMatchEnded) || a.Tic() != 1 {
		t.Fatal("completed match continued simulating")
	}
}

func TestAuthorityDeathmatchSuicidesAndTimeLimitTie(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeDeathmatch)
	for _, id := range []byte{1, 2} {
		if err := a.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.SetRules(AuthorityRules{TimeLimitTics: 2}); err != nil {
		t.Fatal(err)
	}
	killAuthorityTestPlayer(a, 1, 1)
	killAuthorityTestPlayer(a, 2, 0)
	for range 2 {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	state := a.MatchState()
	if !state.Ended || state.Reason != "time_limit" || state.WinnerID != 0 || state.Scores[0].Frags != -1 || state.Scores[1].Frags != -1 {
		t.Fatalf("incorrect tied result: %+v", state)
	}
}

func TestAuthorityFriendlyFirePolicyAndIncarnations(t *testing.T) {
	a := newAuthorityRulesTest(t, gameModeCoop)
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	first := a.g.authorityPlayerGeneration(1)
	if a.g.authorityPlayerDamageAllowed(1, 2) || !a.g.authorityPlayerDamageAllowed(0, 1) || !a.g.authorityPlayerDamageAllowed(1, 1) {
		t.Fatal("default co-op damage policy is incorrect")
	}
	if err := a.SetRules(AuthorityRules{FriendlyFire: true}); err != nil {
		t.Fatal(err)
	}
	if !a.g.authorityPlayerDamageAllowed(1, 2) {
		t.Fatal("friendly fire setting was ignored")
	}
	a.RemovePlayer(1)
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	if a.g.authorityPlayerGeneration(1) <= first {
		t.Fatal("reused player slot kept an old incarnation")
	}
}
