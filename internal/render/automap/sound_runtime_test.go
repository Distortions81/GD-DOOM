package automap

import (
	"testing"

	"gddoom/internal/doomrand"
)

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

func TestClearPendingSoundStateClearsQueues(t *testing.T) {
	g := &game{
		soundQueue: []soundEvent{soundEventDoorOpen, soundEventSwitchOn},
		delayedSfx: []delayedSoundEvent{{ev: soundEventSwitchOff, tics: 3}},
	}
	g.clearPendingSoundState()
	if got := len(g.soundQueue); got != 0 {
		t.Fatalf("soundQueue len=%d want=0", got)
	}
	if got := len(g.delayedSfx); got != 0 {
		t.Fatalf("delayedSfx len=%d want=0", got)
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
	s.bank.AttackClaw = PCMSample{SampleRate: 11025, Data: []byte{15}}
	s.bank.AttackSgt = PCMSample{SampleRate: 11025, Data: []byte{16}}
	s.bank.AttackSkull = PCMSample{SampleRate: 11025, Data: []byte{17}}
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
	if got, ok := s.sampleForEvent(soundEventMonsterAttackClaw); !ok || got.Data[0] != 15 {
		t.Fatalf("claw sample=%v ok=%v want explicit claw", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterAttackSgt); !ok || got.Data[0] != 16 {
		t.Fatalf("sgt attack sample=%v ok=%v want explicit sgt", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventMonsterAttackSkull); !ok || got.Data[0] != 17 {
		t.Fatalf("skull attack sample=%v ok=%v want explicit skull", got, ok)
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
	s.bank.DeathPodth1 = PCMSample{SampleRate: 11025, Data: []byte{21}}
	s.bank.DeathPodth2 = PCMSample{SampleRate: 11025, Data: []byte{22}}
	s.bank.DeathPodth3 = PCMSample{SampleRate: 11025, Data: []byte{23}}
	doomrand.Clear()
	if got, ok := s.sampleForEvent(soundEventDeathShotgunGuy); !ok || len(got.Data) == 0 || got.Data[0] < 21 || got.Data[0] > 23 {
		t.Fatalf("death shotgun sample=%v ok=%v want podth family", got, ok)
	}
	s.bank.DeathBgdth1 = PCMSample{SampleRate: 11025, Data: []byte{31}}
	s.bank.DeathBgdth2 = PCMSample{SampleRate: 11025, Data: []byte{32}}
	doomrand.Clear()
	if got, ok := s.sampleForEvent(soundEventDeathImp); !ok || len(got.Data) == 0 || got.Data[0] < 31 || got.Data[0] > 32 {
		t.Fatalf("death imp sample=%v ok=%v want bgdth family", got, ok)
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

func TestPCMMonoU8ToMonoS16_UsesCleanBitshift(t *testing.T) {
	got := pcmMonoU8ToMonoS16([]byte{0, 128, 255})
	want := []int16{-32768, 0, 32512}
	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample[%d]=%d want=%d", i, got[i], want[i])
		}
	}
}

func TestPrepareSoundBankForSourcePort_Precomputes44kMono(t *testing.T) {
	bank := PrepareSoundBankForSourcePort(SoundBank{
		ShootPistol: PCMSample{SampleRate: 11025, Data: []byte{0, 64, 128, 255}},
	}, 44100)
	s := bank.ShootPistol
	if s.PreparedRate != 44100 {
		t.Fatalf("prepared rate=%d want=44100", s.PreparedRate)
	}
	if len(s.PreparedMono) != sourcePortSFXPadFrames {
		t.Fatalf("prepared len=%d want=%d", len(s.PreparedMono), sourcePortSFXPadFrames)
	}
	if s.PreparedMono[0] >= s.PreparedMono[15] {
		t.Fatalf("prepared mono should preserve rising shape: first=%d sample15=%d", s.PreparedMono[0], s.PreparedMono[15])
	}
	for i := 16; i < len(s.PreparedMono); i++ {
		if s.PreparedMono[i] != 0 {
			t.Fatalf("prepared tail sample[%d]=%d want=0", i, s.PreparedMono[i])
		}
	}
}

func TestPadSourcePortMonoS16_PadsWithZeroSilence(t *testing.T) {
	got := padSourcePortMonoS16([]int16{1, 2, 3})
	if len(got) != sourcePortSFXPadFrames {
		t.Fatalf("len=%d want=%d", len(got), sourcePortSFXPadFrames)
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("prefix=%v want original prefix", got[:3])
	}
	for i := 3; i < len(got); i++ {
		if got[i] != 0 {
			t.Fatalf("tail sample at %d = %d want 0", i, got[i])
		}
	}
}

func TestApplySourcePortPresenceBoost_AccentuatesTransient(t *testing.T) {
	in := []int16{0, 0, 2000, 0, 0}
	got := applySourcePortPresenceBoost(in)
	if len(got) != len(in) {
		t.Fatalf("len=%d want=%d", len(got), len(in))
	}
	if got[2] <= in[2] {
		t.Fatalf("transient sample=%d want > %d", got[2], in[2])
	}
}

func TestDoomAdjustSoundParams_CenteredNearSound(t *testing.T) {
	vol, sep, ok := doomAdjustSoundParams(0, 0, 0, 64*fracUnit, 0, doomSoundMaxVolume, false)
	if !ok {
		t.Fatal("near sound should be audible")
	}
	if vol != doomSoundMaxVolume {
		t.Fatalf("vol=%d want=%d", vol, doomSoundMaxVolume)
	}
	if sep != doomSoundNormalSep {
		t.Fatalf("sep=%d want=%d", sep, doomSoundNormalSep)
	}
}

func TestDoomAdjustSoundParams_ClipsFarSound(t *testing.T) {
	_, _, ok := doomAdjustSoundParams(0, 0, 0, (doomSoundClippingDist + fracUnit), 0, doomSoundMaxVolume, false)
	if ok {
		t.Fatal("far sound should be clipped")
	}
}

func TestDoomSeparationVolumes_BiasesChannels(t *testing.T) {
	left, right := doomSeparationVolumes(doomSoundMaxVolume, 0)
	if right >= left {
		t.Fatalf("sep=0 should bias left: left=%d right=%d", left, right)
	}
	left, right = doomSeparationVolumes(doomSoundMaxVolume, 255)
	if left >= right {
		t.Fatalf("sep=255 should bias right: left=%d right=%d", left, right)
	}
}
