package doomruntime

import "testing"

func TestNextWorldThinkerAfter_SeesNewThinkersAddedMidWalk(t *testing.T) {
	g := &game{
		floors: map[int]*floorThinker{
			1: {order: 1, sector: 1},
		},
	}

	first, ok := g.nextWorldThinkerAfter(0)
	if !ok {
		t.Fatal("expected first thinker")
	}
	if first.kind != worldThinkerFloor || first.key != 1 || first.order != 1 {
		t.Fatalf("first=%+v", first)
	}

	g.doors = map[int]*doorThinker{
		2: {order: 2, sector: 2},
	}

	second, ok := g.nextWorldThinkerAfter(first.order)
	if !ok {
		t.Fatal("expected newly added thinker")
	}
	if second.kind != worldThinkerDoor || second.key != 2 || second.order != 2 {
		t.Fatalf("second=%+v", second)
	}
}
