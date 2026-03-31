package doomruntime

import (
	"testing"

	"gddoom/internal/gameplay"
	"gddoom/internal/mapdata"
)

func TestSessionPersistentSettingsCaptureAndApply(t *testing.T) {
	pal := make([]byte, 256*4)
	sg := &sessionGame{
		g: &game{
			opts: Options{
				SourcePortMode:            true,
				MouseLook:                 false,
				MusicVolume:               0.9,
				SFXVolume:                 0.5,
				SourcePortThingRenderMode: "sprites",
				KageShader:                true,
				DoomPaletteRGBA:           pal,
			},
			detailLevel:       3,
			rotateView:        false,
			alwaysRun:         true,
			autoWeaponSwitch:  false,
			showLegend:        false,
			paletteLUTEnabled: false,
			gammaLevel:        5,
			crtEnabled:        true,
			parity: automapParityState{
				reveal: revealNormal,
				iddt:   2,
			},
		},
		opts: Options{
			SourcePortMode:            true,
			MouseLook:                 true,
			MusicVolume:               0.9,
			SFXVolume:                 0.66,
			AlwaysRun:                 false,
			AutoWeaponSwitch:          true,
			SourcePortThingRenderMode: "glyphs",
			KageShader:                true,
			DoomPaletteRGBA:           pal,
		},
	}

	sg.capturePersistentSettings()
	sg.applyPersistentSettingsToOptions()

	if sg.opts.MouseLook {
		t.Fatal("options mouselook should be persisted as OFF")
	}
	if sg.opts.MusicVolume != 0.9 {
		t.Fatalf("options music volume=%.2f want 0.9", sg.opts.MusicVolume)
	}
	if sg.opts.SFXVolume != 0.5 {
		t.Fatalf("options sfx volume=%.2f want 0.5", sg.opts.SFXVolume)
	}
	if !sg.opts.AlwaysRun {
		t.Fatal("options always-run should be persisted as ON")
	}
	if sg.opts.AutoWeaponSwitch {
		t.Fatal("options auto-weapon-switch should be persisted as OFF")
	}
	if sg.opts.SourcePortThingRenderMode != "sprites" {
		t.Fatalf("options thing render mode=%q want sprites", sg.opts.SourcePortThingRenderMode)
	}

	dst := &game{
		opts: Options{
			SourcePortMode:            true,
			SourcePortThingRenderMode: "glyphs",
			KageShader:                true,
			DoomPaletteRGBA:           pal,
		},
		parity: automapParityState{
			reveal: revealAllMap,
		},
	}
	sg.applyPersistentSettingsToGame(dst)

	if dst.detailLevel != 3 {
		t.Fatalf("detailLevel=%d want 3", dst.detailLevel)
	}
	if dst.rotateView {
		t.Fatal("rotateView should be persisted as OFF")
	}
	if !dst.alwaysRun || dst.autoWeaponSwitch {
		t.Fatal("always-run/auto-weapon-switch persistence mismatch")
	}
	if dst.opts.SourcePortThingRenderMode != "sprites" {
		t.Fatalf("thingRenderMode=%q want sprites", dst.opts.SourcePortThingRenderMode)
	}
	if dst.opts.MusicVolume != 0.9 {
		t.Fatalf("musicVolume=%.2f want 0.9", dst.opts.MusicVolume)
	}
	if dst.opts.SFXVolume != 0.5 {
		t.Fatalf("sfxVolume=%.2f want 0.5", dst.opts.SFXVolume)
	}
	if dst.showLegend {
		t.Fatal("showLegend should be persisted as OFF")
	}
	if dst.paletteLUTEnabled {
		t.Fatal("paletteLUT should be persisted as OFF")
	}
	if dst.gammaLevel != 4 {
		t.Fatalf("gammaLevel=%d want 4", dst.gammaLevel)
	}
	if !dst.crtEnabled {
		t.Fatal("crt should be persisted as ON")
	}
	if dst.parity.reveal != revealNormal || dst.parity.iddt != 2 {
		t.Fatalf("parity persisted as reveal=%d iddt=%d", dst.parity.reveal, dst.parity.iddt)
	}
}

func TestSessionPersistentSettingsApplyClampsInvalidValues(t *testing.T) {
	pal := make([]byte, 256*4)
	sg := &sessionGame{
		settings: gameplay.PersistentSettings{
			DetailLevel: 99,
			GammaLevel:  99,
			MusicVolume: -1,
			SFXVolume:   2,
			Reveal:      int(revealNormal),
			IDDT:        99,
			PaletteLUT:  true,
			CRTEnabled:  true,
		},
	}
	dst := &game{
		opts: Options{
			SourcePortMode:  true,
			KageShader:      false,
			DoomPaletteRGBA: pal,
		},
		parity: automapParityState{
			reveal: revealNormal,
		},
	}
	sg.applyPersistentSettingsToGame(dst)

	if dst.detailLevel != len(sourcePortDetailDivisors)-1 {
		t.Fatalf("detailLevel clamp failed: got %d want %d", dst.detailLevel, len(sourcePortDetailDivisors)-1)
	}
	if dst.gammaLevel != doomGammaLevels-1 {
		t.Fatalf("gamma clamp failed: got %d want %d", dst.gammaLevel, doomGammaLevels-1)
	}
	if dst.opts.MusicVolume != 0 {
		t.Fatalf("music volume clamp failed: got %.2f want 0", dst.opts.MusicVolume)
	}
	if dst.opts.SFXVolume != 1 {
		t.Fatalf("sfx volume clamp failed: got %.2f want 1", dst.opts.SFXVolume)
	}
	if dst.parity.reveal != revealNormal {
		t.Fatalf("reveal default for sourceport should be normal, got %d", dst.parity.reveal)
	}
	if dst.parity.iddt != 2 {
		t.Fatalf("iddt clamp failed: got %d want 2", dst.parity.iddt)
	}
	if dst.paletteLUTEnabled {
		t.Fatal("palette LUT should be disabled when kage/palette support is unavailable")
	}
	if dst.crtEnabled {
		t.Fatal("crt should be disabled when kage is unavailable")
	}
}

func TestRebuildGameWithPersistentSettings_PersistsFaithfulDetailAndGamma(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			SourcePortMode: false,
			Width:          doomLogicalW,
			Height:         doomLogicalH,
		},
		g: &game{
			opts: Options{
				SourcePortMode: false,
				Width:          doomLogicalW,
				Height:         doomLogicalH,
			},
			detailLevel: 1,
			gammaLevel:  4,
		},
	}
	sg.rebuildGameWithPersistentSettings(&mapdata.Map{})
	if sg.g == nil {
		t.Fatal("rebuild should create a new game")
	}
	if sg.g.detailLevel != 1 {
		t.Fatalf("faithful detail level=%d want 1", sg.g.detailLevel)
	}
	if sg.g.gammaLevel != 4 {
		t.Fatalf("faithful gamma level=%d want 4", sg.g.gammaLevel)
	}
}

func TestRebuildGameWithPersistentSettings_PersistsSourcePortDetailAndGamma(t *testing.T) {
	sg := &sessionGame{
		opts: Options{
			SourcePortMode: true,
			Width:          1280,
			Height:         800,
		},
		g: &game{
			opts: Options{
				SourcePortMode: true,
				Width:          1280,
				Height:         800,
			},
			detailLevel: 2,
			gammaLevel:  4,
		},
	}
	sg.rebuildGameWithPersistentSettings(&mapdata.Map{})
	if sg.g == nil {
		t.Fatal("rebuild should create a new game")
	}
	if sg.g.detailLevel != 2 {
		t.Fatalf("sourceport detail level=%d want 2", sg.g.detailLevel)
	}
	if sg.g.gammaLevel != 4 {
		t.Fatalf("sourceport gamma level=%d want 4", sg.g.gammaLevel)
	}
}

func TestFinishIntermissionCarriesWeaponsAmmoAndClearsKeysAndPowers(t *testing.T) {
	cur := &mapdata.Map{
		Name: "E1M1",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 0},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128, Light: 160},
		},
	}
	next := &mapdata.Map{
		Name: "E1M2",
		Things: []mapdata.Thing{
			{Type: 1, X: 0, Y: 0, Angle: 0},
		},
		Sectors: []mapdata.Sector{
			{FloorHeight: 0, CeilingHeight: 128, Light: 160},
		},
	}
	sg := &sessionGame{
		opts: Options{
			Width:  doomLogicalW,
			Height: doomLogicalH,
		},
		gameFactory: newGame,
		g:           newGame(cloneMapForRestart(cur), Options{Width: doomLogicalW, Height: doomLogicalH}),
		current:     cur.Name,
	}
	sg.rt = sg.g
	sg.g.inventory.BlueKey = true
	sg.g.inventory.RedKey = true
	sg.g.inventory.YellowKey = true
	sg.g.inventory.Backpack = true
	sg.g.inventory.Strength = true
	sg.g.inventory.RadSuitTics = 42
	sg.g.inventory.ReadyWeapon = weaponShotgun
	sg.g.inventory.PendingWeapon = weaponRocketLauncher
	sg.g.inventory.Weapons[2001] = true
	sg.g.inventory.Weapons[2003] = true
	sg.g.stats = playerStats{
		Health:    137,
		Armor:     88,
		ArmorType: 2,
		Bullets:   90,
		Shells:    23,
		Rockets:   7,
		Cells:     41,
	}

	sg.startIntermission(cloneMapForRestart(next), next.Name, false)
	sg.finishIntermission()

	if sg.g == nil {
		t.Fatal("finishIntermission should rebuild the runtime")
	}
	if sg.g.m == nil || sg.g.m.Name != next.Name {
		t.Fatalf("next map=%v want %s", sg.g.m, next.Name)
	}
	if sg.g.stats != (playerStats{Health: 137, Armor: 88, ArmorType: 2, Bullets: 90, Shells: 23, Rockets: 7, Cells: 41}) {
		t.Fatalf("stats carryover mismatch: %+v", sg.g.stats)
	}
	if sg.g.inventory.BlueKey || sg.g.inventory.RedKey || sg.g.inventory.YellowKey {
		t.Fatal("keys should be cleared on level completion")
	}
	if sg.g.inventory.Strength {
		t.Fatal("berserk strength should be cleared on level completion")
	}
	if sg.g.inventory.RadSuitTics != 0 {
		t.Fatalf("radsuit=%d want 0", sg.g.inventory.RadSuitTics)
	}
	if !sg.g.inventory.Backpack {
		t.Fatal("backpack should carry over")
	}
	if !sg.g.inventory.Weapons[2001] || !sg.g.inventory.Weapons[2003] {
		t.Fatal("owned weapons should carry over")
	}
	if sg.g.inventory.ReadyWeapon != weaponShotgun {
		t.Fatalf("ready weapon=%v want shotgun", sg.g.inventory.ReadyWeapon)
	}
	if sg.g.inventory.PendingWeapon != 0 {
		t.Fatalf("pending weapon=%v want cleared", sg.g.inventory.PendingWeapon)
	}
	if sg.levelCarryover != nil {
		t.Fatal("level carryover should be cleared after intermission")
	}
}

func TestRestartMapForRespawnSingleUsesPristineTemplate(t *testing.T) {
	template := &mapdata.Map{
		Name:    "E1M1",
		Sectors: []mapdata.Sector{{Light: 160}},
	}
	mutated := cloneMapForRestart(template)
	mutated.Sectors[0].Light = 32
	sg := &sessionGame{
		opts: Options{
			GameMode: gameModeSingle,
		},
		g: &game{
			m: mutated,
		},
		currentTemplate: template,
	}
	got := sg.restartMapForRespawn()
	if got == nil {
		t.Fatal("restart map should not be nil")
	}
	if got == mutated {
		t.Fatal("single-player restart should not reuse mutated map pointer")
	}
	if got.Sectors[0].Light != 160 {
		t.Fatalf("restart map light=%d want=160", got.Sectors[0].Light)
	}
}

func TestRestartMapForRespawnMultiplayerKeepsCurrentMapState(t *testing.T) {
	cur := &mapdata.Map{
		Name:    "E1M1",
		Sectors: []mapdata.Sector{{Light: 32}},
	}
	sg := &sessionGame{
		opts: Options{
			GameMode: gameModeCoop,
		},
		g: &game{
			m: cur,
		},
		currentTemplate: &mapdata.Map{Name: "E1M1", Sectors: []mapdata.Sector{{Light: 160}}},
	}
	got := sg.restartMapForRespawn()
	if got != cur {
		t.Fatal("non-single respawn should keep current map state")
	}
}
