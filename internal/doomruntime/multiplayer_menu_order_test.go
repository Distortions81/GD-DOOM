package doomruntime

import (
	"context"
	"testing"

	"gddoom/internal/gameplay"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestMultiplayerMenuFollowsNewGameInNavigation(t *testing.T) {
	for _, inGame := range []bool{false, true} {
		t.Run(map[bool]string{false: "title", true: "pause"}[inGame], func(t *testing.T) {
			sg := multiplayerMenuTestSession(t)
			sg.frontend.InGame = inGame
			sg.opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
				return runtimecfg.AuthorityJoinResult{}, nil
			}
			defer sg.cancelAuthorityServerRefresh()
			// Action identities are unchanged; arrows and drawing must agree on
			// their new order, including the Quit/New Game wrap boundary.
			items := []int{0, frontendMultiplayerMenuItem, 1, 2, 3, 4, 5}
			for row, item := range items {
				if sg.frontend.ItemOn != item || sg.frontendMainMenuRow(item) != row {
					t.Fatalf("row %d selects action %d at drawn row %d; want action %d", row, sg.frontend.ItemOn, sg.frontendMainMenuRow(item), item)
				}
				menuKey(sg, ebiten.KeyArrowDown)
				if err := sg.tickFrontend(); err != nil {
					t.Fatal(err)
				}
			}
			if sg.frontend.ItemOn != 0 {
				t.Fatal("Quit did not wrap to New Game")
			}
			menuKey(sg, ebiten.KeyArrowUp)
			_ = sg.tickFrontend()
			if sg.frontend.ItemOn != 5 {
				t.Fatal("New Game did not wrap to Quit")
			}
			sg.frontend.ItemOn = 0
			menuKey(sg, ebiten.KeyArrowDown)
			_ = sg.tickFrontend()
			menuKey(sg, ebiten.KeyEnter)
			_ = sg.tickFrontend()
			if sg.frontend.Mode != frontendModeMultiplayer {
				t.Fatal("second row did not open Multiplayer")
			}
			menuKey(sg, ebiten.KeyEscape)
			_ = sg.tickFrontend()
			if sg.frontend.Mode != frontendModeTitle || sg.frontend.ItemOn != frontendMultiplayerMenuItem {
				t.Fatal("Back did not return to the Multiplayer row")
			}
			menuKey(sg, ebiten.KeyArrowDown)
			_ = sg.tickFrontend()
			menuKey(sg, ebiten.KeyEnter)
			_ = sg.tickFrontend()
			if sg.frontend.Mode != frontendModeOptions {
				t.Fatal("third row no longer opens Options")
			}
		})
	}
}

func TestMultiplayerMenuConnectedPauseStartsAtMatchControls(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	_, client := multiplayerMenuTestResult()
	sg.opts.AuthorityClient, sg.g.opts.AuthorityClient = client, client
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{})
	if sg.frontend.ItemOn != frontendMultiplayerMenuItem || sg.frontendMainMenuRow(sg.frontend.ItemOn) != 1 {
		t.Fatal("connected pause did not select Multiplayer below disabled New Game")
	}
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontend()
	if sg.frontend.Mode != frontendModeMultiplayer || sg.multiplayer.row != 0 {
		t.Fatal("connected pause did not default to Return to Game")
	}
	menuKey(sg, ebiten.KeyEscape)
	_ = sg.tickFrontend()
	for _, want := range []int{1, 4, 5, frontendMultiplayerMenuItem} {
		menuKey(sg, ebiten.KeyArrowDown)
		_ = sg.tickFrontend()
		if sg.frontend.ItemOn != want {
			t.Fatalf("connected navigation selected %d, want %d; disabled actions must stay skipped", sg.frontend.ItemOn, want)
		}
	}
}

func TestFrontendWithoutMultiplayerRetainsStockOrder(t *testing.T) {
	sg := multiplayerMenuTestSession(t)
	if sg.frontendMainMenuCount() != len(frontendMainMenuNames) || len(sg.frontendMainMenuSelectableRows()) != 0 {
		t.Fatal("minimal host gained a multiplayer row")
	}
	for item := range frontendMainMenuNames {
		if sg.frontendMainMenuRow(item) != item {
			t.Fatal("stock menu rows shifted without Multiplayer")
		}
	}
	menuKey(sg, ebiten.KeyArrowDown)
	_ = sg.tickFrontend()
	menuKey(sg, ebiten.KeyEnter)
	_ = sg.tickFrontend()
	if sg.frontend.Mode != frontendModeOptions {
		t.Fatal("stock second row no longer opens Options")
	}
}
