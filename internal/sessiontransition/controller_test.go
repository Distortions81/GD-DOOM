package sessiontransition

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestReleaseWorkingSetKeepsLastFrame(t *testing.T) {
	c := &Controller{
		kind:        KindLevel,
		pending:     true,
		initialized: true,
		holdTics:    5,
		width:       320,
		height:      200,
		y:           []int{1, 2, 3},
		fromPix:     []byte{1},
		toPix:       []byte{2},
		workPix:     []byte{3},
		from:        ebiten.NewImage(1, 1),
		to:          ebiten.NewImage(1, 1),
		work:        ebiten.NewImage(1, 1),
		lastFrame:   ebiten.NewImage(2, 2),
	}

	last := c.lastFrame
	c.ReleaseWorkingSet()

	if c.lastFrame != last {
		t.Fatal("lastFrame should be preserved")
	}
	if c.kind != KindLevel {
		t.Fatalf("kind=%v want=%v", c.kind, KindLevel)
	}
	if c.pending || c.initialized || c.holdTics != 0 {
		t.Fatalf("state not reset: pending=%v initialized=%v hold=%d", c.pending, c.initialized, c.holdTics)
	}
	if c.from != nil || c.to != nil || c.work != nil {
		t.Fatal("working images should be released")
	}
	if c.fromPix != nil || c.toPix != nil || c.workPix != nil || c.y != nil {
		t.Fatal("working buffers should be released")
	}
}
