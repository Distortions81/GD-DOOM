package automap

import "testing"

func TestDoorMoveEvent(t *testing.T) {
	if got := doorMoveEvent(doorNormal, 1); got != soundEventDoorOpen {
		t.Fatalf("doorNormal open event=%v", got)
	}
	if got := doorMoveEvent(doorNormal, -1); got != soundEventDoorClose {
		t.Fatalf("doorNormal close event=%v", got)
	}
	if got := doorMoveEvent(doorBlazeRaise, 1); got != soundEventBlazeOpen {
		t.Fatalf("doorBlazeRaise open event=%v", got)
	}
	if got := doorMoveEvent(doorBlazeRaise, -1); got != soundEventBlazeClose {
		t.Fatalf("doorBlazeRaise close event=%v", got)
	}
}

func TestDelayedSoundEventQueue(t *testing.T) {
	g := &game{
		soundQueue: make([]soundEvent, 0, 4),
		delayedSfx: make([]delayedSoundEvent, 0, 4),
	}
	g.emitSoundEventDelayed(soundEventSwitchOff, 2)
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue len=%d want=0", got)
	}
	g.tickDelayedSounds()
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("after 1 tick soundQueue len=%d want=0", got)
	}
	g.tickDelayedSounds()
	if got := len(g.soundQueue); got != 1 {
		t.Fatalf("after 2 ticks soundQueue len=%d want=1", got)
	}
	if got := g.soundQueue[0]; got != soundEventSwitchOff {
		t.Fatalf("event=%v want=%v", got, soundEventSwitchOff)
	}
}
