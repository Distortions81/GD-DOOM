package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/mapdata"
)

func newArchvileRaiseTestGame(x, y int16) *game {
	g := &game{m: &mapdata.Map{
		Sectors: []mapdata.Sector{{CeilingHeight: 128}},
		Things:  []mapdata.Thing{{Type: 64}, {Type: 9, X: x, Y: y, Flags: thingFlagAmbush}},
	}, thingHP: []int{700, -10}, thingCollected: []bool{false, false},
		thingDead: []bool{false, true}, thingAggro: []bool{true, true},
		thingGibbed: []bool{false, false}, stats: playerStats{Health: 100}}
	g.initPhysics()
	g.ensureMonsterAIState()
	g.p.x = 1000 * fracUnit
	g.thingState[0], g.thingStateTics[0] = monsterStateSee, 2
	g.thingState[1], g.thingStatePhase[1], g.thingStateTics[1] = monsterStateDeath, 4, -1
	g.thingMoveDir[0] = monsterDirEast
	g.setMonsterTargetPlayer(0)
	g.setMonsterTargetPlayer(1)
	return g
}

func TestArchvileSearchUsesNextStepSquareAndWaitsForCorpse(t *testing.T) {
	g := newArchvileRaiseTestGame(49, 37)
	g.thingStateTics[1] = 1
	if g.archvileTryRaiseCorpse(0) {
		t.Fatal("raised a corpse before its terminal death frame")
	}
	g.thingStateTics[1] = -1
	g.thingMoveDir[0] = monsterDirNoDir
	if g.archvileTryRaiseCorpse(0) {
		t.Fatal("raised while movedir was DI_NODIR")
	}
	g.thingMoveDir[0] = monsterDirEast
	if !g.archvileTryRaiseCorpse(0) {
		t.Fatal("missed corpse inside next-step square but outside old approximate-distance search")
	}
	if !g.thingTargetPlayer[0] || g.thingTargetPlayer[1] || g.thingTargetIdx[1] != -1 {
		t.Fatal("healing changed the vile's target or kept the corpse's old target")
	}
}

func TestArchvileRejectedCorpseStopsThrustAndKeepsDeathState(t *testing.T) {
	g := newArchvileRaiseTestGame(48, 0)
	g.m.Things = append(g.m.Things, mapdata.Thing{Type: 30, X: 48})
	g.thingCollected = append(g.thingCollected, false)
	g.thingAggro = append(g.thingAggro, false)
	g.thingHP = append(g.thingHP, 0)
	g.ensureMonsterAIState()
	g.thingMomX[1], g.thingMomY[1], g.thingMomZ[1] = fracUnit, -fracUnit, 2*fracUnit
	if g.archvileTryRaiseCorpse(0) {
		t.Fatal("raised corpse overlapping a solid pillar")
	}
	if g.thingMomX[1] != 0 || g.thingMomY[1] != 0 || g.thingMomZ[1] != 2*fracUnit || !g.thingDead[1] || g.thingStateTics[1] != -1 {
		t.Fatal("failed position check did not preserve the original corpse state and vertical momentum")
	}
}

func TestArchvileRaiseAndHealFramesDelayChasing(t *testing.T) {
	g := newArchvileRaiseTestGame(48, 0)
	g.thingReactionTics[1], g.thingThreshold[1], g.thingMoveCount[1] = 7, 13, 9
	if !g.archvileTryRaiseCorpse(0) {
		t.Fatal("expected resurrection")
	}
	if demoTraceThingState(g, 0, 64) != 266 || demoTraceThingState(g, 1, 9) != 236 || g.thingCurrentHeight(1, g.m.Things[1]) != 56*fracUnit {
		t.Fatal("raise did not restore full height and enter the original heal/raise states")
	}
	for tic := 1; tic < 25; tic++ {
		g.tickThingThinker(1, g.m.Things[1])
		if g.thingState[1] != monsterStateRaise || g.thingMoveCount[1] != 9 || g.thingThreshold[1] != 13 || g.thingReactionTics[1] != 7 || g.monsterHasExplicitTarget(1) {
			t.Fatalf("corpse resumed chase during its raise animation at tic %d", tic)
		}
		if got, want := demoTraceThingState(g, 1, 9), 236+tic/5; got != want {
			t.Fatalf("raise state=%d want=%d at tic %d", got, want, tic)
		}
	}
	g.tickThingThinker(1, g.m.Things[1])
	if g.thingState[1] != monsterStateSee || g.thingDoomState[1] != monsterDoomSeeState(9) {
		t.Fatal("raise completion did not enter RUN1")
	}
	for tic := 1; tic < 30; tic++ {
		g.tickThingThinker(0, g.m.Things[0])
		if g.thingState[0] != monsterStateHeal || demoTraceThingState(g, 0, 64) != 266+tic/10 {
			t.Fatalf("heal frames changed at tic %d", tic)
		}
	}
}

func TestArchvileRaisesCrushedGhostWithoutRestoringBounds(t *testing.T) {
	g := newArchvileRaiseTestGame(48, 0)
	g.thingGibbed[1], g.thingState[1] = true, monsterStateGibs
	if !g.archvileTryRaiseCorpse(0) {
		t.Fatal("original arch-vile can raise crusher gibs")
	}
	if g.thingCurrentHeight(1, g.m.Things[1]) != 0 || g.thingCurrentRadius(1, g.m.Things[1]) != 0 || demoTraceThingState(g, 1, 9) != 236 || demoTraceThingTics(g, 1, 9) != 5 {
		t.Fatal("ghost dimensions or raise animation were lost")
	}
	// A second death still animates; zero dimensions alone do not make its
	// state S_GIBS again. The crusher action supplies that state explicitly.
	g.thingDead[1], g.thingState[1], g.thingStateTics[1] = true, monsterStateDeath, 5
	g.thingStatePhase[1] = 0
	g.tickThingThinker(1, g.m.Things[1])
	if g.thingStateTics[1] != 4 || demoTraceThingState(g, 1, 9) == demoTraceStateGibs {
		t.Fatal("zero-sized ghost skipped its second death animation")
	}
}

func TestArchvileCorpseChoiceFollowsBlockmapLinkOrder(t *testing.T) {
	g := newArchvileRaiseTestGame(112, 32)
	g.m.Things[0].X, g.m.Things[0].Y = 64, 64
	g.m.Things = append(g.m.Things, mapdata.Thing{Type: 9, X: 112, Y: 96})
	g.thingCollected = append(g.thingCollected, false)
	g.thingAggro = append(g.thingAggro, false)
	g.thingHP = append(g.thingHP, -10)
	g.ensureMonsterAIState()
	g.setThingPosFixed(0, 64*fracUnit, 64*fracUnit)
	g.thingHP[2], g.thingDead[2] = -10, true
	g.thingState[2], g.thingStatePhase[2], g.thingStateTics[2] = monsterStateDeath, 4, -1
	g.bmapWidth, g.bmapHeight = 1, 1
	g.thingBlockOrder = []int64{1, 5, 4}
	g.rebuildThingBlockmap()
	if !g.archvileTryRaiseCorpse(0) || g.thingDead[1] || !g.thingDead[2] {
		t.Fatal("vile did not choose the newest linked corpse before the higher-index corpse")
	}
}

func TestArchvileRaiseAndGhostSurviveBinarySnapshots(t *testing.T) {
	for _, tc := range []struct {
		name    string
		magic   []byte
		version int
	}{{"save", saveGameMagic, saveGameVersion}, {"keyframe", keyframeMagic, keyframeVersion}} {
		t.Run(tc.name, func(t *testing.T) {
			g := newArchvileRaiseTestGame(48, 0)
			g.thingGibbed[1], g.thingState[1] = true, monsterStateGibs
			if !g.archvileTryRaiseCorpse(0) {
				t.Fatal("expected ghost resurrection")
			}
			for tic := 0; tic < 7; tic++ {
				g.tickThingThinker(0, g.m.Things[0])
				g.tickThingThinker(1, g.m.Things[1])
			}
			blob, err := encodeSnapshot(tc.magic, saveFile{Version: tc.version, Game: captureGameSaveState(g)})
			if err != nil {
				t.Fatal(err)
			}
			file, err := decodeSnapshot(blob, tc.magic)
			if err != nil {
				t.Fatal(err)
			}
			restored := newArchvileRaiseTestGame(48, 0)
			restoreGameSaveState(restored, file.Game)
			// Advance across another raise/heal frame boundary. Loading must
			// preserve the countdown, null corpse target, and ghost dimensions.
			for tic := 0; tic < 7; tic++ {
				for _, sim := range []*game{g, restored} {
					sim.tickThingThinker(0, sim.m.Things[0])
					sim.tickThingThinker(1, sim.m.Things[1])
				}
			}
			if !reflect.DeepEqual(g.thingState, restored.thingState) ||
				!reflect.DeepEqual(g.thingStatePhase, restored.thingStatePhase) ||
				!reflect.DeepEqual(g.thingStateTics, restored.thingStateTics) ||
				restored.monsterHasExplicitTarget(1) || !restored.thingTargetPlayer[0] ||
				restored.thingCurrentRadius(1, restored.m.Things[1]) != 0 ||
				restored.thingCurrentHeight(1, restored.m.Things[1]) != 0 {
				t.Fatal("snapshot changed the resurrection's continuing simulation")
			}
		})
	}
}
