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

func TestMonsterSpriteRotFrameSupportsOneFivePairs(t *testing.T) {
	g := &game{opts: Options{SpritePatchBank: map[string]WallTexture{
		"BAL7A1A5": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
	}}}
	name, flip, ok := g.monsterSpriteRotFrame("BAL7", 'A', 1)
	if !ok || name != "BAL7A1A5" || flip {
		t.Fatalf("rot1 got name=%q flip=%t ok=%t", name, flip, ok)
	}
	name, flip, ok = g.monsterSpriteRotFrame("BAL7", 'A', 5)
	if !ok || name != "BAL7A1A5" || !flip {
		t.Fatalf("rot5 got name=%q flip=%t ok=%t", name, flip, ok)
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

func TestMonsterSpriteNameForView_CyberdemonUsesOneFivePairRotation(t *testing.T) {
	g := &game{opts: Options{SpritePatchBank: map[string]WallTexture{
		"CYBRA1A5": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
	}}}
	th := mapdata.Thing{Type: 16, X: 0, Y: 0, Angle: 0}
	name, flip := g.monsterSpriteNameForView(0, th, 0, -100, 0)
	if name != "CYBRA1A5" || !flip {
		t.Fatalf("back got name=%q flip=%t want CYBRA1A5 flipped", name, flip)
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

func TestMonsterSpriteNameForViewUsesExactDoomHitscannerFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"SPOSF1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDoomState: []int{219},
	}
	th := mapdata.Thing{Type: 9, X: 0, Y: 0, Angle: 0}

	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "SPOSF1" {
		t.Fatalf("state 219 got=%q want=SPOSF1", name)
	}
}

func TestMonsterSpriteNameForViewUsesExactDoomCacodemonFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"HEADC1": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDoomState: []int{505},
	}
	th := mapdata.Thing{Type: 3005, X: 0, Y: 0, Angle: 0}

	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "HEADC1" {
		t.Fatalf("state 505 got=%q want=HEADC1", name)
	}
}

func TestThingRenderPosFixedInterpolatesMonsterPosition(t *testing.T) {
	g := &game{
		prevThingX:        []int64{0},
		prevThingY:        []int64{0},
		prevThingZ:        []int64{0},
		thingX:            []int64{128 * fracUnit},
		thingY:            []int64{64 * fracUnit},
		thingZState:       []int64{32 * fracUnit},
		thingFloorState:   []int64{0},
		thingCeilState:    []int64{128 * fracUnit},
		thingSupportValid: []bool{true},
	}
	th := mapdata.Thing{Type: 3001}
	x, y, z := g.thingRenderPosFixed(0, th, 0.5)
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
	if got, want := string(monsterDeathFrameSeq(65)), "HIJKLMN"; got != want {
		t.Fatalf("chaingunner death seq=%q want=%q", got, want)
	}
	if got, want := string(monsterDeathFrameSeq(3006)), "FGHIJK"; got != want {
		t.Fatalf("lost soul death seq=%q want=%q", got, want)
	}
	if got, want := len(monsterDeathFrameTics(9)), 5; got != want {
		t.Fatalf("shotgun death tics len=%d want=%d", got, want)
	}
	if got, want := len(monsterDeathFrameTics(65)), 7; got != want {
		t.Fatalf("chaingunner death tics len=%d want=%d", got, want)
	}
}

func TestMonsterSpriteNameForView_ChaingunnerUsesDeathFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"CPOSH0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			"CPOSN0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDead:      []bool{true},
		thingDeathTics: []int{monsterDeathAnimTotalTics(65)},
	}
	th := mapdata.Thing{Type: 65, X: 0, Y: 0, Angle: 0}
	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "CPOSH0" {
		t.Fatalf("chaingunner death start got=%q want=CPOSH0", name)
	}
	g.thingDeathTics[0] = 0
	name, _ = g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "CPOSN0" {
		t.Fatalf("chaingunner death end got=%q want=CPOSN0", name)
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

func TestMonsterDeathFrameSeq_HellKnightMatchesBaron(t *testing.T) {
	if got, want := string(monsterDeathFrameSeq(69)), string(monsterDeathFrameSeq(3003)); got != want {
		t.Fatalf("hell knight death seq=%q want baron %q", got, want)
	}
	if got, want := len(monsterDeathFrameTics(69)), len(monsterDeathFrameTics(3003)); got != want {
		t.Fatalf("hell knight death tics len=%d want baron %d", got, want)
	}
	if got, want := monsterDeathAnimTotalTics(69), monsterDeathAnimTotalTics(3003); got != want {
		t.Fatalf("hell knight death total=%d want baron %d", got, want)
	}
	if got, want := monsterDeathSoundDelayTics(69), 8; got != want {
		t.Fatalf("hell knight death sound delay=%d want=%d", got, want)
	}
}

func TestMonsterDeathFrameSeq_ArchvileMatchesDoomSource(t *testing.T) {
	if got, want := string(monsterDeathFrameSeq(64)), "QRSTUVWXYZ"; got != want {
		t.Fatalf("arch-vile death seq=%q want=%q", got, want)
	}
	if got, want := monsterDeathFrameTics(64), []int{7, 7, 7, 7, 7, 7, 7, 5, 5, -1}; len(got) != len(want) {
		t.Fatalf("arch-vile death tics len=%d want=%d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("arch-vile death tics[%d]=%d want=%d", i, got[i], want[i])
			}
		}
	}
	if got, want := monsterDeathSoundDelayTics(64), 7; got != want {
		t.Fatalf("arch-vile death sound delay=%d want=%d", got, want)
	}
	if got, want := monsterDeathSoundDelayTics(67), 6; got != want {
		t.Fatalf("mancubus death sound delay=%d want=%d", got, want)
	}
	if got, want := monsterDeathSoundDelayTics(71), 8; got != want {
		t.Fatalf("pain elemental death sound delay=%d want=%d", got, want)
	}
}

func TestMonsterSpriteNameForView_HellKnightUsesDeathFrame(t *testing.T) {
	g := &game{
		opts: Options{SpritePatchBank: map[string]WallTexture{
			"BOS2I0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
			"BOS2O0": {Width: 1, Height: 1, RGBA: []byte{255, 255, 255, 255}},
		}},
		thingDead:      []bool{true},
		thingDeathTics: []int{monsterDeathAnimTotalTics(69)},
	}
	th := mapdata.Thing{Type: 69, X: 0, Y: 0, Angle: 0}
	name, _ := g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "BOS2I0" {
		t.Fatalf("hell knight death start got=%q want=BOS2I0", name)
	}
	g.thingDeathTics[0] = 0
	name, _ = g.monsterSpriteNameForView(0, th, 0, 100, 0)
	if name != "BOS2O0" {
		t.Fatalf("hell knight death end got=%q want=BOS2O0", name)
	}
}

func TestMonsterSpritePrefixCoversAllMonsterTypes(t *testing.T) {
	for _, typ := range []int16{3004, 9, 84, 3001, 3002, 58, 3006, 3005, 3003, 69, 64, 65, 66, 67, 68, 16, 7, 71} {
		if prefix, ok := monsterSpritePrefix(typ); !ok || prefix == "" {
			t.Fatalf("monster type %d missing sprite prefix", typ)
		}
	}
}

func TestMonsterRenderBaseZ_UsesActualZForLiveFloatMonsters(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 3005, X: 0, Y: 0},
				{Type: 65, X: 0, Y: 0},
			},
		},
		thingDead:         []bool{false, false},
		thingZState:       []int64{32 * fracUnit, 32 * fracUnit},
		thingFloorState:   []int64{0, 0},
		thingCeilState:    []int64{128 * fracUnit, 128 * fracUnit},
		thingSupportValid: []bool{true, true},
	}
	if got := g.monsterRenderBaseZ(0, g.m.Things[0], 0, 0); got != 32*fracUnit {
		t.Fatalf("caco render base z=%d want=%d", got, 32*fracUnit)
	}
	if got := g.monsterRenderBaseZ(1, g.m.Things[1], 0, 0); got != 0 {
		t.Fatalf("chaingunner render base z=%d want=0", got)
	}
}

func TestCacheOriginSpriteItemGeometry_UsesPatchOffsetsLikeDoom(t *testing.T) {
	tex := &WallTexture{Width: 40, Height: 56, OffsetX: 18, OffsetY: 52}
	w, h, dstX, dstY, x0, x1, y0, y1, ok := cacheOriginSpriteItemGeometry(160, 100, 2, tex, 0, 199, 320, 200)
	if !ok {
		t.Fatal("cacheOriginSpriteItemGeometry returned not ok")
	}
	if got, want := w, 80.0; got != want {
		t.Fatalf("width=%v want=%v", got, want)
	}
	if got, want := h, 112.0; got != want {
		t.Fatalf("height=%v want=%v", got, want)
	}
	if got, want := dstX, 124.0; got != want {
		t.Fatalf("dstX=%v want=%v", got, want)
	}
	if got, want := dstY, -4.0; got != want {
		t.Fatalf("dstY=%v want=%v", got, want)
	}
	if got, want := x0, 124; got != want {
		t.Fatalf("x0=%d want=%d", got, want)
	}
	if got, want := x1, 203; got != want {
		t.Fatalf("x1=%d want=%d", got, want)
	}
	if got, want := y0, 0; got != want {
		t.Fatalf("y0=%d want=%d", got, want)
	}
	if got, want := y1, 107; got != want {
		t.Fatalf("y1=%d want=%d", got, want)
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
