package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
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
