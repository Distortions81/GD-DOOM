package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestUseSpecialLineLockedWithoutKeyAndUnlocksWithPickup(t *testing.T) {
	g := &game{
		m:           &mapdata.Map{Things: []mapdata.Thing{{X: 0, Y: 0, Type: 5}}},
		lineSpecial: []uint16{26}, // blue key manual door
		soundQueue:  make([]soundEvent, 0, 4),
		opts:        Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.m.Things[0].Flags = skillMediumBits
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))

	g.useSpecialLine(0, 0)
	if g.useText != "USE: locked" {
		t.Fatalf("useText=%q want locked", g.useText)
	}
	if got := len(g.soundQueue); got != 1 || g.soundQueue[0] != soundEventOof {
		t.Fatalf("soundQueue=%v want [%v]", g.soundQueue, soundEventOof)
	}

	g.processThingPickups()
	if !g.inventory.BlueKey {
		t.Fatal("blue key should be picked up")
	}
}

func TestProcessThingPickups_ConsumesKeyEvenIfAlreadyOwned(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 5, Flags: skillMediumBits}},
		},
		soundQueue: make([]soundEvent, 0, 2),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.inventory.BlueKey = true
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if !g.thingCollected[0] {
		t.Fatal("duplicate blue key should still be collected")
	}
	if got := len(g.soundQueue); got != 1 || g.soundQueue[0] != soundEventItemUp {
		t.Fatalf("soundQueue=%v want [%v]", g.soundQueue, soundEventItemUp)
	}
	if g.useText != "" {
		t.Fatalf("useText=%q want empty", g.useText)
	}
}

func TestProcessThingPickups_ConsumesKeyGrantedByCheat(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 13, Flags: skillMediumBits}},
		},
		soundQueue: make([]soundEvent, 0, 2),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.inventory.BlueKey = true
	g.inventory.RedKey = true
	g.inventory.YellowKey = true
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if !g.thingCollected[0] {
		t.Fatal("cheat-granted red key should still be collected from the map")
	}
	if got := len(g.soundQueue); got != 1 || g.soundQueue[0] != soundEventItemUp {
		t.Fatalf("soundQueue=%v want [%v]", g.soundQueue, soundEventItemUp)
	}
}

func TestApplyPickup_DoomSoundMappingsForSpecials(t *testing.T) {
	g := &game{}
	g.initPlayerState()
	tests := []struct {
		typ  int16
		want soundEvent
	}{
		{typ: 5, want: soundEventItemUp},
		{typ: 13, want: soundEventItemUp},
		{typ: 6, want: soundEventItemUp},
		{typ: 2013, want: soundEventPowerUp},
		{typ: 2018, want: soundEventItemUp},
		{typ: 2019, want: soundEventItemUp},
	}
	for _, tc := range tests {
		_, got, ok := g.applyPickup(tc.typ, false)
		if !ok {
			t.Fatalf("applyPickup(%d) not applied", tc.typ)
		}
		if got != tc.want {
			t.Fatalf("applyPickup(%d) sound=%v want=%v", tc.typ, got, tc.want)
		}
	}
}

func TestProcessThingPickupsMarksCollectedAndUpdatesStats(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 0, Y: 0, Type: 2011, Flags: skillMediumBits},    // stimpack
				{X: 0, Y: 0, Type: 2007, Flags: skillMediumBits},    // clip
				{X: 0, Y: 0, Type: 2018, Flags: skillMediumBits},    // green armor
				{X: 0, Y: 0, Type: 2001, Flags: skillMediumBits},    // shotgun
				{X: 0, Y: 0, Type: 2048, Flags: skillMediumBits},    // box bullets
				{X: 9999, Y: 9999, Type: 5, Flags: skillMediumBits}, // far key, should not pick up
			},
		},
		soundQueue: make([]soundEvent, 0, 8),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
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

func TestProcessThingPickups_ConsumesHealthBonusAtCapLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2014, Flags: skillMediumBits}},
		},
		soundQueue: make([]soundEvent, 0, 2),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.stats.Health = 200
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if !g.thingCollected[0] {
		t.Fatal("health bonus should still be consumed at cap")
	}
	if got := len(g.soundQueue); got != 1 || g.soundQueue[0] != soundEventItemUp {
		t.Fatalf("soundQueue=%v want [%v]", g.soundQueue, soundEventItemUp)
	}
}

func TestProcessThingPickups_ConsumesArmorBonusAtCapLikeDoom(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2015, Flags: skillMediumBits}},
		},
		soundQueue: make([]soundEvent, 0, 2),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.stats.Armor = 200
	g.stats.ArmorType = 2
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if !g.thingCollected[0] {
		t.Fatal("armor bonus should still be consumed at cap")
	}
	if got := len(g.soundQueue); got != 1 || g.soundQueue[0] != soundEventItemUp {
		t.Fatalf("soundQueue=%v want [%v]", g.soundQueue, soundEventItemUp)
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
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2011, Flags: skillMediumBits}},
		},
		isDead: true,
		opts:   Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.stats.Health = 50
	g.thingCollected = make([]bool, len(g.m.Things))
	g.runGameplayTic(moveCmd{}, false, false)
	if g.stats.Health != 50 {
		t.Fatalf("dead player health changed to %d", g.stats.Health)
	}
	if g.thingCollected[0] {
		t.Fatal("dead player should not collect pickups")
	}
}

func TestFilteredPickupDoesNotCollect(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2008, Flags: 0}},
		},
		opts: Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if g.thingCollected[0] {
		t.Fatal("filtered pickup should not collect")
	}
}

func TestCanTouchPickup_DoomStyleBounds(t *testing.T) {
	px, py, pz := int64(0), int64(0), int64(0)
	tx, ty, tz := int64(35*fracUnit), int64(0), int64(0)
	if !canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, tz, 20*fracUnit) {
		t.Fatal("expected touch within blockdist")
	}
	tx = 37 * fracUnit
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, tz, 20*fracUnit) {
		t.Fatal("expected no touch beyond blockdist")
	}
}

func TestCanTouchPickup_ZOverlap(t *testing.T) {
	px, py, pz := int64(0), int64(0), int64(0)
	tx, ty := int64(0), int64(0)
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, 57*fracUnit, 20*fracUnit) {
		t.Fatal("thing above player height should not touch")
	}
	if canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, -9*fracUnit, 20*fracUnit) {
		t.Fatal("thing too far below should not touch")
	}
	if !canTouchPickup(px, py, pz, playerRadius, playerHeight, tx, ty, -8*fracUnit, 20*fracUnit) {
		t.Fatal("thing in lower overlap range should touch")
	}
}

func TestWeaponPickupRespectsAutoSwitchToggle(t *testing.T) {
	baseMap := &mapdata.Map{
		Things: []mapdata.Thing{
			{X: 0, Y: 0, Type: 2001, Flags: skillMediumBits}, // shotgun
		},
	}
	g := &game{
		m:                baseMap,
		autoWeaponSwitch: false,
		opts:             Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
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
				{X: 0, Y: 0, Type: 2001, Flags: skillMediumBits}, // shotgun
			},
		},
		autoWeaponSwitch: true,
		opts:             Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.inventory.ReadyWeapon = weaponPistol
	g.weaponState = weaponStatePistolReady
	g.weaponStateTics = 1

	g.processThingPickups()
	if g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("pending weapon=%v want=%v when auto switch enabled", g.inventory.PendingWeapon, weaponShotgun)
	}
}

func TestDroppedClipGivesHalfClip(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2007}},
		},
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.thingDropped = []bool{true}
	g.stats.Bullets = 0

	g.processThingPickups()

	if g.stats.Bullets != 5 {
		t.Fatalf("bullets=%d want=5 for dropped clip", g.stats.Bullets)
	}
}

func TestDroppedShotgunUsesDroppedAmmoAmount(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2001}},
		},
		autoWeaponSwitch: true,
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.thingDropped = []bool{true}

	g.processThingPickups()

	if !g.inventory.Weapons[2001] {
		t.Fatal("shotgun should be owned")
	}
	if g.stats.Shells != 4 {
		t.Fatalf("shells=%d want=4 for dropped shotgun", g.stats.Shells)
	}
}

func TestDroppedDuplicateShotgunQueuesShotgunWhenShellsRiseFromZero(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2001}},
		},
		autoWeaponSwitch: false,
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.thingDropped = []bool{true}
	g.inventory.Weapons[2001] = true
	g.inventory.ReadyWeapon = weaponPistol
	g.inventory.PendingWeapon = 0
	g.weaponState = weaponStatePistolReady
	g.weaponStateTics = 1
	g.stats.Shells = 0

	g.processThingPickups()

	if g.stats.Shells != 4 {
		t.Fatalf("shells=%d want=4 for dropped duplicate shotgun", g.stats.Shells)
	}
	if g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("pending weapon=%v want shotgun after shells rise from zero", g.inventory.PendingWeapon)
	}
}

func TestProcessThingPickupsAt_UsesProbePositionLikeDoomMoveChecks(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{X: 0, Y: 0, Type: 2001}},
		},
		autoWeaponSwitch: true,
	}
	g.initPlayerState()
	g.thingCollected = make([]bool, len(g.m.Things))
	g.thingDropped = []bool{true}
	g.p.x = 0
	g.p.y = 40 * fracUnit
	g.p.z = 0

	g.processThingPickupsAt(0, 36*fracUnit, g.p.z, playerRadius, playerHeight)

	if !g.thingCollected[0] {
		t.Fatal("pickup should be collected at the probed move position")
	}
	if g.inventory.PendingWeapon != weaponShotgun {
		t.Fatalf("pending weapon=%v want shotgun after probe pickup", g.inventory.PendingWeapon)
	}
}

func TestProcessThingPickups_CollectsVanillaPowerupItems(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{X: 0, Y: 0, Type: 2013, Flags: skillMediumBits},
				{X: 0, Y: 0, Type: 2022, Flags: skillMediumBits},
				{X: 0, Y: 0, Type: 2024, Flags: skillMediumBits},
				{X: 0, Y: 0, Type: 2026, Flags: skillMediumBits},
				{X: 0, Y: 0, Type: 2045, Flags: skillMediumBits},
				{X: 0, Y: 0, Type: 83, Flags: skillMediumBits},
			},
		},
		soundQueue: make([]soundEvent, 0, 8),
		opts:       Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: false},
	}
	g.initPlayerState()
	g.stats.Health = 50
	g.stats.Armor = 0
	g.stats.ArmorType = 0
	g.thingCollected = make([]bool, len(g.m.Things))

	g.processThingPickups()

	if g.stats.Health != 200 {
		t.Fatalf("health=%d want=200 after soulsphere+megasphere", g.stats.Health)
	}
	if g.stats.Armor != 200 || g.stats.ArmorType != 2 {
		t.Fatalf("armor=%d type=%d want 200/2", g.stats.Armor, g.stats.ArmorType)
	}
	if g.inventory.InvulnTics != 30*doomTicsPerSecond {
		t.Fatalf("invuln=%d want=%d", g.inventory.InvulnTics, 30*doomTicsPerSecond)
	}
	if g.inventory.InvisTics != 60*doomTicsPerSecond {
		t.Fatalf("invis=%d want=%d", g.inventory.InvisTics, 60*doomTicsPerSecond)
	}
	if !g.inventory.AllMap {
		t.Fatal("computer map should set all-map power")
	}
	if g.inventory.LightAmpTics != 120*doomTicsPerSecond {
		t.Fatalf("light amp=%d want=%d", g.inventory.LightAmpTics, 120*doomTicsPerSecond)
	}
	for i, got := range g.thingCollected {
		if !got {
			t.Fatalf("pickup %d was not collected", i)
		}
	}
}
