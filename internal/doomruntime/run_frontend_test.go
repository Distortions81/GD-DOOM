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

func TestFrontendPointerInputAtMainMenuSelectsClickedRow(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:     true,
			Mode:       frontendModeTitle,
			MenuActive: true,
		},
	}

	input, ok := sg.frontendPointerInputAt(120, 64+2*16+4)
	if !ok {
		t.Fatal("frontendPointerInputAt() = not handled, want handled")
	}
	if sg.frontend.ItemOn != 2 {
		t.Fatalf("ItemOn=%d want=2", sg.frontend.ItemOn)
	}
	if !input.Select || !input.Skip {
		t.Fatalf("input=%+v want select+skip", input)
	}
}

func TestFrontendPointerInputAtOptionsLeftAndRightThirds(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active:    true,
			Mode:      frontendModeOptions,
			OptionsOn: frontendOptionsSelectableRows[0],
		},
	}

	leftInput, ok := sg.frontendPointerInputAt(60, 37+4*16+4)
	if !ok {
		t.Fatal("left third click not handled")
	}
	if sg.frontend.OptionsOn != 4 {
		t.Fatalf("OptionsOn=%d want=4", sg.frontend.OptionsOn)
	}
	if !leftInput.Left || leftInput.Right || leftInput.Select {
		t.Fatalf("leftInput=%+v want left only", leftInput)
	}

	rightInput, ok := sg.frontendPointerInputAt(300, 37+4*16+4)
	if !ok {
		t.Fatal("right third click not handled")
	}
	if !rightInput.Right || rightInput.Left || rightInput.Select {
		t.Fatalf("rightInput=%+v want right only", rightInput)
	}
}

func TestTouchButtonsHideFrontendOverlay(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{
			Active: true,
		},
	}
	if buttons := sg.touchButtons(320, 200); len(buttons) != 0 {
		t.Fatalf("touchButtons(frontend)=%d want 0", len(buttons))
	}
}
