package scene

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestNodeChildBBoxMaybeVisible(t *testing.T) {
	n := mapdata.Node{BBoxR: [4]int16{8, -8, 8, 16}}
	if !NodeChildBBoxMaybeVisible(n, 0, 0, 0, 1, 0, 2, 1) {
		t.Fatal("expected bbox in front of camera to be visible")
	}
	n = mapdata.Node{BBoxR: [4]int16{8, -8, -16, -8}}
	if NodeChildBBoxMaybeVisible(n, 0, 0, 0, 1, 0, 2, 1) {
		t.Fatal("expected bbox fully behind-left frustum to be culled")
	}
}

func TestNodeChildScreenRange(t *testing.T) {
	n := mapdata.Node{BBoxR: [4]int16{8, -8, 8, 16}}
	l, r, ok := NodeChildScreenRange(n, 0, 0, 0, 1, 0, 2, 160, 320)
	if !ok {
		t.Fatal("expected screen range")
	}
	if l < 0 || r >= 320 || l > r {
		t.Fatalf("invalid range %d..%d", l, r)
	}
}

func TestSegScreenRangeFromWorld(t *testing.T) {
	l, r, ok := SegScreenRangeFromWorld(8, 4, 8, -4, 0, 0, 1, 0, 2, 160, 320)
	if !ok {
		t.Fatal("expected visible seg range")
	}
	if l < 0 || r >= 320 || l > r {
		t.Fatalf("invalid range %d..%d", l, r)
	}
}
