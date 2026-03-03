package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestUseSpecialLineLockedWithoutKeyAndUnlocksWithPickup(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Things: []mapdata.Thing{{X: 0, Y: 0, Type: 5}}},
		lineSpecial: []uint16{26}, // blue key manual door
		soundQueue:  make([]soundEvent, 0, 4),
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))

	g.useSpecialLine(0, 0)
	if g.useText != "USE: locked" {
		t.Fatalf("useText=%q want locked", g.useText)
	}

	g.processThingPickups()
	if !g.inventory.BlueKey {
		t.Fatal("blue key should be picked up")
	}
}

func TestProcessThingPickupsMarksCollectedAndUpdatesStats(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 0, Y: 0, Type: 2011},    // stimpack
				{X: 0, Y: 0, Type: 2007},    // clip
				{X: 0, Y: 0, Type: 2018},    // green armor
				{X: 0, Y: 0, Type: 2001},    // shotgun
				{X: 0, Y: 0, Type: 2048},    // box bullets
				{X: 9999, Y: 9999, Type: 5}, // far key, should not pick up
			},
		},
		soundQueue: make([]soundEvent, 0, 8),
	}
	g.initPlayerState()
	g.stats.Health = 80
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if g.stats.Health <= 80 {
		t.Fatalf("health=%d, expected increased", g.stats.Health)
	}
	if g.stats.Armor < 100 {
		t.Fatalf("armor=%d, expected green armor", g.stats.Armor)
	}
	if g.stats.Bullets <= 50 {
		t.Fatalf("bullets=%d, expected increased", g.stats.Bullets)
	}
	if !g.inventory.Weapons[2001] {
		t.Fatal("shotgun should be owned")
	}
	if g.inventory.BlueKey {
		t.Fatal("far blue key should not be collected")
	}
	if !g.thingCollected[0] || !g.thingCollected[1] || !g.thingCollected[2] || !g.thingCollected[3] || !g.thingCollected[4] {
		t.Fatal("near pickups should be marked collected")
	}
	if g.thingCollected[5] {
		t.Fatal("far pickup should remain uncollected")
	}
}

func TestBackpackDoublesAmmoCap(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	g.stats.Bullets = 200
	if g.gainAmmoNoMsg("bullets", 10) {
		t.Fatal("without backpack, bullets at cap should not increase")
	}
	g.inventory.Backpack = true
	if !g.gainAmmoNoMsg("bullets", 10) {
		t.Fatal("with backpack, bullets cap should be higher")
	}
	if g.stats.Bullets != 210 {
		t.Fatalf("bullets=%d want=210", g.stats.Bullets)
	}
}
