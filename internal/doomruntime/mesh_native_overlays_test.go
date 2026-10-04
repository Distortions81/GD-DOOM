package doomruntime

import (
	"fmt"
	"image/color"
	"path/filepath"
	"testing"
)

func nativeOverlayFixture(t *testing.T) *NativeCampaign {
	t.Helper()
	c := nativeCampaignFixture(t, "E1M1")
	g := c.Game.g
	g.opts.MessageFontBank = make(map[rune]WallTexture)
	for ch := rune(33); ch <= 95; ch++ {
		if tex, ok := g.opts.StatusPatchBank[fmt.Sprintf("STCFN%03d", ch)]; ok {
			g.opts.MessageFontBank[ch] = tex
		}
	}
	if len(g.opts.MessageFontBank) == 0 {
		t.Fatal("fixture has no WAD message font")
	}
	return c
}

func TestNativeFlashUsesStatusCountersPowerupPriorityAndBlink(t *testing.T) {
	c := nativeOverlayFixture(t)
	g := c.Game.g
	// The old ten-tic timers are intentionally unrelated to these palette
	// counters. Damage, pickups and berserk must not use the legacy timers.
	g.damageFlashTic, g.bonusFlashTic = 10, 10
	cases := []struct {
		name                   string
		damage, bonus, berserk int
		suit                   int
		want                   color.RGBA
	}{
		{"idle", 0, 0, 0, 0, color.RGBA{}},
		{"damage beats pickup and suit", 80, 40, 0, 200, color.RGBA{176, 32, 32, 160}},
		{"last damage stage", 1, 40, 0, 200, color.RGBA{176, 32, 32, 36}},
		{"bonus beats suit", 0, 40, 0, 200, color.RGBA{216, 188, 72, 84}},
		{"berserk beats pickup", 0, 40, 1, 200, color.RGBA{176, 32, 32, 54}},
		{"berserk fades", 0, 0, 12 << 6, 0, color.RGBA{}},
		{"suit steady", 0, 0, 0, 129, color.RGBA{48, 160, 48, 56}},
		{"suit blink off", 0, 0, 0, 128, color.RGBA{}},
		{"suit blink on", 0, 0, 0, 8, color.RGBA{48, 160, 48, 56}},
	}
	for _, tc := range cases {
		g.statusDamageCount, g.statusBonusCount = tc.damage, tc.bonus
		g.inventory.StrengthCount, g.inventory.RadSuitTics = tc.berserk, tc.suit
		if got := c.Game.Frame(1).FlashOverlay; got != tc.want {
			t.Fatalf("%s: tint=%v want=%v", tc.name, got, tc.want)
		}
	}
	g.statusDamageCount, g.statusBonusCount = 9, 0
	g.inventory.StrengthCount, g.inventory.RadSuitTics = 0, 0
	c.Game.Tick(NativeMeshInput{})
	if got := c.Game.Frame(1).FlashOverlay; got != (color.RGBA{176, 32, 32, 36}) {
		t.Fatalf("flash did not follow shared tic decay: %v", got)
	}
}

func TestNativeHUDOverlaysUseWADGlyphsWithoutEbitenImages(t *testing.T) {
	c := nativeOverlayFixture(t)
	g := c.Game.g
	g.setHUDMessage("picked up a shotgun!\n\nGAME SAVED", 2)
	for _, size := range [][2]int{{320, 200}, {640, 400}, {1280, 720}} {
		g.hudScaleStep = 2
		patches := c.MessagePatches(size[0], size[1])
		if len(patches) == 0 || patches[0].Texture.Width != g.opts.MessageFontBank['P'].Width || patches[0].X != -float64(g.opts.MessageFontBank['P'].OffsetX)*g.hudScaleValue() {
			t.Fatal("message omitted WAD glyphs or placed them away from the main origin")
		}
		if overlay := c.DeathOverlay(size[0], size[1]); overlay.Tint.A != 0 || len(overlay.Patches) != 0 {
			t.Fatal("living player has a death overlay")
		}
		g.isDead = true
		overlay := c.DeathOverlay(size[0], size[1])
		if overlay.Tint != (color.RGBA{25, 0, 0, 130}) || len(overlay.Patches) == 0 {
			t.Fatal("dead player missing shared tint or prompts")
		}
		first := g.opts.MessageFontBank['Y']
		wantX := (float64(size[0])-float64(g.huTextWidth("YOU DIED"))*2)/2 - float64(first.OffsetX)*2
		if overlay.Patches[0].X != wantX || overlay.Patches[0].Y != float64(size[1]/2)-float64(first.OffsetY)*2 {
			t.Fatal("death prompt differs from main's centered WAD text layout")
		}
		g.isDead = false
	}
	if g.messageFontImg != nil || c.session.nativePatches != nil {
		t.Fatal("native HUD extraction allocated Ebiten images or leaked patch state")
	}
	c.Game.Tick(NativeMeshInput{})
	c.Game.Tick(NativeMeshInput{})
	if len(c.MessagePatches(640, 400)) != 0 {
		t.Fatal("expired HUD message still draws")
	}
}

func TestNativeRecordingMarkerFollowsSharedRecordingLifecycle(t *testing.T) {
	c := nativeOverlayFixture(t)
	if marker := c.RecordingOverlay(640, 400); marker.Radius != 0 || len(marker.Patches) != 0 {
		t.Fatal("idle game shows recording marker")
	}
	path := filepath.Join(t.TempDir(), "record.lmp")
	c.Game.g.opts.RecordDemoPath, c.session.opts.RecordDemoPath = path, path
	for _, width := range []int{320, 640, 960} {
		marker := c.RecordingOverlay(width, 400)
		if marker.X != float32(width-11) || marker.Y != 11 || marker.Radius != 5 || marker.Color != (color.RGBA{220, 30, 30, 255}) || len(marker.Patches) != 3 {
			t.Fatal("recording marker differs from the main HUD layout")
		}
	}
	if c.Game.g.messageFontImg != nil {
		t.Fatal("native recording label allocated Ebiten images")
	}
	c.Game.g.requestLevelExit(false, "recording marker regression")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if marker := c.RecordingOverlay(640, 400); marker.Radius != 0 || len(marker.Patches) != 0 {
		t.Fatal("frozen recording kept its active marker after exit")
	}
}
