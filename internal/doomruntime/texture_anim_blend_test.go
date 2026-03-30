package doomruntime

import "testing"

func TestTextureBlendSample_WrapsRepeatingAnimations(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode:             true,
			TextureAnimCrossfadeFrames: 5,
		},
		worldTic:    4,
		renderAlpha: 0,
	}
	refs := map[string]textureAnimRef{
		"TEST": {frames: []string{"A", "B"}, index: 0},
	}

	got := g.textureBlendSample("TEST", refs)
	if got.fromKey != "A" || got.toKey != "B" || got.alpha < 126 || got.alpha > 129 {
		t.Fatalf("blend=%+v want ~50%% A->B", got)
	}

	g.worldTic = 8
	got = g.textureBlendSample("TEST", refs)
	if got.fromKey != "B" || got.toKey != "" || got.alpha != 0 {
		t.Fatalf("blend=%+v want hard B at start of second half-cycle", got)
	}

	g.worldTic = 12
	got = g.textureBlendSample("TEST", refs)
	if got.fromKey != "B" || got.toKey != "A" || got.alpha < 126 || got.alpha > 129 {
		t.Fatalf("blend=%+v want ~50%% B->A", got)
	}
}

func TestTextureBlendSample_DisabledOutsideSourcePortMode(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode:             false,
			TextureAnimCrossfadeFrames: 5,
		},
		worldTic:    8,
		renderAlpha: 0.5,
	}
	refs := map[string]textureAnimRef{
		"TEST": {frames: []string{"A", "B"}, index: 0},
	}

	got := g.textureBlendSample("TEST", refs)
	if got.fromKey != "B" || got.toKey != "" || got.alpha != 0 {
		t.Fatalf("blend=%+v want stepped B only", got)
	}
}

func TestTextureBlendSample_FrameNameDoesNotReanchorSequence(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode:             true,
			TextureAnimCrossfadeFrames: 5,
		},
		worldTic: 4,
	}
	refs := map[string]textureAnimRef{
		"A": {frames: []string{"A", "B"}, index: 0},
		"B": {frames: []string{"A", "B"}, index: 0},
	}

	gotA := g.textureBlendSample("A", refs)
	gotB := g.textureBlendSample("B", refs)
	if gotA != gotB {
		t.Fatalf("A sample=%+v B sample=%+v want identical sequencing", gotA, gotB)
	}
}
