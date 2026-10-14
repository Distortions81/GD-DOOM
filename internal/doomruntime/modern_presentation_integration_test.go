//go:build integration

package doomruntime

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

type modernPresentationCase struct {
	Name, Reference, Patch string
	Width, Height, Light   int
	X, Y                   int64
	Lighting               bool
}

func modernPresentationFixtures(t *testing.T) (*game, []modernPresentationCase, string) {
	t.Helper()
	out := os.Getenv("GD_MODERN_OUT")
	if out == "" {
		t.Skip("run scripts/modern_presentation_compare.py")
	}
	data, err := os.ReadFile(filepath.Join(out, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []modernPresentationCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	wf, err := wad.Open(os.Getenv("GD_MODERN_WAD"))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := ts.PaletteRGBA(0)
	if err != nil {
		t.Fatal(err)
	}
	lump, _ := wf.LumpByName("COLORMAP")
	cm, err := wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	bank := map[string]WallTexture{}
	for _, def := range weaponPspriteDefs {
		if _, ok := bank[def.sprite]; ok {
			continue
		}
		rgba, w, h, ox, oy, err := ts.BuildPatchRGBA(def.sprite, 0)
		if err != nil {
			continue
		}
		indexed, _, _, _, _, _, _ := ts.BuildPatchIndexedView(def.sprite)
		bank[def.sprite] = WallTexture{RGBA: rgba, Indexed: indexed, Width: w, Height: h, OffsetX: ox, OffsetY: oy}
	}
	g := &game{opts: Options{SourcePortMode: true, SourcePortSectorLighting: true, DoomPaletteRGBA: palette, DoomColorMap: cm, DoomColorMapRows: len(cm) / 256, SpritePatchBank: bank}, m: &mapdata.Map{Sectors: []mapdata.Sector{{Light: 128}}}, mode: viewWalk, screenBlocks: 11, renderAlpha: 1, geometryAspectActive: true, geometryAspectY: 1.2}
	return g, cases, out
}

func applyModernPresentationCase(t *testing.T, g *game, c modernPresentationCase) {
	t.Helper()
	found := false
	for state, def := range weaponPspriteDefs {
		if def.sprite == c.Patch {
			g.weaponState = state
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing patch %s", c.Patch)
	}
	g.prevWeaponState = g.weaponState
	g.weaponFlashState = weaponStateNone
	g.prevWeaponFlashState = weaponStateNone
	g.viewW, g.viewH = c.Width, c.Height
	g.m.Sectors[0].Light = int16(c.Light)
	g.opts.DisableDoomLighting = !c.Lighting
	g.weaponPSpriteY = int(c.Y >> fracBits)
	g.prevWeaponPSpriteY = g.weaponPSpriteY
	g.weaponPose = weaponPresentationPose{X: c.X, FractionY: c.Y & (fracUnit - 1), Valid: true}
	g.prevWeaponPose = g.weaponPose
}

func TestModernPresentationCapture(t *testing.T) {
	g, cases, out := modernPresentationFixtures(t)
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		for _, c := range cases {
			applyModernPresentationCase(t, g, c)
			for _, shader := range []bool{false, true} {
				g.opts.KageShader = shader
				target := ebiten.NewImage(c.Width, c.Height)
				target.Fill(color.Black)
				g.drawWeaponOverlay(target)
				img := image.NewRGBA(target.Bounds())
				target.ReadPixels(img.Pix)
				target.Dispose()
				backend := "cpu"
				if shader {
					backend = "shader"
				}
				file, err := os.Create(filepath.Join(out, c.Name+"-"+backend+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, img)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}
