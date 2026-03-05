package automap

import (
	"strings"
	"testing"
)

func TestWorldThingSpriteName_StateTimedCrossfade_Barrel(t *testing.T) {
	g := &game{
		textureAnimCrossfadeFrames: 2,
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BAR1A0": {Width: 2, Height: 2, OffsetX: 0, OffsetY: 0, RGBA: []byte{
					255, 0, 0, 255, 255, 0, 0, 255,
					255, 0, 0, 255, 255, 0, 0, 255,
				}},
				"BAR1B0": {Width: 2, Height: 2, OffsetX: 0, OffsetY: 0, RGBA: []byte{
					0, 255, 0, 255, 0, 255, 0, 255,
					0, 255, 0, 255, 0, 255, 0, 255,
				}},
			},
		},
	}
	name := g.worldThingSpriteName(2035, 4)
	if !strings.Contains(name, "BAR1A0>BAR1B0#1/2") {
		t.Fatalf("barrel blend tic4 name=%q want token BAR1A0>BAR1B0#1/2", name)
	}
	if _, ok := g.spriteAnimBlendTex[name]; !ok {
		t.Fatalf("missing blended sprite texture for token %q", name)
	}
}

func TestResolveAnimatedWallSample_IncludesAdditionalDoomSequences(t *testing.T) {
	g := &game{textureAnimCrossfadeFrames: 0}
	if got := resolveAnimatedTextureName("FIREMAG1", 8, wallTextureAnimRefs); got != "FIREMAG2" {
		t.Fatalf("FIREMAG frame @8=%q want FIREMAG2", got)
	}
	if got := resolveAnimatedTextureName("BFALL1", 24, wallTextureAnimRefs); got != "BFALL4" {
		t.Fatalf("BFALL frame @24=%q want BFALL4", got)
	}

	g.textureAnimCrossfadeFrames = 2
	token, blended := g.resolveAnimatedTextureSample("FIREMAG1", 3, wallTextureAnimRefs)
	if !blended {
		t.Fatalf("FIREMAG sample @3 blended=%v want true", blended)
	}
	if !strings.Contains(token, "FIREMAG1>FIREMAG2#1/2") {
		t.Fatalf("FIREMAG blend token=%q want FIREMAG1>FIREMAG2#1/2", token)
	}
}

