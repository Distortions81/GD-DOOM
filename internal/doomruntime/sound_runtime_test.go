package doomruntime

import (
	"testing"

	"gddoom/internal/audiofx"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
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

func TestSampleForEventSwitchSounds(t *testing.T) {
	s := &soundSystem{
		bank: SoundBank{
			SwitchOn:  PCMSample{SampleRate: 11025, Data: []byte{1}},
			SwitchOff: PCMSample{SampleRate: 11025, Data: []byte{2}},
		},
	}
	if got, ok := s.sampleForEvent(soundEventSwitchOn); !ok || got.Data[0] != 1 {
		t.Fatalf("switch on sample=%v ok=%v want switch on", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventSwitchExit); !ok || got.Data[0] != 2 {
		t.Fatalf("switch exit sample=%v ok=%v want switch exit", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventSwitchOff); !ok || got.Data[0] != 1 {
		t.Fatalf("switch off sample=%v ok=%v want switch on reset sound", got, ok)
	}
}

func TestMonsterVocalPreDelaySamples_RangeAndEventFilter(t *testing.T) {
	s := newSoundSystem(SoundBank{}, 1, true, false)
	if s == nil || s.player == nil {
		t.Skip("audio context unavailable")
	}
	delay := s.monsterVocalPreDelaySamples(soundEventMonsterSeePosit)
	maxDelay := 25.0 * float64(audiofx.EnsureSharedAudioContext().SampleRate()) / 1000.0
	if delay < 0 || delay > maxDelay {
		t.Fatalf("delay=%f want in [0,%f]", delay, maxDelay)
	}
	if got := s.monsterVocalPreDelaySamples(soundEventMonsterPainHumanoid); got < 0 || got > maxDelay {
		t.Fatalf("pain delay=%f want in [0,%f]", got, maxDelay)
	}
	if got := s.monsterVocalPreDelaySamples(soundEventDeathImp); got < 0 || got > maxDelay {
		t.Fatalf("death delay=%f want in [0,%f]", got, maxDelay)
	}
	if got := s.monsterVocalPreDelaySamples(soundEventShootPistol); got != 0 {
		t.Fatalf("non-alert delay=%f want 0", got)
	}
}

func TestMonsterVocalPreDelaySamples_DoesNotAdvancePRandom(t *testing.T) {
	s := newSoundSystem(SoundBank{}, 1, true, false)
	if s == nil || s.player == nil {
		t.Skip("audio context unavailable")
	}
	doomrand.Clear()
	wantPRandom := doomrand.PRandom()
	doomrand.Clear()
	_ = s.monsterVocalPreDelaySamples(soundEventMonsterSeePosit)
	if got := doomrand.PRandom(); got != wantPRandom {
		t.Fatalf("PRandom advanced after vocal pre-delay: got=%d want=%d", got, wantPRandom)
	}
}

func TestPlayEventSpatial_DefaultPitchShiftOffDoesNotConsumeVanillaPitchRandom(t *testing.T) {
	doomrand.Clear()
	s := &soundSystem{vanillaVolume: 15}
	s.playEventSpatial(soundEventMonsterSeePosit3, queuedSoundOrigin{}, 0, 0, 0, false)
	rnd, prnd := doomrand.State()
	if rnd != 0 || prnd != 0 {
		t.Fatalf("rng state after no-backend sound start with pitch shift off=(%d,%d) want=(0,0)", rnd, prnd)
	}
}

func TestPlayEventSpatial_ConsumesVanillaPitchRandomWhenPitchShiftEnabled(t *testing.T) {
	doomrand.Clear()
	s := &soundSystem{vanillaVolume: 15, pitchShift: true}
	s.playEventSpatial(soundEventMonsterSeePosit3, queuedSoundOrigin{}, 0, 0, 0, false)
	rnd, prnd := doomrand.State()
	if rnd != 1 || prnd != 0 {
		t.Fatalf("rng state after no-backend sound start with pitch shift on=(%d,%d) want=(1,0)", rnd, prnd)
	}
}

func TestPlayEventSpatial_InaudiblePositionedSoundDoesNotConsumeVanillaPitchRandom(t *testing.T) {
	doomrand.Clear()
	s := &soundSystem{vanillaVolume: 15}
	s.playEventSpatial(
		soundEventMonsterSeePosit3,
		queuedSoundOrigin{positioned: true, x: doomSoundClippingDist * 2, y: 0},
		0, 0, 0, false,
	)
	rnd, prnd := doomrand.State()
	if rnd != 0 || prnd != 0 {
		t.Fatalf("rng state after inaudible sound=(%d,%d) want=(0,0)", rnd, prnd)
	}
}

func TestPlayEventSpatial_ItemUpWithoutBackendDoesNotConsumeVanillaPitchRandom(t *testing.T) {
	doomrand.Clear()
	s := &soundSystem{vanillaVolume: 15}
	s.playEventSpatial(soundEventItemUp, queuedSoundOrigin{}, 0, 0, 0, false)
	rnd, prnd := doomrand.State()
	if rnd != 0 || prnd != 0 {
		t.Fatalf("rng state after itemup sound=(%d,%d) want=(0,0)", rnd, prnd)
	}
}

func TestPlayEventSpatial_TinkWithoutBackendDoesNotConsumeVanillaPitchRandom(t *testing.T) {
	doomrand.Clear()
	s := &soundSystem{vanillaVolume: 15}
	s.playEventSpatial(soundEventTink, queuedSoundOrigin{}, 0, 0, 0, false)
	rnd, prnd := doomrand.State()
	if rnd != 0 || prnd != 0 {
		t.Fatalf("rng state after tink sound=(%d,%d) want=(0,0)", rnd, prnd)
	}
}

func TestVanillaPitchModeForEvent_MatchesDoomSoundClasses(t *testing.T) {
	cases := []struct {
		ev   soundEvent
		want vanillaPitchMode
	}{
		{soundEventItemUp, vanillaPitchNone},
		{soundEventTink, vanillaPitchNone},
		{soundEventSawUp, vanillaPitchSaw},
		{soundEventSawIdle, vanillaPitchSaw},
		{soundEventSawFull, vanillaPitchSaw},
		{soundEventSawHit, vanillaPitchSaw},
		{soundEventWeaponUp, vanillaPitchDefault},
		{soundEventPowerUp, vanillaPitchDefault},
		{soundEventNoWay, vanillaPitchDefault},
		{soundEventSwitchOn, vanillaPitchDefault},
		{soundEventSwitchExit, vanillaPitchDefault},
		{soundEventSwitchOff, vanillaPitchDefault},
		{soundEventTeleport, vanillaPitchDefault},
		{soundEventOof, vanillaPitchDefault},
		{soundEventPain, vanillaPitchDefault},
		{soundEventShootPistol, vanillaPitchDefault},
		{soundEventShootShotgun, vanillaPitchDefault},
		{soundEventShootPlasma, vanillaPitchDefault},
		{soundEventShootRocket, vanillaPitchDefault},
		{soundEventShootFireball, vanillaPitchDefault},
		{soundEventPunch, vanillaPitchDefault},
		{soundEventMonsterAttackClaw, vanillaPitchDefault},
		{soundEventMonsterAttackSgt, vanillaPitchDefault},
		{soundEventMonsterAttackSkull, vanillaPitchDefault},
		{soundEventMonsterAttackArchvile, vanillaPitchDefault},
		{soundEventMonsterAttackMancubus, vanillaPitchDefault},
		{soundEventImpactFire, vanillaPitchDefault},
		{soundEventImpactRocket, vanillaPitchDefault},
		{soundEventBarrelExplode, vanillaPitchDefault},
		{soundEventMonsterSeePosit, vanillaPitchDefault},
		{soundEventMonsterSeeImp, vanillaPitchDefault},
		{soundEventMonsterSeeDemon, vanillaPitchDefault},
		{soundEventMonsterSeeCaco, vanillaPitchDefault},
		{soundEventMonsterSeeBaron, vanillaPitchDefault},
		{soundEventMonsterSeeKnight, vanillaPitchDefault},
		{soundEventMonsterSeeSpider, vanillaPitchDefault},
		{soundEventMonsterSeeArachnotron, vanillaPitchDefault},
		{soundEventMonsterSeeCyber, vanillaPitchDefault},
		{soundEventMonsterSeePainElemental, vanillaPitchDefault},
		{soundEventMonsterSeeWolfSS, vanillaPitchDefault},
		{soundEventMonsterSeeArchvile, vanillaPitchDefault},
		{soundEventMonsterSeeRevenant, vanillaPitchDefault},
		{soundEventMonsterActivePosit, vanillaPitchDefault},
		{soundEventMonsterActiveImp, vanillaPitchDefault},
		{soundEventMonsterActiveDemon, vanillaPitchDefault},
		{soundEventMonsterActiveArachnotron, vanillaPitchDefault},
		{soundEventMonsterActiveArchvile, vanillaPitchDefault},
		{soundEventMonsterActiveRevenant, vanillaPitchDefault},
		{soundEventMonsterPainHumanoid, vanillaPitchDefault},
		{soundEventMonsterPainDemon, vanillaPitchDefault},
		{soundEventDeathZombie, vanillaPitchDefault},
		{soundEventDeathShotgunGuy, vanillaPitchDefault},
		{soundEventDeathChaingunner, vanillaPitchDefault},
		{soundEventDeathImp, vanillaPitchDefault},
		{soundEventDeathDemon, vanillaPitchDefault},
		{soundEventDeathCaco, vanillaPitchDefault},
		{soundEventDeathBaron, vanillaPitchDefault},
		{soundEventDeathKnight, vanillaPitchDefault},
		{soundEventDeathCyber, vanillaPitchDefault},
		{soundEventDeathSpider, vanillaPitchDefault},
		{soundEventDeathArachnotron, vanillaPitchDefault},
		{soundEventDeathLostSoul, vanillaPitchDefault},
		{soundEventDeathMancubus, vanillaPitchDefault},
		{soundEventDeathRevenant, vanillaPitchDefault},
		{soundEventDeathPainElemental, vanillaPitchDefault},
		{soundEventDeathWolfSS, vanillaPitchDefault},
		{soundEventDeathArchvile, vanillaPitchDefault},
		{soundEventPlayerDeath, vanillaPitchDefault},
		{soundEventBossBrainSpit, vanillaPitchDefault},
		{soundEventBossBrainCube, vanillaPitchDefault},
		{soundEventBossBrainAwake, vanillaPitchDefault},
		{soundEventBossBrainPain, vanillaPitchDefault},
		{soundEventBossBrainDeath, vanillaPitchDefault},
		{soundEventIntermissionTick, vanillaPitchDefault},
		{soundEventIntermissionDone, vanillaPitchDefault},
	}
	for _, tc := range cases {
		if got := vanillaPitchModeForEvent(tc.ev); got != tc.want {
			t.Fatalf("event=%v mode=%v want=%v", tc.ev, got, tc.want)
		}
	}
}

func TestFlushSoundEvents_DefaultPitchShiftOffDoesNotConsumeVanillaPitchRandom(t *testing.T) {
	doomrand.Clear()
	g := &game{
		soundQueue:       []soundEvent{soundEventMonsterSeePosit3},
		soundQueueOrigin: []queuedSoundOrigin{{}},
		m:                &mapdata.Map{Name: "E1M5"},
		snd:              &soundSystem{vanillaVolume: 15},
	}
	g.flushSoundEvents()
	rnd, prnd := doomrand.State()
	if rnd != 0 || prnd != 0 {
		t.Fatalf("rng state after flushing no-backend sound queue with pitch shift off=(%d,%d) want=(0,0)", rnd, prnd)
	}
	if len(g.soundQueue) != 0 || len(g.soundQueueOrigin) != 0 {
		t.Fatalf("sound queues not cleared: queue=%d origin=%d", len(g.soundQueue), len(g.soundQueueOrigin))
	}
}

func TestFlushSoundEvents_ConsumesVanillaPitchRandomWhenPitchShiftEnabled(t *testing.T) {
	doomrand.Clear()
	g := &game{
		soundQueue:       []soundEvent{soundEventMonsterSeePosit3},
		soundQueueOrigin: []queuedSoundOrigin{{}},
		m:                &mapdata.Map{Name: "E1M5"},
		snd:              &soundSystem{vanillaVolume: 15, pitchShift: true},
	}
	g.flushSoundEvents()
	rnd, prnd := doomrand.State()
	if rnd != 1 || prnd != 0 {
		t.Fatalf("rng state after flushing no-backend sound queue with pitch shift on=(%d,%d) want=(1,0)", rnd, prnd)
	}
}

func TestVanillaSoundWouldStart_UsesDoomVolumeScaleAtClipEdge(t *testing.T) {
	origin := queuedSoundOrigin{positioned: true, x: 10485760, y: 46137344}
	listenerX := int64(-27165025)
	listenerY := int64(-12488865)
	listenerAngle := uint32(452984832)
	if vanillaSoundWouldStart(&soundSystem{vanillaVolume: 15}, origin, listenerX, listenerY, listenerAngle, false) {
		t.Fatalf("edge sound should clip even at Doom max volume")
	}
	if !vanillaSoundWouldStart(&soundSystem{vanillaVolume: 127}, origin, listenerX, listenerY, listenerAngle, false) {
		t.Fatalf("non-Doom 127 scale would incorrectly keep this edge sound audible")
	}
	if vanillaSoundWouldStart(nil, origin, listenerX, listenerY, listenerAngle, false) {
		t.Fatalf("nil sound system should use Doom default volume and clip this edge sound")
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
	if got, ok := s.sampleForEvent(soundEventPunch); !ok || len(got.Data) == 0 || got.Data[0] != 1 {
		t.Fatalf("punch sample=%v ok=%v want claw fallback", got, ok)
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
	s.bank.Punch = PCMSample{SampleRate: 11025, Data: []byte{18}}
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
	if got, ok := s.sampleForEvent(soundEventPunch); !ok || got.Data[0] != 18 {
		t.Fatalf("punch sample=%v ok=%v want explicit punch", got, ok)
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

func TestSampleForEventBossBrainSounds(t *testing.T) {
	s := &soundSystem{
		bank: SoundBank{
			SwitchOn:       PCMSample{SampleRate: 11025, Data: []byte{1}},
			ShootFireball:  PCMSample{SampleRate: 11025, Data: []byte{2}},
			ImpactRocket:   PCMSample{SampleRate: 11025, Data: []byte{3}},
			BossBrainSpit:  PCMSample{SampleRate: 11025, Data: []byte{4}},
			BossBrainCube:  PCMSample{SampleRate: 11025, Data: []byte{5}},
			BossBrainAwake: PCMSample{SampleRate: 11025, Data: []byte{6}},
			BossBrainPain:  PCMSample{SampleRate: 11025, Data: []byte{7}},
			BossBrainDeath: PCMSample{SampleRate: 11025, Data: []byte{8}},
		},
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainSpit); !ok || got.Data[0] != 4 {
		t.Fatalf("boss spit sample=%v ok=%v want explicit boss spit", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainCube); !ok || got.Data[0] != 5 {
		t.Fatalf("boss cube sample=%v ok=%v want explicit boss cube", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainAwake); !ok || got.Data[0] != 6 {
		t.Fatalf("boss awake sample=%v ok=%v want explicit boss awake", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainPain); !ok || got.Data[0] != 7 {
		t.Fatalf("boss pain sample=%v ok=%v want explicit boss pain", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainDeath); !ok || got.Data[0] != 8 {
		t.Fatalf("boss death sample=%v ok=%v want explicit boss death", got, ok)
	}

	s.bank.BossBrainSpit = PCMSample{}
	s.bank.BossBrainCube = PCMSample{}
	s.bank.BossBrainAwake = PCMSample{}
	s.bank.BossBrainPain = PCMSample{}
	s.bank.BossBrainDeath = PCMSample{}
	s.bank.MonsterPainHumanoid = PCMSample{SampleRate: 11025, Data: []byte{9}}
	if got, ok := s.sampleForEvent(soundEventBossBrainSpit); !ok || got.Data[0] != 2 {
		t.Fatalf("boss spit fallback=%v ok=%v want fireball", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainCube); !ok || got.Data[0] != 2 {
		t.Fatalf("boss cube fallback=%v ok=%v want spit fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainAwake); !ok || got.Data[0] != 2 {
		t.Fatalf("boss awake fallback=%v ok=%v want spit fallback", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainPain); !ok || got.Data[0] != 9 {
		t.Fatalf("boss pain fallback=%v ok=%v want monster pain", got, ok)
	}
	if got, ok := s.sampleForEvent(soundEventBossBrainDeath); !ok || got.Data[0] != 3 {
		t.Fatalf("boss death fallback=%v ok=%v want impact rocket", got, ok)
	}
}

func TestSampleForEventVariantSelectionDoesNotAdvancePRandom(t *testing.T) {
	s := &soundSystem{
		bank: SoundBank{
			SeePosit1:   PCMSample{SampleRate: 11025, Data: []byte{1}},
			SeePosit2:   PCMSample{SampleRate: 11025, Data: []byte{2}},
			SeePosit3:   PCMSample{SampleRate: 11025, Data: []byte{3}},
			DeathPodth1: PCMSample{SampleRate: 11025, Data: []byte{4}},
			DeathPodth2: PCMSample{SampleRate: 11025, Data: []byte{5}},
			DeathPodth3: PCMSample{SampleRate: 11025, Data: []byte{6}},
		},
	}

	doomrand.Clear()
	wantPRandom := doomrand.PRandom()
	doomrand.Clear()
	if _, ok := s.sampleForEvent(soundEventMonsterSeePosit); !ok {
		t.Fatalf("see posit sample missing")
	}
	if got := doomrand.PRandom(); got != wantPRandom {
		t.Fatalf("PRandom advanced after see-posit sample selection: got=%d want=%d", got, wantPRandom)
	}

	doomrand.Clear()
	wantPRandom = doomrand.PRandom()
	doomrand.Clear()
	if _, ok := s.sampleForEvent(soundEventDeathShotgunGuy); !ok {
		t.Fatalf("death shotgun sample missing")
	}
	if got := doomrand.PRandom(); got != wantPRandom {
		t.Fatalf("PRandom advanced after death-shotgun sample selection: got=%d want=%d", got, wantPRandom)
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

func TestPCMMonoU8ToStereoS16LEResampledPreservesRamp(t *testing.T) {
	src := []byte{0, 255}
	got := pcmMonoU8ToStereoS16LEResampled(src, 11025, 44100)
	if len(got) != 8*4 {
		t.Fatalf("len=%d want=%d", len(got), 8*4)
	}
	samples := make([]int16, 0, len(got)/2)
	for i := 0; i+1 < len(got); i += 2 {
		samples = append(samples, int16(uint16(got[i])|uint16(got[i+1])<<8))
	}
	if len(samples) < 4 {
		t.Fatalf("samples len=%d want >= 4", len(samples))
	}
	if !(samples[0] < samples[2] && samples[2] < samples[len(samples)-2]) {
		t.Fatalf("expected rising interpolated ramp, got first=%d mid=%d last=%d", samples[0], samples[2], samples[len(samples)-2])
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
	const resampledLen = 16
	if len(s.PreparedMono) != resampledLen {
		t.Fatalf("prepared len=%d want=%d", len(s.PreparedMono), resampledLen)
	}
	if s.PreparedMono[0] >= s.PreparedMono[15] {
		t.Fatalf("prepared mono should preserve rising shape: first=%d sample15=%d", s.PreparedMono[0], s.PreparedMono[15])
	}
}

func TestPrepareSoundBankForFaithful_Precomputes44kMono(t *testing.T) {
	bank := audiofx.PrepareSoundBankForFaithful(SoundBank{
		ShootPistol: PCMSample{SampleRate: 11025, Data: []byte{0, 64, 128, 255}},
	}, 44100)
	s := bank.ShootPistol
	if s.FaithfulPreparedRate != 44100 {
		t.Fatalf("faithful prepared rate=%d want=44100", s.FaithfulPreparedRate)
	}
	const resampledLen = 16
	if len(s.FaithfulPreparedMono) != resampledLen {
		t.Fatalf("faithful prepared len=%d want=%d", len(s.FaithfulPreparedMono), resampledLen)
	}
	if s.FaithfulPreparedMono[0] >= s.FaithfulPreparedMono[15] {
		t.Fatalf("faithful prepared mono should preserve rising shape: first=%d sample15=%d", s.FaithfulPreparedMono[0], s.FaithfulPreparedMono[15])
	}
}

func TestApplyVanillaPitch_UsesDoomExponentialStepTable(t *testing.T) {
	s := PCMSample{SampleRate: 11025, Data: []byte{0, 64, 128, 255}}
	got := applyVanillaPitch(s, 144)
	want := (11025 * doomPitchStep(144)) / 65536
	if got.SampleRate != want {
		t.Fatalf("sample rate=%d want=%d", got.SampleRate, want)
	}
	if got.SampleRate == (11025*144)/128 {
		t.Fatalf("sample rate=%d unexpectedly matches old linear scaling", got.SampleRate)
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
