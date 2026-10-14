package doomruntime

import (
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestHeadlessRunsRealMapWithoutRenderOrAudioDevices(t *testing.T) {
	// The package imports Ebitengine, but importing it does not create a window.
	// Run this test with DISPLAY and WAYLAND_DISPLAY unset to cover a server host.
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(m, Options{Headless: true, SFXVolume: 1, SkillLevel: 3, NoMonsters: true})
	if g.snd == nil || g.snd.player != nil || g.snd.pcSpeaker != nil || g.snd.nativePlay != nil {
		t.Fatal("headless game created an audio output")
	}
	if cap(g.spriteTXScratch) != 0 || cap(g.wallPrepassBuf) != 0 {
		t.Fatal("headless game allocated renderer scratch")
	}
	x, y := g.p.x, g.p.y
	for range 35 {
		g.stepGameplayFromDemoTic(demo.Tic{Forward: 25})
	}
	if g.worldTic != 35 {
		t.Fatalf("world advanced %d tics, want 35", g.worldTic)
	}
	if g.p.x == x && g.p.y == y {
		t.Fatal("headless player did not move on the real map")
	}
}

func TestHeadlessSoundPreservesVanillaBookkeeping(t *testing.T) {
	opts := Options{Headless: true, SFXVolume: 1, SFXPitchShift: true}
	s := newHeadlessSoundSystem(opts)
	if s.vanillaVolume != 15 || !s.pitchShift {
		t.Fatal("headless sound discarded vanilla sound configuration")
	}
	before, gameplayBefore := doomrand.State()
	s.playEvent(soundEventShootPistol)
	after, gameplayAfter := doomrand.State()
	if after == before {
		t.Fatal("headless sound did not consume cosmetic pitch RNG")
	}
	if gameplayAfter != gameplayBefore {
		t.Fatal("headless sound consumed gameplay RNG")
	}
}
