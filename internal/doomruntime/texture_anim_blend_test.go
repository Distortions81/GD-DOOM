package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

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
		worldTic:    9,
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

func TestFaithfulTextureAnimationIdentityThenAliasPhase(t *testing.T) {
	g := &game{}
	refs := buildTextureAnimRefsFromSequences(map[string][]string{"A": {"B", "C", "A"}, "B": {"C", "A", "B"}, "C": {"A", "B", "C"}})
	for _, name := range []string{"A", "B", "C"} {
		g.worldTic = 0
		if got := g.textureBlendSample(name, refs).fromKey; got != name {
			t.Fatalf("initial %s translated to %s", name, got)
		}
		for tic := 1; tic <= 33; tic++ {
			g.worldTic = tic
			want := refs[name].frames[((tic-1)/8)%3]
			if got := g.textureBlendSample(name, refs).fromKey; got != want {
				t.Fatalf("%s tic=%d got=%s want=%s", name, tic, got, want)
			}
		}
	}
}

func TestFaithfulTextureAnimationUsesCompletedSpecialUpdates(t *testing.T) {
	g := &game{}
	refs := map[string]textureAnimRef{"A": {frames: []string{"A", "B", "C"}}}
	// P_Ticker calls P_UpdateSpecials at leveltime 0..N-1, then increments
	// leveltime. Thus the first frame change is visible after update nine.
	for _, tc := range []struct {
		tic  int
		want string
	}{{0, "A"}, {1, "A"}, {7, "A"}, {8, "A"}, {9, "B"}, {16, "B"}, {17, "C"}, {24, "C"}, {25, "A"}} {
		g.worldTic = tc.tic
		sample := g.textureBlendSample("A", refs)
		if sample.fromKey != tc.want || sample.alpha != 0 {
			t.Fatalf("tic %d: sample=%+v want %s", tc.tic, sample, tc.want)
		}
		wantTick := max(0, tc.tic-1) / 8
		if tc.tic == 0 {
			wantTick = -1
		}
		if got := g.textureAnimTick(); got != wantTick {
			t.Fatalf("tic %d cache frame=%d", tc.tic, got)
		}
	}
}

func TestCustomAnimationSpeedAndEmptyOverride(t *testing.T) {
	for _, speed := range []int{1, 2, 3, 4, 5, 6, 8} {
		refs := buildTextureAnimRefsFromSequences(map[string][]string{"A": {"A", "B", "C"}}, map[string]int{"A": speed})
		g := &game{}
		for tic := 0; tic <= speed*6+1; tic++ {
			g.worldTic = tic
			want := []string{"A", "B", "C"}[max(0, tic-1)/speed%3]
			if got := g.textureBlendSample("A", refs); got.fromKey != want {
				t.Fatalf("speed %d tic %d: %v want %s", speed, tic, got, want)
			}
		}
	}
	g := newGame(&mapdata.Map{}, Options{WallTextureAnimSequences: map[string][]string{}, FlatTextureAnimSequences: map[string][]string{}})
	g.worldTic = 99
	if g.resolveAnimatedWallName("FIREWALA") != "FIREWALA" || g.resolveAnimatedFlatName("NUKAGE1") != "NUKAGE1" {
		t.Fatal("empty override animated builtins")
	}
}
