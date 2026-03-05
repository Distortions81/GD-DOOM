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

func TestDeadPlayerDoesNotPickup(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2011}},
		},
		isDead: true,
	}
	g.initPlayerState()
	g.stats.Health = 50
	g.thingCollected = make([]bool, len(g.m.Things))
	g.updatePlayer(moveCmd{})
	if g.stats.Health != 50 {
		t.Fatalf("dead player health changed to %d", g.stats.Health)
	}
	if g.thingCollected[0] {
		t.Fatal("dead player should not collect pickups")
	}
}

func TestCanTouchPickup_DoomStyleBounds(t *testing.T) {
	px, py, pz := int64(0), int64(0), int64(0)
	tx, ty, tz := int64(35*fracUnit), int64(0), int64(0)
	if !canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, tz, 20*fracUnit, 16*fracUnit) {
		t.Fatal("expected touch within blockdist")
	}
	tx = 37 * fracUnit
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, tz, 20*fracUnit, 16*fracUnit) {
		t.Fatal("expected no touch beyond blockdist")
	}
}

func TestCanTouchPickup_ZOverlap(t *testing.T) {
	px, py, pz := int64(0), int64(0), int64(0)
	tx, ty := int64(0), int64(0)
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, 57*fracUnit, 20*fracUnit, 16*fracUnit) {
		t.Fatal("thing above player height should not touch")
	}
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, -25*fracUnit, 20*fracUnit, 16*fracUnit) {
		t.Fatal("thing too far below should not touch")
	}
	if !canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, -20*fracUnit, 20*fracUnit, 16*fracUnit) {
		t.Fatal("thing in lower overlap range should touch")
	}
}

func TestWeaponPickupRespectsAutoSwitchToggle(t *testing.T) {
	baseMap := &mapdata.Map{
		Things: []mapdata.Thing{
			{X: 0, Y: 0, Type: 2001}, // shotgun
		},
	}
	g := &game{
		m:                baseMap,
		autoWeaponSwitch: false,
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.inventory.ReadyWeapon = weaponPistol

	g.processThingPickups()
	if !g.inventory.Weapons[2001] {
		t.Fatal("shotgun should be owned")
	}
	if g.inventory.ReadyWeapon != weaponPistol {
		t.Fatalf("weapon=%v want=%v when auto switch disabled", g.inventory.ReadyWeapon, weaponPistol)
	}
}

func TestWeaponPickupAutoSwitchesWhenEnabled(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 0, Y: 0, Type: 2001}, // shotgun
			},
		},
		autoWeaponSwitch: true,
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.inventory.ReadyWeapon = weaponPistol

	g.processThingPickups()
	if g.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("weapon=%v want=%v when auto switch enabled", g.inventory.ReadyWeapon, weaponShotgun)
	}
}
