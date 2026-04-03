package doomruntime

import (
	"math"
	"reflect"
	"testing"

	"gddoom/internal/media"
	"gddoom/internal/render/scene"
)

func drawWallColumnTexturedIndexedLEColPow2RowRef(pix32 []uint32, pixI, rowStridePix int, col []byte, texVFixed, texVStepFixed int64, hmask, count int, row []uint32) {
	for ; count > 0; count-- {
		ty := int((texVFixed >> fracBits) & int64(hmask))
		pix32[pixI] = row[col[ty]]
		pixI += rowStridePix
		texVFixed += texVStepFixed
	}
}

func TestDrawWallColumnTexturedIndexedLEColPow2RowMatchesReference(t *testing.T) {
	col := make([]byte, 64)
	for i := range col {
		col[i] = byte((i * 37) & 0xFF)
	}
	row := make([]uint32, 256)
	for i := range row {
		row[i] = uint32(i) | 0xABCD0000
	}

	cases := []struct {
		name          string
		texVFixed     int64
		texVStepFixed int64
		count         int
		rowStridePix  int
	}{
		{name: "step zero", texVFixed: 0, texVStepFixed: 0, count: 17, rowStridePix: 1},
		{name: "step one texel", texVFixed: 3 << fracBits, texVStepFixed: 1 << fracBits, count: 21, rowStridePix: 1},
		{name: "step two texels", texVFixed: 5 << fracBits, texVStepFixed: 2 << fracBits, count: 18, rowStridePix: 1},
		{name: "fractional small", texVFixed: 7 << fracBits, texVStepFixed: fracUnit / 3, count: 29, rowStridePix: 1},
		{name: "fractional mixed", texVFixed: (9 << fracBits) + fracUnit/2, texVStepFixed: fracUnit + fracUnit/4, count: 31, rowStridePix: 2},
		{name: "negative step", texVFixed: 20 << fracBits, texVStepFixed: -fracUnit / 2, count: 25, rowStridePix: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.count * tc.rowStridePix
			got := make([]uint32, size)
			want := make([]uint32, size)
			drawWallColumnTexturedIndexedLEColPow2Row(got, 0, tc.rowStridePix, col, tc.texVFixed, tc.texVStepFixed, 63, tc.count, row)
			drawWallColumnTexturedIndexedLEColPow2RowRef(want, 0, tc.rowStridePix, col, tc.texVFixed, tc.texVStepFixed, 63, tc.count, row)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("pix[%d]=%08x want=%08x", i, got[i], want[i])
				}
			}
		})
	}
}

func TestDrawBasicWallColumnTextured_ZeroShadeSkipsTextureRead(t *testing.T) {
	prevColormap := doomColormapEnabled
	t.Cleanup(func() {
		doomColormapEnabled = prevColormap
	})
	doomColormapEnabled = false

	g := &game{
		viewW:     3,
		viewH:     4,
		wallPix32: make([]uint32, 12),
	}
	for i := range g.wallPix32 {
		g.wallPix32[i] = 0xDEADBEEF
	}

	tex := WallTexture{Width: 64, Height: 64}
	g.drawBasicWallColumnTextured(1, 1, 3, 64, 0, 0, 160, wallTextureBlendSample{from: &tex}, 0, 0)

	for y := 0; y < g.viewH; y++ {
		for x := 0; x < g.viewW; x++ {
			got := g.wallPix32[y*g.viewW+x]
			if x == 1 && y >= 1 && y <= 3 {
				if got != pixelOpaqueA {
					t.Fatalf("pix(%d,%d)=%08x want black", x, y, got)
				}
				continue
			}
			if got != 0xDEADBEEF {
				t.Fatalf("pix(%d,%d)=%08x want untouched", x, y, got)
			}
		}
	}
}

func TestDrawBasicWallColumnTextured_RequiresIndexedData(t *testing.T) {
	g := &game{
		viewW:     1,
		viewH:     2,
		wallPix32: []uint32{0x11111111, 0x22222222},
	}
	tex := WallTexture{
		Width:  1,
		Height: 2,
		RGBA: []byte{
			1, 2, 3, 255,
			4, 5, 6, 255,
		},
	}

	g.drawBasicWallColumnTextured(0, 0, 1, 64, 0, 1, 64, wallTextureBlendSample{from: &tex}, 256, 0)

	want := []uint32{0x11111111, 0x22222222}
	if !reflect.DeepEqual(g.wallPix32, want) {
		t.Fatalf("pix=%#v want=%#v", g.wallPix32, want)
	}
}

func installTestWallShadeRow(t *testing.T) []uint32 {
	t.Helper()
	prevOK := wallShadePackedOK
	prevRow := wallShadePackedLUT[256]
	t.Cleanup(func() {
		wallShadePackedOK = prevOK
		wallShadePackedLUT[256] = prevRow
	})
	wallShadePackedOK = true
	for i := range wallShadePackedLUT[256] {
		wallShadePackedLUT[256][i] = 0xA5000000 | uint32(i)
	}
	return wallShadePackedLUT[256][:]
}

func TestDrawBasicWallColumnTexturedMasked_UsesIndexedShadeRow(t *testing.T) {
	row := installTestWallShadeRow(t)
	g := &game{
		viewW:     1,
		viewH:     4,
		wallPix32: make([]uint32, 4),
	}
	tex := WallTexture{
		Width:           1,
		Height:          4,
		Indexed:         []byte{1, 2, 3, 4},
		IndexedColMajor: []byte{1, 2, 3, 4},
		OpaqueMask:      []byte{1, 1, 0, 1},
	}

	g.drawBasicWallColumnTexturedMasked(0, 0, 3, 64, 0, 2, 64, wallTextureBlendSample{from: &tex}, 256, 0)

	want := []uint32{row[1], row[2], 0, row[4]}
	if !reflect.DeepEqual(g.wallPix32, want) {
		t.Fatalf("pix=%#v want=%#v", g.wallPix32, want)
	}
}

func TestTrimMaskedColumnToOpaqueBounds_SingleCycle(t *testing.T) {
	y0, y1, texVFixed, ok := trimMaskedColumnToOpaqueBounds(
		0, 7,
		0,
		fracUnit/2,
		4,
		1, 2,
	)
	if !ok {
		t.Fatal("expected opaque bounds trim to succeed")
	}
	if y0 != 2 || y1 != 5 {
		t.Fatalf("trimmed y=(%d,%d) want (2,5)", y0, y1)
	}
	if texVFixed != fracUnit {
		t.Fatalf("texVFixed=%d want=%d", texVFixed, fracUnit)
	}
}

func TestEnsureOpaqueColumnBounds_ComputesPerColumnExtents(t *testing.T) {
	tex := WallTexture{
		Width:  3,
		Height: 4,
		RGBA: []byte{
			0, 0, 0, 0, 1, 1, 1, 255, 0, 0, 0, 0,
			2, 2, 2, 255, 3, 3, 3, 255, 0, 0, 0, 0,
			4, 4, 4, 255, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 5, 5, 5, 255, 0, 0, 0, 0,
		},
	}
	if !tex.EnsureOpaqueColumnBounds() {
		t.Fatal("expected opaque column bounds")
	}
	if got, want := tex.OpaqueColumnTop, []int16{1, 0, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("top=%v want=%v", got, want)
	}
	if got, want := tex.OpaqueColumnBot, []int16{2, 3, -1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bot=%v want=%v", got, want)
	}
	if got, want := tex.OpaqueRunOffs, []uint32{0, 1, 3, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("offs=%v want=%v", got, want)
	}
	if got, want := tex.OpaqueRuns, []uint32{media.PackOpaqueRun(1, 2), media.PackOpaqueRun(0, 1), media.PackOpaqueRun(3, 3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("runs=%v want=%v", got, want)
	}
	if got, want := tex.OpaqueRowOffs, []uint32{0, 1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("row offs=%v want=%v", got, want)
	}
	if got, want := tex.OpaqueRowRuns, []uint32{media.PackOpaqueRun(1, 1), media.PackOpaqueRun(0, 1), media.PackOpaqueRun(0, 0), media.PackOpaqueRun(1, 1)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("row runs=%v want=%v", got, want)
	}
}

func TestEnsureOpaqueMask_BuildsOneBitCoverage(t *testing.T) {
	tex := WallTexture{
		Width:  2,
		Height: 2,
		RGBA: []byte{
			1, 2, 3, 0, 4, 5, 6, 255,
			7, 8, 9, 255, 10, 11, 12, 0,
		},
	}
	if !tex.EnsureOpaqueMask() {
		t.Fatal("expected opaque mask to build")
	}
	if got, want := tex.OpaqueMask, []byte{0, 1, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mask=%v want=%v", got, want)
	}
}

func TestDrawMaskedColumnOpaqueRuns_SkipsTransparentHole(t *testing.T) {
	g := &game{
		viewW:     1,
		viewH:     8,
		wallPix32: make([]uint32, 8),
	}
	row := installTestWallShadeRow(t)
	tex := WallTexture{
		Width:           1,
		Height:          4,
		Indexed:         []byte{10, 0, 20, 30},
		IndexedColMajor: []byte{10, 0, 20, 30},
		OpaqueMask:      []byte{1, 0, 1, 1},
	}
	if !tex.EnsureOpaqueColumnBounds() {
		t.Fatal("expected opaque run metadata")
	}
	if !drawMaskedColumnOpaqueRuns(g, 0, 0, 7, 0, fracUnit/2, &tex, 0, nil, encodeDepthQ(64), 256, 0, []solidSpan{{L: 0, R: 7}}) {
		t.Fatal("expected run fast-path")
	}
	if g.wallPix32[2] != 0 || g.wallPix32[3] != 0 {
		t.Fatalf("transparent hole was drawn: %v", g.wallPix32)
	}
	if g.wallPix32[0] != row[10] || g.wallPix32[1] != row[10] || g.wallPix32[4] != row[20] {
		t.Fatalf("opaque pixels missing: %v", g.wallPix32)
	}
}

func TestDrawMaskedColumnOpaqueRuns_UsesIndexedShadeRow(t *testing.T) {
	row := installTestWallShadeRow(t)
	g := &game{
		viewW:     1,
		viewH:     4,
		wallPix32: make([]uint32, 4),
	}
	tex := WallTexture{
		Width:           1,
		Height:          4,
		Indexed:         []byte{1, 2, 3, 4},
		IndexedColMajor: []byte{1, 2, 3, 4},
		OpaqueRunOffs:   []uint32{0, 2},
		OpaqueRuns:      []uint32{media.PackOpaqueRun(0, 0), media.PackOpaqueRun(2, 3)},
	}

	if !drawMaskedColumnOpaqueRuns(g, 0, 0, 3, 0, fracUnit, &tex, 0, nil, encodeDepthQ(64), 256, 0, []solidSpan{{L: 0, R: 3}}) {
		t.Fatal("expected run fast-path")
	}

	want := []uint32{row[1], 0, row[3], row[4]}
	if !reflect.DeepEqual(g.wallPix32, want) {
		t.Fatalf("pix=%#v want=%#v", g.wallPix32, want)
	}
}

func TestDrawBasicWallColumnTexturedMasked_FallbackUsesVisibleSpans(t *testing.T) {
	row := installTestWallShadeRow(t)
	g := &game{
		viewW:              1,
		viewH:              6,
		wallPix32:          make([]uint32, 6),
		wallDepthQCol:      []uint16{10},
		wallDepthTopCol:    []int{2},
		wallDepthBottomCol: []int{3},
		wallDepthClosedCol: []bool{false},
		maskedClipCols:     make([][]scene.MaskedClipSpan, 1),
	}
	tex := WallTexture{
		Width:           1,
		Height:          6,
		Indexed:         []byte{1, 2, 3, 4, 5, 6},
		IndexedColMajor: []byte{1, 2, 3, 4, 5, 6},
		OpaqueMask:      []byte{1, 1, 1, 1, 1, 1},
	}

	g.drawBasicWallColumnTexturedMasked(0, 0, 5, 64, 0, 3, 64, wallTextureBlendSample{from: &tex}, 256, 0)

	want := []uint32{row[1], row[2], 0, 0, row[5], row[6]}
	if !reflect.DeepEqual(g.wallPix32, want) {
		t.Fatalf("pix=%#v want=%#v", g.wallPix32, want)
	}
}

func TestDrawBasicWallColumnTexturedMasked_FastRejectsCoveredVisibleSpans(t *testing.T) {
	g := &game{
		viewW:              1,
		viewH:              6,
		wallPix32:          make([]uint32, 6),
		wallDepthQCol:      []uint16{10},
		wallDepthTopCol:    []int{2},
		wallDepthBottomCol: []int{3},
		wallDepthClosedCol: []bool{false},
		maskedClipCols:     make([][]scene.MaskedClipSpan, 1),
		cutoutCoverageBits: make([]uint64, 1),
	}
	for i := range g.wallPix32 {
		g.wallPix32[i] = 0xDEADBEEF
	}
	g.markCutoutCoveredAtIndex(0)
	g.markCutoutCoveredAtIndex(1)
	g.markCutoutCoveredAtIndex(4)
	g.markCutoutCoveredAtIndex(5)
	tex := WallTexture{
		Width:           1,
		Height:          6,
		Indexed:         []byte{1, 2, 3, 4, 5, 6},
		IndexedColMajor: []byte{1, 2, 3, 4, 5, 6},
		OpaqueMask:      []byte{1, 1, 1, 1, 1, 1},
	}

	g.drawBasicWallColumnTexturedMasked(0, 0, 5, 64, 0, 3, 64, wallTextureBlendSample{from: &tex}, 256, 0)

	for i, got := range g.wallPix32 {
		if got != 0xDEADBEEF {
			t.Fatalf("pix[%d]=%08x want untouched", i, got)
		}
	}
}

func TestDrawBillboardRowSpans_OpaqueRunsSkipTransparentGap(t *testing.T) {
	g := &game{wallPix32: make([]uint32, 6)}
	tex := WallTexture{
		Width:  4,
		Height: 1,
		RGBA: []byte{
			10, 0, 0, 255,
			20, 0, 0, 0,
			30, 0, 0, 255,
			40, 0, 0, 255,
		},
	}
	if !tex.EnsureOpaqueColumnBounds() {
		t.Fatal("expected opaque row runs")
	}
	src32, ok := spritePixels32(&tex)
	if !ok {
		t.Fatal("expected rgba32 pixels")
	}
	txLUT := []int{0, 0, 1, 1, 2, 3}
	txRunEndLUT := g.buildSpriteTXRunEnds(txLUT)
	g.drawBillboardRowSpans(0, 0, tex.Width, 0, txLUT, txRunEndLUT, []solidSpan{{L: 0, R: 5}}, &tex, src32, nil, 256, nil, -1)
	if g.wallPix32[2] != 0 || g.wallPix32[3] != 0 {
		t.Fatalf("transparent gap drawn: %v", g.wallPix32)
	}
	if g.wallPix32[0] == 0 || g.wallPix32[1] == 0 || g.wallPix32[4] == 0 || g.wallPix32[5] == 0 {
		t.Fatalf("opaque billboard texels missing: %v", g.wallPix32)
	}
}

func TestDrawBasicWallColumn_InvulnerabilityUsesInverseColormap(t *testing.T) {
	prevColormap := doomColormapEnabled
	prevLighting := doomLightingEnabled
	prevRows := doomColormapRows
	prevRGBA := doomColormapRGBA
	t.Cleanup(func() {
		doomColormapEnabled = prevColormap
		doomLightingEnabled = prevLighting
		doomColormapRows = prevRows
		doomColormapRGBA = prevRGBA
	})

	doomColormapEnabled = true
	doomLightingEnabled = true
	doomColormapRows = doomNumColorMaps + 1
	doomColormapRGBA = make([]uint32, doomColormapRows*256)
	doomColormapRGBA[1] = 0x11111111
	doomColormapRGBA[doomNumColorMaps*256+1] = 0xDEADBEEF

	g := &game{
		viewW:     1,
		viewH:     1,
		wallPix32: make([]uint32, 1),
		inventory: playerInventory{InvulnTics: 5 * 32},
	}
	tex := WallTexture{
		Width:           1,
		Height:          1,
		Indexed:         []byte{1},
		IndexedColMajor: []byte{1},
	}

	wallTop := []int{1}
	wallBottom := []int{-1}
	g.drawBasicWallColumn(wallTop, wallBottom, 0, 0, 0, 64, 160, 0, 0, 0, 64, wallTextureBlendSample{from: &tex})

	if got := g.wallPix32[0]; got != 0xDEADBEEF {
		t.Fatalf("pix=%08x want inverse colormap row value", got)
	}
}

func TestDrawBasicWallColumn_InvulnerabilityUsesInverseColormapInSourcePortMode(t *testing.T) {
	prevColormap := doomColormapEnabled
	prevLighting := doomLightingEnabled
	prevRows := doomColormapRows
	prevRGBA := doomColormapRGBA
	t.Cleanup(func() {
		doomColormapEnabled = prevColormap
		doomLightingEnabled = prevLighting
		doomColormapRows = prevRows
		doomColormapRGBA = prevRGBA
	})

	doomColormapEnabled = false
	doomLightingEnabled = true
	doomColormapRows = doomNumColorMaps + 1
	doomColormapRGBA = make([]uint32, doomColormapRows*256)
	doomColormapRGBA[doomNumColorMaps*256+1] = 0xFEEDBEEF

	g := &game{
		opts:      Options{SourcePortMode: true},
		viewW:     1,
		viewH:     1,
		wallPix32: make([]uint32, 1),
		inventory: playerInventory{InvulnTics: 5 * 32},
	}
	tex := WallTexture{
		Width:           1,
		Height:          1,
		Indexed:         []byte{1},
		IndexedColMajor: []byte{1},
	}

	wallTop := []int{1}
	wallBottom := []int{-1}
	g.drawBasicWallColumn(wallTop, wallBottom, 0, 0, 0, 64, 160, 0, 0, 0, 64, wallTextureBlendSample{from: &tex})

	if got := g.wallPix32[0]; got != 0xFEEDBEEF {
		t.Fatalf("pix=%08x want inverse colormap row value in sourceport mode", got)
	}
}

func TestDrawBillboardRowSpans_InvulnerabilityUsesFixedDOOMRow(t *testing.T) {
	prevRows := doomColormapRows
	prevRGBA := doomColormapRGBA
	t.Cleanup(func() {
		doomColormapRows = prevRows
		doomColormapRGBA = prevRGBA
	})

	doomColormapRows = doomNumColorMaps + 1
	doomColormapRGBA = make([]uint32, doomColormapRows*256)
	doomColormapRGBA[doomNumColorMaps*256+7] = 0xFACEB00C

	g := &game{wallPix32: make([]uint32, 1)}
	tex := WallTexture{
		Width:      1,
		Height:     1,
		Indexed:    []byte{7},
		OpaqueMask: []byte{1},
	}

	txLUT := []int{0}
	txRunEndLUT := g.buildSpriteTXRunEnds(txLUT)
	g.drawBillboardRowSpans(0, 0, tex.Width, 0, txLUT, txRunEndLUT, []solidSpan{{L: 0, R: 0}}, &tex, nil, tex.Indexed, 32, nil, doomNumColorMaps)

	if got := g.wallPix32[0]; got != 0xFACEB00C {
		t.Fatalf("pix=%08x want fixed colormap row value", got)
	}
}

func TestMaskedTextureColumnHasOpaque_UsesPrecomputedRuns(t *testing.T) {
	tex := WallTexture{
		Width:  3,
		Height: 4,
		RGBA: []byte{
			0, 0, 0, 0, 1, 1, 1, 255, 0, 0, 0, 0,
			2, 2, 2, 255, 3, 3, 3, 255, 0, 0, 0, 0,
			4, 4, 4, 255, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 5, 5, 5, 255, 0, 0, 0, 0,
		},
	}
	if !tex.EnsureOpaqueColumnBounds() {
		t.Fatal("expected opaque run metadata")
	}
}

func TestBuildTexturePointerCache_PrecomputesOpaqueRuns(t *testing.T) {
	store, ptrs := buildTexturePointerCache(map[string]WallTexture{
		"T": {
			Width:  1,
			Height: 2,
			RGBA: []byte{
				1, 1, 1, 255,
				0, 0, 0, 0,
			},
		},
	})
	if len(store) != 1 || len(ptrs) != 1 {
		t.Fatalf("store=%d ptrs=%d want 1,1", len(store), len(ptrs))
	}
	tex := ptrs["T"]
	if tex == nil {
		t.Fatal("expected cached texture pointer")
	}
	if len(tex.OpaqueRunOffs) != tex.Width+1 {
		t.Fatalf("opaque offs len=%d want %d", len(tex.OpaqueRunOffs), tex.Width+1)
	}
	if len(tex.OpaqueRuns) == 0 {
		t.Fatal("expected precomputed opaque runs")
	}
}

func TestMaskedColumnVisibleSpans_ClipsWallAndMaskedOccluders(t *testing.T) {
	g := &game{
		viewW:              1,
		viewH:              10,
		wallDepthQCol:      []uint16{10},
		wallDepthTopCol:    []int{2},
		wallDepthBottomCol: []int{4},
		wallDepthClosedCol: []bool{false},
		maskedClipCols: [][]scene.MaskedClipSpan{{
			{Y0: 7, Y1: 8, DepthQ: 20},
		}},
	}
	got := g.maskedColumnVisibleSpans(0, 0, 9, 30)
	want := []solidSpan{{L: 0, R: 1}, {L: 5, R: 6}, {L: 9, R: 9}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans=%v want=%v", got, want)
	}
}

func TestMaskedColumnVisibleSpans_RespectsPortalGap(t *testing.T) {
	g := &game{
		viewW: 1,
		viewH: 10,
		maskedClipCols: [][]scene.MaskedClipSpan{{
			{OpenY0: 3, OpenY1: 5, DepthQ: 10, HasOpen: true},
		}},
	}
	got := g.maskedColumnVisibleSpans(0, 0, 9, 20)
	want := []solidSpan{{L: 3, R: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans=%v want=%v", got, want)
	}
}

func TestAppendMaskedClipSpan_MaintainsSortedOrder(t *testing.T) {
	g := &game{
		viewW:                2,
		viewH:                10,
		maskedClipCols:       make([][]scene.MaskedClipSpan, 2),
		maskedClipLastDepthQ: make([]uint16, 2),
	}

	g.appendSpriteClipColumnSpan(0, 1, 2, 10)
	g.appendSpriteClipColumnSpan(0, 3, 4, 20)
	g.appendSpriteClipColumnSpan(1, 1, 2, 20)
	g.appendSpriteClipColumnSpan(1, 3, 4, 10)
	if got := g.maskedClipCols[0][0].DepthQ; got != 10 {
		t.Fatalf("ordered column changed unexpectedly: first depth=%d", got)
	}
	if got := g.maskedClipCols[0][1].DepthQ; got != 20 {
		t.Fatalf("ordered column changed unexpectedly: second depth=%d", got)
	}
	if got := g.maskedClipCols[1][0].DepthQ; got != 10 {
		t.Fatalf("dirty column first depth=%d want 10", got)
	}
	if got := g.maskedClipCols[1][1].DepthQ; got != 20 {
		t.Fatalf("dirty column second depth=%d want 20", got)
	}

	g.finalizeMaskedClipColumns()

	if got := g.maskedClipCols[1][0].DepthQ; got != 10 {
		t.Fatalf("finalize changed first depth=%d want 10", got)
	}
	if got := g.maskedClipCols[1][1].DepthQ; got != 20 {
		t.Fatalf("finalize changed second depth=%d want 20", got)
	}
}

func TestAppendMaskedMidSegsToBillboardQueue_QuantizesSortDist(t *testing.T) {
	g := &game{
		maskedMidSegsScratch: []maskedMidSeg{
			{
				MaskedMidSeg: scene.MaskedMidSeg{
					Projection: scene.WallProjection{SX1: 0, SX2: 1, InvDepth1: 1.0 / 9.9, InvDepth2: 1.0 / 9.9},
					Dist:       9.9,
					X0:         0,
					X1:         1,
				},
				tex: wallTextureBlendSample{from: &WallTexture{Width: 1, Height: 1}},
			},
			{
				MaskedMidSeg: scene.MaskedMidSeg{
					Projection: scene.WallProjection{SX1: 0, SX2: 1, InvDepth1: 1.0 / 14.1, InvDepth2: 1.0 / 14.1},
					Dist:       14.1,
					X0:         0,
					X1:         1,
				},
				tex: wallTextureBlendSample{from: &WallTexture{Width: 1, Height: 1}},
			},
		},
	}

	g.appendMaskedMidSegsToBillboardQueue()

	if len(g.billboardQueueScratch) != 2 {
		t.Fatalf("queue len=%d want 2", len(g.billboardQueueScratch))
	}
	if got := g.billboardQueueScratch[0].dist; math.Abs(got-8) > 1e-9 {
		t.Fatalf("first dist=%f want 8", got)
	}
	if got := g.billboardQueueScratch[1].dist; math.Abs(got-16) > 1e-9 {
		t.Fatalf("second dist=%f want 16", got)
	}
}

func TestSortCutoutItemsFrontToBack_UsesDepthQAfterQuantizedDist(t *testing.T) {
	g := &game{
		maskedMidSegsScratch: []maskedMidSeg{
			{
				MaskedMidSeg: scene.MaskedMidSeg{
					Projection: scene.WallProjection{SX1: 10, SX2: 20, InvDepth1: 1.0 / 11.9, InvDepth2: 1.0 / 11.9},
					Dist:       11.9,
					X0:         10,
					X1:         20,
				},
				tex: wallTextureBlendSample{from: &WallTexture{Width: 1, Height: 1}},
			},
			{
				MaskedMidSeg: scene.MaskedMidSeg{
					Projection: scene.WallProjection{SX1: 10, SX2: 20, InvDepth1: 1.0 / 9.9, InvDepth2: 1.0 / 9.9},
					Dist:       9.9,
					X0:         10,
					X1:         20,
				},
				tex: wallTextureBlendSample{from: &WallTexture{Width: 1, Height: 1}},
			},
		},
	}

	g.appendMaskedMidSegsToCutoutItems()
	if len(g.billboardQueueScratch) != 2 {
		t.Fatalf("queue len=%d want 2", len(g.billboardQueueScratch))
	}
	if g.billboardQueueScratch[0].dist != g.billboardQueueScratch[1].dist {
		t.Fatalf("expected quantized tie, got %f and %f", g.billboardQueueScratch[0].dist, g.billboardQueueScratch[1].dist)
	}

	g.sortCutoutItemsFrontToBack()

	if got := g.billboardQueueScratch[0].idx; got != 1 {
		t.Fatalf("first idx=%d want nearer masked mid first", got)
	}
	if got := g.billboardQueueScratch[1].idx; got != 0 {
		t.Fatalf("second idx=%d want farther masked mid second", got)
	}
}

func TestAppendMaskedMidSegsToCutoutItems_SplitsAtTextureColumns(t *testing.T) {
	g := &game{
		maskedMidSegsScratch: []maskedMidSeg{
			{
				MaskedMidSeg: scene.MaskedMidSeg{
					Projection: scene.WallProjection{
						SX1:         0,
						SX2:         7,
						InvDepth1:   1.0 / 12.0,
						InvDepth2:   1.0 / 12.0,
						UOverDepth1: 0,
						UOverDepth2: 8.0 / 12.0,
					},
					X0: 0,
					X1: 7,
				},
				tex: wallTextureBlendSample{from: &WallTexture{Width: 8, Height: 1}},
			},
		},
	}

	g.appendMaskedMidSegsToCutoutItems()

	if len(g.billboardQueueScratch) != 2 {
		t.Fatalf("queue len=%d want 2", len(g.billboardQueueScratch))
	}
	for i, want := range []struct{ x0, x1 int }{{0, 3}, {4, 7}} {
		got := g.billboardQueueScratch[i]
		if got.x0 != want.x0 || got.x1 != want.x1 {
			t.Fatalf("item %d range=(%d,%d) want (%d,%d)", i, got.x0, got.x1, want.x0, want.x1)
		}
	}
}

func TestSortCutoutItemsFrontToBack_UsesScreenBoundsBeforeKind(t *testing.T) {
	g := &game{
		billboardQueueScratch: []cutoutItem{
			{dist: 16, depthQ: encodeDepthQ(16), kind: billboardQueueWorldThings, idx: 9, x0: 12, x1: 20, y0: 30, y1: 40},
			{dist: 16, depthQ: encodeDepthQ(16), kind: billboardQueueMonsters, idx: 3, x0: 4, x1: 10, y0: 30, y1: 40},
		},
	}

	g.sortCutoutItemsFrontToBack()

	if got := g.billboardQueueScratch[0].x0; got != 4 {
		t.Fatalf("first x0=%d want leftmost item first", got)
	}
	if got := g.billboardQueueScratch[1].x0; got != 12 {
		t.Fatalf("second x0=%d want right item second", got)
	}
}

func TestMaskedMidBillboardDepthGuess_UsesFartherEdge(t *testing.T) {
	proj, status := scene.ProjectWallSegment(20, -2, 0, 80, 2, 1, 320, 160)
	if status != scene.WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	got, ok := maskedMidBillboardDepthGuess(proj, proj.MinX, proj.MaxX)
	if !ok {
		t.Fatal("expected billboard depth guess")
	}
	if math.Abs(got-80) > 1 {
		t.Fatalf("depth=%f want near farther edge depth around 80", got)
	}
}

func TestMaskedMidDepthSamples_ReturnsCenterAndSortDepth(t *testing.T) {
	proj, status := scene.ProjectWallSegment(20, -2, 0, 80, 2, 1, 320, 160)
	if status != scene.WallProjectionOK {
		t.Fatalf("status=%v want ok", status)
	}
	center, sortDepth, ok := maskedMidDepthSamples(proj, proj.MinX, proj.MaxX)
	if !ok {
		t.Fatal("expected masked mid depth samples")
	}
	midX := (proj.MinX + proj.MaxX) >> 1
	wantCenter, _, ok := scene.ProjectedWallSampleAtX(proj, midX)
	if !ok {
		t.Fatal("expected center sample")
	}
	if math.Abs(center-wantCenter) > 1e-9 {
		t.Fatalf("center=%f want %f", center, wantCenter)
	}
	if math.Abs(sortDepth-80) > 1 {
		t.Fatalf("sortDepth=%f want near farther edge depth around 80", sortDepth)
	}
}

func TestMaskedMidSegFullyOccluded_UsesCachedOcclusionBBox(t *testing.T) {
	g := &game{
		viewW:              4,
		viewH:              10,
		wallDepthQCol:      []uint16{0, 10, 10, 0},
		wallDepthTopCol:    []int{0, 2, 2, 0},
		wallDepthBottomCol: []int{0, 7, 7, 0},
		wallDepthClosedCol: []bool{false, false, false, false},
		maskedClipCols:     make([][]scene.MaskedClipSpan, 4),
	}
	ms := maskedMidSeg{
		MaskedMidSeg: scene.MaskedMidSeg{
			X0: 1,
			X1: 2,
		},
		occlusionY0:      2,
		occlusionY1:      7,
		occlusionDepthQ:  20,
		hasOcclusionBBox: true,
	}
	if !g.maskedMidSegFullyOccluded(ms, 0, 0) {
		t.Fatal("expected cached occlusion bbox to report fully occluded")
	}
}
