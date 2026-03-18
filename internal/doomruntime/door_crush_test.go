package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestNormalDoorReopensWhenPlayerBlocksClose(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.p.x = int64(math.Round(1519.95 * fracUnit))
	g.p.y = int64(math.Round(-2508.87 * fracUnit))
	g.p.angle = doomAngleFromDegrees(2.0)

	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 {
		t.Fatalf("expected spawn door use target, got line=%d trace=%v", lineIdx, tr)
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		t.Fatalf("target line %d is not a door special", lineIdx)
	}
	if !g.activateDoorLine(lineIdx, info) {
		t.Fatalf("failed to activate spawn door line %d", lineIdx)
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		t.Fatalf("door target sectors for line %d: %v", lineIdx, err)
	}
	doorSec := targets[0]
	d := g.doors[doorSec]
	if d == nil {
		t.Fatal("expected active door thinker")
	}

	// Put the player inside the door sector footprint so the closing step would
	// intersect the player body.
	px, py, ok := e1m1DoorSectorSample(g, doorSec)
	if !ok {
		t.Fatalf("failed to find sample point in door sector %d", doorSec)
	}
	g.p.x = px
	g.p.y = py
	g.p.floorz = g.sectorFloor[doorSec]
	g.p.z = g.p.floorz
	g.p.ceilz = g.sectorCeil[doorSec]

	// Force a partially open close step on a normal door.
	d.typ = doorNormal
	d.direction = -1
	d.topCountdown = 0
	g.sectorCeil[doorSec] = g.sectorFloor[doorSec] + playerHeight - fracUnit
	before := g.sectorCeil[doorSec]

	g.tickDoors()

	if g.sectorCeil[doorSec] != before {
		t.Fatalf("door ceiling moved into blocking player: got %d want %d", g.sectorCeil[doorSec], before)
	}
	if d.direction != 1 {
		t.Fatalf("blocking normal door should reverse open, direction=%d", d.direction)
	}
}

func e1m1DoorSectorSample(g *game, doorSec int) (int64, int64, bool) {
	if g == nil || g.m == nil || doorSec < 0 || doorSec >= len(g.m.Sectors) {
		return 0, 0, false
	}
	for segIdx, seg := range g.m.Segs {
		if int(seg.Linedef) < 0 || int(seg.Linedef) >= len(g.m.Linedefs) {
			continue
		}
		frontIdx, backIdx := g.segSectorIndices(segIdx)
		if frontIdx != doorSec && backIdx != doorSec {
			continue
		}
		if int(seg.StartVertex) < 0 || int(seg.StartVertex) >= len(g.m.Vertexes) ||
			int(seg.EndVertex) < 0 || int(seg.EndVertex) >= len(g.m.Vertexes) {
			continue
		}
		v1 := g.m.Vertexes[seg.StartVertex]
		v2 := g.m.Vertexes[seg.EndVertex]
		mx := (int64(v1.X) + int64(v2.X)) << (fracBits - 1)
		my := (int64(v1.Y) + int64(v2.Y)) << (fracBits - 1)
		dx := int64(v2.X - v1.X)
		dy := int64(v2.Y - v1.Y)
		normalX := -dy
		normalY := dx
		samples := [][2]int64{
			{mx, my},
			{mx + normalX*(8*fracUnit)/max64(abs(dx)+abs(dy), 1), my + normalY*(8*fracUnit)/max64(abs(dx)+abs(dy), 1)},
			{mx - normalX*(8*fracUnit)/max64(abs(dx)+abs(dy), 1), my - normalY*(8*fracUnit)/max64(abs(dx)+abs(dy), 1)},
		}
		for _, s := range samples {
			if g.sectorAt(s[0], s[1]) == doorSec {
				return s[0], s[1], true
			}
		}
	}
	return 0, 0, false
}
