package automap

import (
	"testing"

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
				LineColorMode:             "doom",
				SourcePortThingRenderMode: "sprites",
				KageShader:                true,
				DoomPaletteRGBA:           pal,
			},
			detailLevel:       3,
			rotateView:        false,
			walkRender:        walkRendererPseudo,
			pseudo3D:          true,
			alwaysRun:         true,
			autoWeaponSwitch:  false,
			showLegend:        false,
			mapTexDiag:        true,
			floor2DPath:       floor2DPathSubsector,
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
			LineColorMode:             "parity",
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
	if sg.opts.LineColorMode != "doom" {
		t.Fatalf("options line color mode=%q want doom", sg.opts.LineColorMode)
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
	if dst.walkRender != walkRendererPseudo || !dst.pseudo3D {
		t.Fatal("walk renderer should persist pseudo mode")
	}
	if !dst.alwaysRun || dst.autoWeaponSwitch {
		t.Fatal("always-run/auto-weapon-switch persistence mismatch")
	}
	if dst.opts.LineColorMode != "doom" {
		t.Fatalf("lineColorMode=%q want doom", dst.opts.LineColorMode)
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
	if !dst.mapTexDiag {
		t.Fatal("mapTexDiag should be persisted as ON")
	}
	if dst.floor2DPath != floor2DPathSubsector {
		t.Fatalf("floor2DPath=%d want %d", dst.floor2DPath, floor2DPathSubsector)
	}
	if dst.paletteLUTEnabled {
		t.Fatal("paletteLUT should be persisted as OFF")
	}
	if dst.gammaLevel != 5 {
		t.Fatalf("gammaLevel=%d want 5", dst.gammaLevel)
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
		settings: sessionPersistentSettings{
			detailLevel: 99,
			walkRender:  walkRendererPseudo,
			floor2DPath: floor2DPathMode(99),
			gammaLevel:  99,
			musicVolume: -1,
			sfxVolume:   2,
			reveal:      revealMode(99),
			iddt:        99,
			paletteLUT:  true,
			crtEnabled:  true,
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
	if dst.floor2DPath != floor2DPathRasterized {
		t.Fatalf("floor2DPath clamp failed: got %d want %d", dst.floor2DPath, floor2DPathRasterized)
	}
	if dst.gammaLevel != len(gammaTargets)-1 {
		t.Fatalf("gamma clamp failed: got %d want %d", dst.gammaLevel, len(gammaTargets)-1)
	}
	if dst.opts.MusicVolume != 0 {
		t.Fatalf("music volume clamp failed: got %.2f want 0", dst.opts.MusicVolume)
	}
	if dst.opts.SFXVolume != 1 {
		t.Fatalf("sfx volume clamp failed: got %.2f want 1", dst.opts.SFXVolume)
	}
	if dst.parity.reveal != revealAllMap {
		t.Fatalf("reveal default for sourceport should be allmap, got %d", dst.parity.reveal)
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
			gammaLevel:  6,
		},
	}
	sg.rebuildGameWithPersistentSettings(&mapdata.Map{})
	if sg.g == nil {
		t.Fatal("rebuild should create a new game")
	}
	if sg.g.detailLevel != 2 {
		t.Fatalf("sourceport detail level=%d want 2", sg.g.detailLevel)
	}
	if sg.g.gammaLevel != 6 {
		t.Fatalf("sourceport gamma level=%d want 6", sg.g.gammaLevel)
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
