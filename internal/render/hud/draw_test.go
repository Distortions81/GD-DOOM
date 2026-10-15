package hud

import (
	"image/color"
	"strings"
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

func TestFlashOverlayTimingAndPriority(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		damage, bonus, strength, suit int
		want                          color.RGBA
	}{
		{"idle", 0, 0, 0, 0, color.RGBA{}},
		{"last damage", 1, 16, 0, 200, color.RGBA{56, 0, 0, 56}},
		{"damage boundary", 8, 0, 0, 0, color.RGBA{56, 0, 0, 56}},
		{"next damage", 9, 0, 0, 0, color.RGBA{85, 0, 0, 85}},
		{"max damage", 80, 0, 0, 0, color.RGBA{226, 0, 0, 226}},
		{"last pickup", 0, 1, 0, 200, color.RGBA{53, 46, 17, 63}},
		{"pickup boundary", 0, 8, 0, 0, color.RGBA{53, 46, 17, 63}},
		{"next pickup", 0, 9, 0, 0, color.RGBA{80, 69, 25, 95}},
		{"max pickup", 0, 40, 0, 0, color.RGBA{107, 93, 34, 127}},
		{"berserk priority", 0, 40, 1, 200, color.RGBA{85, 0, 0, 85}},
		{"berserk faded", 0, 0, 12 << 6, 0, color.RGBA{}},
		{"suit steady", 0, 0, 0, 129, color.RGBA{0, 32, 0, 32}},
		{"suit blink off", 0, 0, 0, 128, color.RGBA{}},
		{"suit blink on", 0, 0, 0, 8, color.RGBA{0, 32, 0, 32}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := FlashOverlayColor(tc.damage, tc.bonus, tc.strength, tc.suit); got != tc.want {
				t.Fatalf("flash=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestDrawDeathOverlayShowsFireRestartPromptForTouchControls(t *testing.T) {
	screen := ebiten.NewImage(doomLogicalW, doomLogicalH)
	var lines []string
	DrawDeathOverlay(screen, DeathOverlayInputs{ViewW: doomLogicalW, ViewH: doomLogicalH, TouchControls: true},
		func(text string) int { return len(text) * 4 },
		func(_ *ebiten.Image, text string, _, _, _, _ float64) {
			lines = append(lines, text)
		},
	)
	if len(lines) != 2 {
		t.Fatalf("drawn lines=%d want=2", len(lines))
	}
	if !strings.Contains(lines[1], "PRESS FIRE TO RESTART") {
		t.Fatalf("restart prompt=%q want fire prompt", lines[1])
	}
}

func TestDrawDeathOverlayShowsEnterRestartPromptWithoutTouchControls(t *testing.T) {
	screen := ebiten.NewImage(doomLogicalW, doomLogicalH)
	var lines []string
	DrawDeathOverlay(screen, DeathOverlayInputs{ViewW: doomLogicalW, ViewH: doomLogicalH},
		func(text string) int { return len(text) * 4 },
		func(_ *ebiten.Image, text string, _, _, _, _ float64) {
			lines = append(lines, text)
		},
	)
	if len(lines) != 2 {
		t.Fatalf("drawn lines=%d want=2", len(lines))
	}
	if !strings.Contains(lines[1], "PRESS ENTER TO RESTART") {
		t.Fatalf("restart prompt=%q want enter prompt", lines[1])
	}
}
