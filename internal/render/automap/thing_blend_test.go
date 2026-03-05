package automap

import "testing"

func TestPickAnimatedThingSpriteNameBlendsDifferentSpriteSizes(t *testing.T) {
	g := &game{
		textureAnimCrossfadeFrames: 2,
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"TESTA0": {
					RGBA:    []byte{255, 0, 0, 255},
					Width:   1,
					Height:  1,
					OffsetX: 0,
					OffsetY: 10,
				},
				"TESTB0": {
					RGBA: []byte{
						0, 255, 0, 255,
						0, 255, 0, 255,
						0, 255, 0, 255,
						0, 255, 0, 255,
					},
					Width:   2,
					Height:  2,
					OffsetX: 1,
					OffsetY: 20,
				},
			},
		},
	}
	name := g.pickAnimatedThingSpriteName(3, 8, "TESTA0", "TESTB0")
	if name == "" || name == "TESTA0" || name == "TESTB0" {
		t.Fatalf("expected blend token name, got %q", name)
	}
	tex, ok := g.spriteAnimBlendTex[name]
	if !ok {
		t.Fatalf("missing blended texture for token %q", name)
	}
	if tex.Width < 2 || tex.Height < 2 {
		t.Fatalf("blended texture size=%dx%d want at least 2x2", tex.Width, tex.Height)
	}
	if tex.OffsetY < 0 {
		t.Fatalf("blended offsetY=%d want non-negative", tex.OffsetY)
	}
	// Ensure blend actually contains non-zero data from composition.
	nonZero := false
	for i := 3; i < len(tex.RGBA); i += 4 {
		if tex.RGBA[i] != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-empty alpha coverage in blended texture")
	}
}

func TestBlendSpriteTexturesInterpolatesAnchor(t *testing.T) {
	a := WallTexture{
		RGBA:    []byte{255, 0, 0, 255},
		Width:   1,
		Height:  1,
		OffsetX: 0,
		OffsetY: 8,
	}
	b := WallTexture{
		RGBA: []byte{
			0, 255, 0, 255,
			0, 255, 0, 255,
			0, 255, 0, 255,
			0, 255, 0, 255,
		},
		Width:   2,
		Height:  2,
		OffsetX: 2,
		OffsetY: 14,
	}
	tex, ok := blendSpriteTextures(a, b, 0.5)
	if !ok {
		t.Fatal("blendSpriteTextures returned not ok")
	}
	if tex.OffsetY < 0 || tex.OffsetX < 0 {
		t.Fatalf("offsets=(%d,%d) should be non-negative", tex.OffsetX, tex.OffsetY)
	}
	if len(tex.RGBA) != tex.Width*tex.Height*4 {
		t.Fatalf("rgba len=%d does not match %dx%d", len(tex.RGBA), tex.Width, tex.Height)
	}
	// Mid-blend should preserve contributions from both source sprites.
	hasRed := false
	hasGreen := false
	for i := 0; i+3 < len(tex.RGBA); i += 4 {
		if tex.RGBA[i+3] == 0 {
			continue
		}
		if tex.RGBA[i+0] > 0 {
			hasRed = true
		}
		if tex.RGBA[i+1] > 0 {
			hasGreen = true
		}
	}
	if !hasRed || !hasGreen {
		t.Fatalf("expected both source color contributions, hasRed=%v hasGreen=%v", hasRed, hasGreen)
	}
}
