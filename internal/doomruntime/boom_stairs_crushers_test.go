package doomruntime

import (
	"gddoom/internal/mapdata"
	"reflect"
	"testing"
)

func generalizedStairGame(special uint16) *game {
	m := &mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 256, FloorPic: "SAME", Tag: 7}, {CeilingHeight: 256, FloorPic: "SAME"}, {CeilingHeight: 256, FloorPic: "SAME"}}, Sidedefs: []mapdata.Sidedef{{Sector: 0}, {Sector: 1}, {Sector: 2}}, Linedefs: []mapdata.Linedef{{Flags: mlTwoSided, SideNum: [2]int32{0, 1}, Special: special, Tag: 7}, {Flags: mlTwoSided, SideNum: [2]int32{1, 2}}}}
	return &game{m: m, boomMomentumDropoff: true, sectorFloor: []int64{0, 0, 0}, sectorCeil: []int64{256 * fracUnit, 256 * fracUnit, 256 * fracUnit}, lineSpecial: []uint16{special, 0}}
}
func orderedSectorTick(g *game) {
	for order := int64(0); ; {
		ref, ok := g.nextWorldThinkerAfter(order)
		if !ok {
			return
		}
		order = ref.order
		switch ref.kind {
		case worldThinkerFloor, worldThinkerCeiling, worldThinkerPlat:
			g.tickWorldThinker(ref)
		}
	}
}
func TestGeneralizedStairChainLockCheckpoint(t *testing.T) {
	// Slow four-unit ascending SR stairs: first floor finishes before the chain.
	for _, version := range []int{saveGameVersion, keyframeVersion} {
		special := uint16(0x3103)
		g := generalizedStairGame(special)
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("stairs did not activate")
		}
		for range 17 {
			orderedSectorTick(g)
		}
		if g.floors[0] != nil || g.boomStairs[0].Lock != -1 || g.boomStairs[2].Lock != -2 {
			t.Fatal("completed step did not remain chain-locked")
		}
		if g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("partially complete staircase retriggered")
		}
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		loaded := generalizedStairGame(special)
		restoreGameSaveState(loaded, file.Game)
		for range 40 {
			orderedSectorTick(g)
			orderedSectorTick(loaded)
			if !reflect.DeepEqual(g.boomStairs, loaded.boomStairs) || !reflect.DeepEqual(g.sectorFloor, loaded.sectorFloor) {
				t.Fatal("stair checkpoint diverged")
			}
		}
		for _, s := range loaded.boomStairs {
			if s.Lock != 0 {
				t.Fatal("finished staircase stayed locked")
			}
		}
		if !loaded.useSpecialLineForActor(0, 0, true) || loaded.floors[0].direction != -1 {
			t.Fatal("finished staircase did not reverse on repeat activation")
		}
	}
}
func TestGeneralizedStairsRespectTextureAndBusyStep(t *testing.T) {
	for _, ignore := range []bool{false, true} {
		special := uint16(0x3103)
		if ignore {
			special |= 512
		}
		g := generalizedStairGame(special)
		g.m.Sectors[1].FloorPic = "OTHER"
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("first step rejected")
		}
		if (g.floors[2] != nil) != ignore {
			t.Fatal("ignore texture bit did not control chain continuation")
		}
	}
	g := generalizedStairGame(0x3103)
	g.floors = map[int]*floorThinker{1: {direction: 1, speed: fracUnit, destHeight: 128 * fracUnit}}
	g.useSpecialLineForActor(0, 0, true)
	if g.boomStairs[1].Lock != 0 || g.boomStairs[0].Next != -1 {
		t.Fatal("busy neighbor joined a stair chain")
	}
}
func TestGeneralizedCrusherStasisCheckpointAndOriginalSpeed(t *testing.T) {
	for _, version := range []int{saveGameVersion, keyframeVersion} {
		g := boomCeilingGame(0x2f8b)
		g.boomMomentumDropoff = true
		if !g.useSpecialLineForActor(0, 0, true) {
			t.Fatal("crusher did not activate")
		}
		ct := g.ceilings[1]
		ct.speed = fracUnit / 8
		if !g.stopBoomCeilings(7) {
			t.Fatal("crusher did not stop")
		}
		g.m.Sectors[1].Tag = 9
		data, err := encodeSnapshot(saveGameMagic, saveFile{Version: version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		file, err := decodeSnapshot(data, saveGameMagic)
		if err != nil {
			t.Fatal(err)
		}
		loaded := boomCeilingGame(0x2f8b)
		loaded.boomMomentumDropoff = true
		restoreGameSaveState(loaded, file.Game)
		if !loaded.activateGeneralizedCrusher(0, *mapdata.LookupLineSpecial(0x2f8b).Ceiling) {
			t.Fatal("saved crusher did not resume by original tag")
		}
		ct = loaded.ceilings[1]
		if ct.direction != -1 || ct.oldSpeed != 2*fracUnit || ct.speed != fracUnit/8 {
			t.Fatal("resumed crusher lost slowdown or original speed")
		}
		loaded.sectorCeil[1] = ct.bottomHeight
		loaded.tickCeiling(1, ct)
		if ct.direction != 1 || ct.speed != 2*fracUnit {
			t.Fatal("slow crusher did not restore original speed at bottom")
		}
	}
}
func TestPlayerCrushedCorpseCheckpoint(t *testing.T) {
	g := boomCeilingGame(0x2fb3)
	g.p.corpseGibbed = true
	g.isDead = true
	data, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: captureGameSaveState(g)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(data, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	loaded := boomCeilingGame(0x2fb3)
	restoreGameSaveState(loaded, file.Game)
	if !loaded.p.corpseGibbed || loaded.playerMobjHeight() != 0 {
		t.Fatal("flattened player corpse state was lost")
	}
	loaded.initPlayerState()
	if loaded.p.corpseGibbed {
		t.Fatal("reborn player retained flattened collider")
	}
}
func TestBoomStairStateShapeValidation(t *testing.T) {
	if validBoomStairs([]boomStairState{{Prev: -1, Next: -1}}, 2) || validBoomStairs([]boomStairState{{Prev: -1, Next: 1}}, 1) || validBoomStairs([]boomStairState{{Lock: -3, Prev: -1, Next: -1}}, 1) {
		t.Fatal("invalid stair links accepted")
	}
}
