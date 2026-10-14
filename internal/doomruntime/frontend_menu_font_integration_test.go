//go:build integration

package doomruntime

import (
	"bytes"
	"context"
	"image/color"
	"math"
	"os"
	"testing"

	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestFrontendMultiplayerLargeLabelRendering(t *testing.T) {
	if os.Getenv("GD_MENU_INTEGRATION") == "" {
		t.Skip("set GD_MENU_INTEGRATION=1 for the menu label pixel comparison")
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		patch := WallTexture{Width: 150, Height: 15, OffsetX: 2, OffsetY: 1, RGBA: make([]byte, 150*15*4)}
		for y := 0; y < patch.Height; y++ {
			for x := 0; x < patch.Width; x++ {
				if (x+y)%5 != 0 {
					i := (y*patch.Width + x) * 4
					patch.RGBA[i], patch.RGBA[i+1], patch.RGBA[i+2], patch.RGBA[i+3] = byte(80+x), byte(20+y*3), 12, 255
				}
			}
		}
		for _, inGame := range []bool{false, true} {
			for _, size := range [][2]int{{320, 200}, {640, 480}} {
				opts := Options{MenuPatchBank: map[string]WallTexture{"M_MULTI": patch}}
				opts.AuthorityJoin = func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
					return runtimecfg.AuthorityJoinResult{}, nil
				}
				r := &menuFontFallbackRecorder{game: &game{opts: opts}}
				sg := &sessionGame{opts: opts, rt: r, frontend: frontendState{Active: true, MenuActive: true, InGame: inGame, Mode: frontendModeTitle}}
				got, want := ebiten.NewImage(size[0], size[1]), ebiten.NewImage(size[0], size[1])
				sg.drawFrontend(got)
				if r.text != "" {
					t.Fatal("valid large artwork used the small HUD font fallback")
				}
				// Isolate the menu row's placement from font construction. Use its
				// actual cached image so atlas sampling does not affect the oracle.
				img := sg.menuPatchImages["M_MULTI"]
				if img == nil {
					t.Fatal("large label was not drawn through the menu artwork cache")
				}
				scale := math.Min(float64(size[0])/320, float64(size[1])/200)
				ox, oy := (float64(size[0])-320*scale)/2, (float64(size[1])-200*scale)/2
				want.Fill(color.Black)
				op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
				op.GeoM.Scale(scale, scale)
				op.GeoM.Translate(ox+float64(97-patch.OffsetX)*scale, oy+float64(80-patch.OffsetY)*scale)
				want.DrawImage(img, op)
				actual, expected := make([]byte, size[0]*size[1]*4), make([]byte, size[0]*size[1]*4)
				got.ReadPixels(actual)
				want.ReadPixels(expected)
				if !bytes.Equal(actual, expected) {
					t.Fatalf("large label differs at inGame=%t size=%v", inGame, size)
				}
				got.Deallocate()
				want.Deallocate()
			}
		}
	}
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
