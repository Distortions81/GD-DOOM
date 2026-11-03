package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestBoomResurrectionRestoresCrushedBodyAndRejectsBlockedBounds(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		g := newArchvileRaiseTestGame(48, 0)
		g.boomMomentumDropoff = true
		g.thingGibbed[1], g.thingState[1] = true, monsterStateGibs
		if blocked {
			// A pillar outside a zero-radius corpse but inside its restored body.
			g.m.Things = append(g.m.Things, mapdata.Thing{Type: 30, X: 80})
			g.thingCollected = append(g.thingCollected, false)
			g.ensureMonsterAIState()
		}
		if got := g.archvileTryRaiseCorpse(0); got == blocked {
			t.Fatalf("blocked=%v raised=%v", blocked, got)
		}
		if !blocked && (g.thingCurrentRadius(1, g.m.Things[1]) != 20*fracUnit || g.thingCurrentHeight(1, g.m.Things[1]) != 56*fracUnit) {
			t.Fatal("Boom retained vanilla ghost bounds")
		}
		if blocked && (!g.thingDead[1] || !g.thingGibbed[1]) {
			t.Fatal("rejected corpse was changed")
		}
	}
}

func TestBoomAirborneViewAndTeleportFreeze(t *testing.T) {
	g := &game{boomMomentumDropoff: true, p: player{z: 16 * fracUnit, ceilz: 64 * fracUnit, viewHeight: 30 * fracUnit}}
	g.updatePlayer(moveCmd{})
	g.tickPlayerViewHeight()
	if g.playerViewZ != 57*fracUnit {
		t.Fatalf("airborne eye=%d", g.playerViewZ)
	}
	g.p.ceilz = 48 * fracUnit
	g.tickPlayerViewHeight()
	if g.playerViewZ != 44*fracUnit {
		t.Fatal("airborne eye was not ceiling-clamped")
	}
	// Teleport from air to floor while P_MovePlayer is frozen. Preserve the
	// cached onground decision through the tic which decrements reactiontime to 0.
	g.p.z, g.p.floorz, g.p.ceilz = 0, 0, 128*fracUnit
	g.p.reactionTime = 2
	g.p.momx = 8 * fracUnit
	for i := 0; i < 2; i++ {
		g.updatePlayer(moveCmd{})
		g.tickPlayerViewHeight()
		if g.playerViewZ != playerViewHeight {
			t.Fatal("teleport freeze resumed squat/bob early")
		}
	}
	g.updatePlayer(moveCmd{})
	if g.p.viewGroundState != 1 {
		t.Fatal("movement did not refresh onground")
	}
}

func TestBoomTracerUsesLogicPhaseWithoutDemoCursor(t *testing.T) {
	old := doomrand.Capture()
	defer doomrand.Restore(old)
	for phase := uint32(0); phase < 4; phase++ {
		doomrand.SetLogicTic(phase)
		g := &game{p: player{x: 128 * fracUnit, y: 128 * fracUnit}, stats: playerStats{Health: 100}, demoTick: 0}
		p := projectile{kind: projectileTracer, tracerPlayer: true, z: 32 * fracUnit, vx: 10 * fracUnit}
		g.tickProjectileSpecial(&p)
		if got := p.vy != 0; got != (phase == 0) {
			t.Fatalf("logic phase=%d homed=%v", phase, got)
		}
	}
}

func TestBoomViewGroundStateSurvivesSnapshots(t *testing.T) {
	for _, format := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		g := newArchvileRaiseTestGame(48, 0)
		g.p.viewGroundState = -1
		g.p.reactionTime = 11
		state := saveFile{Version: format.version, Game: captureGameSaveState(g), RNG: saveRNGState{Extended: doomrand.Capture()}}
		blob, err := encodeSnapshot(format.magic, state)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSnapshot(blob, format.magic)
		if err != nil {
			t.Fatal(err)
		}
		restored := restorePlayerSaveState(decoded.Game.Player)
		if restored.viewGroundState != -1 || restored.reactionTime != 11 {
			t.Fatal("snapshot lost teleport camera phase")
		}
	}
}

func TestPreviousBoomSnapshotsRetainRNGWithoutViewGroundCache(t *testing.T) {
	old := doomrand.Capture()
	defer doomrand.Restore(old)
	doomrand.ConfigureBoom(1993, true)
	doomrand.SetLogicTic(37)
	doomrand.PRandom(doomrand.Tracer)
	rng := doomrand.Capture()
	for _, format := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, 36}, {keyframeMagic, 23}} {
		file := saveFile{Version: format.version, RNG: saveRNGState{Extended: rng, MenuIndex: rng.MenuIndex, PlayIndex: rng.PlayIndex}, Game: gameSaveState{Player: playerSaveState{ViewGroundState: -1}}}
		blob, err := encodeSnapshot(format.magic, file)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSnapshot(blob, format.magic)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.RNG.Extended != rng || decoded.Game.Player.ViewGroundState != 0 {
			t.Fatal("previous format lost Boom RNG or invented a camera cache")
		}
	}
}
