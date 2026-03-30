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

func TestProjectileSpriteName_UsesOneFivePairsForBaronBall(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"BAL7A1A5": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
				"BAL7B1B5": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			},
		},
	}
	if got := g.projectileSpriteName(projectileBaronBall, 0); got != "BAL7A1A5" {
		t.Fatalf("baron frame 0 sprite=%q want BAL7A1A5", got)
	}
	if got := g.projectileSpriteName(projectileBaronBall, 6); got != "BAL7B1B5" {
		t.Fatalf("baron frame 1 sprite=%q want BAL7B1B5", got)
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

func TestProjectileRenderPosFixedInterpolatesPosition(t *testing.T) {
	g := &game{}
	p := projectile{
		x:     96 * fracUnit,
		y:     48 * fracUnit,
		z:     24 * fracUnit,
		prevX: 32 * fracUnit,
		prevY: 16 * fracUnit,
		prevZ: 8 * fracUnit,
	}
	x, y, z := g.projectileRenderPosFixed(p, 0.5)
	if got, want := x, int64(64*fracUnit); got != want {
		t.Fatalf("x=%d want=%d", got, want)
	}
	if got, want := y, int64(32*fracUnit); got != want {
		t.Fatalf("y=%d want=%d", got, want)
	}
	if got, want := z, int64(16*fracUnit); got != want {
		t.Fatalf("z=%d want=%d", got, want)
	}
}

func TestPrecacheProjectileSpriteRefs_WarmsProjectileAndEffectCaches(t *testing.T) {
	g := &game{
		opts: Options{
			SpritePatchBank: map[string]WallTexture{
				"MISLA0": {Width: 1, Height: 1, RGBA: []byte{1, 2, 3, 255}},
				"MISLB0": {Width: 1, Height: 1, RGBA: []byte{4, 5, 6, 255}},
				"MISLC0": {Width: 1, Height: 1, RGBA: []byte{7, 8, 9, 255}},
				"MISLD0": {Width: 1, Height: 1, RGBA: []byte{10, 11, 12, 255}},
				"BAL1A0": {Width: 1, Height: 1, RGBA: []byte{13, 14, 15, 255}},
				"BAL1B0": {Width: 1, Height: 1, RGBA: []byte{16, 17, 18, 255}},
				"BAL1C0": {Width: 1, Height: 1, RGBA: []byte{19, 20, 21, 255}},
				"BAL1D0": {Width: 1, Height: 1, RGBA: []byte{22, 23, 24, 255}},
				"BAL1E0": {Width: 1, Height: 1, RGBA: []byte{25, 26, 27, 255}},
				"PUFFA0": {Width: 1, Height: 1, RGBA: []byte{28, 29, 30, 255}},
				"PUFFB0": {Width: 1, Height: 1, RGBA: []byte{31, 32, 33, 255}},
				"PUFFC0": {Width: 1, Height: 1, RGBA: []byte{34, 35, 36, 255}},
				"PUFFD0": {Width: 1, Height: 1, RGBA: []byte{37, 38, 39, 255}},
				"BLUDA0": {Width: 1, Height: 1, RGBA: []byte{40, 41, 42, 255}},
				"BLUDB0": {Width: 1, Height: 1, RGBA: []byte{43, 44, 45, 255}},
				"BLUDC0": {Width: 1, Height: 1, RGBA: []byte{46, 47, 48, 255}},
				"TFOGA0": {Width: 1, Height: 1, RGBA: []byte{49, 50, 51, 255}},
				"TFOGB0": {Width: 1, Height: 1, RGBA: []byte{52, 53, 54, 255}},
				"TFOGC0": {Width: 1, Height: 1, RGBA: []byte{55, 56, 57, 255}},
				"TFOGD0": {Width: 1, Height: 1, RGBA: []byte{58, 59, 60, 255}},
				"TFOGE0": {Width: 1, Height: 1, RGBA: []byte{61, 62, 63, 255}},
				"TFOGF0": {Width: 1, Height: 1, RGBA: []byte{64, 65, 66, 255}},
				"TFOGG0": {Width: 1, Height: 1, RGBA: []byte{67, 68, 69, 255}},
				"TFOGH0": {Width: 1, Height: 1, RGBA: []byte{70, 71, 72, 255}},
				"TFOGI0": {Width: 1, Height: 1, RGBA: []byte{73, 74, 75, 255}},
				"TFOGJ0": {Width: 1, Height: 1, RGBA: []byte{76, 77, 78, 255}},
				"BOSFA0": {Width: 1, Height: 1, RGBA: []byte{79, 80, 81, 255}},
				"BOSFB0": {Width: 1, Height: 1, RGBA: []byte{82, 83, 84, 255}},
				"BOSFC0": {Width: 1, Height: 1, RGBA: []byte{85, 86, 87, 255}},
				"BOSFD0": {Width: 1, Height: 1, RGBA: []byte{88, 89, 90, 255}},
				"FIREA0": {Width: 1, Height: 1, RGBA: []byte{91, 92, 93, 255}},
				"FIREB0": {Width: 1, Height: 1, RGBA: []byte{94, 95, 96, 255}},
				"FIREC0": {Width: 1, Height: 1, RGBA: []byte{97, 98, 99, 255}},
				"FIRED0": {Width: 1, Height: 1, RGBA: []byte{100, 101, 102, 255}},
				"FIREE0": {Width: 1, Height: 1, RGBA: []byte{103, 104, 105, 255}},
				"FIREF0": {Width: 1, Height: 1, RGBA: []byte{106, 107, 108, 255}},
				"FIREG0": {Width: 1, Height: 1, RGBA: []byte{109, 110, 111, 255}},
				"FIREH0": {Width: 1, Height: 1, RGBA: []byte{112, 113, 114, 255}},
			},
		},
	}
	g.spritePatchStore, g.spritePatchPtrs = buildTexturePointerCache(g.opts.SpritePatchBank)

	g.precacheProjectileSpriteRefs()

	for _, name := range []string{"MISLA0", "MISLC0", "PUFFA0", "BLUDA0", "TFOGJ0", "BOSFD0", "FIREH0"} {
		if ref := g.spriteRenderRefCache[name]; ref == nil {
			t.Fatalf("expected spriteRenderRefCache[%q] to be warmed", name)
		}
	}
}
