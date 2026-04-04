package doomruntime

import "testing"

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
