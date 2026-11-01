package doomruntime

import (
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"path/filepath"
	"reflect"
	"testing"
)

func boomControlledScrollerGame(special uint16) *game {
	return newGame(&mapdata.Map{Things: []mapdata.Thing{{Type: 1, Flags: 7}}, Vertexes: []mapdata.Vertex{{}, {X: 32, Y: 16}}, Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Special: special, Tag: 7, SideNum: [2]int32{0, -1}}}, Sidedefs: []mapdata.Sidedef{{Sector: 1}}, Sectors: []mapdata.Sector{{Tag: 7, CeilingHeight: 128}, {CeilingHeight: 128}}}, Options{Headless: true})
}
func TestBoomControlledPlaneScrollerMotion(t *testing.T) {
	for _, special := range []uint16{245, 246, 248, 214, 215, 217} {
		t.Run(fmt.Sprint(special), func(t *testing.T) {
			g := boomControlledScrollerGame(special)
			if len(g.boomScrollers) == 0 {
				t.Fatal("missing controller")
			}
			step := func() {
				for i := range g.boomScrollers {
					g.tickBoomScroller(i)
				}
			}
			step() // No control displacement, including nonzero initial heights.
			if g.boomScrollers[0].OffsetX != 0 {
				t.Fatal("initial height counted as displacement")
			}
			g.sectorFloor[1] += 2 * fracUnit
			step()
			if v := g.boomScrollers[0]; v.OffsetX != -2*fracUnit || v.OffsetY != fracUnit {
				t.Fatalf("first displacement: %+v", v)
			}
			g.sectorCeil[1] += 2 * fracUnit
			step()
			accelerating := special < 245
			want := int64(-4 * fracUnit)
			if accelerating {
				want = -6 * fracUnit
			}
			if g.boomScrollers[0].OffsetX != want {
				t.Fatalf("second offset=%d want=%d", g.boomScrollers[0].OffsetX, want)
			}
			step()
			if accelerating {
				want -= 4 * fracUnit
			}
			if g.boomScrollers[0].OffsetX != want {
				t.Fatal("stopped control did not preserve correct velocity")
			}
			g.sectorFloor[1] += fracUnit
			g.sectorCeil[1] -= fracUnit
			step()
			if accelerating {
				want -= 4 * fracUnit
			}
			if g.boomScrollers[0].OffsetX != want {
				t.Fatal("floor/ceiling height changes did not cancel")
			}
		})
	}
}
func TestBoomWallScrollTargetsRotationAndOffsets(t *testing.T) {
	m := &mapdata.Map{Things: []mapdata.Thing{{Type: 1, Flags: 7}}, Vertexes: []mapdata.Vertex{{}, {X: 32, Y: 16}, {X: 128}, {X: 128, Y: 128}}, Sectors: []mapdata.Sector{{CeilingHeight: 128}}, Sidedefs: []mapdata.Sidedef{{}, {}, {}}, Linedefs: []mapdata.Linedef{{V1: 0, V2: 1, Special: 254, Tag: 7, SideNum: [2]int32{0, -1}}, {V1: 2, V2: 3, Tag: 7, SideNum: [2]int32{1, 2}}}}
	g := newGame(m, Options{Headless: true})
	g.worldTic = 7
	if g.boomWallOffsets(0) != (boomScrollVector{}) || g.boomWallOffsets(2) != (boomScrollVector{}) {
		t.Fatal("source/back side was scrolled")
	}
	v := g.boomWallScroll[1]
	if v.x >= 0 || v.y >= 0 || abs(v.x+fracUnit/2) > 4 || abs(v.y+fracUnit) > 4 {
		t.Fatalf("wall rotation: %+v", v)
	}
	g.m.Linedefs[0].Special = 255
	g.m.Sidedefs[0].TextureOffset = 3
	g.m.Sidedefs[0].RowOffset = -2
	g.initBoomScrollers()
	if got := g.boomWallOffsets(0); got.x != -21*fracUnit || got.y != -14*fracUnit {
		t.Fatalf("255 offsets: %+v", got)
	}
	if x, y := boomWallScrollVector(fracUnit, fracUnit, 0, 0); x != 0 || y != 0 {
		t.Fatal("degenerate wall scrolled")
	}
}
func TestBoomScrollerFixedWrapAndAdditiveTargets(t *testing.T) {
	g := boomControlledScrollerGame(246)
	second := g.boomScrollers[0]
	second.Order++
	g.boomScrollers = append(g.boomScrollers, second)
	g.sectorFloor[1] += fracUnit
	for i := range g.boomScrollers {
		g.tickBoomScroller(i)
	}
	if g.boomPlaneOffset(0, true).x != -2*fracUnit {
		t.Fatal("multiple scrollers did not add")
	}
	g.boomScrollers = g.boomScrollers[:1]
	g.boomScrollers[0].OffsetX = -2147483648
	g.rebuildBoomScrollOffsets()
	g.sectorFloor[1] += fracUnit
	g.tickBoomScroller(0)
	if g.boomFloorOffset[0].x != 2147418112 {
		t.Fatal("fixed-point scroll did not wrap at 32 bits")
	}
}
func loadBoomScrollerFixture(t *testing.T) *game {
	t.Helper()
	w, err := wad.Open(filepath.Join("..", "..", "testdata", "boom", "scrollers.wad"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, "MAP09")
	if err != nil {
		t.Fatal(err)
	}
	return newGame(m, Options{Headless: true})
}
func TestBoomScrollerSaveAndKeyframeContinuation(t *testing.T) {
	for _, format := range []struct {
		magic   []byte
		version int
	}{{saveGameMagic, saveGameVersion}, {keyframeMagic, keyframeVersion}} {
		g := loadBoomScrollerFixture(t)
		for i := 0; i < 1055; i++ {
			g.runGameplayTic(moveCmd{}, false, false)
		}
		raw, err := encodeSnapshot(format.magic, saveFile{Version: format.version, Game: captureGameSaveState(g)})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSnapshot(raw, format.magic)
		if err != nil {
			t.Fatal(err)
		}
		restored := loadBoomScrollerFixture(t)
		restoreGameSaveState(restored, decoded.Game)
		for i := 0; i < 30; i++ {
			g.runGameplayTic(moveCmd{}, false, false)
			restored.runGameplayTic(moveCmd{}, false, false)
		}
		if !reflect.DeepEqual(g.boomScrollers, restored.boomScrollers) || !reflect.DeepEqual(g.boomFloorOffset, restored.boomFloorOffset) || g.p.x != restored.p.x || g.p.momx != restored.p.momx {
			t.Fatal("restored acceleration/carry diverged")
		}
		bad := append([]boomScrollerState(nil), g.boomScrollers...)
		bad[0].Target++
		if restored.validBoomScrollerStates(bad) {
			t.Fatal("snapshot changed static target")
		}
	}
}
func TestBoomCarryUnderwaterFloatingActors(t *testing.T) {
	g := boomConveyorGame()
	g.sectorFloor = append(g.sectorFloor, 8*fracUnit)
	g.boomHeightControl[0] = 1
	g.p.z = 4 * fracUnit
	g.thingZState[0] = 4 * fracUnit
	g.m.Things[0].Type = 3005
	g.tickBoomCarry(g.boomCarries[0])
	if g.p.momx == 0 || g.thingMomX[0] == 0 {
		t.Fatal("underwater carry excluded airborne/floating actor")
	}
	g.p.momx = 0
	g.noClip = true
	g.tickBoomCarry(g.boomCarries[0])
	if g.p.momx != 0 {
		t.Fatal("noclip actor was carried")
	}
	g.noClip = false
	g.projectiles = []projectile{{x: g.p.x, y: g.p.y, z: 4 * fracUnit, vz: 7}, {x: g.p.x, y: g.p.y, z: 8 * fracUnit, vz: 7}}
	g.p.momx = 2147483647
	g.tickBoomCarry(g.boomCarries[0])
	if g.projectiles[0].vx == 0 || g.projectiles[1].vx != 0 || g.projectiles[0].vz != 7 {
		t.Fatal("submerged missile carry/equality incorrect")
	}
	if g.p.momx >= 0 {
		t.Fatal("carry momentum did not wrap")
	}

}

func TestBoomWallScrollFeedsTransferredSkyOffsets(t *testing.T) {
	tex := WallTexture{Width: 256, Height: 128, RGBA: make([]byte, 256*128*4), Indexed: make([]byte, 256*128)}
	g := newGame(loadBoomTransferMap(t, "MAP02"), Options{Headless: true, WallTexBank: map[string]WallTexture{"SKY2": tex}})
	g.viewW, g.viewH = 320, 200
	g.worldTic = 1
	line := g.boomSkyLine[0]
	side := int(g.m.Linedefs[line-1].SideNum[0])
	before := g.transferredSkyFrame(line, 0, 160)
	g.boomWallScroll[side] = boomScrollVector{64 * fracUnit, 2 * fracUnit}
	after := g.transferredSkyFrame(line, 0, 160)
	if after.columns[160] != wrapIndex(before.columns[160]-1, 256) || after.rows[100] != wrapIndex(before.rows[100]+2, 128) {
		t.Fatal("scrolling sky control sidedef lost its offsets")
	}
}

func TestBoomScrollerAuthoritySnapshotRestoresPhase(t *testing.T) {
	fixture := loadBoomScrollerFixture(t)
	a, err := NewAuthority(cloneMapForRestart(fixture.restartTemplate), Options{SkillLevel: 3, NoMonsters: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1055; i++ {
		if err := a.Step(nil); err != nil {
			t.Fatal(err)
		}
	}
	data, err := a.Snapshot(1)
	if err != nil {
		t.Fatal(err)
	}
	client := loadBoomScrollerFixture(t)
	if err := client.applyAuthoritySnapshot(data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.boomScrollers, a.g.boomScrollers) || client.boomPlaneOffset(0, true) != a.g.boomPlaneOffset(0, true) {
		t.Fatal("authority snapshot lost accumulated scroller state")
	}
}
