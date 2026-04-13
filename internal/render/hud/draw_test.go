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

func TestFlashOverlayStateMatchesDoomPalettePriority(t *testing.T) {
	stage, clr := flashOverlayState(0, 0, 0, 0)
	if stage != 0 || clr != (color.RGBA{}) {
		t.Fatalf("idle flash=%d %#v want 0 zero", stage, clr)
	}

	stage, clr = flashOverlayState(0, 6, 0, 0)
	if stage != 1 || clr != (color.RGBA{R: 216, G: 188, B: 72}) {
		t.Fatalf("bonus flash=%d %#v want 1 gold", stage, clr)
	}

	stage, clr = flashOverlayState(12, 6, 0, 0)
	if stage != 2 || clr != (color.RGBA{R: 176, G: 32, B: 32}) {
		t.Fatalf("damage priority flash=%d %#v want 2 red", stage, clr)
	}

	stage, clr = flashOverlayState(0, 0, 1, 0)
	if stage != 2 || clr != (color.RGBA{R: 176, G: 32, B: 32}) {
		t.Fatalf("berserk flash=%d %#v want 2 red", stage, clr)
	}

	stage, clr = flashOverlayState(0, 0, 12<<6, 0)
	if stage != 0 || clr != (color.RGBA{}) {
		t.Fatalf("expired berserk flash=%d %#v want 0 zero", stage, clr)
	}

	stage, clr = flashOverlayState(0, 0, 0, 5*32)
	if stage != 1 || clr != (color.RGBA{R: 48, G: 160, B: 48}) {
		t.Fatalf("radiation flash=%d %#v want 1 green", stage, clr)
	}

	stage, clr = flashOverlayState(0, 0, 0, 8)
	if stage != 1 || clr != (color.RGBA{R: 48, G: 160, B: 48}) {
		t.Fatalf("radiation blink flash=%d %#v want 1 green", stage, clr)
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
