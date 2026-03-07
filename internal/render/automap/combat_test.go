package automap

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestMonsterDeathsSpawnVanillaDrops(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},   // zombieman
				{Type: 9, X: 64, Y: 0},     // shotgun guy
				{Type: 65, X: 128, Y: 0},   // chaingunner
				{Type: 3001, X: 192, Y: 0}, // imp
			},
		},
		thingCollected: make([]bool, 4),
		thingDropped:   make([]bool, 4),
		thingHP:        []int{1, 1, 1, 1},
		thingDead:      make([]bool, 4),
		thingDeathTics: make([]int, 4),
	}
	g.damageMonster(0, 1)
	g.damageMonster(1, 1)
	g.damageMonster(2, 1)
	g.damageMonster(3, 1)

	if got, want := len(g.m.Things), 7; got != want {
		t.Fatalf("thing count=%d want=%d", got, want)
	}
	drops := g.m.Things[4:]
	if drops[0].Type != 2007 || drops[1].Type != 2001 || drops[2].Type != 2002 {
		t.Fatalf("drop types=%v want [2007 2001 2002]", []int16{drops[0].Type, drops[1].Type, drops[2].Type})
	}
	for i := 4; i < 7; i++ {
		if !g.thingDropped[i] {
			t.Fatalf("drop %d should be marked dropped", i)
		}
	}
}

func TestMonsterDropPreservesRuntimeFixedPosition(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 0, Y: 0},
			},
		},
		thingCollected: make([]bool, 1),
		thingDropped:   make([]bool, 1),
		thingX:         []int64{(10 << fracBits) + fracUnit/2},
		thingY:         []int64{(20 << fracBits) + fracUnit/4},
		thingHP:        []int{1},
		thingDead:      make([]bool, 1),
		thingDeathTics: make([]int, 1),
		thingSectorCache: []int{
			0,
		},
	}
	g.damageMonster(0, 1)
	if len(g.m.Things) != 2 {
		t.Fatalf("thing count=%d want=2", len(g.m.Things))
	}
	if len(g.thingX) != 2 || len(g.thingY) != 2 {
		t.Fatalf("runtime position slices not extended: lenX=%d lenY=%d", len(g.thingX), len(g.thingY))
	}
	if g.thingX[1] != g.thingX[0] || g.thingY[1] != g.thingY[0] {
		t.Fatalf("drop runtime pos=(%d,%d) want (%d,%d)", g.thingX[1], g.thingY[1], g.thingX[0], g.thingY[0])
	}
}

func TestPickHitscanMonsterTarget(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
				{Type: 3004, X: 96, Y: 32},
			},
		},
		thingCollected: []bool{false, false},
		thingHP:        []int{20, 20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
	}
	idx, ok := g.pickHitscanMonsterTarget()
	if !ok {
		t.Fatal("expected a target")
	}
	if idx != 0 {
		t.Fatalf("target idx=%d want=0", idx)
	}
}

func TestHandleFireConsumesAmmoAndDamages(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
		stats:          playerStats{Bullets: 10, Health: 100},
		inventory:      playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.stats.Bullets != 9 {
		t.Fatalf("bullets=%d want=9", g.stats.Bullets)
	}
	if g.thingHP[0] >= 20 {
		t.Fatalf("monster hp=%d want < 20", g.thingHP[0])
	}
	if !hasSoundEvent(g.soundQueue, soundEventShootPistol) {
		t.Fatalf("soundQueue=%v missing %v", g.soundQueue, soundEventShootPistol)
	}
}

func TestHandleFireNoAmmoFallsBackToFist(t *testing.T) {
	g := &game{
		stats:     playerStats{Bullets: 0},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.inventory.ReadyWeapon != weaponFist {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponFist)
	}
}

func TestShotgunConsumesShellAndDealsPelletDamage(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: []bool{false},
		thingHP:        []int{40},
		p:              player{x: 0, y: 0, angle: degToAngle(0)},
		stats:          playerStats{Health: 100, Shells: 2},
		inventory: playerInventory{
			ReadyWeapon: weaponShotgun,
			Weapons:     map[int16]bool{2001: true},
		},
	}
	g.handleFire()
	if g.stats.Shells != 1 {
		t.Fatalf("shells=%d want=1", g.stats.Shells)
	}
	if g.thingHP[0] >= 40 {
		t.Fatalf("monster hp=%d want < 40", g.thingHP[0])
	}
}

func TestNoAmmoAutoSwitchesToFist(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{},
		},
	}
	g.ensureWeaponHasAmmo()
	if g.inventory.ReadyWeapon != weaponFist {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponFist)
	}
}

func TestPistolFirstShotConsumesSingleDamageRoll(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:     playerStats{Bullets: 1},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 1 {
		t.Fatalf("p-random calls=%d want=1 for accurate first pistol shot", d)
	}
}

func TestPistolRefireConsumesSpreadAndDamageRolls(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:        playerStats{Bullets: 1},
		inventory:    playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
		weaponRefire: true,
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 3 {
		t.Fatalf("p-random calls=%d want=3 for refire pistol shot", d)
	}
}

func TestShotgunConsumesSevenPelletRandomRolls(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats: playerStats{Shells: 1},
		inventory: playerInventory{
			ReadyWeapon: weaponShotgun,
			Weapons:     map[int16]bool{2001: true},
		},
	}
	_, p0 := doomrand.State()
	g.handleFire()
	_, p1 := doomrand.State()
	if d := prandDelta(p0, p1); d != 21 {
		t.Fatalf("p-random calls=%d want=21 for 7-pellet shotgun shot", d)
	}
}

func prandDelta(before, after int) int {
	d := after - before
	if d < 0 {
		d += 256
	}
	return d
}

func TestTickWeaponFirePistolCadence(t *testing.T) {
	doomrand.Clear()
	g := &game{
		stats:     playerStats{Bullets: 3},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.setAttackHeld(true)
	g.tickWeaponFire()
	if g.stats.Bullets != 2 {
		t.Fatalf("bullets=%d want=2 after first shot", g.stats.Bullets)
	}
	for i := 0; i < 13; i++ {
		g.tickWeaponFire()
		if g.stats.Bullets != 2 {
			t.Fatalf("shot fired too early at tic %d: bullets=%d", i+1, g.stats.Bullets)
		}
	}
	g.tickWeaponFire()
	if g.stats.Bullets != 1 {
		t.Fatalf("bullets=%d want=1 at tic 14 refire cadence", g.stats.Bullets)
	}
}

func TestPistolAutoaimSlopeHitsLowerTarget(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3004, X: 64, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{20},
		p: player{
			x:      0,
			y:      0,
			z:      44 * fracUnit,
			floorz: 44 * fracUnit,
			ceilz:  128 * fracUnit,
			angle:  degToAngle(0),
		},
		stats:     playerStats{Bullets: 1},
		inventory: playerInventory{ReadyWeapon: weaponPistol, Weapons: map[int16]bool{}},
	}
	g.handleFire()
	if g.thingHP[0] >= 20 {
		t.Fatalf("monster hp=%d want < 20 (autoaim slope should hit lower target)", g.thingHP[0])
	}
}

func TestCycleWeaponSkipsUnowned(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons:     map[int16]bool{2002: true},
		},
	}
	g.cycleWeapon(1)
	if g.inventory.ReadyWeapon != weaponChaingun {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponChaingun)
	}
}

func TestCycleWeaponWrapsBackward(t *testing.T) {
	g := &game{
		inventory: playerInventory{
			ReadyWeapon: weaponFist,
			Weapons:     map[int16]bool{2006: true},
		},
	}
	g.cycleWeapon(-1)
	if g.inventory.ReadyWeapon != weaponBFG {
		t.Fatalf("weapon=%v want=%v", g.inventory.ReadyWeapon, weaponBFG)
	}
}

func TestFireGunShotSpawnsHitscanPuffOnWallImpact(t *testing.T) {
	doomrand.Clear()
	g := &game{
		lines: []physLine{
			{
				x1:       64 * fracUnit,
				y1:       -32 * fracUnit,
				x2:       64 * fracUnit,
				y2:       32 * fracUnit,
				flags:    0, // one-sided blocker
				sideNum1: -1,
			},
		},
		p: player{x: 0, y: 0, z: 0, angle: degToAngle(0)},
	}
	if g.fireGunShot(g.p.angle, pistolRange, 0, true) {
		t.Fatal("expected no monster hit")
	}
	if len(g.hitscanPuffs) == 0 {
		t.Fatal("expected wall-impact hitscan puff to spawn")
	}
	if g.hitscanPuffs[0].kind != hitscanFxPuff {
		t.Fatalf("effect kind=%d want puff", g.hitscanPuffs[0].kind)
	}
}

func TestFireGunShotSpawnsHitscanBloodOnMonsterHit(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{{Type: 3004, X: 64, Y: 0}},
		},
		thingCollected: make([]bool, 1),
		thingHP:        []int{20},
		p:              player{x: 0, y: 0, z: 0, angle: degToAngle(0)},
	}
	slope := g.bulletSlopeForAim(g.p.angle, pistolRange)
	if !g.fireGunShot(g.p.angle, pistolRange, slope, true) {
		t.Fatal("expected monster hit")
	}
	if len(g.hitscanPuffs) == 0 {
		t.Fatal("expected blood effect to spawn")
	}
	if g.hitscanPuffs[0].kind != hitscanFxBlood {
		t.Fatalf("effect kind=%d want blood", g.hitscanPuffs[0].kind)
	}
}

func TestHitscanPuffsExpire(t *testing.T) {
	g := &game{}
	g.spawnHitscanPuff(0, 0, 0)
	if len(g.hitscanPuffs) != 1 {
		t.Fatalf("puffs=%d want=1", len(g.hitscanPuffs))
	}
	for i := 0; i < 16; i++ {
		g.tickHitscanPuffs()
	}
	if len(g.hitscanPuffs) != 0 {
		t.Fatalf("puffs=%d want=0 after expiry", len(g.hitscanPuffs))
	}
}

func TestMonsterPainSoundEventMapping(t *testing.T) {
	if got := monsterPainSoundEvent(3006); got != soundEventMonsterPainDemon {
		t.Fatalf("lost soul pain event=%v want=%v", got, soundEventMonsterPainDemon)
	}
	if got := monsterPainSoundEvent(3001); got != soundEventMonsterPainHumanoid {
		t.Fatalf("imp pain event=%v want=%v", got, soundEventMonsterPainHumanoid)
	}
}

func TestMonsterDeathSoundEventMapping(t *testing.T) {
	tests := []struct {
		typ  int16
		want soundEvent
	}{
		{typ: 3004, want: soundEventDeathZombie},
		{typ: 9, want: soundEventDeathShotgunGuy},
		{typ: 3001, want: soundEventDeathImp},
		{typ: 3002, want: soundEventDeathDemon},
		{typ: 3005, want: soundEventDeathCaco},
		{typ: 3003, want: soundEventDeathBaron},
		{typ: 16, want: soundEventDeathCyber},
		{typ: 7, want: soundEventDeathSpider},
		{typ: 3006, want: soundEventDeathLostSoul},
	}
	for _, tc := range tests {
		if got := monsterDeathSoundEvent(tc.typ); got != tc.want {
			t.Fatalf("type=%d death event=%v want=%v", tc.typ, got, tc.want)
		}
	}
}

func TestDamageMonsterDelaysShotgunDeathSoundToScreamFrame(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 9, X: 0, Y: 0},
			},
		},
		thingCollected: []bool{false},
		thingHP:        []int{1},
		thingDead:      []bool{false},
		thingDeathTics: []int{0},
		soundQueue:     make([]soundEvent, 0, 2),
		delayedSfx:     make([]delayedSoundEvent, 0, 2),
	}
	g.damageMonster(0, 1)
	if hasSoundEvent(g.soundQueue, soundEventDeathShotgunGuy) {
		t.Fatalf("death sound should be delayed; queue=%v", g.soundQueue)
	}
	if got := len(g.delayedSfx); got != 1 {
		t.Fatalf("delayedSfx len=%d want=1", got)
	}
	for i := 0; i < 4; i++ {
		g.tickDelayedSounds()
		if hasSoundEvent(g.soundQueue, soundEventDeathShotgunGuy) {
			t.Fatalf("death sound fired early at tick %d", i+1)
		}
	}
	g.tickDelayedSounds()
	if !hasSoundEvent(g.soundQueue, soundEventDeathShotgunGuy) {
		t.Fatalf("queue=%v missing delayed shotgun death sound", g.soundQueue)
	}
}

func hasSoundEvent(queue []soundEvent, want soundEvent) bool {
	for _, ev := range queue {
		if ev == want {
			return true
		}
	}
	return false
}
