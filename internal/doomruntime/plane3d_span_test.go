package doomruntime

import (
	"math"
	"reflect"
	"testing"

	"gddoom/internal/render/scene"
)

func TestPlane3DFractionalHeightsSurviveSceneConversionAndBucketing(t *testing.T) {
	for _, height := range []float64{-12.75, -1.0 / fracUnit, 1.0 / fracUnit, 8.25, 8.75} {
		key := plane3DKey{height: height, light: 128, flatID: 7, floor: true}
		if got := plane3DKeyFromScene(plane3DKeyToScene(key)); got != key {
			t.Fatalf("fractional plane key lost height: got=%+v want=%+v", got, key)
		}
	}
	first := plane3DKey{height: 8.25, light: 128, flatID: 7, floor: true}
	second := first
	second.height = 8.75
	planes := make(map[plane3DKey][]*plane3DVisplane)
	a, _ := ensurePlane3DForRange(planes, first, 0, 3, 8)
	b, _ := ensurePlane3DForRange(planes, second, 4, 7, 8)
	if a == nil || b == nil || a == b || a.key != first || b.key != second || len(planes) != 2 {
		t.Fatal("different sub-unit floor heights merged into one visplane bucket")
	}
	spans := appendMergedPlane3DSpan(nil, 100, 0, 3, first)
	spans = appendMergedPlane3DSpan(spans, 100, 4, 7, second)
	if len(spans) != 2 {
		t.Fatal("adjacent spans at different fractional heights merged")
	}
}

func TestPlaneSpanDepthUsesFractionalMapHeight(t *testing.T) {
	key := plane3DKey{height: 0.25, floor: true}
	depth, _, ok := planeSpanDepth(199, key, 41, 100, 100, 100)
	want := (41.0 - 0.25) * 100 / 99.5
	if !ok || math.Abs(depth-want) > 1e-12 {
		t.Fatalf("plane projection truncated fractional floor: got %.12f want %.12f", depth, want)
	}
}

func TestAuthorityFractionalPlaneBucketsStayBoundedAndReusePool(t *testing.T) {
	g := &game{
		clientPrediction:  &ClientPrediction{},
		plane3DVisBuckets: make(map[plane3DKey]plane3DVisBucket),
	}
	static := plane3DKey{height: 128, light: 128, flatID: 1}
	var firstMoving, firstStatic *plane3DVisplane
	var staticBucketStorage **plane3DVisplane
	for frame := 0; frame < 2000; frame++ {
		g.beginPlane3DFrame(8)
		moving := plane3DKey{height: float64(frame) / fracUnit, light: 128, flatID: 1, floor: true}
		movingPlane, _ := g.ensurePlane3DForRangeCached(moving, 0, 7, 8)
		staticPlane, _ := g.ensurePlane3DForRangeCached(static, 0, 7, 8)
		if frame == 0 {
			firstMoving, firstStatic = movingPlane, staticPlane
			staticBucketStorage = &g.plane3DVisBuckets[static].list[0]
		}
		if len(g.plane3DVisBuckets) > 3 {
			t.Fatalf("frame %d retained %d historical fractional plane keys", frame, len(g.plane3DVisBuckets))
		}
		if len(g.plane3DPool) != 2 || movingPlane != firstMoving || staticPlane != firstStatic || staticBucketStorage != &g.plane3DVisBuckets[static].list[0] {
			t.Fatal("bounded key retention discarded reusable visplane objects or static bucket storage")
		}
	}
}

func TestSinglePlayerPlaneBucketsRetainExistingReuse(t *testing.T) {
	g := &game{plane3DVisBuckets: make(map[plane3DKey]plane3DVisBucket)}
	for frame := 0; frame < 3; frame++ {
		g.beginPlane3DFrame(8)
		g.ensurePlane3DForRangeCached(plane3DKey{height: float64(frame)}, 0, 7, 8)
	}
	if len(g.plane3DVisBuckets) != 3 {
		t.Fatal("replica key pruning changed single-player bucket retention")
	}
}

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
