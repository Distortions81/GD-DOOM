package automap

import "testing"

func TestWorldThingSpriteName_PickupAndDecor(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"STIMA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAR1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"POSSL0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	if got := g.worldThingSpriteName(2011, 0); got != "STIMA0" {
		t.Fatalf("stimpack sprite=%q want STIMA0", got)
	}
	if got := g.worldThingSpriteName(2035, 0); got != "BAR1A0" {
		t.Fatalf("barrel sprite=%q want BAR1A0", got)
	}
	if got := g.worldThingSpriteName(18, 0); got != "POSSL0" {
		t.Fatalf("corpse sprite=%q want POSSL0", got)
	}
}
