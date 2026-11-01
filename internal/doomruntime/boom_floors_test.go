package doomruntime

import (
	"gddoom/internal/mapdata"
	"testing"
)

func boomFloorGame(special uint16) *game {
	m := &mapdata.Map{
		Vertexes: []mapdata.Vertex{{}, {X: 64}},
		Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Flags: mlTwoSided, Special: special, Tag: 7, SideNum: [2]int32{0, 1}}},
		Sidedefs: []mapdata.Sidedef{{Sector: 0, Bottom: "SHORT"}, {Sector: 1, Bottom: "SHORT"}},
		Sectors:  []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128, FloorPic: "MODEL", Special: 32}, {FloorHeight: 32, CeilingHeight: 96, FloorPic: "OLD", Special: 64, Tag: 7}},
	}
	return &game{m: m, sectorFloor: []int64{0, 32 * fracUnit}, sectorCeil: []int64{128 * fracUnit, 96 * fracUnit}, lineSpecial: []uint16{special}, opts: Options{WallTextureHeights: map[string]int{"SHORT": 16}}}
}

func TestBoomGeneralizedFloorTargetsAndSpeeds(t *testing.T) {
	for target := uint16(0); target < 8; target++ {
		for speed := uint16(0); speed < 4; speed++ {
			for _, up := range []bool{false, true} {
				special := uint16(0x6002) | (target << 7) | (speed << 3)
				if up {
					special |= 64
				}
				g := boomFloorGame(special)
				if !g.useSpecialLineForActor(0, 0, true) {
					t.Fatalf("special %x did not activate", special)
				}
				want := int64(0)
				switch target {
				case 2:
					if up {
						want = 32
					}
				case 3:
					want = 128
				case 4:
					want = 96
				case 5:
					want = 16
					if up {
						want = 48
					}
				case 6:
					want = 8
					if up {
						want = 56
					}
				case 7:
					want = 0
					if up {
						want = 64
					}
				}
				ft := g.floors[1]
				if ft == nil || ft.destHeight != want*fracUnit || ft.speed != fracUnit<<speed {
					t.Fatalf("special %x mover=%+v expected dest=%d speed=%d", special, ft, want, fracUnit<<speed)
				}
			}
		}
	}
}

func TestBoomGeneralizedFloorTextureChangesOnCompletion(t *testing.T) {
	for change := uint16(1); change < 4; change++ {
		g := boomFloorGame(0x6002 | (change << 10))
		g.useSpecialLineForActor(0, 0, true)
		if g.m.Sectors[1].FloorPic != "OLD" {
			t.Fatal("texture changed before completion")
		}
		// Texture-only must retain a type changed while the floor is moving.
		g.m.Sectors[1].Special = 96
		for i := 0; i < 33; i++ {
			if ft := g.floors[1]; ft != nil {
				g.tickFloor(1, ft)
			}
		}
		want := int16(0)
		if change == 2 {
			want = 96
		}
		if change == 3 {
			want = 32
		}
		if g.m.Sectors[1].FloorPic != "MODEL" || g.m.Sectors[1].Special != want || g.floors[1] != nil {
			t.Fatalf("change %d: sector=%+v mover=%+v", change, g.m.Sectors[1], g.floors[1])
		}
	}
}

func TestBoomGeneralizedManualFloorAndIndependentCeiling(t *testing.T) {
	g := boomFloorGame(0x6006 | 64 | (6 << 7)) // P1 raise 24
	g.m.Linedefs[0].Tag = 0
	g.ceilings = map[int]*ceilingThinker{1: {sector: 1}}
	if !g.useSpecialLineForActor(0, 0, true) || g.floors[1] == nil || g.lineSpecial[0] != 0 {
		t.Fatal("manual floor did not run alongside ceiling or consume P1")
	}
}

func TestBoomGeneralizedFloorNumericModelAndNoModel(t *testing.T) {
	g := boomFloorGame(0x6002 | 32 | (3 << 10))
	g.useSpecialLineForActor(0, 0, true)
	if g.floors[1].finishFlat != "MODEL" || g.floors[1].finishSpecial != 32 {
		t.Fatal("numeric model not selected")
	}
	g = boomFloorGame(0x6002 | 32 | (3 << 10) | (6 << 7))
	g.useSpecialLineForActor(0, 0, true)
	if g.floors[1].finish != floorFinishNone {
		t.Fatal("nonexistent numeric model changed texture/type")
	}
}
