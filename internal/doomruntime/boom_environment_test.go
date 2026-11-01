package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func environmentFixtureGame(t *testing.T, name string) *game {
	t.Helper()
	f, err := wad.Open("../../testdata/boom/environment.wad")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(f, mapdata.MapName(name))
	if err != nil {
		t.Fatal(err)
	}
	return newGame(m, Options{SkillLevel: 3})
}

func TestBoomEnvironmentFlagsAndForceRestrictions(t *testing.T) {
	for _, tc := range []struct {
		mapname      string
		z            int64
		wantX, wantY int64
	}{
		{"MAP06", 0, 40 << 9, -17 << 9}, {"MAP06", 24 * fracUnit, 81 << 9, -33 << 9},
		{"MAP07", 0, 0, 0}, {"MAP07", 32 * fracUnit, 40 << 9, -17 << 9}, {"MAP07", 64 * fracUnit, 40 << 9, -17 << 9}, {"MAP07", 65 * fracUnit, 81 << 9, -33 << 9},
		{"MAP08", 0, 81 << 9, -33 << 9}, {"MAP08", fracUnit, 0, 0},
		{"MAP09", 0, 81 << 9, -33 << 9}, {"MAP09", 64 * fracUnit, 81 << 9, -33 << 9}, {"MAP09", 65 * fracUnit, 0, 0},
	} {
		g := environmentFixtureGame(t, tc.mapname)
		g.p.z = tc.z
		g.playerViewZ = tc.z + playerViewHeight
		for _, p := range g.boomPushers {
			g.tickBoomPusher(p)
		}
		if g.p.momx != tc.wantX || g.p.momy != tc.wantY {
			t.Fatalf("%s z=%d: (%d,%d), want (%d,%d)", tc.mapname, tc.z, g.p.momx, g.p.momy, tc.wantX, tc.wantY)
		}
		if g.thingMomX[1] != 0 || g.thingMomY[1] != 0 {
			t.Fatal("Boom constant force affected monster")
		}
		g.p.momx, g.p.momy = 0, 0
		g.noClip = true
		for _, p := range g.boomPushers {
			g.tickBoomPusher(p)
		}
		if g.p.momx != 0 || g.p.momy != 0 {
			t.Fatal("force affected noclip player")
		}
		g.noClip = false
		g.m.Sectors[0].Special &^= 0x200
		for _, p := range g.boomPushers {
			g.tickBoomPusher(p)
		}
		if g.p.momx != 0 || g.p.momy != 0 {
			t.Fatal("disabled force still active")
		}
	}
}

func TestBoomFrictionStateAndMudAcceleration(t *testing.T) {
	g := environmentFixtureGame(t, "MAP01")
	if got := g.boomPlayerMoveFactor(); got != 2048 {
		t.Fatalf("first-tic movefactor=%d", got)
	}
	for _, f := range g.boomFrictions {
		g.tickBoomFriction(f)
	}
	f := g.boomFrictions[0]
	if g.p.friction != f.friction || g.p.movefactor != f.movefactor {
		t.Fatal("friction thinker failed")
	}
	if g.thingFriction[1] != 0 {
		t.Fatal("Boom friction affected monster")
	}
	for _, tc := range []struct{ momentum, scale int64 }{{0, 1}, {15000, 1}, {15001, 2}, {30001, 4}, {60001, 8}} {
		g.p.movefactor = f.movefactor
		g.p.momx = tc.momentum
		g.p.momy = 0
		if got := g.boomPlayerMoveFactor(); got != f.movefactor*tc.scale {
			t.Fatalf("momentum=%d factor=%d", tc.momentum, got)
		}
		if g.p.movefactor != boomMoveFactor {
			t.Fatal("consumed movefactor not reset")
		}
	}
}

func TestBoomFrictionBinarySnapshotRoundTrip(t *testing.T) {
	g := environmentFixtureGame(t, "MAP01")
	g.p.friction = 56197
	g.p.movefactor = 6
	g.thingFriction[0] = 63078
	g.thingMoveFactor[0] = 847
	state := captureGameSaveState(g)
	for _, version := range []int{saveGameVersion, keyframeVersion, 30, 17} {
		file := saveFile{Version: version, Game: state}
		data, err := encodeSnapshot(saveGameMagic, file)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		restored := environmentFixtureGame(t, "MAP01")
		restoreGameSaveState(restored, loaded.Game)
		if version == saveGameVersion || version == keyframeVersion {
			if restored.p.friction != g.p.friction || restored.p.movefactor != g.p.movefactor || restored.thingFriction[0] != g.thingFriction[0] || restored.thingMoveFactor[0] != g.thingMoveFactor[0] {
				t.Fatal("friction state lost")
			}
		} else {
			if storedBoomFriction(restored.p.friction) != friction || len(restored.thingFriction) != len(g.m.Things) {
				t.Fatal("legacy friction defaults missing")
			}
			for _, f := range restored.boomFrictions {
				restored.tickBoomFriction(f)
			}
		}
	}
}

func TestBoomFrictionCheckpointContinuation(t *testing.T) {
	g := environmentFixtureGame(t, "MAP15")
	g.thingStateTics[1] = 100000
	cmd := moveCmd{forward: 25, side: -7}
	for tic := 0; tic < 5; tic++ {
		g.runGameplayTic(cmd, false, false)
	}
	data, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: captureGameSaveState(g)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	restored := environmentFixtureGame(t, "MAP15")
	restoreGameSaveState(restored, file.Game)
	for tic := 0; tic < 20; tic++ {
		g.runGameplayTic(cmd, false, false)
		restored.runGameplayTic(cmd, false, false)
		if g.p != restored.p || g.worldTic != restored.worldTic {
			t.Fatalf("restored friction diverged at tic %d", tic)
		}
	}
	bad := captureGameSaveState(g)
	bad.ThingMoveFactor = nil
	if validBoomFrictionState(&bad, len(g.m.Things)) {
		t.Fatal("accepted mismatched friction arrays")
	}
}

func TestBoomEnvironmentVoodooBodiesAndPointMarkers(t *testing.T) {
	for _, mapname := range []string{"MAP01", "MAP06"} {
		f, err := wad.Open("../../testdata/boom/environment.wad")
		if err != nil {
			t.Fatal(err)
		}
		m, err := mapdata.LoadMap(f, mapdata.MapName(mapname))
		if err != nil {
			t.Fatal(err)
		}
		m.Things = append(m.Things, mapdata.Thing{X: 128, Y: 192, Type: 1, Flags: 7})
		g := newGame(m, Options{SkillLevel: 3})
		if len(g.boomVoodooBodies) != 1 || g.boomVoodooBodies[0] != 0 {
			t.Fatal("voodoo force targets not rebuilt")
		}
		g.noClip = true // The cheat affects the controlled body, not its voodoo copy.
		if mapname == "MAP01" {
			for _, f := range g.boomFrictions {
				g.tickBoomFriction(f)
			}
			if g.thingFriction[0] != g.boomFrictions[0].friction || g.p.friction != 0 {
				t.Fatal("friction confused doll and controlled body")
			}
		} else {
			for _, p := range g.boomPushers {
				g.tickBoomPusher(p)
			}
			if g.thingMomX[0] != 40<<9 || g.thingMomY[0] != -17<<9 || g.p.momx != 0 || g.p.momy != 0 {
				t.Fatal("wind confused doll and controlled body")
			}
		}
	}
	for _, typ := range []int16{5001, 5002} {
		info, ok := demoTraceThingInfoForType(typ)
		if !ok || thingTypeUsesBlockmap(typ) || info.radius != 8*fracUnit || info.height != 8*fracUnit {
			t.Fatal("point marker physics incorrect")
		}
	}
}
