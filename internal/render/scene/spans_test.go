package scene

import (
	"reflect"
	"testing"
)

func TestSpriteColumnOccludesPoint_ComposesWallAndMasked(t *testing.T) {
	wall := WallDepthColumn{DepthQ: 100, Top: 10, Bottom: 20}
	if !SpriteColumnOccludesPoint(wall, nil, 15, 101) {
		t.Fatal("expected wall depth occluder")
	}

	wall = WallDepthColumn{DepthQ: 0xFFFF, Top: 1, Bottom: 0}
	masked := []MaskedClipSpan{{Y0: 10, Y1: 20, DepthQ: 100}}
	if !SpriteColumnOccludesPoint(wall, masked, 15, 101) {
		t.Fatal("expected masked clip occluder")
	}
}

func TestAppendVisibleRowSpans_SplitsOnOcclusion(t *testing.T) {
	var got [][2]int
	AppendVisibleRowSpans(0, 8, 0, nil, func(x int) bool {
		return x == 2 || x == 5
	}, func(l, r int) {
		got = append(got, [2]int{l, r})
	})
	want := [][2]int{{0, 1}, {3, 4}, {6, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans=%v want %v", got, want)
	}
}

func TestAppendVisibleRowSpans_ClipsInputRanges(t *testing.T) {
	clipSpans := [][2]int{{-2, 2}, {4, 10}}
	var got [][2]int
	AppendVisibleRowSpans(0, 6, len(clipSpans), func(i int) (int, int) {
		return clipSpans[i][0], clipSpans[i][1]
	}, func(x int) bool {
		return x == 1 || x == 5
	}, func(l, r int) {
		got = append(got, [2]int{l, r})
	})
	want := [][2]int{{0, 0}, {2, 2}, {4, 4}, {6, 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans=%v want %v", got, want)
	}
}
