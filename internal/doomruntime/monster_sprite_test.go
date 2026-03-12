package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestMonsterSpriteRotationIndexFacingEast(t *testing.T) {
	th := mapdata.Thing{X: 0, Y: 0, Angle: 0}
	if got := monsterSpriteRotationIndex(th, 100, 0); got != 1 {
		t.Fatalf("front rot=%d want=1", got)
	}
	if got := monsterSpriteRotationIndex(th, 0, 100); got != 3 {
		t.Fatalf("left rot=%d want=3", got)
	}
	if got := monsterSpriteRotationIndex(th, -100, 0); got != 5 {
		t.Fatalf("back rot=%d want=5", got)
	}
	if got := monsterSpriteRotationIndex(th, 0, -100); got != 7 {
		t.Fatalf("right rot=%d want=7", got)
	}
}

func TestMonsterSpriteRotFramePrefersPairAndFlip(t *testing.T) {
	g := &game{opts: Options{SpritePatchBank: map[string]WallTexture{
		"TROOA2A8": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
	}}}
	name, flip, ok := g.monsterSpriteRotFrame("TROO", 'A', 2)
	if !ok || name != "TROOA2A8" || flip {
		t.Fatalf("rot2 got name=%q flip=%t ok=%t", name, flip, ok)
	}
	name, flip, ok = g.monsterSpriteRotFrame("TROO", 'A', 8)
	if !ok || name != "TROOA2A8" || !flip {
		t.Fatalf("rot8 got name=%q flip=%t ok=%t", name, flip, ok)
	}
}

func TestMonsterSpriteNameForViewUsesRotation(t *testing.T) {
	g := &game{opts: Options{SpritePatchBank: map[string]WallTexture{
		"TROOA1":   {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		"TROOA2A8": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		"TROOA3A7": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		"TROOA4A6": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		"TROOA5":   {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
	}}}
	th := mapdata.Thing{Type: 3001, X: 0, Y: 0, Angle: 0}
	name, flip := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOA1" || flip {
		t.Fatalf("front got name=%q flip=%t", name, flip)
	}
	name, flip = g.monsterSpriteNameForView(0, th, 0, -100, 0)
	if name != "TROOA5" || flip {
		t.Fatalf("back got name=%q flip=%t", name, flip)
	}
}

func TestMonsterSpriteNameForViewUsesAttackFrames(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"TROOE1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			"TROOF1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingAttackTics: []int{16},
	}
	th := mapdata.Thing{Type: 3001, X: 0, Y: 0, Angle: 0}

	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOE1" {
		t.Fatalf("attack start got=%q want=TROOE1", name)
	}
	g.thingAttackTics[0] = 6
	name, _ = g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOF1" {
		t.Fatalf("attack late got=%q want=TROOF1", name)
	}
}

func TestMonsterSpriteNameForViewUsesPainFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"TROOG1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingPainTics: []int{6},
	}
	th := mapdata.Thing{Type: 3001, X: 0, Y: 0, Angle: 0}
	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOG1" {
		t.Fatalf("pain got=%q want=TROOG1", name)
	}
}

func TestMonsterSpriteNameForViewUsesDeathFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"TROOI0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			"TROOM0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDead:      []bool{true},
		thingDeathTics: []int{monsterDeathAnimTotalTics(3001)},
	}
	th := mapdata.Thing{Type: 3001, X: 0, Y: 0, Angle: 0}
	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOI0" {
		t.Fatalf("death start got=%q want=TROOI0", name)
	}
	g.thingDeathTics[0] = 0
	name, _ = g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "TROOM0" {
		t.Fatalf("death end got=%q want=TROOM0", name)
	}
}

func TestMonsterDeathFrameSeqMatchesDoomForShotgunGuyAndLostSoul(t *testing.T) {
	if got, want := string(monsterDeathFrameSeq(9)), "HIJKL"; got != want {
		t.Fatalf("shotgun death seq=%q want=%q", got, want)
	}
	if got, want := string(monsterDeathFrameSeq(3006)), "FGHIJK"; got != want {
		t.Fatalf("lost soul death seq=%q want=%q", got, want)
	}
	if got, want := len(monsterDeathFrameTics(9)), 5; got != want {
		t.Fatalf("shotgun death tics len=%d want=%d", got, want)
	}
}

func TestMonsterDeathFrameSeq_SpectreMatchesDemon(t *testing.T) {
	if got, want := string(monsterDeathFrameSeq(58)), string(monsterDeathFrameSeq(3002)); got != want {
		t.Fatalf("spectre death seq=%q want demon %q", got, want)
	}
	if got, want := len(monsterDeathFrameTics(58)), len(monsterDeathFrameTics(3002)); got != want {
		t.Fatalf("spectre death tics len=%d want demon %d", got, want)
	}
	if got, want := monsterDeathAnimTotalTics(58), monsterDeathAnimTotalTics(3002); got != want {
		t.Fatalf("spectre death total=%d want demon %d", got, want)
	}
}

func TestMonsterSpritePrefixCoversAllMonsterTypes(t *testing.T) {
	for _, typ := range []int16{3004, 9, 84, 3001, 3002, 58, 3006, 3005, 3003, 69, 64, 65, 66, 67, 68, 16, 7, 71} {
		if prefix, ok := monsterSpritePrefix(typ); !ok || prefix == "" {
			t.Fatalf("monster type %d missing sprite prefix", typ)
		}
	}
}

func TestMonsterSpriteNameForView_SpectreUsesDeathFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"SARGI0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			"SARGN0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDead:      []bool{true},
		thingDeathTics: []int{monsterDeathAnimTotalTics(58)},
	}
	th := mapdata.Thing{Type: 58, X: 0, Y: 0, Angle: 0}
	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "SARGI0" {
		t.Fatalf("spectre death start got=%q want=SARGI0", name)
	}
	g.thingDeathTics[0] = 0
	name, _ = g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "SARGN0" {
		t.Fatalf("spectre death end got=%q want=SARGN0", name)
	}
}

func TestMonsterVisibleAfterDeath_LostSoulDisappearsWhenAnimEnds(t *testing.T) {
	g := &game{
		thingDead:      []bool{true, true, true},
		thingDeathTics: []int{1, 0, 0},
	}
	if !g.monsterVisibleAfterDeath(0, 3006) {
		t.Fatalf("lost soul should remain visible while death animation is active")
	}
	if g.monsterVisibleAfterDeath(1, 3006) {
		t.Fatalf("lost soul should disappear after death animation finishes")
	}
	if !g.monsterVisibleAfterDeath(2, 3002) {
		t.Fatalf("demon corpse should remain visible after death animation finishes")
	}
}
