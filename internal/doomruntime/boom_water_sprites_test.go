package doomruntime

import (
	"gddoom/internal/demo"
	"testing"
)

func TestBoomSpriteWaterAndCeilingClips(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	g.renderPX, g.renderPY = 128, 0
	for _, tc := range []struct {
		name        string
		eye         float64
		gz          int64
		gzt         float64
		top, bottom int
		visible     bool
	}{
		{"above-partial", 41, -32 * fracUnit, 192, 0, 167, true},
		{"below-partial", -16, -32 * fracUnit, 192, 74, 199, true},
		{"surface-exact", 0, -32 * fracUnit, 192, 0, 99, true},
		{"surface-below", -1.0 / 65536, -32 * fracUnit, 192, 100, 199, true},
		{"fake-ceiling-exact", 128, -32 * fracUnit, 192, 0, 99, true},
		{"above-fake-ceiling", 144, -32 * fracUnit, 192, 0, 125, true},
		{"submerged-hidden-from-above", 41, -32 * fracUnit, -8, 0, 0, false},
		{"above-surface-hidden-from-below", -16, 0, 64, 0, 0, false},
		{"below-ceiling-hidden-from-above", 144, -32 * fracUnit, 120, 0, 0, false},
		{"above-ceiling-hidden-from-below", 41, 128 * fracUnit, 192, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g.prepareHeightTransfers(128, 0, tc.eye)
			top, bottom, visible := g.boomSpriteClip(128*fracUnit, 0, tc.gz, tc.gzt, tc.eye, 96, 160, 200, 0, 199)
			if visible != tc.visible || (visible && (top != tc.top || bottom != tc.bottom)) {
				t.Fatalf("clip=(%d,%d,%v), want=(%d,%d,%v)", top, bottom, visible, tc.top, tc.bottom, tc.visible)
			}
		})
	}
	g.boomHeightFrameValid = false
	if a, b, ok := g.boomSpriteClip(128*fracUnit, 0, -32*fracUnit, -8, 41, 96, 160, 200, 4, 190); !ok || a != 4 || b != 190 {
		t.Fatal("height clip leaked outside world rendering")
	}
}

func TestBoomSpriteLightsAndDemoCompatibility(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	g.viewW = 320
	g.boomFloorLight[2], g.boomCeilingLight[2] = 0, 1
	g.prepareHeightTransfers(128, 0, -16)
	if got := g.boomSpriteLightLevel(1); got != (192+160)/2 {
		t.Fatalf("fake plane average=%d", got)
	}
	if g.boomSpriteLightRow(1, true, 96) != 0 {
		t.Fatal("fullbright sprite lost row zero")
	}
	g.opts.DemoScript = &DemoScript{Header: demo.Header{Version: 202}}
	if got := g.boomSpriteLightLevel(1); got != 160 {
		t.Fatalf("Boom202 sprite light=%d", got)
	}
	g.opts.DemoScript = nil
	g.prepareHeightTransfers(128, 0, 41)
	if g.boomSpriteLightLevel(1) != 160 {
		t.Fatal("underwater light cached across waterline")
	}
}
