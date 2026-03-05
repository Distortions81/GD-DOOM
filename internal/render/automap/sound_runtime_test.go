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
	if got, ok := s.sampleForEvent(soundEventShootFireball); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("fireball sample=%v ok=%v want switch fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventImpactRocket); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("impact-rocket sample=%v ok=%v want switch fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterPainHumanoid); !ok || len(got.Data) == 0 || got.Data[0] != 2 {
		t.Fatalf("monster pain humanoid sample=%v ok=%v want oof fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterPainDemon); !ok || len(got.Data) == 0 || got.Data[0] != 2 {
		t.Fatalf("monster pain demon sample=%v ok=%v want oof fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventDeathImp); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("death imp sample=%v ok=%v want switch fallback", got, ok)
	}

	s.bank.Pain = PCMSample{SampleRate: 11025, Data: []byte{3}}
	s.bank.ShootPistol = PCMSample{SampleRate: 11025, Data: []byte{4}}
	s.bank.ShootShotgun = PCMSample{SampleRate: 11025, Data: []byte{5}}
	s.bank.ShootFireball = PCMSample{SampleRate: 11025, Data: []byte{6}}
	s.bank.ShootRocket = PCMSample{SampleRate: 11025, Data: []byte{7}}
	s.bank.ImpactFire = PCMSample{SampleRate: 11025, Data: []byte{8}}
	s.bank.ImpactRocket = PCMSample{SampleRate: 11025, Data: []byte{9}}
	s.bank.MonsterPainHumanoid = PCMSample{SampleRate: 11025, Data: []byte{10}}
	s.bank.MonsterPainDemon = PCMSample{SampleRate: 11025, Data: []byte{11}}
	s.bank.DeathImp = PCMSample{SampleRate: 11025, Data: []byte{12}}
	s.bank.DeathDemon = PCMSample{SampleRate: 11025, Data: []byte{13}}
	s.bank.DeathZombie = PCMSample{SampleRate: 11025, Data: []byte{14}}
	if got, ok := s.sampleForEvent(soundEventPain); !ok || got.Data[0] != 3 {
		t.Fatalf("pain sample=%v ok=%v want explicit pain", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootPistol); !ok || got.Data[0] != 4 {
		t.Fatalf("pistol sample=%v ok=%v want explicit pistol", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootShotgun); !ok || got.Data[0] != 5 {
		t.Fatalf("shotgun sample=%v ok=%v want explicit shotgun", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootFireball); !ok || got.Data[0] != 6 {
		t.Fatalf("fireball sample=%v ok=%v want explicit fireball", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventShootRocket); !ok || got.Data[0] != 7 {
		t.Fatalf("rocket sample=%v ok=%v want explicit rocket", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventImpactFire); !ok || got.Data[0] != 8 {
		t.Fatalf("impact-fire sample=%v ok=%v want explicit impact-fire", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventImpactRocket); !ok || got.Data[0] != 9 {
		t.Fatalf("impact-rocket sample=%v ok=%v want explicit impact-rocket", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterPainHumanoid); !ok || got.Data[0] != 10 {
		t.Fatalf("monster pain humanoid sample=%v ok=%v want explicit humanoid", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterPainDemon); !ok || got.Data[0] != 11 {
		t.Fatalf("monster pain demon sample=%v ok=%v want explicit demon", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventDeathImp); !ok || got.Data[0] != 12 {
		t.Fatalf("death imp sample=%v ok=%v want explicit imp death", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventDeathDemon); !ok || got.Data[0] != 13 {
		t.Fatalf("death demon sample=%v ok=%v want explicit demon death", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventDeathShotgunGuy); !ok || got.Data[0] != 14 {
		t.Fatalf("death shotgun sample=%v ok=%v want zombie fallback", got, ok)
	}
}

func TestPCMMonoU8ToStereoS16LEResampledLength(t *testing.T) {
	src := []byte{0, 64, 128, 255}
	got := pcmMonoU8ToStereoS16LEResampled(src, 11025, 44100)
	// 4x upsample: 4 input frames -> 16 output frames, 4 bytes per frame.
	if len(got) != 16*4 {
		t.Fatalf("len=%d want=%d", len(got), 16*4)
	}
}

func TestPCMMonoU8ToStereoS16LEResampledEmpty(t *testing.T) {
	if got := pcmMonoU8ToStereoS16LEResampled(nil, 11025, 44100); len(got) != 0 {
		t.Fatalf("len=%d want=0", len(got))
	}
	if got := pcmMonoU8ToStereoS16LEResampled([]byte{1}, 0, 44100); len(got) != 0 {
		t.Fatalf("len=%d want=0", len(got))
	}
}
