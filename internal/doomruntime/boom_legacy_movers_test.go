package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/mapdata"
)

func TestBoomElevatorReservesBothSlotsAndSurvivesSnapshot(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion} {
		g := boomCeilingGame(231)
		g.boomMomentumDropoff = true
		if !g.activateElevator(0, mapdata.ElevatorDown) {
			t.Fatal("elevator did not start")
		}
		if g.activateElevator(0, mapdata.ElevatorUp) || g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorLowerToNearest, UsesTag: true}) || g.activateCeilingLine(0, mapdata.CeilingInfo{Action: mapdata.CeilingLowerToFloor, UsesTag: true}) {
			t.Fatal("elevator did not reserve both plane slots")
		}
		if g.activateGeneralizedFloor(0, mapdata.FloorInfo{UsesTag: true, Generalized: &mapdata.GeneralizedFloorInfo{Up: true}}) || g.activateGeneralizedCeiling(0, mapdata.CeilingInfo{UsesTag: true, Generalized: &mapdata.GeneralizedFloorInfo{Up: true}}) || g.activateDoorSectorsWithFloorSlot([]int{1}, mapdata.DoorOpen, true) {
			t.Fatal("generalized mover or door bypassed elevator slot")
		}
		for range 3 {
			g.tickFloors()
		}
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		loaded := boomCeilingGame(231)
		loaded.boomMomentumDropoff = true
		restoreGameSaveState(loaded, file.Game)
		if !reflect.DeepEqual(g.floors, loaded.floors) {
			t.Fatalf("elevator checkpoint differs: %+v / %+v", g.floors[1], loaded.floors[1])
		}
		for range 12 {
			g.tickFloors()
			loaded.tickFloors()
			if !reflect.DeepEqual(g.sectorFloor, loaded.sectorFloor) || !reflect.DeepEqual(g.sectorCeil, loaded.sectorCeil) || !reflect.DeepEqual(g.floors, loaded.floors) {
				t.Fatal("elevator diverged after load")
			}
		}
		if loaded.floorSlotBusy(1) || loaded.ceilingSlotBusy(1) {
			t.Fatal("finished elevator left a reserved slot")
		}
	}
}

func TestBoomLegacyPlaneSlotsPreserveVanilla(t *testing.T) {
	for _, boom := range []bool{false, true} {
		g := boomCeilingGame(219)
		g.boomMomentumDropoff = boom
		if !g.activateCeilingLine(0, mapdata.CeilingInfo{Action: mapdata.CeilingLowerToFloor, UsesTag: true}) {
			t.Fatal("ceiling did not start")
		}
		if got := g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorLowerToNearest, UsesTag: true}); got != boom {
			t.Fatalf("parallel floor allowed=%v Boom=%v", got, boom)
		}
		g = boomCeilingGame(219)
		g.boomMomentumDropoff = boom
		g.activateFloorLine(0, mapdata.FloorInfo{Action: mapdata.FloorLowerToNearest, UsesTag: true})
		if got := g.activateDoorSectors([]int{1}, mapdata.DoorOpen); got != boom {
			t.Fatalf("parallel door allowed=%v Boom=%v", got, boom)
		}
	}
}

func TestBoomToggleStasisCheckpointAndReverse(t *testing.T) {
	g := boomCeilingGame(211)
	g.boomMomentumDropoff = true
	if !g.activatePlatLine(0, *mapdata.LookupLineSpecial(211).Plat) {
		t.Fatal("toggle did not start")
	}
	initial := g.sectorFloor[1]
	g.tickPlats()
	if g.sectorFloor[1] != g.sectorCeil[1] || g.plats[1].status != platStatusInStasis {
		t.Fatal("toggle did not stop at ceiling in stasis")
	}
	state := captureGameSaveState(g)
	data, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: state})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	loaded := boomCeilingGame(211)
	loaded.boomMomentumDropoff = true
	restoreGameSaveState(loaded, file.Game)
	if !loaded.activatePlatLine(0, *mapdata.LookupLineSpecial(211).Plat) {
		t.Fatal("saved toggle did not reactivate")
	}
	loaded.tickPlats()
	if loaded.sectorFloor[1] != initial || loaded.plats[1].status != platStatusInStasis {
		t.Fatal("saved toggle did not reverse to original floor")
	}
}

func TestBoomFloorCrushCheckpointAndPreviousCardExtension(t *testing.T) {
	g := boomCeilingGame(257)
	g.boomMomentumDropoff = true
	g.activateStairsLine(0, *mapdata.LookupLineSpecial(257).Stairs)
	for _, version := range []int{saveGameVersion, keyframeVersion, 32, 19} {
		g.inventory = lockInventory(63)
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		if restorePlayerInventorySaveState(file.Game.Inventory).keys() != g.inventory.keys() {
			t.Fatal("previous card extension lost")
		}
		if version == saveGameVersion || version == keyframeVersion {
			if !file.Game.Floors[1].Crush {
				t.Fatal("turbo stairs lost crush state")
			}
		}
	}
}

func TestBoomNumericChangeOnlyMapEnablesBoomRules(t *testing.T) {
	m := &mapdata.Map{Linedefs: []mapdata.Linedef{{Special: 78}}}
	if !boomMomentumDropoffEnabled(m, Options{}) {
		t.Fatal("special 78 alone did not enable Boom map rules")
	}
	if boomMomentumDropoffEnabled(m, Options{DemoScript: &DemoScript{Header: DemoHeader{Version: 109}}}) {
		t.Fatal("vanilla demo rules were overridden by a Boom map special")
	}
}
