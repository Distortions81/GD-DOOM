package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestCollectPlayerStarts(t *testing.T) {
	m := &mapdata.Map{
		Things: []mapdata.Thing{
			{Type: 3, X: 30, Y: 40, Angle: 90},
			{Type: 9, X: 10, Y: 20, Angle: 0},
			{Type: 1, X: 10, Y: 20, Angle: 0},
		},
	}
	starts := collectPlayerStarts(m)
	if len(starts) != 2 {
		t.Fatalf("starts len=%d want=2", len(starts))
	}
	if got, want := starts[0].angle, degToAngle(90); got != want {
		t.Fatalf("slot3 angle=%08x want=%08x", got, want)
	}
	if got, want := starts[1].angle, degToAngle(0); got != want {
		t.Fatalf("slot1 angle=%08x want=%08x", got, want)
	}
}

func TestChooseSpawnStartRequestedSlot(t *testing.T) {
	starts := []playerStart{
		{slot: 1, x: 1},
		{slot: 3, x: 3},
	}
	got, ok := chooseSpawnStart(starts, 3)
	if !ok || got.slot != 3 {
		t.Fatalf("chooseSpawnStart slot=%d ok=%t want slot=3", got.slot, ok)
	}
}

func TestChooseSpawnStartFallbackToSlot1(t *testing.T) {
	starts := []playerStart{
		{slot: 1, x: 1},
		{slot: 2, x: 2},
	}
	got, ok := chooseSpawnStart(starts, 4)
	if !ok || got.slot != 1 {
		t.Fatalf("chooseSpawnStart fallback slot=%d ok=%t want slot=1", got.slot, ok)
	}
}

func TestNonLocalStarts(t *testing.T) {
	starts := []playerStart{
		{slot: 1},
		{slot: 2},
		{slot: 3},
	}
	out := nonLocalStarts(starts, 2)
	if len(out) != 2 {
		t.Fatalf("nonLocal len=%d want=2", len(out))
	}
	for _, s := range out {
		if s.slot == 2 {
			t.Fatal("local slot should be excluded")
		}
	}
}
