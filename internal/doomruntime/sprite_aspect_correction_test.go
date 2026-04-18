package doomruntime

import "testing"

func TestSpriteScaleYForAspect_ExemptPrefixesStayCircular(t *testing.T) {
	g := &game{geometryAspectActive: true, geometryAspectY: doomPixelAspect}

	got := g.spriteScaleYForAspect(&spriteRenderRef{aspectExempt: true}, 2, 2*doomPixelAspect)
	if got != 2 {
		t.Fatalf("scaleY=%v want %v", got, 2.0)
	}

	got = g.spriteScaleYForAspect(&spriteRenderRef{aspectExempt: true}, 3, 3*doomPixelAspect)
	if got != 3 {
		t.Fatalf("scaleY=%v want %v", got, 3.0)
	}

	got = g.spriteScaleYForAspect(&spriteRenderRef{aspectExempt: true}, 4, 4*doomPixelAspect)
	if got != 4 {
		t.Fatalf("scaleY=%v want %v", got, 4.0)
	}
}

func TestSpriteScaleYForAspect_NonExemptSpritesKeepAspectCorrection(t *testing.T) {
	g := &game{geometryAspectActive: true, geometryAspectY: doomPixelAspect}

	want := 2 * doomPixelAspect
	got := g.spriteScaleYForAspect(&spriteRenderRef{}, 2, want)
	if got != want {
		t.Fatalf("scaleY=%v want %v", got, want)
	}
}

func TestSpriteScaleYForAspect_DisabledWhenGeometryAspectCorrectionOff(t *testing.T) {
	g := &game{}

	got := g.spriteScaleYForAspect(&spriteRenderRef{aspectExempt: true}, 2, 5)
	if got != 5 {
		t.Fatalf("scaleY=%v want %v", got, 5.0)
	}
}

func TestSpritePatchExemptFromAspect_NormalizesPrefixWithoutAllocatingCallers(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "SOULA0", want: true},
		{key: " soulA0", want: true},
		{key: "bal1c0", want: true},
		{key: "TROOA1", want: false},
		{key: "ab", want: false},
	}

	for _, tc := range tests {
		if got := spritePatchExemptFromAspect(tc.key); got != tc.want {
			t.Fatalf("spritePatchExemptFromAspect(%q)=%v want %v", tc.key, got, tc.want)
		}
	}
}
