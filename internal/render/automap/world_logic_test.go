package automap

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestHazardDamagePeriodicSpecial5(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 5}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
		soundQueue:  make([]soundEvent, 0, 4),
	}
	for i := 0; i < 31; i++ {
		g.tickWorldLogic()
	}
	if g.stats.Health != 100 {
		t.Fatalf("health before period=%d want=100", g.stats.Health)
	}
	g.tickWorldLogic()
	if g.stats.Health != 90 {
		t.Fatalf("health after period=%d want=90", g.stats.Health)
	}
}

func TestHazardDamageBlockedByRadSuit(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 5}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
		inventory:   playerInventory{RadSuitTics: 40},
	}
	for i := 0; i < 32; i++ {
		g.tickWorldLogic()
	}
	if g.stats.Health != 100 {
		t.Fatalf("health with suit=%d want=100", g.stats.Health)
	}
	if g.inventory.RadSuitTics != 8 {
		t.Fatalf("radsuit tics=%d want=8", g.inventory.RadSuitTics)
	}
}

func TestHazardDamageSpecial16WithoutSuit(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m:           &mapdata.Map{Sectors: []mapdata.Sector{{Special: 16}}},
		sectorFloor: []int64{0},
		p:           player{x: 0, y: 0, z: 0, floorz: 0},
		stats:       playerStats{Health: 100},
	}
	for i := 0; i < 32; i++ {
		g.tickWorldLogic()
	}
	if g.stats.Health != 80 {
		t.Fatalf("health=%d want=80", g.stats.Health)
	}
}

func TestPickupRadSuitSetsTimer(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	msg, _, ok := g.applyPickup(2025)
	if !ok {
		t.Fatal("radsuit pickup should succeed")
	}
	if msg == "" {
		t.Fatal("radsuit pickup should return a message")
	}
	if g.inventory.RadSuitTics != 60*doomTicsPerSecond {
		t.Fatalf("radsuit tics=%d want=%d", g.inventory.RadSuitTics, 60*doomTicsPerSecond)
	}
}
