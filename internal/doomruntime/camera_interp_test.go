package doomruntime

import (
	"math"
	"testing"
)

func TestInterpolateCameraAngle_ConstantTurnMatchesLinear(t *testing.T) {
	prevPrev := uint32(100)
	prev := uint32(200)
	curr := uint32(300)

	for _, alpha := range []float64{0.25, 0.5, 0.75} {
		got := interpolateCameraAngle(prevPrev, prev, curr, alpha)
		want := lerpAngle(prev, curr, alpha)
		if got != want {
			t.Fatalf("alpha=%.2f got=%d want=%d", alpha, got, want)
		}
	}
}

func TestInterpolateCameraAngle_SmoothsAcceleration(t *testing.T) {
	prevPrev := uint32(0)
	prev := uint32(10)
	curr := uint32(30)

	got := interpolateCameraAngle(prevPrev, prev, curr, 0.5)
	linear := lerpAngle(prev, curr, 0.5)
	if got >= linear {
		t.Fatalf("midpoint=%d want less than linear=%d during acceleration", got, linear)
	}
	if d := shortestAngleDelta(prev, got); d <= 0 || d >= shortestAngleDelta(prev, curr) {
		t.Fatalf("midpoint delta=%v want strictly between 0 and target delta", d)
	}
}

func TestInterpolateCameraAngle_SmoothsTurnReversal(t *testing.T) {
	prevPrev := uint32(0)
	prev := uint32(10)
	curr := uint32(0)

	got := interpolateCameraAngle(prevPrev, prev, curr, 0.5)
	want := lerpAngle(prev, curr, 0.5)
	if got <= curr || got >= prev {
		t.Fatalf("got=%d want strictly between curr=%d and prev=%d", got, curr, prev)
	}
	if got <= want {
		t.Fatalf("got=%d want greater than linear midpoint=%d for smoother reversal", got, want)
	}
}

func TestInterpolateCameraAngle_UsesShortestWrappedArc(t *testing.T) {
	prevPrev := ^uint32(0) - 20
	prev := ^uint32(0) - 10
	curr := uint32(5)

	got := interpolateCameraAngle(prevPrev, prev, curr, 0.5)
	total := shortestAngleDelta(prev, curr)
	mid := shortestAngleDelta(prev, got)
	if total <= 0 {
		t.Fatalf("total delta=%v want positive wrapped delta", total)
	}
	if mid <= 0 || mid >= total {
		t.Fatalf("mid delta=%v want strictly between 0 and total=%v", mid, total)
	}
	if math.Abs(shortestAngleDelta(got, curr)) >= math.Abs(shortestAngleDelta(prev, curr)) {
		t.Fatalf("midpoint should move closer to current angle")
	}
}
