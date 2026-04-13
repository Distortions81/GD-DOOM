package doomruntime

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

type liveRuntimeRoundTripState struct {
	Player              player
	Inventory           playerInventorySaveState
	Stats               playerStats
	WorldTic            int
	IsDead              bool
	PlayerMobjHealth    int
	PlayerViewZ         int64
	DamageFlashTic      int
	BonusFlashTic       int
	ThingCollected      []bool
	ThingDropped        []bool
	ThingThinkerOrder   []int64
	ThingX              []int64
	ThingY              []int64
	ThingMomX           []int64
	ThingMomY           []int64
	ThingMomZ           []int64
	ThingAngleState     []uint32
	ThingZState         []int64
	ThingFloorState     []int64
	ThingCeilState      []int64
	ThingSupportValid   []bool
	ThingSectorCache    []int
	ThingBlockOrder     []int64
	ThingBlockCell      []int
	ThingHP             []int
	ThingAggro          []bool
	ThingAmbush         []bool
	ThingTargetPlayer   []bool
	ThingTargetIdx      []int
	ThingThreshold      []int
	ThingCooldown       []int
	ThingMoveDir        []uint8
	ThingMoveCount      []int
	ThingJustAtk        []bool
	ThingInFloat        []bool
	ThingJustHit        []bool
	ThingReactionTics   []int
	ThingWakeTics       []int
	ThingLastLook       []int
	ThingDead           []bool
	ThingGibbed         []bool
	ThingGibTick        []int
	ThingXDeath         []bool
	ThingDeathTics      []int
	ThingAttackTics     []int
	ThingAttackPhase    []int
	ThingAttackFireTics []int
	ThingPainTics       []int
	ThingThinkWait      []int
	ThingDoomState      []int
	ThingState          []monsterThinkState
	ThingStateTics      []int
	ThingStatePhase     []int
	PlayerBlockOrder    int64
	NextThinkerOrder    int64
	NextBlockmapOrder   int64
	SecretFound         []bool
	SecretsFound        int
	SecretsTotal        int
	SectorSoundTarget   []bool
	SectorLightFx       []sectorLightEffectSaveState
	SectorFloor         []int64
	SectorCeil          []int64
	LineSpecial         []uint16
	Sidedefs            []mapdata.Sidedef
	Sectors             []mapdata.Sector
	Things              []mapdata.Thing
	Doors               map[int]doorThinkerSaveState
	Floors              map[int]floorThinkerSaveState
	Plats               map[int]platThinkerSaveState
	Ceilings            map[int]ceilingThinkerSaveState
	DelayedSwitches     []delayedSwitchTextureSaveState
}

func captureLiveRuntimeRoundTripState(g *game) liveRuntimeRoundTripState {
	s := liveRuntimeRoundTripState{
		Player:              g.p,
		Inventory:           capturePlayerInventorySaveState(g.inventory),
		Stats:               g.stats,
		WorldTic:            g.worldTic,
		IsDead:              g.isDead,
		PlayerMobjHealth:    g.playerMobjHealth,
		PlayerViewZ:         g.playerViewZ,
		DamageFlashTic:      g.damageFlashTic,
		BonusFlashTic:       g.bonusFlashTic,
		ThingCollected:      append([]bool(nil), g.thingCollected...),
		ThingDropped:        append([]bool(nil), g.thingDropped...),
		ThingThinkerOrder:   append([]int64(nil), g.thingThinkerOrder...),
		ThingX:              append([]int64(nil), g.thingX...),
		ThingY:              append([]int64(nil), g.thingY...),
		ThingMomX:           append([]int64(nil), g.thingMomX...),
		ThingMomY:           append([]int64(nil), g.thingMomY...),
		ThingMomZ:           append([]int64(nil), g.thingMomZ...),
		ThingAngleState:     append([]uint32(nil), g.thingAngleState...),
		ThingZState:         append([]int64(nil), g.thingZState...),
		ThingFloorState:     append([]int64(nil), g.thingFloorState...),
		ThingCeilState:      append([]int64(nil), g.thingCeilState...),
		ThingSupportValid:   append([]bool(nil), g.thingSupportValid...),
		ThingSectorCache:    append([]int(nil), g.thingSectorCache...),
		ThingBlockOrder:     append([]int64(nil), g.thingBlockOrder...),
		ThingBlockCell:      append([]int(nil), g.thingBlockCell...),
		ThingHP:             append([]int(nil), g.thingHP...),
		ThingAggro:          append([]bool(nil), g.thingAggro...),
		ThingAmbush:         append([]bool(nil), g.thingAmbush...),
		ThingTargetPlayer:   append([]bool(nil), g.thingTargetPlayer...),
		ThingTargetIdx:      append([]int(nil), g.thingTargetIdx...),
		ThingThreshold:      append([]int(nil), g.thingThreshold...),
		ThingCooldown:       append([]int(nil), g.thingCooldown...),
		ThingMoveDir:        cloneMonsterMoveDirSlice(g.thingMoveDir),
		ThingMoveCount:      append([]int(nil), g.thingMoveCount...),
		ThingJustAtk:        append([]bool(nil), g.thingJustAtk...),
		ThingInFloat:        append([]bool(nil), g.thingInFloat...),
		ThingJustHit:        append([]bool(nil), g.thingJustHit...),
		ThingReactionTics:   append([]int(nil), g.thingReactionTics...),
		ThingWakeTics:       append([]int(nil), g.thingWakeTics...),
		ThingLastLook:       append([]int(nil), g.thingLastLook...),
		ThingDead:           append([]bool(nil), g.thingDead...),
		ThingGibbed:         append([]bool(nil), g.thingGibbed...),
		ThingGibTick:        append([]int(nil), g.thingGibTick...),
		ThingXDeath:         append([]bool(nil), g.thingXDeath...),
		ThingDeathTics:      append([]int(nil), g.thingDeathTics...),
		ThingAttackTics:     append([]int(nil), g.thingAttackTics...),
		ThingAttackPhase:    append([]int(nil), g.thingAttackPhase...),
		ThingAttackFireTics: append([]int(nil), g.thingAttackFireTics...),
		ThingPainTics:       append([]int(nil), g.thingPainTics...),
		ThingThinkWait:      append([]int(nil), g.thingThinkWait...),
		ThingDoomState:      append([]int(nil), g.thingDoomState...),
		ThingState:          append([]monsterThinkState(nil), g.thingState...),
		ThingStateTics:      append([]int(nil), g.thingStateTics...),
		ThingStatePhase:     append([]int(nil), g.thingStatePhase...),
		PlayerBlockOrder:    g.playerBlockOrder,
		NextThinkerOrder:    g.nextThinkerOrder,
		NextBlockmapOrder:   g.nextBlockmapOrder,
		SecretFound:         append([]bool(nil), g.secretFound...),
		SecretsFound:        g.secretsFound,
		SecretsTotal:        g.secretsTotal,
		SectorSoundTarget:   append([]bool(nil), g.sectorSoundTarget...),
		SectorLightFx:       captureSectorLightEffects(g.sectorLightFx),
		SectorFloor:         append([]int64(nil), g.sectorFloor...),
		SectorCeil:          append([]int64(nil), g.sectorCeil...),
		LineSpecial:         append([]uint16(nil), g.lineSpecial...),
		Doors:               captureDoorThinkers(g.doors),
		Floors:              captureFloorThinkers(g.floors),
		Plats:               capturePlatThinkers(g.plats),
		Ceilings:            captureCeilingThinkers(g.ceilings),
		DelayedSwitches:     captureDelayedSwitchTextures(g.delayedSwitchReverts),
	}
	if g != nil && g.m != nil {
		s.Sidedefs = append([]mapdata.Sidedef(nil), g.m.Sidedefs...)
		s.Sectors = append([]mapdata.Sector(nil), g.m.Sectors...)
		s.Things = append([]mapdata.Thing(nil), g.m.Things...)
	}
	normalizeLiveRuntimeRoundTripState(&s)
	return s
}

func normalizeLiveRuntimeRoundTripState(s *liveRuntimeRoundTripState) {
	s.Inventory.Weapons = normalizeOwnedWeapons(s.Inventory.Weapons)
}

func normalizeOwnedWeapons(src map[int16]bool) map[int16]bool {
	if len(src) == 0 {
		return nil
	}
	owned := make(map[int16]bool)
	for k, v := range src {
		if v {
			owned[k] = true
		}
	}
	if len(owned) == 0 {
		return nil
	}
	return owned
}

func TestSaveGamePathUsesNumberedSlotFiles(t *testing.T) {
	if got, want := saveGamePath(0), "saves/quicksave.dsg"; got != want {
		t.Fatalf("saveGamePath(0)=%q want %q", got, want)
	}
	if got, want := saveGamePath(1), "saves/dsg1.dsg"; got != want {
		t.Fatalf("saveGamePath(1)=%q want %q", got, want)
	}
	if got, want := saveGamePath(6), "saves/dsg6.dsg"; got != want {
		t.Fatalf("saveGamePath(6)=%q want %q", got, want)
	}
	if got, want := saveGameThumbnailPath(0), "saves/quicksave.png"; got != want {
		t.Fatalf("saveGameThumbnailPath(0)=%q want %q", got, want)
	}
	if got, want := saveGameThumbnailPath(1), "saves/dsg1.png"; got != want {
		t.Fatalf("saveGameThumbnailPath(1)=%q want %q", got, want)
	}
}

func TestSaveSnapshotIncludesWADSourcesAndChecksumFooter(t *testing.T) {
	slot := 90
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			WADSources: []runtimecfg.WADSource{{Name: "DOOM2.WAD", Hash: "abc123"}, {Name: "PATCH.WAD", Hash: "def456"}},
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read save failed: %v", err)
	}
	file, err := decodeSnapshot(raw, saveGameMagic)
	if err != nil {
		t.Fatalf("decodeSnapshot() error = %v", err)
	}
	if got, want := len(file.WADSources), 2; got != want {
		t.Fatalf("WADSources len=%d want %d", got, want)
	}
	if got, want := file.WADSources[0].Name, "DOOM2.WAD"; got != want {
		t.Fatalf("WADSources[0].Name=%q want %q", got, want)
	}
	if got, want := file.WADSources[0].Hash, "abc123"; got != want {
		t.Fatalf("WADSources[0].Hash=%q want %q", got, want)
	}
	corrupted := append([]byte(nil), raw...)
	corrupted[len(corrupted)-1] ^= 0x01
	if _, err := decodeSnapshot(corrupted, saveGameMagic); !errors.Is(err, errBadSaveChecksum) {
		t.Fatalf("decodeSnapshot(corrupted) error = %v want errBadSaveChecksum", err)
	}
}

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
	sg.g.inventory.Strength = true
	sg.g.inventory.StrengthCount = 123
	sg.g.inventory.ReadyWeapon = weaponShotgun
	sg.g.inventory.Weapons[2003] = true
	sg.g.worldTic = 321
	sg.g.thingCollected[1] = true
	sg.g.thingThinkerOrder[1] = 444
	sg.g.thingHP[1] = 12
	sg.g.thingMomX[1] = 3 * fracUnit
	sg.g.thingMomY[1] = -2 * fracUnit
	sg.g.thingMomZ[1] = fracUnit
	sg.g.thingSkullFly[1] = true
	sg.g.thingResumeChaseNow[1] = true
	sg.g.thingAmbush[1] = true
	sg.g.thingInFloat[1] = true
	sg.g.thingXDeath[1] = true
	sg.g.secretFound[0] = true
	sg.g.secretsFound = 1
	sg.g.playerBlockOrder = 777
	sg.g.nextThinkerOrder = 888
	sg.g.nextBlockmapOrder = 999
	sg.g.sectorFloor[0] = 16 * fracUnit
	sg.g.sectorCeil[0] = 144 * fracUnit
	sg.g.lineSpecial[0] = 99
	sg.g.damageFlashTic = 7
	sg.g.doors = map[int]*doorThinker{
		0: {
			order:        101,
			sector:       0,
			typ:          doorBlazeRaise,
			direction:    1,
			topHeight:    200 * fracUnit,
			topWait:      12,
			topCountdown: 3,
			speed:        4 * fracUnit,
		},
	}
	sg.g.plats = map[int]*platThinker{
		0: {
			order:         202,
			sector:        0,
			typ:           platTypePerpetualRaise,
			status:        platStatusUp,
			oldStatus:     platStatusWaiting,
			speed:         fracUnit,
			low:           8 * fracUnit,
			high:          40 * fracUnit,
			wait:          35,
			count:         11,
			finishFlat:    "FLAT1",
			finishSpecial: 7,
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
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
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
	if loaded.g.thingMomX[1] != 3*fracUnit || loaded.g.thingMomY[1] != -2*fracUnit || loaded.g.thingMomZ[1] != fracUnit {
		t.Fatalf("thing momentum=(%d,%d,%d) want=(%d,%d,%d)", loaded.g.thingMomX[1], loaded.g.thingMomY[1], loaded.g.thingMomZ[1], 3*fracUnit, -2*fracUnit, fracUnit)
	}
	if loaded.g.thingThinkerOrder[1] != 444 {
		t.Fatalf("thing thinker order=%d want=444", loaded.g.thingThinkerOrder[1])
	}
	if !loaded.g.thingSkullFly[1] || !loaded.g.thingResumeChaseNow[1] || !loaded.g.thingAmbush[1] || !loaded.g.thingInFloat[1] {
		t.Fatalf("thing flags not restored: skull=%t resume=%t ambush=%t float=%t", loaded.g.thingSkullFly[1], loaded.g.thingResumeChaseNow[1], loaded.g.thingAmbush[1], loaded.g.thingInFloat[1])
	}
	if !loaded.g.thingXDeath[1] {
		t.Fatal("thing xdeath flag not restored")
	}
	if !loaded.g.secretFound[0] || loaded.g.secretsFound != 1 {
		t.Fatalf("secret state not restored: found=%v count=%d", loaded.g.secretFound, loaded.g.secretsFound)
	}
	if loaded.g.playerBlockOrder != 777 || loaded.g.nextThinkerOrder != 888 || loaded.g.nextBlockmapOrder != 999 {
		t.Fatalf("order counters not restored: player=%d thinker=%d block=%d", loaded.g.playerBlockOrder, loaded.g.nextThinkerOrder, loaded.g.nextBlockmapOrder)
	}
	if loaded.g.sectorFloor[0] != 16*fracUnit || loaded.g.sectorCeil[0] != 144*fracUnit {
		t.Fatalf("sector heights=(%d,%d)", loaded.g.sectorFloor[0], loaded.g.sectorCeil[0])
	}
	if loaded.g.m.Sectors[0].FloorHeight != 16 || loaded.g.m.Sectors[0].CeilingHeight != 144 {
		t.Fatalf("map sector heights=(%d,%d) want=(16,144)", loaded.g.m.Sectors[0].FloorHeight, loaded.g.m.Sectors[0].CeilingHeight)
	}
	if loaded.g.lineSpecial[0] != 99 {
		t.Fatalf("lineSpecial=%d want=99", loaded.g.lineSpecial[0])
	}
	if loaded.g.damageFlashTic != 7 {
		t.Fatalf("damageFlashTic=%d want=7", loaded.g.damageFlashTic)
	}
	if !loaded.g.inventory.Strength || loaded.g.inventory.StrengthCount != 123 {
		t.Fatalf("strength restore=%v/%d want true/123", loaded.g.inventory.Strength, loaded.g.inventory.StrengthCount)
	}
	if door := loaded.g.doors[0]; door == nil || door.typ != doorBlazeRaise || door.topHeight != 200*fracUnit || door.order != 101 {
		t.Fatalf("door thinker not restored: %#v", door)
	}
	if plat := loaded.g.plats[0]; plat == nil || plat.typ != platTypePerpetualRaise || plat.high != 40*fracUnit || plat.order != 202 {
		t.Fatalf("plat thinker not restored: %#v", plat)
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

func TestSaveLoadRoundTrip_PreservesRuntimeThingThinkerOrder(t *testing.T) {
	slot := 96
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	idx := sg.g.appendRuntimeThing(mapdata.Thing{Type: 3004, X: 64, Y: 0}, false)
	if idx < 0 {
		t.Fatal("appendRuntimeThing() failed")
	}
	wantOrder := sg.g.thingThinkerOrder[idx]
	if wantOrder <= 1 {
		t.Fatalf("runtime thinker order=%d want >1", wantOrder)
	}

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if got := loaded.g.thingThinkerOrder[idx]; got != wantOrder {
		t.Fatalf("thingThinkerOrder[%d]=%d want=%d", idx, got, wantOrder)
	}
}

func TestSaveLoadRoundTrip_PreservesXDeathFlag(t *testing.T) {
	slot := 95
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 3004, X: 64, Y: 0},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	spawnHP := monsterSpawnHealth(3004)
	sg.g.thingHP[1] = spawnHP
	sg.g.damageMonster(1, spawnHP*2+1)
	if !sg.g.thingDead[1] || !sg.g.thingXDeath[1] {
		t.Fatalf("precondition failed: dead=%t xdeath=%t", sg.g.thingDead[1], sg.g.thingXDeath[1])
	}

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if !loaded.g.thingDead[1] || !loaded.g.thingXDeath[1] {
		t.Fatalf("xdeath restore failed: dead=%t xdeath=%t", loaded.g.thingDead[1], loaded.g.thingXDeath[1])
	}
}

func TestLoadGameRestoresSavedSessionGameplayOptionsForPickups(t *testing.T) {
	slot := 94
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 5, X: 24, Y: 0, Flags: skillMediumBits},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			SkillLevel: 1,
			GameMode:   gameModeSingle,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if got := loaded.g.opts.SkillLevel; got != 3 {
		t.Fatalf("loaded skill=%d want=3 from save", got)
	}
	if !loaded.g.thingActiveInSession(1) {
		t.Fatal("pickup should be active after restoring saved gameplay options")
	}

	loaded.g.runGameplayTic(moveCmd{forward: forwardMove[1]}, false, false)

	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not collected after load with restored gameplay options")
	}
}

func TestLoadGameBroadcastsImmediateKeyframeWhenLiveSinkPresent(t *testing.T) {
	slot := 92
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 5, X: 24, Y: 0, Flags: skillMediumBits},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g
	sg.g.worldTic = 123

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	sink := &testLiveTicSink{}
	loaded := &sessionGame{
		opts: Options{
			Width:       doomLogicalW,
			Height:      doomLogicalH,
			PlayerSlot:  1,
			SkillLevel:  3,
			GameMode:    gameModeSingle,
			LiveTicSink: sink,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if got := len(sink.keyframes); got != 1 {
		t.Fatalf("broadcast keyframes=%d want=1", got)
	}
	if got, want := sink.keyframeTics[0], uint32(123); got != want {
		t.Fatalf("broadcast keyframe tic=%d want=%d", got, want)
	}
	if got, want := sink.keyframeFlags[0], byte(1); got != want {
		t.Fatalf("broadcast keyframe flags=%d want=%d", got, want)
	}

	replayed := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected keyframe map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := replayed.unmarshalNetplayKeyframe(sink.keyframes[0]); err != nil {
		t.Fatalf("unmarshalNetplayKeyframe() error = %v", err)
	}
	if got, want := replayed.g.worldTic, 123; got != want {
		t.Fatalf("replayed worldTic=%d want=%d", got, want)
	}
}

func TestSaveLoadRoundTrip_PreservesRuntimeAddedPickupThings(t *testing.T) {
	slot := 93
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	idx := sg.g.appendRuntimeThing(mapdata.Thing{Type: 5, X: 24, Y: 0, Flags: skillMediumBits}, true)
	if idx < 0 {
		t.Fatal("appendRuntimeThing() failed")
	}
	sg.g.setThingPosFixed(idx, 24*fracUnit, 0)
	sg.g.setThingSupportState(idx, 0, 0, 128*fracUnit)

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			SkillLevel: 3,
			GameMode:   gameModeSingle,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if got, want := len(loaded.g.m.Things), len(loaded.g.thingCollected); got != want {
		t.Fatalf("thing count mismatch after load: len(m.Things)=%d len(thingCollected)=%d", got, want)
	}
	if got, want := len(loaded.g.m.Things), len(loaded.g.thingDropped); got != want {
		t.Fatalf("thing count mismatch after load: len(m.Things)=%d len(thingDropped)=%d", got, want)
	}

	loaded.g.runGameplayTic(moveCmd{forward: forwardMove[1]}, false, false)

	if !loaded.g.inventory.BlueKey {
		t.Fatal("runtime-added key not collected after save/load")
	}
}

func TestLoadGameRejectsUnknownHeader(t *testing.T) {
	sg := &sessionGame{}
	err := sg.unmarshalSaveGame([]byte("not-a-gddoom-save"))
	if !errors.Is(err, errBadSaveMagic) {
		t.Fatalf("err=%v want=%v", err, errBadSaveMagic)
	}
}

func TestSaveLoadRoundTrip_ContinuesKeyPickupPlayback(t *testing.T) {
	slot := 97
	path := saveGamePath(slot)
	_ = os.Remove(path)
	defer os.Remove(path)

	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 5, X: 24, Y: 0, Flags: skillMediumBits},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	if err := sg.SaveGameToSlot(slot); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			SkillLevel: 3,
			GameMode:   gameModeSingle,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.LoadGameFromSlot(slot); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loaded.g.runGameplayTic(moveCmd{forward: forwardMove[1]}, false, false)

	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not collected after save/load continuation")
	}
	if !loaded.g.thingCollected[1] {
		t.Fatal("key thing not marked collected after save/load continuation")
	}
}

func TestLoadGameRejectsBadChecksum(t *testing.T) {
	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
		},
	}
	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g
	data, err := sg.marshalSaveGame("Checksum")
	if err != nil {
		t.Fatalf("marshalSaveGame() error = %v", err)
	}
	data[len(data)-1] ^= 0xff

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				return cloneMapForRestart(base), nil
			},
		},
	}
	err = loaded.unmarshalSaveGame(data)
	if !errors.Is(err, errBadSaveChecksum) {
		t.Fatalf("err=%v want=%v", err, errBadSaveChecksum)
	}
}

func TestCompareSaveWADSourcesReportsMissingAndChecksumMismatch(t *testing.T) {
	expected := []saveWADSource{
		{Name: "DOOM.WAD", Hash: "aaa"},
		{Name: "PATCH.WAD", Hash: "bbb"},
	}
	actual := []saveWADSource{
		{Name: "DOOM.WAD", Hash: "aaa"},
		{Name: "PATCH.WAD", Hash: "ccc"},
	}
	if got := compareSaveWADSources(expected, actual); !strings.Contains(got, "checksum mismatch WAD 2: PATCH.WAD") {
		t.Fatalf("checksum warning=%q", got)
	}

	actual = []saveWADSource{{Name: "DOOM.WAD", Hash: "aaa"}}
	if got := compareSaveWADSources(expected, actual); !strings.Contains(got, "missing WAD 2: PATCH.WAD") {
		t.Fatalf("missing warning=%q", got)
	}
}

func TestSaveSlotMetadataFormatters(t *testing.T) {
	if got, want := formatSaveLevelLabel(mapdata.MapName("E1M3")), "3 (E1M3)"; got != want {
		t.Fatalf("formatSaveLevelLabel(E1M3)=%q want %q", got, want)
	}
	if got, want := formatSaveLevelLabel(mapdata.MapName("MAP07")), "7 (MAP07)"; got != want {
		t.Fatalf("formatSaveLevelLabel(MAP07)=%q want %q", got, want)
	}
	if got, want := formatSavePlaytime(35*65+17), "1:05"; got != want {
		t.Fatalf("formatSavePlaytime()=%q want %q", got, want)
	}
	wantTime := time.Date(2026, time.April, 12, 14, 3, 0, 0, time.Local)
	if got, want := formatSaveModTime(wantTime), "2026-04-12 14:03"; got != want {
		t.Fatalf("formatSaveModTime()=%q want %q", got, want)
	}
	if got, want := formatSaveWADNames([]saveWADSource{{Name: "DOOM.WAD"}, {Name: "PATCH.WAD"}}), "DOOM.WAD, PATCH.WAD"; got != want {
		t.Fatalf("formatSaveWADNames()=%q want %q", got, want)
	}
	if got, want := formatSaveHealthLabel(87), "87"; got != want {
		t.Fatalf("formatSaveHealthLabel()=%q want %q", got, want)
	}
	if got, want := formatSaveHealthLabel(0), "0"; got != want {
		t.Fatalf("formatSaveHealthLabel(0)=%q want %q", got, want)
	}
}

func TestSaveThumbnailDimensionsDoNotUpscale(t *testing.T) {
	if gotW, gotH := saveThumbnailDimensions(160, 100); gotW != 160 || gotH != 100 {
		t.Fatalf("saveThumbnailDimensions(160,100)=%dx%d want 160x100", gotW, gotH)
	}
	if gotW, gotH := saveThumbnailDimensions(640, 400); gotW != 320 || gotH != 200 {
		t.Fatalf("saveThumbnailDimensions(640,400)=%dx%d want 320x200", gotW, gotH)
	}
}

func TestLoadGameRefreshesPlayerSectorCache(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	base, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g
	wantSector := sg.g.sectorAt(sg.g.p.x, sg.g.p.y)
	if wantSector < 0 || len(base.Sectors) < 2 {
		t.Fatalf("invalid initial sector=%d sectors=%d", wantSector, len(base.Sectors))
	}
	sg.g.p.sector = (wantSector + 1) % len(base.Sectors)
	sg.g.p.subsector = -1

	data, err := sg.marshalSaveGame("test")
	if err != nil {
		t.Fatalf("marshalSaveGame() error = %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalSaveGame(data); err != nil {
		t.Fatalf("unmarshalSaveGame() error = %v", err)
	}
	if got := loaded.g.playerSector(); got != wantSector {
		t.Fatalf("playerSector()=%d want=%d", got, wantSector)
	}
}

func TestLoadGameRefreshesThingSectorCache(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	base, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	target := -1
	playerSector := sg.g.sectorAt(sg.g.p.x, sg.g.p.y)
	for i, th := range sg.g.m.Things {
		if th.Type == 1 {
			continue
		}
		if sec := sg.g.thingSectorCache[i]; sec >= 0 && sec != playerSector {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("no movable thing from a different sector found in E1M1")
	}

	sg.g.thingX[target] = sg.g.p.x
	sg.g.thingY[target] = sg.g.p.y
	sg.g.m.Things[target].X = int16(sg.g.p.x >> fracBits)
	sg.g.m.Things[target].Y = int16(sg.g.p.y >> fracBits)

	data, err := sg.marshalSaveGame("thing-sector")
	if err != nil {
		t.Fatalf("marshalSaveGame() error = %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalSaveGame(data); err != nil {
		t.Fatalf("unmarshalSaveGame() error = %v", err)
	}

	wantSector := loaded.g.sectorAt(loaded.g.thingX[target], loaded.g.thingY[target])
	if got := loaded.g.thingSectorCache[target]; got != wantSector {
		t.Fatalf("thingSectorCache[%d]=%d want=%d", target, got, wantSector)
	}
}

func TestLoadGameRoundTrip_MatchesLiveMapState(t *testing.T) {
	wadPath := findLocalWADOrSkip(t, "DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	base, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	sg.g.runGameplayTic(moveCmd{forward: forwardMove[1]}, false, false)
	sg.g.worldTic = 123
	if len(sg.g.m.Sectors) == 0 || len(sg.g.m.Sidedefs) == 0 {
		t.Fatal("E1M1 missing sectors or sidedefs")
	}
	sg.g.m.Sectors[0].Light = 111
	sg.g.m.Sectors[0].Special = 7
	sg.g.m.Sectors[0].FloorPic = "NUKAGE1"
	sg.g.m.Sectors[0].CeilingPic = "FLOOR4_8"
	sg.g.m.Sidedefs[0].Top = "SW1BRCOM"
	sg.g.m.Sidedefs[0].Bottom = "BRICK1"
	sg.g.m.Sidedefs[0].Mid = "STARTAN3"

	wantState := captureGameSaveState(sg.g)
	wantLive := captureLiveRuntimeRoundTripState(sg.g)

	data, err := sg.marshalSaveGame("diff")
	if err != nil {
		t.Fatalf("marshalSaveGame() error = %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			SkillLevel: 3,
			GameMode:   gameModeSingle,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalSaveGame(data); err != nil {
		t.Fatalf("unmarshalSaveGame() error = %v", err)
	}

	gotState := captureGameSaveState(loaded.g)
	gotLive := captureLiveRuntimeRoundTripState(loaded.g)
	normalizeRoundTripComparableState := func(s *gameSaveState) {
		s.UseText = ""
		s.UseFlash = 0
		s.Inventory.Weapons = normalizeOwnedWeapons(s.Inventory.Weapons)
	}
	normalizeRoundTripComparableState(&wantState)
	normalizeRoundTripComparableState(&gotState)
	for _, tc := range []struct {
		name string
		got  any
		want any
	}{
		{name: "player", got: gotState.Player, want: wantState.Player},
		{name: "view", got: gotState.View, want: wantState.View},
		{name: "thing positions", got: []any{gotState.ThingX, gotState.ThingY, gotState.ThingZState, gotState.ThingFloorState, gotState.ThingCeilState}, want: []any{wantState.ThingX, wantState.ThingY, wantState.ThingZState, wantState.ThingFloorState, wantState.ThingCeilState}},
		{name: "thing momentum", got: []any{gotState.ThingMomX, gotState.ThingMomY, gotState.ThingMomZ}, want: []any{wantState.ThingMomX, wantState.ThingMomY, wantState.ThingMomZ}},
		{name: "thing flags", got: []any{gotState.ThingCollected, gotState.ThingDropped, gotState.ThingSupportValid, gotState.ThingAggro, gotState.ThingTargetPlayer, gotState.ThingJustAtk, gotState.ThingJustHit, gotState.ThingSkullFly, gotState.ThingResumeChaseNow, gotState.ThingDead, gotState.ThingAmbush, gotState.ThingInFloat, gotState.ThingGibbed}, want: []any{wantState.ThingCollected, wantState.ThingDropped, wantState.ThingSupportValid, wantState.ThingAggro, wantState.ThingTargetPlayer, wantState.ThingJustAtk, wantState.ThingJustHit, wantState.ThingSkullFly, wantState.ThingResumeChaseNow, wantState.ThingDead, wantState.ThingAmbush, wantState.ThingInFloat, wantState.ThingGibbed}},
		{name: "thing thinker", got: []any{gotState.ThingHP, gotState.ThingTargetIdx, gotState.ThingThreshold, gotState.ThingCooldown, gotState.ThingMoveDir, gotState.ThingMoveCount, gotState.ThingReactionTics, gotState.ThingWakeTics, gotState.ThingLastLook, gotState.ThingGibTick, gotState.ThingDeathTics, gotState.ThingAttackTics, gotState.ThingAttackPhase, gotState.ThingAttackFireTics, gotState.ThingPainTics, gotState.ThingThinkWait, gotState.ThingDoomState, gotState.ThingState, gotState.ThingStateTics, gotState.ThingStatePhase}, want: []any{wantState.ThingHP, wantState.ThingTargetIdx, wantState.ThingThreshold, wantState.ThingCooldown, wantState.ThingMoveDir, wantState.ThingMoveCount, wantState.ThingReactionTics, wantState.ThingWakeTics, wantState.ThingLastLook, wantState.ThingGibTick, wantState.ThingDeathTics, wantState.ThingAttackTics, wantState.ThingAttackPhase, wantState.ThingAttackFireTics, wantState.ThingPainTics, wantState.ThingThinkWait, wantState.ThingDoomState, wantState.ThingState, wantState.ThingStateTics, wantState.ThingStatePhase}},
		{name: "world stats", got: gotState.Stats, want: wantState.Stats},
		{name: "world inventory", got: gotState.Inventory, want: wantState.Inventory},
		{name: "world player", got: []any{gotState.WorldTic, gotState.IsDead, gotState.PlayerMobjHealth}, want: []any{wantState.WorldTic, wantState.IsDead, wantState.PlayerMobjHealth}},
		{name: "world order", got: []any{gotState.PlayerBlockOrder, gotState.NextThinkerOrder, gotState.NextBlockmapOrder}, want: []any{wantState.PlayerBlockOrder, wantState.NextThinkerOrder, wantState.NextBlockmapOrder}},
		{name: "world secrets", got: []any{gotState.SecretFound, gotState.SecretsFound, gotState.SecretsTotal, gotState.SectorSoundTarget}, want: []any{wantState.SecretFound, wantState.SecretsFound, wantState.SecretsTotal, wantState.SectorSoundTarget}},
		{name: "world flashes", got: []any{gotState.DamageFlashTic, gotState.BonusFlashTic}, want: []any{wantState.DamageFlashTic, wantState.BonusFlashTic}},
		{name: "map dynamic", got: []any{gotState.Sidedefs, gotState.Sectors, gotState.SectorLightFx, gotState.SectorFloor, gotState.SectorCeil, gotState.LineSpecial}, want: []any{wantState.Sidedefs, wantState.Sectors, wantState.SectorLightFx, wantState.SectorFloor, wantState.SectorCeil, wantState.LineSpecial}},
		{name: "thinker maps", got: []any{gotState.Doors, gotState.Floors, gotState.Plats, gotState.Ceilings, gotState.DelayedSwitchReverts}, want: []any{wantState.Doors, wantState.Floors, wantState.Plats, wantState.Ceilings, wantState.DelayedSwitchReverts}},
	} {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Fatalf("%s mismatch after round trip: got=%#v want=%#v", tc.name, tc.got, tc.want)
		}
	}
	if !reflect.DeepEqual(gotLive, wantLive) {
		t.Fatalf("live runtime mismatch after round trip: got=%#v want=%#v", gotLive, wantLive)
	}
}

func TestNetplayKeyframeRoundTrip(t *testing.T) {
	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 3004, X: 64, Y: 0},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g
	sg.g.worldTic = 777
	sg.g.p.x = 42 * fracUnit
	sg.g.inventory.BlueKey = true
	sg.g.sectorFloor[0] = 12 * fracUnit
	sg.g.sectorCeil[0] = 144 * fracUnit
	sg.g.m.Sectors[0].FloorHeight = 12
	sg.g.m.Sectors[0].CeilingHeight = 144

	data, err := sg.marshalNetplayKeyframe()
	if err != nil {
		t.Fatalf("marshalNetplayKeyframe() error = %v", err)
	}
	if !bytes.HasPrefix(data, keyframeMagic) {
		t.Fatalf("keyframe missing magic prefix: %q", data[:min(len(data), len(keyframeMagic))])
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalNetplayKeyframe(data); err != nil {
		t.Fatalf("unmarshalNetplayKeyframe() error = %v", err)
	}
	if loaded.g == nil {
		t.Fatal("loaded keyframe game is nil")
	}
	if loaded.g.worldTic != 777 {
		t.Fatalf("worldTic=%d want=777", loaded.g.worldTic)
	}
	if loaded.g.p.x != 42*fracUnit {
		t.Fatalf("player x=%d want=%d", loaded.g.p.x, 42*fracUnit)
	}
	if loaded.g.m.Sectors[0].FloorHeight != 12 || loaded.g.m.Sectors[0].CeilingHeight != 144 {
		t.Fatalf("map sector heights=(%d,%d) want=(12,144)", loaded.g.m.Sectors[0].FloorHeight, loaded.g.m.Sectors[0].CeilingHeight)
	}
	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not restored")
	}
}

func TestNetplayKeyframeRoundTrip_ContinuesKeyPickupPlayback(t *testing.T) {
	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 5, X: 24, Y: 0, Flags: skillMediumBits},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	data, err := sg.marshalNetplayKeyframe()
	if err != nil {
		t.Fatalf("marshalNetplayKeyframe() error = %v", err)
	}

	loaded := &sessionGame{
		opts: Options{
			Width:      doomLogicalW,
			Height:     doomLogicalH,
			PlayerSlot: 1,
			SkillLevel: 3,
			GameMode:   gameModeSingle,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalNetplayKeyframe(data); err != nil {
		t.Fatalf("unmarshalNetplayKeyframe() error = %v", err)
	}

	loaded.g.runGameplayTic(moveCmd{forward: forwardMove[1]}, false, false)

	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not collected after keyframe continuation")
	}
	if !loaded.g.thingCollected[1] {
		t.Fatal("key thing not marked collected after keyframe continuation")
	}
}

func TestNetplayKeyframeRoundTrip_WatchUpdateContinuesKeyPickupPlayback(t *testing.T) {
	base := &mapdata.Map{
		Name: "MAP01",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 90},
			{Type: 5, X: 24, Y: 0, Flags: skillMediumBits},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128},
		},
	}

	sg := &sessionGame{
		current:         base.Name,
		currentTemplate: cloneMapForRestart(base),
		opts:            Options{Width: doomLogicalW, Height: doomLogicalH, PlayerSlot: 1, SkillLevel: 3, GameMode: gameModeSingle},
	}
	sg.g = sg.buildGame(cloneMapForRestart(base), sg.opts)
	sg.rt = sg.g

	data, err := sg.marshalNetplayKeyframe()
	if err != nil {
		t.Fatalf("marshalNetplayKeyframe() error = %v", err)
	}

	src := &testLiveTicSource{
		tics: []demo.Tic{{Forward: int8(forwardMove[1])}},
	}
	loaded := &sessionGame{
		opts: Options{
			Width:         doomLogicalW,
			Height:        doomLogicalH,
			PlayerSlot:    1,
			SkillLevel:    3,
			GameMode:      gameModeSingle,
			LiveTicSource: src,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapdata.MapName(mapName) != base.Name {
					t.Fatalf("unexpected map load %q want %q", mapName, base.Name)
				}
				return cloneMapForRestart(base), nil
			},
		},
	}
	if err := loaded.unmarshalNetplayKeyframe(data); err != nil {
		t.Fatalf("unmarshalNetplayKeyframe() error = %v", err)
	}

	if err := loaded.g.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if !loaded.g.inventory.BlueKey {
		t.Fatal("blue key not collected after keyframe watch update")
	}
	if !loaded.g.thingCollected[1] {
		t.Fatal("key thing not marked collected after keyframe watch update")
	}
}
