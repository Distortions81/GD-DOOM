package doomruntime

import "testing"

func TestSpriteFootprintClipYBounds_IgnoresAdjacentRaisedSectorAcrossWall(t *testing.T) {
	g := newDoorTimingGame(1)
	g.sectorFloor[0] = 0
	g.sectorFloor[1] = 64 * fracUnit
	g.sectorCeil[0] = 128 * fracUnit
	g.sectorCeil[1] = 128 * fracUnit
	g.viewH = 200

	x := int64(-4 * fracUnit)
	y := int64(0)
	radius := int64(8 * fracUnit)
	eyeZ := 41.0
	depth := 64.0
	focal := doomFocalLength(320)

	gotTop, gotBottom, ok := g.spriteFootprintClipYBounds(x, y, radius, g.viewH, eyeZ, depth, focal)
	if !ok {
		t.Fatal("spriteFootprintClipYBounds unexpectedly rejected sprite")
	}

	wantTop, wantBottom, wantOK := spriteSectorClipYBounds(g.viewH, eyeZ, depth, focal, g.sectorFloor[0], g.sectorCeil[0])
	if !wantOK {
		t.Fatal("spriteSectorClipYBounds unexpectedly rejected center sector")
	}
	if gotTop != wantTop || gotBottom != wantBottom {
		t.Fatalf("clip bounds=(%d,%d) want center sector bounds (%d,%d)", gotTop, gotBottom, wantTop, wantBottom)
	}
}
