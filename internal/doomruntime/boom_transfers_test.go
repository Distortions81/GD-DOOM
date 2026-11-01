package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"path/filepath"
	"testing"
)

func loadBoomTransferMap(t *testing.T, name mapdata.MapName) *mapdata.Map {
	t.Helper()
	w, err := wad.Open(filepath.Join("..", "..", "testdata", "boom", "transfers.wad"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, name)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestBoomPlaneLightTransfers(t *testing.T) {
	g := newGame(loadBoomTransferMap(t, "MAP01"), Options{Headless: true})
	if g.boomFloorLight[0] != 1 || g.boomCeilingLight[0] != 2 {
		t.Fatal("control sectors not mapped")
	}
	floor := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], true)
	ceil := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], false)
	if floor.light != 48 || ceil.light != 224 || g.sectorLightForRender(0, &g.m.Sectors[0]) != 160 {
		t.Fatalf("floor=%d ceiling=%d walls=%d", floor.light, ceil.light, g.m.Sectors[0].Light)
	}
	g.m.Sectors[1].Light = 96
	g.m.Sectors[2].Light = 192
	g.refreshSectorPlaneCacheLighting()
	if g.planeLightForRender(0, &g.m.Sectors[0], true) != 96 || g.planeLightForRender(0, &g.m.Sectors[0], false) != 192 {
		t.Fatal("transferred light froze at initial control value")
	}
	// Transfers read the source's own light, not another transfer recursively.
	g.boomFloorLight[1] = 2
	if g.planeLightForRender(0, &g.m.Sectors[0], true) != 96 {
		t.Fatal("light transfer recursed through control sector")
	}
	saved := captureGameSaveState(g)
	restored := newGame(loadBoomTransferMap(t, "MAP01"), Options{Headless: true})
	restoreGameSaveState(restored, saved)
	if restored.planeLightForRender(0, &restored.m.Sectors[0], true) != 96 {
		t.Fatal("light references did not survive restore")
	}
}
func TestBoomTransferSetupOrderAndTagZero(t *testing.T) {
	g := &game{m: &mapdata.Map{Sectors: []mapdata.Sector{{Tag: 0}, {Tag: 0}, {Tag: 7}}, Sidedefs: []mapdata.Sidedef{{Sector: 1}, {Sector: 2}}, Linedefs: []mapdata.Linedef{{Special: 213, SideNum: [2]int32{0, -1}}, {Special: 213, SideNum: [2]int32{1, -1}}, {Special: 261, Tag: 7, SideNum: [2]int32{-1, -1}}}}}
	g.initBoomTransfers()
	if g.boomFloorLight[0] != 2 || g.boomFloorLight[1] != 2 || g.boomFloorLight[2] != -1 || g.boomCeilingLight[2] != -1 {
		t.Fatal("setup order, tag zero, or invalid sidedef handling")
	}
}
func TestBoomSkyTransferCoordinatesAndIdentity(t *testing.T) {
	for _, sourcePort := range []bool{false, true} {
		g := newGame(loadBoomTransferMap(t, "MAP02"), Options{Headless: true, SourcePortMode: sourcePort, WallTexBank: map[string]WallTexture{"SKY2": {Width: 256, Height: 128, Indexed: make([]byte, 256*128), RGBA: make([]byte, 256*128*4)}}})
		g.viewW, g.viewH = 320, 200
		frame := g.transferredSkyFrame(g.boomSkyLine[0], 0, 160)
		if frame == nil {
			t.Fatal("missing transferred sky")
		}
		first := frame.columns[160]
		line := g.boomSkyLine[0] - 1
		g.m.Linedefs[line].Special = 272
		flipped := g.transferredSkyFrame(line+1, 0, 160)
		if first != 255-flipped.columns[160] {
			t.Fatal("271/272 horizontal orientation incorrect")
		}
		side := g.m.Linedefs[line].SideNum[0]
		g.m.Sidedefs[side].TextureOffset = 8192
		g.m.Sidedefs[side].RowOffset = 160
		offset := g.transferredSkyFrame(line+1, 0, 160)
		if offset.columns[160] != wrapIndex(flipped.columns[160]+128, 256) || offset.rows[100] != 4 {
			t.Fatalf("BAM/vertical offsets: %d %d", offset.columns[160], offset.rows[100])
		}
		key := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], false)
		if !key.sky || key.skyLine != line+1 || key.light != 0 || key.height != 0 || plane3DKeyFromScene(plane3DKeyToScene(key)) != key {
			t.Fatal("sky identity lost during plane grouping")
		}
	}
	g := newGame(loadBoomTransferMap(t, "MAP04"), Options{Headless: true})
	if !g.sectorsLightDifferForRender(0, 1, &g.m.Sectors[0], &g.m.Sectors[1]) {
		t.Fatal("multi-sky/plane-light portal was culled")
	}
	a := g.plane3DKeyForSectorCached(0, &g.m.Sectors[0], false)
	b := g.plane3DKeyForSectorCached(1, &g.m.Sectors[1], false)
	if a == b {
		t.Fatal("distinct transferred skies merged")
	}
}

func TestBoomTransferredSkyRetainsTallTexture(t *testing.T) {
	tex := WallTexture{Width: 256, Height: 256, RGBA: make([]byte, 256*256*4), Indexed: make([]byte, 256*256)}
	tex.SetClassicOpaque(make([]byte, 256*128))
	g := newGame(loadBoomTransferMap(t, "MAP02"), Options{Headless: true, WallTexBank: map[string]WallTexture{"SKY2": tex}})
	g.viewW, g.viewH = 320, 200
	side := g.m.Linedefs[g.boomSkyLine[0]-1].SideNum[0]
	g.m.Sidedefs[side].RowOffset = 8
	sky := g.transferredSkyFrame(g.boomSkyLine[0], 0, 160)
	if sky == nil || sky.texture.Height != 256 || sky.rows[0] != 136 {
		t.Fatalf("tall sky was truncated to vanilla 128 rows: %+v", sky)
	}
}
func TestBoomTransferredLightFollowsGlow(t *testing.T) {
	m := loadBoomTransferMap(t, "MAP01")
	m.Sectors[1].Special = 8
	g := newGame(m, Options{Headless: true})
	g.sectorLightFx[1].minLight = 0 // A darker neighbor bounds this control-sector glow.
	start := g.m.Sectors[1].Light
	for i := 0; i < 8; i++ {
		g.runGameplayTic(moveCmd{}, false, false)
		if g.planeLightForRender(0, &g.m.Sectors[0], true) != g.m.Sectors[1].Light {
			t.Fatal("plane light lost control sector glow")
		}
	}
	if start == g.m.Sectors[1].Light || g.sectorLightForRender(0, &g.m.Sectors[0]) != 160 {
		t.Fatal("control did not animate or altered wall light")
	}
}
