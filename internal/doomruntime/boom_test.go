package doomruntime

import (
	"bytes"
	"fmt"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestBoomUnsignedSidedefSnapshotRoundTrip(t *testing.T) {
	m := &mapdata.Map{Linedefs: []mapdata.Linedef{
		{SideNum: [2]int32{32767, -1}},
		{SideNum: [2]int32{32768, 65534}},
	}}
	var buf bytes.Buffer
	if err := (saveBinaryWriter{buf: &buf}).mapData(m); err != nil {
		t.Fatal(err)
	}
	got, err := (saveBinaryReader{r: bytes.NewReader(buf.Bytes())}).mapData()
	if err != nil {
		t.Fatal(err)
	}
	for i := range m.Linedefs {
		if got.Linedefs[i].SideNum != m.Linedefs[i].SideNum {
			t.Fatalf("sides changed in snapshot: %v", got.Linedefs)
		}
	}
	for _, invalid := range []int32{-2, 65535, 65536} {
		m.Linedefs[0].SideNum[0] = invalid
		buf.Reset()
		if err := (saveBinaryWriter{buf: &buf}).mapData(m); err == nil {
			t.Fatalf("unrepresentable side %d accepted", invalid)
		}
	}
}

func TestBoomUnsignedSidedefPhysics(t *testing.T) {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{{}, {X: 64}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, SideNum: [2]int32{32768, -1}}},
		Sidedefs: make([]mapdata.Sidedef, 32769),
		Sectors:  []mapdata.Sector{{CeilingHeight: 128}},
	}
	g := &game{m: m}
	g.initPhysics()
	if len(g.lines) != 1 || g.lines[0].sideNum0 != 32768 {
		t.Fatal("large sidedef lost in physics line")
	}
	if sector, ok := g.sectorIndexFromSidedef(g.lines[0].sideNum0); !ok || sector != 0 {
		t.Fatal("large sidedef did not resolve its sector")
	}
}

func TestBoomSimpleMapLoadsPlaysAndRestores(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	f, err := wad.Open("../../testdata/boom/simple.wad")
	if err != nil {
		t.Fatal(err)
	}
	load := func() *mapdata.Map {
		m, err := mapdata.LoadMap(f, "MAP01")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	g := newGame(load(), Options{})
	if g.secretsTotal != 1 || g.sectorLightFx[0].kind != sectorLightEffectGlow || g.m.Sectors[0].Special != 160 {
		t.Fatalf("combined secret/damage/glow did not initialize: secrets=%d special=%d light=%v", g.secretsTotal, g.m.Sectors[0].Special, g.sectorLightFx[0])
	}
	g.tickPlayerSpecialSector()
	if g.secretsFound != 1 || g.stats.Health != 95 || g.m.Sectors[0].Special != 32 {
		t.Fatalf("secret and damage did not coexist: found=%d health=%d special=%d", g.secretsFound, g.stats.Health, g.m.Sectors[0].Special)
	}
	if !g.useSpecialLineForActor(1, 0, true) || g.lineSpecial[1] != 0 || g.floors[0] == nil {
		t.Fatal("Boom S1 floor switch failed")
	}
	// Existing save state must retain independent flags and an active Boom mover.
	saved := captureGameSaveState(g)
	restored := newGame(load(), Options{})
	restoreGameSaveState(restored, saved)
	if restored.m.Sectors[0].Special != 32 || restored.secretsFound != 1 || restored.floors[0] == nil || restored.lineSpecial[1] != 0 {
		t.Fatal("Boom flags, secret count, mover or consumed switch lost on restore")
	}
	for i := 0; i < 30; i++ {
		restored.runGameplayTic(moveCmd{}, false, false)
	}
	if restored.sectorFloor[0] != 24*fracUnit || restored.p.z != 24*fracUnit || restored.floors[0] != nil {
		t.Fatalf("floor did not finish carrying player: floor=%d player=%d mover=%+v", restored.sectorFloor[0], restored.p.z, restored.floors[0])
	}
	if !restored.activateShootLineSpecial(2, mapdata.LookupLineSpecial(197)) || !restored.levelExitRequested {
		t.Fatal("Boom gun exit failed")
	}
}

func TestBoomWalkOnceRetriesBusySector(t *testing.T) {
	for _, special := range []uint16{142, 58} {
		t.Run(fmt.Sprint(special), func(t *testing.T) {
			g := &game{
				m: &mapdata.Map{
					Vertexes: []mapdata.Vertex{{X: 0, Y: 64}, {X: 0, Y: -64}},
					Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Special: special, Tag: 7, SideNum: [2]int32{0, 1}, Flags: mlTwoSided}},
					Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 0}},
					Sectors:  []mapdata.Sector{{Tag: 7, CeilingHeight: 1024}},
				},
				lineSpecial: []uint16{special},
				sectorFloor: []int64{0}, sectorCeil: []int64{1024 * fracUnit},
				floors: map[int]*floorThinker{0: {sector: 0}},
			}
			g.initPhysics()
			cross := func() { g.checkWalkSpecialLinesWithCandidates(8*fracUnit, 0, -8*fracUnit, 0, []int{0}) }
			cross()
			want := special
			if special == 58 {
				want = 0
			}
			if g.lineSpecial[0] != want {
				t.Fatalf("busy crossing left special=%d want=%d", g.lineSpecial[0], want)
			}
			if special == 142 {
				delete(g.floors, 0)
				cross()
				if g.lineSpecial[0] != 0 || g.floors[0] == nil || g.floors[0].destHeight != 512*fracUnit {
					t.Fatal("Boom crossing did not retry after sector became idle")
				}
			}
		})
	}
}

func TestBoomDamageBitsAndSuit(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, tc := range []struct {
		special int16
		damage  int
	}{
		{32, 5}, {64 | 8, 10}, {96 | 128, 20}, {128 | 5, 0}, {128 | 11, 0},
	} {
		if got := hazardDamage(tc.special, false); got != tc.damage {
			t.Fatalf("special=%d damage=%d want=%d", tc.special, got, tc.damage)
		}
	}
	for _, special := range []int16{32, 64} {
		if got := hazardDamage(special, true); got != 0 {
			t.Fatalf("suit did not protect special %d", special)
		}
	}
	doomrand.SetState(0, 0)
	for i := 0; i < 7; i++ {
		hazardDamage(96, true)
	}
	if _, rng := doomrand.State(); rng != 7 {
		t.Fatalf("suit leak checks consumed %d RNG values, want 7", rng)
	}
}

func TestBoomUnconditionalSwitchesWithoutTaggedTargets(t *testing.T) {
	for _, special := range []uint16{163, 169, 170, 171, 172, 173, 181, 182, 192, 193, 194} {
		g := &game{
			m:           &mapdata.Map{Linedefs: []mapdata.Linedef{{Special: special, Tag: 7}}, Sectors: []mapdata.Sector{{}}},
			lineSpecial: []uint16{special},
		}
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatalf("switch %d did not activate without a tagged target", special)
		}
		want := special
		if !mapdata.LookupLineSpecial(special).Repeat {
			want = 0
		}
		if g.lineSpecial[0] != want {
			t.Fatalf("switch %d remaining special=%d want=%d", special, g.lineSpecial[0], want)
		}
	}
}

func TestBoomSectorInitialization(t *testing.T) {
	t.Cleanup(doomrand.Clear)
	for _, typ := range []int16{1, 2, 3, 4, 8, 12, 13, 17} {
		g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{Special: 128 | 32 | typ, Light: 192}}}}
		g.initSectorLightEffects()
		want := int16(128 | 32)
		if typ == 4 {
			want = 128 | 96
		}
		if g.m.Sectors[0].Special != want || g.sectorLightFx[0].kind == sectorLightEffectNone {
			t.Fatalf("lighting type %d did not retain flags: %d", typ, g.m.Sectors[0].Special)
		}
	}
	for _, typ := range []int16{10, 14} {
		g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{Special: 128 | 32 | typ}}}, sectorFloor: []int64{0}, sectorCeil: []int64{128 * fracUnit}}
		g.initTimedDoorSpecials()
		if g.m.Sectors[0].Special != 0 || g.doors[0] == nil {
			t.Fatalf("timed door %d did not consume its special or create a thinker", typ)
		}
	}
}
