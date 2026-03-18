package doomruntime

import "testing"

func TestProjectileSpriteName_SelectsAvailableFrame(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"MISLA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MISLB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL7A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL7B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FATBA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FATBB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MANFA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"MANFB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2A0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2B0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2C0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2D0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL2E0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSED0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEE0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
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
	if got := g.projectileSpriteName(projectileTracer, 0); got != "FATBA0" {
		t.Fatalf("tracer frame 0 sprite=%q want FATBA0", got)
	}
	if got := g.projectileSpriteName(projectileFatShot, 6); got != "MANFB0" {
		t.Fatalf("fatshot frame 1 sprite=%q want MANFB0", got)
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
				"PLSEA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSED0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"PLSEE0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FBXPA0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FBXPB0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"FBXPC0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
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
	if got := g.projectileImpactSpriteName(projectilePlayerPlasma, 0); got != "PLSEA0" {
		t.Fatalf("player plasma impact start=%q want PLSEA0", got)
	}
	if got := g.projectileImpactSpriteName(projectilePlayerPlasma, 4); got != "PLSEB0" {
		t.Fatalf("player plasma impact second=%q want PLSEB0", got)
	}
	if got := g.projectileImpactSpriteName(projectilePlayerPlasma, 16); got != "PLSEE0" {
		t.Fatalf("player plasma impact end=%q want PLSEE0", got)
	}
	if got := g.projectileImpactSpriteName(projectileTracer, 0); got != "FBXPA0" {
		t.Fatalf("tracer impact start=%q want FBXPA0", got)
	}
	if got := g.projectileImpactSpriteName(projectileTracer, 8); got != "FBXPB0" {
		t.Fatalf("tracer impact mid=%q want FBXPB0", got)
	}
	if got := g.projectileImpactSpriteName(projectileTracer, 14); got != "FBXPC0" {
		t.Fatalf("tracer impact end=%q want FBXPC0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 0); got != "MISLB0" {
		t.Fatalf("rocket impact start=%q want MISLB0", got)
	}
	if got := g.projectileImpactSpriteName(projectileFatShot, 0); got != "MISLB0" {
		t.Fatalf("fatshot impact start=%q want MISLB0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 8); got != "MISLC0" {
		t.Fatalf("rocket impact mid=%q want MISLC0", got)
	}
	if got := g.projectileImpactSpriteName(projectileRocket, 14); got != "MISLD0" {
		t.Fatalf("rocket impact end=%q want MISLD0", got)
	}
}
