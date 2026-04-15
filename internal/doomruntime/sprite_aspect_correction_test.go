package doomruntime

import "testing"

func TestSpriteScaleYForAspect_ExemptPrefixesStayCircular(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}

	got := g.spriteScaleYForAspect("SOULA0", 2, 2*doomPixelAspect)
	if got != 2 {
		t.Fatalf("scaleY=%v want %v", got, 2.0)
	}

	got = g.spriteScaleYForAspect("BAL1C0", 3, 3*doomPixelAspect)
	if got != 3 {
		t.Fatalf("scaleY=%v want %v", got, 3.0)
	}

	got = g.spriteScaleYForAspect("MEGAC0", 4, 4*doomPixelAspect)
	if got != 4 {
		t.Fatalf("scaleY=%v want %v", got, 4.0)
	}
}

func TestSpriteScaleYForAspect_NonExemptSpritesKeepAspectCorrection(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true}}

	want := 2 * doomPixelAspect
	got := g.spriteScaleYForAspect("TROOA1", 2, want)
	if got != want {
		t.Fatalf("scaleY=%v want %v", got, want)
	}
}

func TestSpriteScaleYForAspect_DisabledWhenGeometryAspectCorrectionOff(t *testing.T) {
	g := &game{opts: Options{SourcePortMode: true, DisableGeometryAspectCorrect: true}}

	got := g.spriteScaleYForAspect("SOULA0", 2, 5)
	if got != 5 {
		t.Fatalf("scaleY=%v want %v", got, 5.0)
	}
}
