package automap

import "testing"

func TestUpdatePlayerTurnAccelerationMatchesDoomStyle(t *testing.T) {
	g := &game{}

	// First held turn uses slow turn speed.
	g.updatePlayer(moveCmd{turn: 1, run: false})
	if got, want := g.p.angle, angleTurn[2]; got != want {
		t.Fatalf("first held turn angle=%d want=%d", got, want)
	}

	// Hold a bit longer; once threshold reached, normal speed applies.
	for i := 0; i < slowTurnTics; i++ {
		g.updatePlayer(moveCmd{turn: 1, run: false})
	}
	afterNormal := g.p.angle
	g.updatePlayer(moveCmd{turn: 1, run: false})
	normalDelta := g.p.angle - afterNormal
	if got, want := normalDelta, angleTurn[0]; got != want {
		t.Fatalf("normal held turn delta=%d want=%d", got, want)
	}

	// With run held and past threshold, fast turn applies.
	afterFastBase := g.p.angle
	g.updatePlayer(moveCmd{turn: 1, run: true})
	fastDelta := g.p.angle - afterFastBase
	if got, want := fastDelta, angleTurn[1]; got != want {
		t.Fatalf("fast held turn delta=%d want=%d", got, want)
	}

	// Releasing turn resets hold counter.
	g.updatePlayer(moveCmd{})
	if g.turnHeld != 0 {
		t.Fatalf("turnHeld=%d want=0", g.turnHeld)
	}
}
