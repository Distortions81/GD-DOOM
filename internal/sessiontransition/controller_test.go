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

func TestTickClearsActiveStateWhenTransitionCompletes(t *testing.T) {
	c := &Controller{
		kind:        KindLevel,
		initialized: true,
		width:       1,
		height:      1,
		y:           []int{1},
		fromPix:     make([]byte, 4),
		toPix:       []byte{255, 255, 255, 255},
		workPix:     make([]byte, 4),
		to:          ebiten.NewImage(1, 1),
		work:        ebiten.NewImage(1, 1),
	}

	if !c.Active() {
		t.Fatal("transition should start active")
	}
	if !c.Initialized() {
		t.Fatal("transition should start initialized")
	}

	for i := 0; i < 32 && c.Active(); i++ {
		c.Tick(false, 1, 1)
	}

	if c.Active() {
		t.Fatal("transition stayed active after completion")
	}
	if c.Kind() != KindNone {
		t.Fatalf("kind=%v want=%v", c.Kind(), KindNone)
	}
	if c.Initialized() {
		t.Fatal("transition should not stay initialized after completion")
	}
	if c.pending {
		t.Fatal("transition should not stay pending after completion")
	}
	if c.lastFrame == nil {
		t.Fatal("lastFrame should be captured on completion")
	}
}
