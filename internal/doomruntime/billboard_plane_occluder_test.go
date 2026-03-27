package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestClipRangeAgainstBillboardPlaneOccludersDepthAware(t *testing.T) {
	occluders := []billboardPlaneOccluderSpan{
		{L: 10, R: 20, DepthQ: 100},
		{L: 15, R: 25, DepthQ: 200},
		{L: 30, R: 35, DepthQ: 50},
	}
	got := clipRangeAgainstBillboardPlaneOccluders(5, 40, 120, occluders, nil)
	want := []solidSpan{
		{L: 5, R: 9},
		{L: 21, R: 29},
		{L: 36, R: 40},
	}
	if len(got) != len(want) {
		t.Fatalf("span count=%d want=%d spans=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("span[%d]=%+v want %+v", i, got[i], want[i])
		}
	}

	got = clipRangeAgainstBillboardPlaneOccluders(5, 40, 250, occluders, nil)
	want = []solidSpan{
		{L: 5, R: 9},
		{L: 26, R: 29},
		{L: 36, R: 40},
	}
	if len(got) != len(want) {
		t.Fatalf("deep span count=%d want=%d spans=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("deep span[%d]=%+v want %+v", i, got[i], want[i])
		}
	}
}

func TestFlipSpriteOpaqueRectX(t *testing.T) {
	rect := packSpriteOpaqueRect(1, 3, 2, 4)
	got := flipSpriteOpaqueRectX(rect, 8)
	want := packSpriteOpaqueRect(4, 6, 2, 4)
	if got != want {
		t.Fatalf("flipped rect=%+v want %+v", got, want)
	}
}

func TestBuildBillboardPlaneOccludersFromQueueUsesOpaqueRects(t *testing.T) {
	g := &game{
		viewW: 16,
		viewH: 12,
		billboardQueueScratch: []billboardQueueItem{
			{
				kind:       billboardQueueMonsters,
				dist:       8,
				depthQ:     encodeDepthQ(8),
				clipTop:    0,
				clipBottom: 11,
				tex:        &WallTexture{Width: 4, Height: 4},
				dstX:       4,
				dstY:       6,
				scale:      1,
				opaque: spriteOpaqueShape{
					rects: []spriteOpaqueRect{packSpriteOpaqueRect(1, 2, 1, 2)},
				},
				hasOpaque: true,
				boundsOK:  true,
			},
		},
	}

	g.buildBillboardPlaneOccludersFromQueue()

	for y := 0; y < g.viewH; y++ {
		row := g.billboardPlaneOccluderRows[y]
		switch y {
		case 7, 8:
			if len(row) != 1 {
				t.Fatalf("row %d span count=%d want=1 spans=%v", y, len(row), row)
			}
			if row[0].L != 5 || row[0].R != 6 {
				t.Fatalf("row %d span=%+v want L=5 R=6", y, row[0])
			}
		default:
			if len(row) != 0 {
				t.Fatalf("row %d should be empty, got %v", y, row)
			}
		}
	}
}

func TestAppendBillboardPlaneOccluderRowMaintainsOrder(t *testing.T) {
	g := &game{
		viewW:                      32,
		billboardPlaneOccluderRows: make([][]billboardPlaneOccluderSpan, 1),
	}
	g.appendBillboardPlaneOccluderRow(0, 12, 20, 100)
	g.appendBillboardPlaneOccluderRow(0, 4, 8, 200)
	g.appendBillboardPlaneOccluderRow(0, 12, 18, 90)
	g.appendBillboardPlaneOccluderRow(0, 12, 18, 110)

	got := g.billboardPlaneOccluderRows[0]
	want := []billboardPlaneOccluderSpan{
		{L: 4, R: 8, DepthQ: 200},
		{L: 12, R: 18, DepthQ: 90},
		{L: 12, R: 18, DepthQ: 110},
		{L: 12, R: 20, DepthQ: 100},
	}
	if len(got) != len(want) {
		t.Fatalf("row len=%d want=%d row=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row[%d]=%+v want %+v", i, got[i], want[i])
		}
	}
}

func TestFillUndrawnAreasWithSkySkipsBillboardOccluderEdges(t *testing.T) {
	g := &game{
		viewW:              6,
		viewH:              1,
		skyOutputW:         6,
		skyOutputH:         1,
		frameSkyFillActive: true,
		frameSkyTex32:      []uint32{packRGBA(17, 34, 51)},
		frameSkyTexW:       1,
		frameSkyColU:       []int{0, 0, 0, 0, 0, 0},
		frameSkyRowV:       []int{0},
		wallPix32:          []uint32{1, 1, 1, 1, 1, 1},
		frameCoverageBits:  make([]uint64, 1),
		billboardPlaneOccluderRows: [][]billboardPlaneOccluderSpan{{
			{L: 2, R: 3, DepthQ: 100},
		}},
		wallTexPtrs: map[string]*WallTexture{
			"SKY1": {
				Width:  1,
				Height: 1,
				RGBA:   []byte{17, 34, 51, 255},
				RGBA32: []uint32{packRGBA(17, 34, 51)},
			},
		},
		m: &mapdata.Map{Name: "E1M1"},
	}
	wallTop := []int{1, 1, 1, 1, 1, 1}
	wallBottom := []int{-1, -1, -1, -1, -1, -1}

	_ = g.fillUndrawnAreasWithSky(wallTop, wallBottom, 0, doomFocalLength(g.viewW))

	sky := packRGBA(17, 34, 51)
	for _, x := range []int{0, 1, 4, 5} {
		if got := g.wallPix32[x]; got != sky {
			t.Fatalf("x=%d sky fill=%#08x want %#08x", x, got, sky)
		}
	}
	for _, x := range []int{2, 3} {
		if got := g.wallPix32[x]; got != 1 {
			t.Fatalf("x=%d occluder pixel overwritten=%#08x want %#08x", x, got, uint32(1))
		}
	}
}
