//go:build integration

package doomruntime

import (
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/hud"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestFlashOverlayStaysCloseToPLAYPAL(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	palettes := make([][]byte, 14)
	for i := range palettes {
		palettes[i], err = doomtex.LoadPaletteRGBA(wf, i)
		if err != nil {
			t.Fatal(err)
		}
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		g := &game{viewW: 256, viewH: 1}
		target := ebiten.NewImage(g.viewW, g.viewH)
		defer target.Dispose()
		pixels := make([]byte, len(palettes[0]))
		worst := 0
		for _, modern := range []bool{false, true} {
			g.opts.SourcePortMode = modern
			for _, shader := range []bool{false, true} {
				g.opts.KageShader = shader
				for effect := range 4 {
					for count := range 256 {
						g.statusDamageCount, g.statusBonusCount = 0, 0
						g.inventory.StrengthCount, g.inventory.RadSuitTics = 0, 0
						switch effect {
						case 0:
							g.statusDamageCount = count
						case 1:
							g.statusBonusCount = count
						case 2:
							g.inventory.StrengthCount = count * 4
						case 3:
							g.inventory.RadSuitTics = count
						}
						index := hud.FlashPaletteIndex(g.statusDamageCount, g.statusBonusCount, g.inventory.StrengthCount, g.inventory.RadSuitTics)
						target.WritePixels(palettes[0])
						g.drawFlashOverlay(target)
						target.ReadPixels(pixels)
						for i, got := range pixels {
							delta := int(got) - int(palettes[index][i])
							worst = max(worst, max(delta, -delta))
							if delta < -2 || delta > 2 {
								t.Fatalf("modern=%v shader=%v palette=%d color=%d channel=%d got=%d want=%d", modern, shader, index, i/4, i%4, got, palettes[index][i])
							}
						}
					}
				}
			}
		}
		t.Logf("maximum rendered channel difference from PLAYPAL: %d/255", worst)
	}
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
