package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestWASMMapStartPrewarmEvents_IncludesCommonWeaponsAndNearbyMonsterSounds(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Name: "MAP01",
			Things: []mapdata.Thing{
				{X: 128, Y: 0, Type: 3004},
				{X: 192, Y: 0, Type: 3001},
				{X: 3200, Y: 0, Type: 3005},
				{X: 4000, Y: 0, Type: 16},
			},
		},
		p: player{x: 0, y: 0},
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons: map[int16]bool{
				2001: true,
				82:   true,
			},
		},
	}

	world, projectile, monsters := g.wasmMapStartPrewarmBuckets()
	if !hasPrewarmSoundEvent(world, soundEventItemUp) {
		t.Fatal("missing common item pickup prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventSwitchOn) {
		t.Fatal("missing common switch prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventWeaponUp) {
		t.Fatal("missing weapon pickup prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventShootPistol) {
		t.Fatal("missing pistol prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventShootShotgun) {
		t.Fatal("missing shotgun prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventShootSuperShotgun) {
		t.Fatal("missing super shotgun prewarm")
	}
	if !hasPrewarmSoundEvent(world, soundEventShotgunOpen) || !hasPrewarmSoundEvent(world, soundEventShotgunLoad) || !hasPrewarmSoundEvent(world, soundEventShotgunClose) {
		t.Fatal("missing super shotgun reload prewarm")
	}
	if !hasPrewarmSoundEvent(monsters, soundEventMonsterSeePosit1) || !hasPrewarmSoundEvent(monsters, soundEventMonsterSeePosit2) || !hasPrewarmSoundEvent(monsters, soundEventMonsterSeePosit3) {
		t.Fatal("missing nearby zombieman see-sound variants")
	}
	if !hasPrewarmSoundEvent(monsters, soundEventMonsterActivePosit) {
		t.Fatal("missing nearby monster active prewarm")
	}
	if !hasPrewarmSoundEvent(monsters, soundEventMonsterPainHumanoid) {
		t.Fatal("missing nearby monster pain prewarm")
	}
	if !hasPrewarmSoundEvent(monsters, soundEventMonsterAttackClaw) {
		t.Fatal("missing nearby monster attack prewarm")
	}
	if !hasPrewarmSoundEvent(projectile, soundEventShootFireball) {
		t.Fatal("missing projectile launch prewarm")
	}
	if !hasPrewarmSoundEvent(projectile, soundEventImpactFire) {
		t.Fatal("missing projectile impact prewarm")
	}
	if !hasPrewarmSoundEvent(projectile, soundEventShootRocket) {
		t.Fatal("missing map-wide rocket launch prewarm")
	}
	if !hasPrewarmSoundEvent(projectile, soundEventBarrelExplode) {
		t.Fatal("missing map-wide rocket impact prewarm")
	}
	if hasPrewarmSoundEvent(monsters, soundEventMonsterSeeCaco) {
		t.Fatal("far monster sound should not be prewarmed")
	}
	if hasPrewarmSoundEvent(monsters, soundEventMonsterSeeCyber) {
		t.Fatal("far monster vocal sound should not be prewarmed")
	}
}

func TestWASMMapStartPrewarmBuckets_DoesNotAdvanceRNG(t *testing.T) {
	doomrand.Clear()
	g := &game{
		m: &mapdata.Map{
			Name: "MAP01",
			Things: []mapdata.Thing{
				{X: 128, Y: 0, Type: 3004},
				{X: 256, Y: 0, Type: 3001},
				{X: 512, Y: 0, Type: 16},
				{X: 1024, Y: 0, Type: 88},
			},
		},
		p: player{x: 0, y: 0},
		inventory: playerInventory{
			ReadyWeapon: weaponPistol,
			Weapons: map[int16]bool{
				2001: true,
				2003: true,
				2004: true,
				2006: true,
			},
		},
	}

	r0, p0 := doomrand.State()
	worldA, projectileA, monstersA := g.wasmMapStartPrewarmBuckets()
	r1, p1 := doomrand.State()

	if r1 != r0 || p1 != p0 {
		t.Fatalf("doomrand state changed after prewarm buckets: before=%d/%d after=%d/%d", r0, p0, r1, p1)
	}

	doomrand.Clear()
	worldB, projectileB, monstersB := g.wasmMapStartPrewarmBuckets()
	r2, p2 := doomrand.State()

	if r2 != 0 || p2 != 0 {
		t.Fatalf("doomrand state changed after repeated prewarm buckets: got=%d/%d want=0/0", r2, p2)
	}
	if !sameSoundEventSlice(worldA, worldB) || !sameSoundEventSlice(projectileA, projectileB) || !sameSoundEventSlice(monstersA, monstersB) {
		t.Fatal("prewarm buckets are not deterministic across runs")
	}
}

func hasPrewarmSoundEvent(events []soundEvent, want soundEvent) bool {
	for _, ev := range events {
		if ev == want {
			return true
		}
	}
	return false
}

func sameSoundEventSlice(a, b []soundEvent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
