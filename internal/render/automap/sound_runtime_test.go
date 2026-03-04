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

func TestSampleForEventPainShootFallbacks(t *testing.T) {
	s := &soundSystem{
		bank: SoundBank{
			SwitchOn: PCMSample{SampleRate: 11025, Data: []byte{1}},
			Oof:      PCMSample{SampleRate: 11025, Data: []byte{2}},
		},
	}
	if got, ok := s.sampleForEvent(soundEventPain); !ok || len(got.Data) == 0 || got.Data[0] != 2 {
		t.Fatalf("pain sample=%v ok=%v want oof fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootPistol); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("pistol sample=%v ok=%v want switch fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootShotgun); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("shotgun sample=%v ok=%v want switch fallback", got, ok)
	}

	s.bank.Pain = PCMSample{SampleRate: 11025, Data: []byte{3}}
	s.bank.ShootPistol = PCMSample{SampleRate: 11025, Data: []byte{4}}
	s.bank.ShootShotgun = PCMSample{SampleRate: 11025, Data: []byte{5}}
	if got, ok := s.sampleForEvent(soundEventPain); !ok || got.Data[0] != 3 {
		t.Fatalf("pain sample=%v ok=%v want explicit pain", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootPistol); !ok || got.Data[0] != 4 {
		t.Fatalf("pistol sample=%v ok=%v want explicit pistol", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootShotgun); !ok || got.Data[0] != 5 {
		t.Fatalf("shotgun sample=%v ok=%v want explicit shotgun", got, ok)
	}
}
