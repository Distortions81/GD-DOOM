package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/media"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFrontendMenuItemDisabledForWatchMode(t *testing.T) {
	sg := &sessionGame{
		opts:     Options{LiveTicSource: &testLiveTicSource{}},
		frontend: frontendState{InGame: true},
	}
	for _, item := range []int{0, 2, 3} {
		if !sg.frontendMenuItemDisabled(item) {
			t.Fatalf("item %d should be disabled in watch mode", item)
		}
	}
	for _, item := range []int{1, 4, 5} {
		if sg.frontendMenuItemDisabled(item) {
			t.Fatalf("item %d should remain enabled in watch mode", item)
		}
	}
}

func TestStartGameFromFrontendBroadcastsMandatoryKeyframe(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	sink := &testLiveTicSink{}
	sg := &sessionGame{
		gameFactory: newGame,
		bootMap:     cloneMapForRestart(base.m),
		current:     base.m.Name,
		opts: Options{
			Width:          1067,
			Height:         960,
			SourcePortMode: true,
			StartInMapMode: true,
			Episodes:       []int{1},
			FlatBank:       base.opts.FlatBank,
			WallTexBank:    base.opts.WallTexBank,
			BootSplash: media.WallTexture{
				Width:  1,
				Height: 1,
				RGBA:   []byte{0, 0, 0, 255},
			},
			LiveTicSink: sink,
			NewGameLoader: func(mapName string) (*mapdata.Map, error) {
				if mapName != "E1M1" {
					t.Fatalf("NewGameLoader mapName=%q want E1M1", mapName)
				}
				return cloneMapForRestart(base.m), nil
			},
		},
		g: base,
	}

	sg.startGameFromFrontend(3)

	if sg.err != nil {
		t.Fatalf("startGameFromFrontend error = %v", sg.err)
	}
	if got := len(sink.keyframes); got != 1 {
		t.Fatalf("broadcast keyframes=%d want=1", got)
	}
	if got, want := sink.keyframeFlags[0], saveMandatoryApplyKeyframeFlag; got != want {
		t.Fatalf("broadcast keyframe flags=%d want=%d", got, want)
	}
	if got := sg.current; got != "E1M1" {
		t.Fatalf("current=%q want E1M1", got)
	}
}

func TestTickFrontendClosedTitleTreatsEnterAsSkip(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:     true,
			Mode:       frontendModeTitle,
			ItemOn:     1,
			Attract:    false,
			InGame:     false,
			MenuActive: false,
		},
		input: sessionInputSnapshot{
			justPressedKeys: map[ebiten.Key]int{
				ebiten.KeyEnter: 1,
			},
		},
	}

	if err := sg.tickFrontend(); err != nil {
		t.Fatalf("tickFrontend() error = %v", err)
	}
	if !sg.frontend.MenuActive {
		t.Fatal("expected enter to open the closed title menu")
	}
}

func TestFrontendStartPromptUsesTouchCopyAfterTouchSeen(t *testing.T) {
	sg := &sessionGame{}
	if got := sg.frontendStartPrompt(); got != "PRESS ANY KEY TO START" {
		t.Fatalf("prompt=%q want keyboard prompt", got)
	}
	sg.touch.seen = true
	if got := sg.frontendStartPrompt(); got != "TOUCH SCREEN TO START" {
		t.Fatalf("prompt=%q want touch prompt", got)
	}
}

func TestTickFrontendTouchBackClosesMenuAndSuppressesUntilRelease(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:     true,
			Mode:       frontendModeTitle,
			Attract:    true,
			MenuActive: true,
		},
		touch: touchControllerState{
			seen:               true,
			latchedJustPressed: touchActionBack,
		},
	}

	if err := sg.tickFrontend(); err != nil {
		t.Fatalf("tickFrontend() error = %v", err)
	}
	if sg.frontend.MenuActive {
		t.Fatal("expected touch back to close the attract menu")
	}
	if !sg.touch.suppressUntilClear {
		t.Fatal("expected touch close to suppress touch until release")
	}
}

func TestTickFrontendMusicPlayerTouchBackSuppressesUntilRelease(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:     true,
			Mode:       frontendModeMusicPlayer,
			MenuActive: true,
		},
		touch: touchControllerState{
			seen:               true,
			latchedJustPressed: touchActionBack,
		},
	}

	if err := sg.tickFrontendMusicPlayer(); err != nil {
		t.Fatalf("tickFrontendMusicPlayer() error = %v", err)
	}
	if sg.frontend.Mode != frontendModeSound {
		t.Fatalf("mode=%d want sound mode after close", sg.frontend.Mode)
	}
	if !sg.frontend.MenuActive {
		t.Fatal("expected touch back to return to the active sound submenu")
	}
	if !sg.touch.suppressUntilClear {
		t.Fatal("expected touch close to suppress touch until release")
	}
}
