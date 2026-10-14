package sessiontransition

import (
	"gddoom/internal/doomrand"
	"reflect"
	"testing"
)

func TestAdvanceMeltPositionsUsesDoomDelayAccelerationAndCompletion(t *testing.T) {
	y := []int{-2, 0, 15, 16, 190, 199, 200}
	rnd, prnd := doomrand.State()
	if AdvanceMeltPositions(y, 200, len(y)) {
		t.Fatal("melt finished before moving columns")
	}
	if !reflect.DeepEqual(y, []int{-1, 1, 31, 24, 198, 200, 200}) {
		t.Fatalf("first tic positions=%v", y)
	}
	if AdvanceMeltPositions(y, 200, len(y)) {
		t.Fatal("melt finished too early")
	}
	if !reflect.DeepEqual(y, []int{0, 3, 39, 32, 200, 200, 200}) {
		t.Fatalf("second tic positions=%v", y)
	}
	afterRnd, afterPrnd := doomrand.State()
	if afterRnd != rnd || afterPrnd != prnd {
		t.Fatal("moving melt columns consumed Doom RNG")
	}
	tics := 0
	for !AdvanceMeltPositions(y, 200, len(y)) && tics < 100 {
		tics++
	}
	if tics >= 100 {
		t.Fatal("melt never finished")
	}
	for _, position := range y {
		if position != 200 {
			t.Fatal("completed column did not reach bottom")
		}
	}
	if !AdvanceMeltPositions(nil, 200, 160) {
		t.Fatal("invalid columns did not terminate")
	}
}
