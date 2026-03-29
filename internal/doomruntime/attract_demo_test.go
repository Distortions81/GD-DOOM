package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionmusic"
)

func TestStartFrontendBeginsOnTitlePicAttractPage(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{{
				Path:   "DEMO1",
				Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
				Tics:   []DemoTic{{Forward: 25}},
			}},
		},
		g: base,
	}

	sg.startFrontend()

	if !sg.frontend.Active {
		t.Fatal("frontend should be active")
	}
	if sg.frontend.MenuActive {
		t.Fatal("title loop should not start with menu open")
	}
	if got := sg.frontend.AttractPage; got != "TITLEPIC" {
		t.Fatalf("attractPage=%q want TITLEPIC", got)
	}
	if sg.g != nil && sg.g.opts.DemoScript != nil {
		t.Fatal("title page should not immediately start demo playback")
	}
}

func TestStartFrontendQueuesTitleMusicWhileStartupLocked(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap:            boot,
		startupMusicLocked: true,
		musicCtl:           &sessionmusic.Playback{},
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{{
				Path:   "DEMO1",
				Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
				Tics:   []DemoTic{{Forward: 25}},
			}},
		},
		g: base,
	}

	sg.startFrontend()

	if got := sg.startupMusicPending.kind; got != musicPlaybackSourceTitle {
		t.Fatalf("startupMusicPending.kind=%d want title", got)
	}
	if got := sg.currentMusicSource.kind; got != musicPlaybackSourceNone {
		t.Fatalf("currentMusicSource.kind=%d want none while startup locked", got)
	}
}

func TestReleaseStartupMusicStartsQueuedTitleAfterBoot(t *testing.T) {
	sg := &sessionGame{
		startupMusicLocked:      true,
		startupMusicVisualReady: true,
		startupMusicPending:     musicPlaybackSource{kind: musicPlaybackSourceTitle},
		musicCtl:                &sessionmusic.Playback{},
	}

	sg.releaseStartupMusicIfReady()

	if sg.startupMusicLocked {
		t.Fatal("startupMusicLocked should be cleared")
	}
	if got := sg.startupMusicPending.kind; got != musicPlaybackSourceNone {
		t.Fatalf("startupMusicPending.kind=%d want none", got)
	}
	if got := sg.currentMusicSource.kind; got != musicPlaybackSourceTitle {
		t.Fatalf("currentMusicSource.kind=%d want title", got)
	}
}

func TestReleaseStartupMusicWaitsForBootFrame(t *testing.T) {
	sg := &sessionGame{
		startupMusicLocked:  true,
		startupMusicPending: musicPlaybackSource{kind: musicPlaybackSourceTitle},
		musicCtl:            &sessionmusic.Playback{},
	}

	sg.releaseStartupMusicIfReady()

	if !sg.startupMusicLocked {
		t.Fatal("startupMusicLocked should remain set before a boot frame is drawn")
	}
	if got := sg.currentMusicSource.kind; got != musicPlaybackSourceNone {
		t.Fatalf("currentMusicSource.kind=%d want none before a boot frame is drawn", got)
	}
}

func TestAdvanceFrontendAttractStartsDemoPlayback(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{{
				Path:   "DEMO1",
				Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
				Tics:   []DemoTic{{Forward: 25}},
			}},
		},
		g: base,
	}

	sg.startFrontend()
	if !sg.advanceFrontendAttract() {
		t.Fatal("advanceFrontendAttract() = false, want true")
	}
	if sg.g == nil || sg.g.opts.DemoScript == nil {
		t.Fatal("expected attract demo game to be active")
	}
	if sg.g.opts.DemoQuitOnComplete {
		t.Fatal("frontend attract demo should not use benchmark quit mode")
	}
}

func TestAttractDemoBuildDoesNotInheritPriorRNGState(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	demo := &DemoScript{
		Path:   "DEMO1",
		Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
		Tics:   []DemoTic{{Forward: 25}},
	}

	doomrand.SetState(123, 231)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{demo},
		},
		g: base,
	}
	if !sg.startAttractDemoByName("DEMO1") {
		t.Fatal("startAttractDemoByName() = false, want true")
	}
	gotRndA, gotPRndA := doomrand.State()

	doomrand.SetState(17, 99)
	sg = &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{demo},
		},
		g: base,
	}
	if !sg.startAttractDemoByName("DEMO1") {
		t.Fatal("startAttractDemoByName() = false, want true")
	}
	gotRndB, gotPRndB := doomrand.State()

	if gotRndA != gotRndB || gotPRndA != gotPRndB {
		t.Fatalf("attract demo build inherited prior RNG state: first=%d/%d second=%d/%d", gotRndA, gotPRndA, gotRndB, gotPRndB)
	}
}

func TestAttractDemoPlaybackIgnoresLaunchGameplayOverrides(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	demo := &DemoScript{
		Path: "DEMO1",
		Header: DemoHeader{
			Version:       demoVersion110,
			Skill:         1,
			Episode:       1,
			Map:           1,
			Fast:          true,
			Respawn:       true,
			NoMonsters:    true,
			ConsolePlayer: 0,
			PlayerInGame:  [4]bool{true},
		},
		Tics: []DemoTic{{Forward: 25}},
	}
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SkillLevel:       5,
			GameMode:         gameModeDeathmatch,
			ShowNoSkillItems: true,
			ShowAllItems:     true,
			AutoWeaponSwitch: false,
			CheatLevel:       3,
			Invulnerable:     true,
			AllCheats:        true,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{demo},
		},
		g: base,
	}

	if !sg.startAttractDemoByName("DEMO1") {
		t.Fatal("startAttractDemoByName() = false, want true")
	}
	if sg.g == nil {
		t.Fatal("expected active demo game")
	}
	if got := sg.g.opts.SkillLevel; got != 2 {
		t.Fatalf("SkillLevel=%d want 2", got)
	}
	if got := sg.g.opts.GameMode; got != gameModeSingle {
		t.Fatalf("GameMode=%q want %q", got, gameModeSingle)
	}
	if !sg.g.opts.FastMonsters || !sg.g.opts.RespawnMonsters || !sg.g.opts.NoMonsters {
		t.Fatalf("demo header flags not applied: fast=%t respawn=%t nomonsters=%t", sg.g.opts.FastMonsters, sg.g.opts.RespawnMonsters, sg.g.opts.NoMonsters)
	}
	if sg.g.opts.ShowNoSkillItems || sg.g.opts.ShowAllItems {
		t.Fatalf("demo playback inherited item filter overrides: shownoskill=%t showall=%t", sg.g.opts.ShowNoSkillItems, sg.g.opts.ShowAllItems)
	}
	if !sg.g.opts.AutoWeaponSwitch {
		t.Fatal("demo playback should force Doom-style auto weapon switching")
	}
	if sg.g.cheatLevel != 0 || sg.g.invulnerable || sg.g.opts.AllCheats {
		t.Fatalf("demo playback inherited cheats: cheat=%d invuln=%t allcheats=%t", sg.g.cheatLevel, sg.g.invulnerable, sg.g.opts.AllCheats)
	}
}

func TestAttractDemoPlaybackResetsPersistentAutomapCheatState(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	base.parity.reveal = revealAllMap
	base.parity.iddt = 2
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: false,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{{
				Path:   "DEMO1",
				Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
				Tics:   []DemoTic{{Forward: 25}},
			}},
		},
		g: base,
	}

	if !sg.startAttractDemoByName("DEMO1") {
		t.Fatal("startAttractDemoByName() = false, want true")
	}
	if sg.g == nil {
		t.Fatal("expected active demo game")
	}
	if sg.g.parity.reveal != revealNormal || sg.g.parity.iddt != 0 {
		t.Fatalf("attract demo inherited automap cheat state: reveal=%d iddt=%d", sg.g.parity.reveal, sg.g.parity.iddt)
	}
}

func TestStartGameFromFrontendClearsAttractDemoScript(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap:         boot,
		current:         boot.Name,
		currentTemplate: cloneMapForRestart(boot),
		opts: Options{
			SkillLevel: 2,
		},
		g: base,
	}
	sg.g.opts.DemoScript = &DemoScript{
		Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
		Tics:   []DemoTic{{Forward: 25}},
	}
	sg.g.opts.DemoQuitOnComplete = false
	sg.frontend.Active = true

	sg.startGameFromFrontend(4)

	if sg.frontend.Active {
		t.Fatal("frontend should be closed after starting game")
	}
	if sg.g == nil {
		t.Fatal("expected active game")
	}
	if sg.g.opts.DemoScript != nil {
		t.Fatal("gameplay should not inherit attract demo script")
	}
	if got := sg.g.opts.SkillLevel; got != 4 {
		t.Fatalf("skill=%d want=4", got)
	}
}

func TestAttractDemoCompletionAdvancesSequence(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			SourcePortMode: base.opts.SourcePortMode,
			DemoMapLoader: func(demo *DemoScript) (*mapdata.Map, error) {
				return cloneMapForRestart(boot), nil
			},
			AttractDemos: []*DemoScript{{
				Path:   "DEMO1",
				Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}},
				Tics:   []DemoTic{{Forward: 25}},
			}},
		},
		g: base,
	}

	sg.startFrontend()
	if !sg.advanceFrontendAttract() {
		t.Fatal("advanceFrontendAttract() = false, want true")
	}
	if sg.g == nil || sg.g.opts.DemoScript == nil {
		t.Fatal("expected attract demo to start")
	}

	if err := sg.Update(); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	if err := sg.Update(); err != nil {
		t.Fatalf("update 2: %v", err)
	}

	if sg.g != nil && sg.g.opts.DemoScript != nil {
		t.Fatal("expected attract demo to end and clear active demo playback")
	}
	if got := sg.frontend.AttractPage; got != "CREDIT" {
		t.Fatalf("attractPage=%q want CREDIT", got)
	}
}

func TestStartFrontendOpensMenuWhenConfiguredWithoutAttractDemos(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	boot := cloneMapForRestart(base.m)
	sg := &sessionGame{
		bootMap: boot,
		opts: Options{
			OpenMenuOnFrontendStart: true,
		},
		g: base,
	}

	sg.startFrontend()

	if !sg.frontend.Active {
		t.Fatal("frontend should be active")
	}
	if sg.frontend.MenuActive {
		t.Fatal("frontend menu should wait until after boot presentation")
	}
	if got := sg.frontend.AttractPage; got != "TITLEPIC" {
		t.Fatalf("attractPage=%q want TITLEPIC", got)
	}
	if got := sg.frontend.AttractPageTic; got != 0 {
		t.Fatalf("attractPageTic=%d want 0", got)
	}
	if !sg.frontendMenuPending {
		t.Fatal("frontend menu should be pending")
	}
}
