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
