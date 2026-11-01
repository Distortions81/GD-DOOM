package doomruntime

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	"path/filepath"
	"testing"
)

func boomWaterGame(t *testing.T, name mapdata.MapName) *game {
	t.Helper()
	w, err := wad.Open(filepath.Join("..", "..", "testdata", "boom", "water.wad"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, name)
	if err != nil {
		t.Fatal(err)
	}
	return newGame(m, Options{Headless: true})
}
func TestBoomHeightTransferCameraRegions(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	for _, tc := range []struct {
		name              string
		eye               float64
		floor, ceil       int64
		floorPic, ceilPic string
		light             int16
	}{
		{"above-water", 41, 0, 128 * fracUnit, "BWSURF", "BWROOM", 160},
		{"below-water", -32, -64 * fracUnit, -1, "BWDEEP", "BWCEIL", 96},
		{"at-surface", 0, -64 * fracUnit, -1, "BWDEEP", "BWCEIL", 96},
		{"one-unit-below", -1.0 / 65536, -64 * fracUnit, -1, "BWDEEP", "BWCEIL", 96},
		{"one-unit-above", 1.0 / 65536, 0, 128 * fracUnit, "BWSURF", "BWROOM", 160},
		{"above-fake-ceiling", 144, 128*fracUnit + 1, 160 * fracUnit, "BWDEEP", "BWCEIL", 96},
	} {
		g.prepareHeightTransfers(128, 0, tc.eye)
		v := g.heightView(1, false)
		if v == nil || v.floor != tc.floor || v.ceiling != tc.ceil || v.sector.FloorPic != tc.floorPic || v.sector.CeilingPic != tc.ceilPic || v.wallLight != tc.light || v.floorLight != tc.light || v.ceilingLight != tc.light {
			t.Fatalf("%s view=%+v", tc.name, v)
		}
		physicalFloor, physicalCeil, _ := g.sectorHeightSnapshot(1)
		if physicalFloor != -64*fracUnit || physicalCeil != 160*fracUnit || g.m.Sectors[1].FloorPic != "BWSURF" || g.m.Sectors[1].Light != 160 {
			t.Fatal("render view modified physical sector")
		}
	}
	g.prepareHeightTransfers(-128, 0, -32)
	if v := g.heightView(1, false); v.floor != 0 || v.wallLight != 160 {
		t.Fatal("outside camera incorrectly treated as underwater")
	}
}
func TestBoomHeightTransferBackSideAndOffsets(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	g.worldTic = 3
	g.boomFloorScroll[2] = boomScrollVector{x: fracUnit, y: 2 * fracUnit}
	g.boomCeilingScroll[2] = boomScrollVector{x: 4 * fracUnit, y: 5 * fracUnit}
	g.prepareHeightTransfers(128, 0, -32)
	front, back := g.heightView(1, false), g.heightView(1, true)
	if back.floor != front.floor || back.ceiling != front.ceiling || back.wallLight != 160 || back.sector.FloorPic != "BWSURF" || back.sector.CeilingPic != "BWROOM" {
		t.Fatal("underwater back-side substitution differs from R_FakeFlat")
	}
	floor := g.plane3DKeyForSectorCached(1, g.renderSector(1, false), true)
	ceil := g.plane3DKeyForSectorCached(1, g.renderSector(1, false), false)
	if floor.height != -64 || ceil.height != -1.0/65536 || floor.offsetX != 3*fracUnit || floor.offsetY != 6*fracUnit || ceil.offsetX != 12*fracUnit || ceil.offsetY != 15*fracUnit {
		t.Fatalf("fake plane precision/offsets: floor=%+v ceiling=%+v", floor, ceil)
	}
	g.sectorFloor[2] = 8*fracUnit + fracUnit/2
	g.prepareHeightTransfers(128, 0, 8.5)
	if g.heightView(1, false).ceiling != g.sectorFloor[2]-1 {
		t.Fatal("moving control floor did not update clipping")
	}
}
func TestBoomHeightTransferSkyExceptions(t *testing.T) {
	g := boomWaterGame(t, "MAP02")
	g.prepareHeightTransfers(128, 0, 144)
	v := g.heightView(1, false)
	if v.floor != 128*fracUnit+1 || v.ceiling != 128*fracUnit || v.sector.FloorPic != "BWCEIL" || v.sector.CeilingPic != "BWCEIL" {
		t.Fatalf("above-ceiling sky-floor exception: %+v", v)
	}
	g = boomWaterGame(t, "MAP03")
	g.prepareHeightTransfers(128, 0, -32)
	v = g.heightView(1, false)
	if v.floor != 0 || v.ceiling != -1 || v.sector.CeilingPic != "BWDEEP" {
		t.Fatalf("underwater sky-ceiling exception: %+v", v)
	}
	if !g.forceHeightPlane(1, true) {
		t.Fatal("sky control ceiling must expose the fake floor from below")
	}
	key := g.plane3DKeyForSectorCached(1, g.renderSector(1, false), true)
	if plane3DKeyFromScene(plane3DKeyToScene(key)) != key {
		t.Fatal("fake-plane identity lost during scene grouping")
	}
	if _, _, ok := planeSpanDepth(190, key, -32, 160, 160, 100); !ok {
		t.Fatal("forced fake plane rejected from the opposite side")
	}
}

func TestBoomHeightTransferLivePlaneLights(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	// Underwater walls use the model's own light; its planes can independently
	// transfer from another sector. These references are deliberately single-hop.
	g.boomFloorLight[2], g.boomCeilingLight[2] = 0, 1
	g.boomFloorLight[0] = 1
	g.m.Sectors[0].Light, g.m.Sectors[1].Light, g.m.Sectors[2].Light = 224, 48, 112
	g.refreshSectorPlaneCacheLighting()
	g.prepareHeightTransfers(128, 0, -32)
	v := g.heightView(1, false)
	if v.wallLight != 112 || v.floorLight != 224 || v.ceilingLight != 48 {
		t.Fatalf("model plane lights: %+v", v)
	}
	g.prepareHeightTransfers(128, 0, 41)
	if g.heightView(1, false).wallLight != 48 {
		t.Fatal("underwater light survived the surface transition")
	}
	g.boomHeightFrameValid = false
	if f, _, _ := g.sectorHeightRenderSnapshot(1); f != -64*fracUnit || g.renderSector(1, false) != &g.m.Sectors[1] {
		t.Fatal("fake frame leaked into physical sector queries")
	}
}
func TestBoomHeightTransferRestoreAndRealMovement(t *testing.T) {
	g := boomWaterGame(t, "MAP01")
	g.prepareHeightTransfers(128, 0, -32)
	if !g.tryMove(128*fracUnit, 0) {
		t.Fatal("fake floor blocked physical movement")
	}
	if g.p.floorz != -64*fracUnit {
		t.Fatalf("physical support floor=%d", g.p.floorz)
	}
	g.sectorFloor[2] = 16 * fracUnit
	g.m.Sectors[2].FloorHeight = 16
	raw, err := encodeSnapshot(saveGameMagic, saveFile{Version: saveGameVersion, Game: captureGameSaveState(g)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := decodeSnapshot(raw, saveGameMagic)
	if err != nil {
		t.Fatal(err)
	}
	restored := boomWaterGame(t, "MAP01")
	restoreGameSaveState(restored, file.Game)
	restored.prepareHeightTransfers(128, 0, 8)
	if restored.heightControl(1) != 2 || restored.heightView(1, false).ceiling != 16*fracUnit-1 {
		t.Fatal("height transfer/control state lost on load")
	}
}
