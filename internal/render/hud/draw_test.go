package hud

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestDrawStatusBarWeaponSlotPositionsMatchDoomLayout(t *testing.T) {
	screen := ebiten.NewImage(doomLogicalW, doomLogicalH)
	var got [6][2]float64
	index := 0
	drawPatch := func(_ *ebiten.Image, name string, x, y, sx, sy float64) bool {
		if len(name) == len("STGNUM2") && name[:6] == "STGNUM" {
			if index >= len(got) {
				t.Fatalf("captured too many weapon slot patches: %d", index+1)
			}
			got[index] = [2]float64{x, y}
			index++
		}
		return true
	}
	drawNum := func(*ebiten.Image, int, int, float64, float64, float64, float64) {}
	drawPercent := func(*ebiten.Image, int, float64, float64, float64, float64) {}
	DrawStatusBar(screen, StatusBarInputs{ViewW: doomLogicalW, ViewH: doomLogicalH}, drawPatch, drawNum, drawNum, drawPercent)

	want := [6][2]float64{
		{111, 172},
		{123, 172},
		{135, 172},
		{111, 182},
		{123, 182},
		{135, 182},
	}
	if index != len(want) {
		t.Fatalf("captured %d weapon slot patches, want %d", index, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slot %d drawn at %v, want %v", i+2, got[i], want[i])
		}
	}
}
