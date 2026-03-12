package doomruntime

import (
	"sync"
	"testing"

	"gddoom/internal/mapdata"
)

func resetLightingMathState(t *testing.T) {
	t.Helper()
	prevFullbright := fullbrightNoLighting
	prevSectorLighting := doomSectorLighting
	t.Cleanup(func() {
		fullbrightNoLighting = prevFullbright
		doomSectorLighting = prevSectorLighting
	})
	fullbrightNoLighting = false
	doomSectorLighting = true
}

func TestSectorLightMulCoversFullRange(t *testing.T) {
	resetLightingMathState(t)
	sectorLightLUTOnce = sync.Once{}
	if got := sectorLightMul(0); got != 0 {
		t.Fatalf("sectorLightMul(0)=%d want=0", got)
	}
	if got := sectorLightMul(255); got != 255 {
		t.Fatalf("sectorLightMul(255)=%d want=255", got)
	}
	if got := sectorLightMul(-10); got != 0 {
		t.Fatalf("sectorLightMul(-10)=%d want=0", got)
	}
	if got := sectorLightMul(300); got != 255 {
		t.Fatalf("sectorLightMul(300)=%d want=255", got)
	}
}

func TestCombineShadeMulIsMultiplicative(t *testing.T) {
	if got := combineShadeMul(256, 256); got != 256 {
		t.Fatalf("combine(256,256)=%d want=256", got)
	}
	if got := combineShadeMul(128, 128); got != 64 {
		t.Fatalf("combine(128,128)=%d want=64", got)
	}
	if got := combineShadeMul(0, 200); got != 0 {
		t.Fatalf("combine(0,200)=%d want=0", got)
	}
}

func TestSectorDistanceShadeMul_DisabledUsesSectorOnly(t *testing.T) {
	resetLightingMathState(t)
	if got := sectorDistanceShadeMul(160, 2000, false); got != 160 {
		t.Fatalf("sectorDistanceShadeMul disabled=%d want=160", got)
	}
}

func TestSectorDistanceShadeMul_EnabledDimsWithDistance(t *testing.T) {
	resetLightingMathState(t)
	near := sectorDistanceShadeMul(160, 64, true)
	far := sectorDistanceShadeMul(160, 2000, true)
	if near <= far {
		t.Fatalf("distance shading expected near > far, got near=%d far=%d", near, far)
	}
	if near <= 0 || near > 160 {
		t.Fatalf("near shade=%d out of expected range", near)
	}
	if far < 0 || far >= near {
		t.Fatalf("far shade=%d out of expected range (near=%d)", far, near)
	}
}

func TestDoomShadeRowsCapsToVanillaRange(t *testing.T) {
	prev := doomColormapRows
	defer func() { doomColormapRows = prev }()
	doomColormapRows = 34
	if got := doomShadeRows(); got != 32 {
		t.Fatalf("doomShadeRows()=%d want=32", got)
	}
}

func TestDoomWallLightBiasMatchesVanillaAxisRules(t *testing.T) {
	verts := []mapdata.Vertex{
		{X: 0, Y: 0},
		{X: 64, Y: 0},
		{X: 0, Y: 64},
		{X: 64, Y: 64},
	}
	if got := doomWallLightBias(&mapdata.Linedef{V1: 0, V2: 1}, verts); got != -1 {
		t.Fatalf("horizontal wall light bias=%d want=-1", got)
	}
	if got := doomWallLightBias(&mapdata.Linedef{V1: 0, V2: 2}, verts); got != 1 {
		t.Fatalf("vertical wall light bias=%d want=1", got)
	}
	if got := doomWallLightBias(&mapdata.Linedef{V1: 0, V2: 3}, verts); got != 0 {
		t.Fatalf("diagonal wall light bias=%d want=0", got)
	}
}

func TestDoomWallLightRowOrdersByDistanceAndBias(t *testing.T) {
	resetLightingMathState(t)
	prev := doomColormapRows
	defer func() { doomColormapRows = prev }()
	doomColormapRows = 32

	near := doomWallLightRow(160, 0, 128, 160)
	far := doomWallLightRow(160, 0, 1024, 160)
	if near >= far {
		t.Fatalf("wall row should darken with distance: near=%d far=%d", near, far)
	}

	horizontal := doomWallLightRow(160, -1, 256, 160)
	vertical := doomWallLightRow(160, 1, 256, 160)
	if horizontal <= vertical {
		t.Fatalf("horizontal should be darker than vertical: horizontal=%d vertical=%d", horizontal, vertical)
	}
}

func TestDoomPlaneLightRowDarkensWithDistance(t *testing.T) {
	resetLightingMathState(t)
	prev := doomColormapRows
	defer func() { doomColormapRows = prev }()
	doomColormapRows = 32

	near := doomPlaneLightRow(160, 128)
	far := doomPlaneLightRow(160, 2048)
	if near >= far {
		t.Fatalf("plane row should darken with distance: near=%d far=%d", near, far)
	}
}

func TestDoomShadeMulFromRowDarkensAsRowIncreases(t *testing.T) {
	prev := doomColormapRows
	prevLUT := doomRowShadeMulLUT
	defer func() {
		doomColormapRows = prev
		doomRowShadeMulLUT = prevLUT
	}()
	doomColormapRows = 32
	doomRowShadeMulLUT = nil

	near := doomShadeMulFromRow(0)
	far := doomShadeMulFromRow(31)
	if near <= far {
		t.Fatalf("expected row 0 to be brighter than row 31: near=%d far=%d", near, far)
	}
	if near != 256 {
		t.Fatalf("row 0 mul=%d want=256", near)
	}
	if far < 0 || far > 255 {
		t.Fatalf("row 31 mul=%d out of range [0,255]", far)
	}
}

func TestDoomShadeMulFromRowFInterpolatesBetweenRows(t *testing.T) {
	prev := doomColormapRows
	prevLUT := doomRowShadeMulLUT
	defer func() {
		doomColormapRows = prev
		doomRowShadeMulLUT = prevLUT
	}()
	doomColormapRows = 32
	doomRowShadeMulLUT = nil

	m0 := doomShadeMulFromRow(10)
	m1 := doomShadeMulFromRow(11)
	mf := doomShadeMulFromRowF(10.5)
	if mf == m0 || mf == m1 {
		t.Fatalf("expected interpolated mul between rows, got m0=%d mf=%d m1=%d", m0, mf, m1)
	}
	if !(m0 > mf && mf > m1) {
		t.Fatalf("expected monotonic interpolation, got m0=%d mf=%d m1=%d", m0, mf, m1)
	}
}
