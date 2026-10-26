//go:build integration

package doomruntime

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

// Exercise the actual world renderer and possession HUD, optionally retaining
// captures for visual inspection. Run this test alone because it owns RunGame.
func TestPossessionRender(t *testing.T) {
	out := os.Getenv("GD_POSSESSION_CAPTURE")
	if out == "" {
		t.Skip("set GD_POSSESSION_CAPTURE to a capture directory")
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	walls := buildWallTextureBankForTest(t, ts)
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	indexed, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := ts.PaletteRGBA(0)
	if err != nil {
		t.Fatal(err)
	}
	sprites := buildBillboardSpritePatchBankForTest(t, ts)
	status := buildAlphaPatchBankForDump(t, wf, ts)
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		for _, modern := range []bool{false, true} {
			m, err := mapdata.LoadMap(wf, "E1M1")
			if err != nil {
				t.Fatal(err)
			}
			g := newGame(m, Options{Width: 320, Height: 200, SkillLevel: 3, PlayerSlot: 1,
				SourcePortMode: modern, PlayAsMonster: true, NoFPS: true,
				WallTexBank: walls, FlatBank: flats, FlatBankIndexed: indexed, SpritePatchBank: sprites, StatusPatchBank: status, DoomPaletteRGBA: palette})
			g.mode = viewWalk
			// Use a real map monster rather than creating a fake render-only body.
			idx := -1
			for i, th := range m.Things {
				if isMonster(th.Type) && g.thingActiveInSession(i) {
					idx = i
					break
				}
			}
			if idx < 0 {
				t.Fatal("map has no possessable monster")
			}
			angle := g.thingWorldAngle(idx, m.Things[idx])
			g.possession = &monsterPossession{thing: idx, prevAngle: angle}
			g.syncRenderState()
			g.prepareRenderState()
			img := ebiten.NewImage(320, 200)
			g.Draw(img)
			rect := g.walkRenderViewportRect()
			c := img.At(rect.Dx()/2, rect.Dy()/2)
			r, gr, b, _ := c.RGBA()
			if r != 65535 || gr != 65535 || b != 65535 {
				t.Fatalf("crosshair not rendered: %v", c)
			}
			name := "faithful.png"
			if modern {
				name = "modern.png"
			}
			f, err := os.Create(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			// Returning must restore the marine's camera and HUD immediately.
			g.releasePossession()
			g.Draw(img)
			if g.renderPX != float64(g.p.x)/fracUnit || g.renderPY != float64(g.p.y)/fracUnit {
				t.Fatal("return did not restore marine camera")
			}
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
