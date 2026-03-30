package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/render/scene"
)

func TestAppendMergedPlane3DSpan(t *testing.T) {
	keyA := plane3DKey{height: 0, light: 160, flatID: 1, floor: true}
	keyB := plane3DKey{height: 0, light: 128, flatID: 1, floor: true}

	var spans []plane3DSpan
	spans = appendMergedPlane3DSpan(spans, 10, 4, 7, keyA)
	spans = appendMergedPlane3DSpan(spans, 10, 8, 12, keyA)
	spans = appendMergedPlane3DSpan(spans, 10, 14, 15, keyA)
	spans = appendMergedPlane3DSpan(spans, 11, 0, 3, keyA)
	spans = appendMergedPlane3DSpan(spans, 11, 4, 6, keyB)

	if len(spans) != 4 {
		t.Fatalf("len(spans)=%d want 4", len(spans))
	}
	if spans[0].y != 10 || spans[0].x1 != 4 || spans[0].x2 != 12 || spans[0].key != keyA {
		t.Fatalf("merged span=%+v want y=10 x1=4 x2=12 keyA", spans[0])
	}
	if spans[1].y != 10 || spans[1].x1 != 14 || spans[1].x2 != 15 || spans[1].key != keyA {
		t.Fatalf("separate gap span=%+v want y=10 x1=14 x2=15 keyA", spans[1])
	}
	if spans[2].y != 11 || spans[2].x1 != 0 || spans[2].x2 != 3 || spans[2].key != keyA {
		t.Fatalf("row-changed span=%+v want y=11 x1=0 x2=3 keyA", spans[2])
	}
	if spans[3].y != 11 || spans[3].x1 != 4 || spans[3].x2 != 6 || spans[3].key != keyB {
		t.Fatalf("key-changed span=%+v want y=11 x1=4 x2=6 keyB", spans[3])
	}
}

func TestMakePlane3DSpansWithScratchMatchesScene(t *testing.T) {
	key := plane3DKey{height: 0, light: 160, flatID: 7, floor: true}
	pl := newPlane3DVisplane(key, 1, 6, 8)
	for i := range pl.top {
		pl.top[i] = plane3DUnset
		pl.bottom[i] = plane3DUnset
	}
	cols := []struct {
		x int
		t int16
		b int16
	}{
		{1, 2, 6},
		{2, 2, 6},
		{3, 3, 5},
		{4, 1, 7},
		{5, 1, 7},
		{6, 4, 4},
	}
	for _, c := range cols {
		pl.top[c.x+1] = c.t
		pl.bottom[c.x+1] = c.b
	}

	got := makePlane3DSpansWithScratch(pl, 10, nil, make([]int, 10))
	sp := &scene.PlaneVisplane{
		Key:    plane3DKeyToScene(pl.key),
		MinX:   pl.minX,
		MaxX:   pl.maxX,
		Top:    append([]int16(nil), pl.top...),
		Bottom: append([]int16(nil), pl.bottom...),
	}
	sceneSpans := scene.MakePlaneSpansWithScratch(sp, 10, nil, make([]int, 10))
	want := make([]plane3DSpan, 0, len(sceneSpans))
	for _, s := range sceneSpans {
		want = appendMergedPlane3DSpan(want, s.Y, s.X1, s.X2, key)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestDrawPlaneTexturedSpanAtDepth_ZeroShadeSkipsTextureRead(t *testing.T) {
	prevColormap := doomColormapEnabled
	t.Cleanup(func() {
		doomColormapEnabled = prevColormap
	})
	doomColormapEnabled = false

	pix := []uint32{1, 2, 3, 4, 5, 6}
	g := &game{}
	state := planeRowRenderState{defaultShade: 0}

	g.drawPlaneTexturedSpanAtDepth(pix, 0, 1, 4, plane3DKey{}, flatTextureBlendSample{}, state)

	want := []uint32{1, pixelOpaqueA, pixelOpaqueA, pixelOpaqueA, pixelOpaqueA, 6}
	if !reflect.DeepEqual(pix, want) {
		t.Fatalf("pix=%v want=%v", pix, want)
	}
}

func TestDrawPlaneTexturedSpanAtDepth_RequiresIndexedFlat(t *testing.T) {
	pix := []uint32{1, 2, 3, 4, 5, 6}
	g := &game{}
	state := planeRowRenderState{
		defaultShade:   256,
		rowBaseWXFixed: 0,
		rowBaseWYFixed: 0,
		stepWXFixed:    fracUnit,
	}

	g.drawPlaneTexturedSpanAtDepth(pix, 0, 1, 4, plane3DKey{}, flatTextureBlendSample{}, state)

	want := []uint32{1, 2, 3, 4, 5, 6}
	if !reflect.DeepEqual(pix, want) {
		t.Fatalf("pix=%v want=%v", pix, want)
	}
}
