package automap

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
