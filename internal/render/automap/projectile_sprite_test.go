package automap

import "testing"

func TestProjectileSpriteName_SelectsAvailableFrame(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"MISLA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL7A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL7B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSSA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSSB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	if got := g.projectileSpriteName(projectileRocket, 0); got != "MISLA0" {
		t.Fatalf("rocket frame 0 sprite=%q want MISLA0", got)
	}
	if got := g.projectileSpriteName(projectileRocket, 4); got != "MISLB0" {
		t.Fatalf("rocket frame 1 sprite=%q want MISLB0", got)
	}
	if got := g.projectileSpriteName(projectileBaronBall, 0); got != "BAL7A0" {
		t.Fatalf("baron frame 0 sprite=%q want BAL7A0", got)
	}
	if got := g.projectileSpriteName(projectileBaronBall, 6); got != "BAL7B0" {
		t.Fatalf("baron frame 1 sprite=%q want BAL7B0", got)
	}
	if got := g.projectileSpriteName(projectilePlasmaBall, 0); got != "PLSSA0" {
		t.Fatalf("plasma frame 0 sprite=%q want PLSSA0", got)
	}
	if got := g.projectileSpriteName(projectileFireball, 6); got != "BAL1B0" {
		t.Fatalf("fireball frame 1 sprite=%q want BAL1B0", got)
	}
}

