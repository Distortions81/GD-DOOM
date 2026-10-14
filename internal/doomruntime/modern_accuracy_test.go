package doomruntime

import (
	"gddoom/internal/mapdata"
	"math"
	"testing"
)

func TestModernWeaponLightingIndependentOfResolution(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true, SourcePortSectorLighting: true}, m: &mapdata.Map{Sectors: []mapdata.Sector{{Light: 64}}}, screenBlocks: 11}
	for _, light := range []int16{0, 64, 128, 192, 255} {
		g.m.Sectors[0].Light = light
		g.viewW, g.viewH = 320, 200
		want := g.weaponLightRow(false)
		for _, size := range [][2]int{{640, 480}, {960, 540}, {1280, 720}, {1920, 1080}, {3440, 1440}} {
			g.viewW, g.viewH = size[0], size[1]
			if got := g.weaponLightRow(false); got != want {
				t.Errorf("%v sector=%d: row=%d want logical row=%d", size, light, got, want)
			}
		}
	}
}

func TestModernNativeWeaponAnchorMatchesMain(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true, DisableDoomLighting: true, SpritePatchBank: map[string]WallTexture{"PISGA0": {Width: 1, Height: 1, OffsetX: -160, OffsetY: -100, RGBA: []byte{255, 255, 255, 255}}}, StatusPatchBank: map[string]WallTexture{"STBAR": {Width: 320, Height: 32}}}, m: &mapdata.Map{}, viewW: 960, viewH: 540, screenBlocks: 11, renderAlpha: 1, weaponState: weaponStatePistolReady, weaponPSpriteY: 35, weaponPose: weaponPresentationPose{X: fracUnit, FractionY: fracUnit / 2, Valid: true}}
	n := &NativeMeshGame{g: g}
	for _, blocks := range []int{doomScreenBlocksOverlay, doomScreenBlocksFull} {
		g.screenBlocks = blocks
		n.buildPresentation()
		if len(n.weapon) != 1 {
			t.Fatal("missing weapon")
		}
		want := 135.5
		if g.statusBarDisplayMode() != statusBarDisplayBottom {
			want -= 8
		}
		if math.Abs(n.weapon[0].Y-want) > 1e-9 {
			t.Errorf("HUD mode %v: native y=%g want %g", g.statusBarDisplayMode(), n.weapon[0].Y, want)
		}
	}
}
