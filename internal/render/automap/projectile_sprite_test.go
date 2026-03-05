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
				"BAL2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2E0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1E0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	if got := g.projectileSpriteName(projectileRocket, 0); got != "MISLA0" {
		t.Fatalf("rocket frame 0 sprite=%q want MISLA0", got)
	}
	if got := g.projectileSpriteName(projectileRocket, 8); got != "MISLA0" {
		t.Fatalf("rocket flight sprite=%q want MISLA0", got)
	}
	if got := g.projectileSpriteName(projectileBaronBall, 0); got != "BAL7A0" {
		t.Fatalf("baron frame 0 sprite=%q want BAL7A0", got)
	}
	if got := g.projectileSpriteName(projectileBaronBall, 6); got != "BAL7B0" {
		t.Fatalf("baron frame 1 sprite=%q want BAL7B0", got)
	}
	if got := g.projectileSpriteName(projectilePlasmaBall, 0); got != "BAL2A0" {
		t.Fatalf("caco frame 0 sprite=%q want BAL2A0", got)
	}
	if got := g.projectileSpriteName(projectileFireball, 6); got != "BAL1B0" {
		t.Fatalf("fireball frame 1 sprite=%q want BAL1B0", got)
	}
}

func TestProjectileImpactSpriteName_DoomTimings(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BAL1C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL1E0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2E0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLD0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}

	if got := g.projectileImpactSpriteName(projectileFireball, 0); got != "BAL1C0" {
		t.Fatalf("imp impact start=%q want BAL1C0", got)
	}
	if got := g.projectileImpactSpriteName(projectileFireball, 6); got != "BAL1D0" {
		t.Fatalf("imp impact mid=%q want BAL1D0", got)
	}
	if got := g.projectileImpactSpriteName(projectileFireball, 12); got != "BAL1E0" {
		t.Fatalf("imp impact end=%q want BAL1E0", got)
	}
	if got := g.projectileImpactSpriteName(projectilePlasmaBall, 0); got != "BAL2C0" {
		t.Fatalf("caco impact start=%q want BAL2C0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 0); got != "MISLB0" {
		t.Fatalf("rocket impact start=%q want MISLB0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 8); got != "MISLC0" {
		t.Fatalf("rocket impact mid=%q want MISLC0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 14); got != "MISLD0" {
		t.Fatalf("rocket impact end=%q want MISLD0", got)
	}
}
