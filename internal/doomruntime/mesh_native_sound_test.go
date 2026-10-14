package doomruntime

import (
	"reflect"
	"testing"

	"gddoom/internal/audiofx"
	"gddoom/internal/doomrand"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

type nativeSpeakerProbe struct{ effects [][]sound.PCSpeakerTone }

func (p *nativeSpeakerProbe) Play(seq []sound.PCSpeakerTone) {
	p.effects = append(p.effects, append([]sound.PCSpeakerTone(nil), seq...))
}
func (*nativeSpeakerProbe) SetMusic([]sound.PCSpeakerTone, int, bool) {}
func (*nativeSpeakerProbe) ClearMusic()                               {}
func (*nativeSpeakerProbe) Stop()                                     {}
func (*nativeSpeakerProbe) SetVolume(float64)                         {}
func (*nativeSpeakerProbe) Close() error                              { return nil }

func TestNativePCSpeakerEventPolicyMatchesMainAndSurvivesRebirth(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	bank := sound.BuildPCSpeakerBank(sound.ImportPCSpeakerSounds(wf))
	c := nativeCampaignFixture(t, "E1M1")
	g := c.Game.g
	mainSink, nativeSink := &nativeSpeakerProbe{}, &nativeSpeakerProbe{}
	main := newSoundSystem(SoundBank{}, bank, mainSink, .7, 1, true, true, audiofx.PCSpeakerVariantSmallSpeaker)
	digital := 0
	c.Game.SetSoundSink(func(NativeSound) { digital++ })
	c.Game.SetPCSpeakerSound(bank, nativeSink, .7)
	g.snd.pitchShift = true
	events := []soundEvent{soundEventShootPistol, soundEventShootShotgun, soundEventSawIdle, soundEventMonsterActivePosit, soundEventItemUp, soundEventDoorOpen}
	doomrand.SetState(9, 17)
	for _, ev := range events {
		main.playEventSpatial(ev, queuedSoundOrigin{positioned: true}, 0, 0, 0, false)
	}
	rnd, prnd := doomrand.State()
	doomrand.SetState(9, 17)
	for _, ev := range events {
		g.snd.playEventSpatial(ev, queuedSoundOrigin{positioned: true}, 0, 0, 0, false)
	}
	nrnd, nprnd := doomrand.State()
	if len(nativeSink.effects) == 0 || !reflect.DeepEqual(mainSink.effects, nativeSink.effects) || nrnd != rnd || nprnd != prnd || digital != 0 {
		t.Fatalf("native PC event policy differs: events=%d/%d rng=%d/%d vs %d/%d digital=%d", len(nativeSink.effects), len(mainSink.effects), nrnd, nprnd, rnd, prnd, digital)
	}
	before := len(nativeSink.effects)
	g.snd.playEventSpatial(soundEventShootPistol, queuedSoundOrigin{positioned: true, x: 5000 * fracUnit}, 0, 0, 0, false)
	if len(nativeSink.effects) != before {
		t.Fatal("inaudible effect reached the PC speaker")
	}
	g.respawnDemoPlayer()
	g.snd.playEvent(soundEventShootPistol)
	if len(nativeSink.effects) != before+1 {
		t.Fatal("rebirth lost shared PC-speaker output")
	}
	c.Game.SetPCSpeakerSound(bank, nativeSink, 0)
	g.snd.playEvent(soundEventShootPistol)
	if len(nativeSink.effects) != before+1 {
		t.Fatal("muted PC-speaker effects played")
	}
	doomrand.Clear()
}
