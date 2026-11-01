package doomruntime

import (
	"bytes"
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func boomCeilingGame(special uint16) *game {
	g := boomFloorGame(special)
	g.m.Sectors[0].CeilingPic = "MODEL"
	g.m.Sectors[1].CeilingPic = "OLD"
	g.m.Sidedefs[0].Top = "SHORT"
	g.m.Sidedefs[1].Top = "SHORT"
	return g
}
func TestBoomGeneralizedCeilingTargets(t *testing.T) {
	for target := uint16(0); target < 8; target++ {
		for speed := uint16(0); speed < 4; speed++ {
			for _, up := range []bool{false, true} {
				special := uint16(0x4002) | target<<7 | speed<<3
				if up {
					special |= 64
				}
				g := boomCeilingGame(special)
				if !g.useSpecialLineForActor(0, 0, true) {
					t.Fatalf("%x failed", special)
				}
				want := int64(128)
				switch target {
				case 2:
					if !up {
						want = 96
					}
				case 3:
					want = 0
				case 4:
					want = 32
				case 5:
					want = 80
					if up {
						want = 112
					}
				case 6:
					want = 72
					if up {
						want = 120
					}
				case 7:
					want = 64
					if up {
						want = 128
					}
				}
				ct := g.ceilings[1]
				dest := ct.bottomHeight
				if up {
					dest = ct.topHeight
				}
				if dest != want*fracUnit || ct.speed != fracUnit<<speed || ct.crush {
					t.Fatalf("%x mover=%+v want=%d", special, ct, want)
				}
			}
		}
	}
}
func TestBoomGeneralizedCeilingChangesAndSave(t *testing.T) {
	for _, numeric := range []bool{false, true} {
		for change := uint16(1); change < 4; change++ {
			special := uint16(0x4002) | 64 | change<<10
			if numeric {
				special |= 32
			}
			g := boomCeilingGame(special)
			if !g.useSpecialLineForActor(0, 0, true) {
				t.Fatal("ceiling failed")
			}
			if g.m.Sectors[1].CeilingPic != "OLD" {
				t.Fatal("changed before completion")
			}
			// Persist the pending change through the actual binary snapshot codec.
			file := saveFile{Version: saveGameVersion, Game: gameSaveState{Ceilings: captureCeilingThinkers(g.ceilings)}}
			raw, err := encodeSnapshot(saveGameMagic, file)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeSnapshot(raw, saveGameMagic)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(file.Game.Ceilings, got.Game.Ceilings) {
				t.Fatal("binary snapshot lost ceiling finish")
			}
			g.ceilings = restoreCeilingThinkers(got.Game.Ceilings)
			g.m.Sectors[1].Special = 96
			for i := 0; i < 33; i++ {
				if ct := g.ceilings[1]; ct != nil {
					g.tickCeiling(1, ct)
				}
			}
			want := int16(0)
			if change == 2 {
				want = 96
			}
			if change == 3 {
				want = 32
			}
			if g.ceilings[1] != nil || g.m.Sectors[1].CeilingPic != "MODEL" || g.m.Sectors[1].Special != want {
				t.Fatalf("numeric=%v change=%d sector=%+v", numeric, change, g.m.Sectors[1])
			}
		}
	}
}
func TestBoomGeneralizedCeilingIndependentSlotAndCrush(t *testing.T) {
	g := boomCeilingGame(0x4006 | 4096 | 4<<7)
	g.m.Linedefs[0].Tag = 0
	g.floors = map[int]*floorThinker{1: {sector: 1}}
	if !g.useSpecialLineForActor(0, 0, true) || g.ceilings[1] == nil || !g.ceilings[1].crush || g.lineSpecial[0] != 0 {
		t.Fatal("manual ceiling failed alongside floor")
	}
	g = boomCeilingGame(0x4002 | 32 | 3<<10 | 6<<7)
	g.useSpecialLineForActor(0, 0, true)
	if g.ceilings[1].finish != floorFinishNone {
		t.Fatal("nonexistent numeric model selected")
	}
}
func TestBoomGeneralizedLiftTargetsSpeedsAndWaits(t *testing.T) {
	menu, play := doomrand.State()
	defer doomrand.SetState(menu, play)
	for target := uint16(0); target < 4; target++ {
		for speed := uint16(0); speed < 4; speed++ {
			for delay := uint16(0); delay < 4; delay++ {
				special := uint16(0x3402) | target<<8 | delay<<6 | speed<<3
				g := boomFloorGame(special)
				g.ceilings = map[int]*ceilingThinker{1: {sector: 1}}
				if !g.useSpecialLineForActor(0, 0, true) {
					t.Fatalf("%x failed beside ceiling", special)
				}
				pt := g.plats[1]
				low := int64(0)
				if target == 2 {
					low = 32
				}
				if pt == nil || pt.low != low*fracUnit || pt.high != 32*fracUnit || pt.speed != 2*fracUnit<<speed || pt.wait != [...]int{35, 105, 175, 350}[delay] {
					t.Fatalf("%x mover=%+v", special, pt)
				}
			}
		}
	}
}
func TestBoomGeneralizedLiftFullCycleAndStasis(t *testing.T) {
	g := boomFloorGame(0x3406)
	g.m.Linedefs[0].Tag = 0
	if !g.useSpecialLineForActor(0, 0, true) {
		t.Fatal("manual lift failed")
	}
	for i := 0; i < 17; i++ {
		g.tickPlat(1, g.plats[1])
	}
	pt := g.plats[1]
	if pt.status != platStatusWaiting || pt.count != 35 || g.sectorFloor[1] != 0 {
		t.Fatal("lift bottom/wait timing")
	}
	// A stopped lift stays frozen, including across serialization/restoration.
	g.m.Sectors[1].Tag = 7
	if g.stopTaggedPlats(7) || !g.stopTaggedPlats(0) {
		t.Fatal("manual lift lost its trigger tag")
	}
	state := capturePlatThinkers(g.plats)
	raw, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: gameSaveState{Plats: state}})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(raw, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	g.plats = restorePlatThinkers(file.Game.Plats)
	pt = g.plats[1]
	g.tickPlat(1, pt)
	if pt.status != platStatusInStasis || pt.count != 35 {
		t.Fatal("stopped lift advanced")
	}
	g.m.Linedefs[0].Tag = 0
	gen := mapdata.LookupLineSpecial(0x3700).Plat
	if g.activateGeneralizedLift(0, *gen) || pt.status != platStatusWaiting {
		t.Fatal("perpetual trigger did not resume stasis or reported false new activation")
	}
	for i := 0; i < 35; i++ {
		g.tickPlat(1, pt)
	}
	if pt.status != platStatusUp || g.sectorFloor[1] != 0 {
		t.Fatal("lift returned before delay elapsed")
	}
	for i := 0; i < 17; i++ {
		if g.plats[1] != nil {
			g.tickPlat(1, pt)
		}
	}
	if g.plats[1] != nil || g.sectorFloor[1] != 32*fracUnit {
		t.Fatal("returning lift did not finish")
	}
}
func TestBoomCeilingSnapshotOlderVersions(t *testing.T) {
	for _, version := range []int{26, 27, 28, 29, 13, 14, 15, 16} {
		file := saveFile{Version: version, Game: gameSaveState{Ceilings: map[int]ceilingThinkerSaveState{1: {Sector: 1, Finish: floorFinishSetTexture, FinishFlat: "MODEL", FinishSpecial: 32}}, WeaponPose: weaponPresentationPose{Valid: true}, Inventory: playerInventorySaveState{SkullKeys: [3]bool{true}}}}
		raw, err := encodeSnapshot(saveGameMagic, file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeSnapshot(raw, saveGameMagic)
		if err != nil {
			t.Fatalf("version %d: %v", version, err)
		}
		current := version == 29 || version == 16
		if (got.Game.Ceilings[1].Finish != floorFinishNone) != current {
			t.Fatalf("version %d finish migration", version)
		}
		var buf bytes.Buffer
		if err := (saveBinaryWriter{buf: &buf}).saveFile(got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf.Bytes(), raw[len(saveGameMagic):len(raw)-snapshotChecksumSize]) {
			t.Fatalf("version %d failed stable round trip", version)
		}
	}
}

func TestBoomGeneralizedLockedDoorGatesTriggers(t *testing.T) {
	// Blue card/skull equivalent, fast S1 open/stay.
	special := uint16(0x3a00 | 2<<6 | 1<<5 | 2<<3 | 2)
	for _, trigger := range []uint16{2, 4, 6} {
		special = special&^7 | trigger
		g := boomCeilingGame(special)
		g.sectorCeil[1] = 32 * fracUnit
		g.m.Linedefs[0].Tag = 7
		info := mapdata.LookupLineSpecial(special)
		activate := func() bool {
			if trigger == 4 {
				return g.activateShootLineSpecial(0, info)
			}
			return g.useSpecialLineForActor(0, 0, true)
		}
		if activate() || g.activeDoorThinker(1) != nil || g.lineSpecial[0] != special {
			t.Fatal("locked door activated without key or consumed trigger")
		}
		g.inventory.BlueKey = true
		if g.useSpecialLineForActor(0, 0, false) {
			t.Fatal("monster activated locked door")
		}
		if !activate() {
			t.Fatalf("keyed trigger %d did not activate", trigger)
		}
		d := g.activeDoorThinker(1)
		if d == nil || d.speed != 8*fracUnit || d.topWait != 140 {
			t.Fatalf("locked door speed/wait: %+v", d)
		}
		direction := d.direction
		if g.activateGeneralizedDoor(0, info, true) || d.direction != direction {
			t.Fatal("busy locked door retriggered")
		}
	}
}

func TestBoomVoodooSlideRetainsRealPlayerCollisionBody(t *testing.T) {
	g := boomConveyorGame()
	before := g.p
	// An unobstructed slide fallback must move the doll, rather than testing
	// it against the temporary copy used for tracing its own slide shape.
	g.thingMomY[0] = fracUnit
	g.slideVoodooDoll(0, g.m.Things[0])
	if g.thingY[0] != fracUnit {
		t.Fatalf("doll collided with its slide state: %d", g.thingY[0])
	}
	if !reflect.DeepEqual(before, g.p) {
		t.Fatal("doll slide changed real player state")
	}
}
func TestBoomCeilingBlockedVersusCrushing(t *testing.T) {
	for _, crush := range []bool{false, true} {
		g := boomConveyorGame()
		g.setSectorCeilingHeight(0, 64*fracUnit)
		special := uint16(0x4002 | 4<<7)
		if crush {
			special |= 4096
		}
		g.m.Linedefs[0].Special = special
		g.lineSpecial[0] = special
		if !g.activateCeilingLine(0, *mapdata.LookupLineSpecial(special).Ceiling) {
			t.Fatal("ceiling activation failed")
		}
		ct := g.ceilings[0]
		for tic := 1; tic <= 24; tic++ {
			g.worldTic = tic
			g.tickCeiling(0, ct)
		}
		if ct.speed != fracUnit {
			t.Fatal("plain generalized ceiling incorrectly slowed like crusher")
		}
		if crush {
			if g.stats.Health >= 100 || g.sectorCeil[0] >= 56*fracUnit {
				t.Fatal("crushing ceiling failed to damage/move")
			}
		} else if g.stats.Health != 100 || g.sectorCeil[0] != 56*fracUnit {
			t.Fatal("noncrushing ceiling passed through actor")
		}
	}
}
func TestBoomGeneralizedLiftReversesWhenBlocked(t *testing.T) {
	g := boomConveyorGame()
	g.setSectorCeilingHeight(0, 64*fracUnit)
	pt := &platThinker{sector: 0, typ: platTypeDownWaitUpStay, status: platStatusUp, low: 0, high: 32 * fracUnit, speed: 16 * fracUnit, wait: 35}
	g.plats = map[int]*platThinker{0: pt}
	g.tickPlat(0, pt)
	if g.sectorFloor[0] != 0 || pt.status != platStatusDown || pt.count != 35 {
		t.Fatal("blocked lift did not reverse and restore floor")
	}
}
func TestBoomLockedDoorUsesIndependentCeilingSlot(t *testing.T) {
	g := boomCeilingGame(0x3a02 | 2<<6)
	g.inventory.BlueKey = true
	g.floors = map[int]*floorThinker{1: {sector: 1}}
	if !g.useSpecialLineForActor(0, 0, true) || g.activeDoorThinker(1) == nil {
		t.Fatal("generalized door incorrectly blocked by floor mover")
	}
}
