package doomruntime

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	slot := 98
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 3004, X: 64, Y: 0},
		},
		Linedefs: []mapdata.Linedef{
			{Special: 11},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128, Special: 9},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	sg.g.State.SetFollowMode(false)
	sg.g.State.SetCamera(12.5, 34.5)
	sg.g.State.SetZoom(3.25)
	sg.g.p.x = 123 * fracUnit
	sg.g.p.y = -45 * fracUnit
	sg.g.p.z = 8 * fracUnit
	sg.g.stats.Health = 17
	sg.g.stats.Armor = 88
	sg.g.inventory.BlueKey = true
	sg.g.inventory.ReadyWeapon = weaponShotgun
	sg.g.inventory.Weapons[2003] = true
	sg.g.worldTic = 321
	sg.g.thingCollected[1] = true
	sg.g.thingHP[1] = 12
	sg.g.secretFound[0] = true
	sg.g.secretsFound = 1
	sg.g.sectorFloor[0] = 16 * fracUnit
	sg.g.sectorCeil[0] = 144 * fracUnit
	sg.g.lineSpecial[0] = 99
	sg.g.damageFlashTic = 7
	sg.g.doors = map[int]*doorThinker{
		0: {
			sector:       0,
			typ:          doorBlazeRaise,
			direction:    1,
			topHeight:    200 * fracUnit,
			topWait:      12,
			topCountdown: 3,
			speed:        4 * fracUnit,
		},
	}
	sg.g.projectiles = []projectile{{
		x:           10,
		y:           20,
		z:           30,
		vx:          40,
		vy:          50,
		vz:          60,
		radius:      7,
		height:      8,
		ttl:         9,
		sourceThing: 1,
		sourceType:  3004,
		angle:       1234,
		kind:        projectileRocket,
	}}
	sg.g.delayedSwitchReverts = []delayedSwitchTexture{{
		line:    0,
		sidedef: 0,
		top:     "TOP",
		bottom:  "BOT",
		mid:     "MID",
		tics:    15,
	}}
	sg.g.sectorLightFx = []sectorLightEffect{{
		kind:       sectorLightEffectGlow,
		minLight:   96,
		maxLight:   160,
		count:      4,
		direction:  -1,
		brightTime: 5,
	}}

	doomrand.Clear()
	_ = doomrand.MRandom()
	_ = doomrand.PRandom()
	_ = doomrand.PRandom()
	wantRnd, wantPRnd := doomrand.State()

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read save failed: %v", err)
	}
	if !bytes.HasPrefix(raw, saveGameMagic) {
		t.Fatalf("save missing magic prefix: %q", raw[:min(len(raw), len(saveGameMagic))])
	}

	loaded := &sessionGame{
		opts: Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.current != base.Name {
		t.Fatalf("current=%q want=%q", loaded.current, base.Name)
	}
	if loaded.g == nil {
		t.Fatal("loaded game is nil")
	}
	if loaded.g.stats.Health != 17 || loaded.g.stats.Armor != 88 {
		t.Fatalf("stats=%+v", loaded.g.stats)
	}
	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not restored")
	}
	if loaded.g.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("ready weapon=%v want=%v", loaded.g.inventory.ReadyWeapon, weaponShotgun)
	}
	if !loaded.g.inventory.Weapons[2003] {
		t.Fatal("rocket launcher ownership not restored")
	}
	if loaded.g.p.x != 123*fracUnit || loaded.g.p.y != -45*fracUnit || loaded.g.p.z != 8*fracUnit {
		t.Fatalf("player position=(%d,%d,%d)", loaded.g.p.x, loaded.g.p.y, loaded.g.p.z)
	}
	if loaded.g.worldTic != 321 {
		t.Fatalf("worldTic=%d want=321", loaded.g.worldTic)
	}
	if !loaded.g.thingCollected[1] || loaded.g.thingHP[1] != 12 {
		t.Fatalf("thing state not restored: collected=%v hp=%d", loaded.g.thingCollected[1], loaded.g.thingHP[1])
	}
	if !loaded.g.secretFound[0] || loaded.g.secretsFound != 1 {
		t.Fatalf("secret state not restored: found=%v count=%d", loaded.g.secretFound, loaded.g.secretsFound)
	}
	if loaded.g.sectorFloor[0] != 16*fracUnit || loaded.g.sectorCeil[0] != 144*fracUnit {
		t.Fatalf("sector heights=(%d,%d)", loaded.g.sectorFloor[0], loaded.g.sectorCeil[0])
	}
	if loaded.g.lineSpecial[0] != 99 {
		t.Fatalf("lineSpecial=%d want=99", loaded.g.lineSpecial[0])
	}
	if loaded.g.damageFlashTic != 7 {
		t.Fatalf("damageFlashTic=%d want=7", loaded.g.damageFlashTic)
	}
	if door := loaded.g.doors[0]; door == nil || door.typ != doorBlazeRaise || door.topHeight != 200*fracUnit {
		t.Fatalf("door thinker not restored: %#v", door)
	}
	if len(loaded.g.projectiles) != 1 || loaded.g.projectiles[0].kind != projectileRocket || loaded.g.projectiles[0].sourceThing != 1 {
		t.Fatalf("projectiles not restored: %#v", loaded.g.projectiles)
	}
	if len(loaded.g.delayedSwitchReverts) != 1 || loaded.g.delayedSwitchReverts[0].mid != "MID" {
		t.Fatalf("switch reverts not restored: %#v", loaded.g.delayedSwitchReverts)
	}
	if len(loaded.g.sectorLightFx) != 1 || loaded.g.sectorLightFx[0].kind != sectorLightEffectGlow {
		t.Fatalf("sector light fx not restored: %#v", loaded.g.sectorLightFx)
	}
	gotRnd, gotPRnd := doomrand.State()
	if gotRnd != wantRnd || gotPRnd != wantPRnd {
		t.Fatalf("rng=(%d,%d) want=(%d,%d)", gotRnd, gotPRnd, wantRnd, wantPRnd)
	}
}

func TestLoadGameRejectsUnknownHeader(t *testing.T) {
	sg := &sessionGame{}
	err := sg.unmarshalSaveGame([]byte("not-a-gddoom-save"))
	if !errors.Is(err, errBadSaveMagic) {
		t.Fatalf("err=%v want=%v", err, errBadSaveMagic)
	}
}
