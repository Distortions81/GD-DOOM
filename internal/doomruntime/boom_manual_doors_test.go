package doomruntime

import (
	"gddoom/internal/mapdata"
	"reflect"
	"testing"
)

func sharedDoorGame(special uint16) *game {
	g := newDoorTimingGame(0)
	g.boomMomentumDropoff = true
	g.m.Linedefs[0].Special = special
	g.m.Linedefs[0].Tag = 7
	g.m.Sectors[0].Tag = 7
	g.m.Sectors[0].CeilingHeight = 96
	g.m.Sectors[1].CeilingHeight = 128
	g.sectorCeil = []int64{96 * fracUnit, 128 * fracUnit}
	g.lineSpecial = []uint16{special}
	return g
}
func stepSharedDoorGame(g *game) {
	g.prunePendingDoors()
	for order := int64(0); ; {
		ref, ok := g.nextWorldThinkerAfter(order)
		if !ok {
			return
		}
		order = ref.order
		switch ref.kind {
		case worldThinkerDoor, worldThinkerCeiling, worldThinkerFloor, worldThinkerPlat:
			g.tickWorldThinker(ref)
		}
	}
}
func compareSharedDoorState(t *testing.T, a, b *game) {
	t.Helper()
	if !reflect.DeepEqual(a.sectorCeil, b.sectorCeil) || !reflect.DeepEqual(a.ceilingOwners, b.ceilingOwners) || !reflect.DeepEqual(captureDoorThinkers(a.doors), captureDoorThinkers(b.doors)) || !reflect.DeepEqual(captureExtraDoors(a.extraDoors), captureExtraDoors(b.extraDoors)) || !reflect.DeepEqual(captureCeilingThinkers(a.ceilings), captureCeilingThinkers(b.ceilings)) {
		t.Fatal("overlapping mover state diverged after checkpoint")
	}
}
func TestBoomManualDoorLeavesFloorMoverAndVanillaCast(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := sharedDoorGame(1)
		g.boomMomentumDropoff = boom
		g.plats = map[int]*platThinker{0: {order: 1, sector: 0, status: platStatusWaiting, wait: 105, count: 20}}
		g.nextThinkerOrder = 2
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("manual door line was rejected")
		}
		if boom {
			if g.activeDoorThinker(0) == nil || g.plats[0].wait != 105 {
				t.Fatal("Boom door cast or blocked the floor-only lift")
			}
		} else {
			if g.activeDoorThinker(0) != nil || g.plats[0].wait != -1 {
				t.Fatal("vanilla platform cast was not preserved")
			}
		}
	}
}
func TestBoomGeneralizedManualDoorRejectsBusyDoor(t *testing.T) {
	g := sharedDoorGame(0x3c07)
	if !g.useSpecialLineForActor(0, 0, true) {
		t.Fatal("generalized door did not start")
	}
	d := g.activeDoorThinker(0)
	if g.useSpecialLineForActor(0, 0, true) || d.direction != 1 || g.lineSpecial[0] != 0x3c07 {
		t.Fatal("generalized door retriggered while ceiling slot was occupied")
	}
}
func TestBoomDoorRecyclingClearsCountdown(t *testing.T) {
	g := sharedDoorGame(117)
	d := g.allocDoorThinker(0)
	d.topCountdown = 139
	g.doors = map[int]*doorThinker{0: d}
	g.removeDoorThinker(0)
	if !g.useSpecialLineForActor(0, 0, true) || g.doors[0].topCountdown != 0 {
		t.Fatal("reused Boom door retained a stale countdown")
	}
}
func TestBoomDisplacedCrusherAndEmptySlotCheckpoint(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion} {
		g := sharedDoorGame(31)
		g.sectorCeil[1] = 64 * fracUnit
		if !g.activateGeneralizedCrusher(0, *mapdata.LookupLineSpecial(0x2f93).Ceiling) {
			t.Fatal("crusher did not start")
		}
		old := g.ceilings[0]
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("D1 did not replace ceiling ownership")
		}
		stepSharedDoorGame(g)
		if g.ceilingSlotBusy(0) || g.ceilings[0] != old {
			t.Fatal("finished D1 did not leave the old crusher live with an empty slot")
		}
		if !g.activateGeneralizedCrusher(0, *mapdata.LookupLineSpecial(0x2f93).Ceiling) || len(g.ceilings) != 2 {
			t.Fatal("new crusher lost the displaced thinker")
		}
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		if !validManualDoorState(file.Game, 2) {
			t.Fatal("valid overlapping snapshot rejected")
		}
		loaded := sharedDoorGame(31)
		restoreGameSaveState(loaded, file.Game)
		compareSharedDoorState(t, g, loaded)
		for range 120 {
			stepSharedDoorGame(g)
			stepSharedDoorGame(loaded)
			compareSharedDoorState(t, g, loaded)
		}
	}
}
func TestBoomDisplacedDoorCheckpoint(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion} {
		g := sharedDoorGame(31)
		g.activateDoorSectors([]int{0}, mapdata.DoorRaise)
		if !g.useSpecialLineForActor(0, 0, true) || len(g.extraDoors) != 1 {
			t.Fatal("D1 discarded an existing door")
		}
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		loaded := sharedDoorGame(31)
		restoreGameSaveState(loaded, file.Game)
		for range 80 {
			stepSharedDoorGame(g)
			stepSharedDoorGame(loaded)
			compareSharedDoorState(t, g, loaded)
		}
		if loaded.ceilingSlotBusy(0) || len(captureExtraDoors(loaded.extraDoors)) != 1 {
			t.Fatal("displaced waiting door incorrectly reclaimed ceiling ownership")
		}
	}
}
func TestBoomManualOwnerValidationAndCanonicalExtraDoors(t *testing.T) {
	if validManualDoorState(gameSaveState{CeilingOwners: map[int]int64{2: 0}}, 2) || validManualDoorState(gameSaveState{CeilingOwners: map[int]int64{0: 3}}, 2) {
		t.Fatal("invalid ceiling ownership accepted")
	}
	active := &doorThinker{order: 4, sector: 0}
	saved := captureExtraDoors([]*doorThinker{{order: 2, sector: 0, pendingRemove: true}, active})
	if !reflect.DeepEqual(saved, captureExtraDoors(restoreExtraDoors(saved))) {
		t.Fatal("extra-door snapshot was not canonical")
	}
}
func TestBoomPreviousGeneralizedStateExtension(t *testing.T) {
	for _, version := range []int{34, 21} {
		g := sharedDoorGame(31)
		g.p.corpseGibbed = true
		g.activateGeneralizedCrusher(0, *mapdata.LookupLineSpecial(0x2f8b).Ceiling)
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		if !file.Game.Player.CorpseGibbed || file.Game.Ceilings[0].OldSpeed != 2*fracUnit {
			t.Fatal("previous generalized snapshot extension changed layout")
		}
	}
}

func TestAuthorityManualDoorOverlapSurvivesApply(t *testing.T) {
	a, client, _ := snapshotFixture(t)
	old := &doorThinker{order: a.g.allocThinkerOrder(), sector: 0, typ: doorNormal, direction: 1, speed: 2 * fracUnit, topHeight: 256 * fracUnit, topWait: 150}
	current := &doorThinker{order: a.g.allocThinkerOrder(), sector: 0, typ: doorOpen, direction: 1, speed: 2 * fracUnit, topHeight: 256 * fracUnit, topWait: 150}
	a.g.doors = map[int]*doorThinker{0: current}
	a.g.extraDoors = []*doorThinker{old}
	a.g.ceilingOwners = map[int]int64{0: current.order}
	data, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.applyAuthoritySnapshot(data); err != nil {
		t.Fatal(err)
	}
	if len(client.extraDoors) != 1 || client.extraDoors[0].order != old.order || client.ceilingOwners[0] != current.order {
		t.Fatal("authority apply discarded displaced mover or slot identity")
	}
}
