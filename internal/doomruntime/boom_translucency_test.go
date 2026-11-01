package doomruntime

import (
	"bytes"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/wad"
	"path/filepath"
	"testing"
)

func translucencyTestOptions(w *wad.File) Options {
	palette, cm := make([]byte, 1024), make([]byte, 34*256)
	for i := 0; i < 256; i++ {
		palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(i), byte(i), 255
	}
	for i := range cm {
		cm[i] = byte(i)
	}
	return Options{Headless: true, DoomPaletteRGBA: palette, DoomColorMap: cm, DoomColorMapRows: 34, BoomTranMaps: doomtex.LoadBoomTranMaps(w)}
}
func loadTranslucencyTestMap(t *testing.T, number mapdata.MapName) (*wad.File, *mapdata.Map) {
	t.Helper()
	w, err := wad.Open(filepath.Join("..", "..", "testdata", "boom", "translucency.wad"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, number)
	if err != nil {
		t.Fatal(err)
	}
	return w, m
}
func TestBoomTranslucencyTagZeroAndNamedControls(t *testing.T) {
	w, m := loadTranslucencyTestMap(t, "MAP01")
	opts := translucencyTestOptions(w)
	g := newGame(m, opts)
	if len(g.tranMapForLine(4)) != 65536 || g.tranMapForLine(0) != nil {
		t.Fatal("tag-zero translucency leaked onto another line")
	}
	if g.renderSidedef(int(m.Linedefs[4].SideNum[0])).Mid != "BGLASSA" {
		t.Fatal("ordinary middle texture treated as a table")
	}
	w, m = loadTranslucencyTestMap(t, "MAP03")
	opts = translucencyTestOptions(w)
	m.Linedefs[len(m.Linedefs)-1].Tag = 17
	g = newGame(m, opts)
	if !bytes.Equal(g.tranMapForLine(4), opts.BoomTranMaps["REVMAP"]) || g.tranMapForLine(5) != nil {
		t.Fatal("tag transfer/setup order incorrect")
	}
	side := int(m.Linedefs[len(m.Linedefs)-1].SideNum[0])
	if g.renderSidedef(side).Mid != "-" || g.m.Sidedefs[side].Mid != "REVMAP" {
		t.Fatal("custom map was rendered as a texture or changed physical map")
	}
	saved := captureGameSaveState(g)
	restored := newGame(cloneMapForRestart(g.restartTemplate), opts)
	restoreGameSaveState(restored, saved)
	if !bytes.Equal(restored.tranMapForLine(4), g.tranMapForLine(4)) {
		t.Fatal("static map references did not rebuild on restore")
	}
}
func TestBoomTranslucencySharedControlSidedef(t *testing.T) {
	w, m := loadTranslucencyTestMap(t, "MAP03")
	opts := translucencyTestOptions(w)
	m.Linedefs[len(m.Linedefs)-1].SideNum[0] = m.Linedefs[len(m.Linedefs)-2].SideNum[0]
	g := newGame(m, opts)
	if !bytes.Equal(g.tranMapForLine(4), opts.BoomTranMaps["CUSTMAP"]) || !bytes.Equal(g.tranMapForLine(5), opts.BoomTranMaps["CUSTMAP"]) {
		t.Fatal("shared control sidedef lost its custom table")
	}
}

func TestBoomTranslucencyInvalidTableFallsBackToTexture(t *testing.T) {
	w, m := loadTranslucencyTestMap(t, "MAP02")
	opts := translucencyTestOptions(w)
	opts.BoomTranMaps["CUSTMAP"] = []byte{1}
	g := newGame(m, opts)
	side := int(m.Linedefs[len(m.Linedefs)-1].SideNum[0])
	if g.renderSidedef(side).Mid != "CUSTMAP" || !bytes.Equal(g.tranMapForLine(4), doomtex.GenerateBoomTranMap(opts.DoomPaletteRGBA)) {
		t.Fatal("invalid custom table did not retain texture/default filter")
	}
}
func TestBoomTranslucencyLookupOrientationLightingAndLayers(t *testing.T) {
	w, m := loadTranslucencyTestMap(t, "MAP03")
	opts := translucencyTestOptions(w)
	g := newGame(m, opts)
	g.setGammaLevel(0)
	g.wallPix32 = []uint32{packRGBA(40, 40, 40)}
	table := opts.BoomTranMaps["CUSTMAP"]
	g.blendBoomPixel(0, 200, 256, 0, table)
	if got := channelR(g.wallPix32[0]); got != 140 {
		t.Fatalf("destination/source lookup reversed: %d", got)
	}
	g.blendBoomPixel(0, 80, 256, 0, table)
	if got := channelR(g.wallPix32[0]); got != 102 {
		t.Fatalf("near layer ignored composed background: %d", got)
	}
	// Lighting remaps the source index before the TRANMAP, not the result.
	g.opts.DoomColorMap[256+200] = 80
	g.wallPix32[0] = packRGBA(40, 40, 40)
	g.blendBoomPixel(0, 200, 256, 1, table)
	if got := channelR(g.wallPix32[0]); got != 65 {
		t.Fatalf("lighting applied after translucency: %d", got)
	}
}
func TestBoomTranslucencyAbsentMapRetainsFastPath(t *testing.T) {
	g := newGame(&mapdata.Map{Sectors: []mapdata.Sector{{CeilingHeight: 128}}}, Options{Headless: true})
	if g.boomHasTranslucency || g.boomLineTranMap != nil || g.tranMapForLine(0) != nil {
		t.Fatal("ordinary map allocated translucency state")
	}
}
func TestBoomSpriteLightingUsesFixedCameraDepth(t *testing.T) {
	w, m := loadTranslucencyTestMap(t, "MAP01")
	g := newGame(m, translucencyTestOptions(w))
	g.viewW = 320
	g.renderPX, g.renderPY, g.renderAngle = -192, 0, 0
	if row := g.boomSpriteLightRowAt(0, false, -64*fracUnit, 72*fracUnit); row != 11 {
		t.Fatalf("fixed camera light boundary row=%d want 11", row)
	}
}
