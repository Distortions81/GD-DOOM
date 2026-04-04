package doomruntime

import (
	"testing"

	"gddoom/internal/media"
)

func TestInitSessionBroadcastModeStartsRunnableGameplay(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	sg := &sessionGame{
		gameFactory: newGame,
		bootMap:     cloneMapForRestart(base.m),
		current:     base.m.Name,
		opts: Options{
			Width:          1067,
			Height:         960,
			SourcePortMode: true,
			StartInMapMode: true,
			FlatBank:       base.opts.FlatBank,
			WallTexBank:    base.opts.WallTexBank,
			BootSplash: media.WallTexture{
				Width:  1,
				Height: 1,
				RGBA:   []byte{0, 0, 0, 255},
			},
			LiveTicSink: &testLiveTicSink{},
		},
	}

	sg.initSession()

	if sg.g == nil || sg.rt == nil {
		t.Fatal("expected initialized game runtime")
	}
	if sg.transitionActive() {
		t.Fatal("transition should be inactive for broadcast mode")
	}
	if sg.frontend.Active {
		t.Fatal("frontend should be inactive for broadcast mode")
	}
	if sg.intermission.state.Active {
		t.Fatal("intermission should be inactive for broadcast mode")
	}
	if err := sg.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sg.g.worldTic != 1 {
		t.Fatalf("worldTic=%d want=1", sg.g.worldTic)
	}
}

func TestInitSessionWatchModeStartsRunnableGameplay(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	sg := &sessionGame{
		gameFactory: newGame,
		bootMap:     cloneMapForRestart(base.m),
		current:     base.m.Name,
		opts: Options{
			Width:          1067,
			Height:         960,
			SourcePortMode: true,
			StartInMapMode: true,
			FlatBank:       base.opts.FlatBank,
			WallTexBank:    base.opts.WallTexBank,
			BootSplash: media.WallTexture{
				Width:  1,
				Height: 1,
				RGBA:   []byte{0, 0, 0, 255},
			},
			LiveTicSource: &testLiveTicSource{
				tics: []DemoTic{{Forward: 25}},
			},
		},
	}

	sg.initSession()

	if sg.g == nil || sg.rt == nil {
		t.Fatal("expected initialized game runtime")
	}
	if sg.transitionActive() {
		t.Fatal("transition should be inactive for watch mode")
	}
	if sg.frontend.Active {
		t.Fatal("frontend should be inactive for watch mode")
	}
	if err := sg.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sg.g.worldTic != 1 {
		t.Fatalf("worldTic=%d want=1", sg.g.worldTic)
	}
}
