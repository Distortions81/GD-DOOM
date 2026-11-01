package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestToggleSwitchTexture(t *testing.T) {
	got, ok := toggleSwitchTexture("SW1BRCOM")
	if !ok || got != "SW2BRCOM" {
		t.Fatalf("toggle SW1=> got=%q ok=%t", got, ok)
	}
	got, ok = toggleSwitchTexture("SW2LION")
	if !ok || got != "SW1LION" {
		t.Fatalf("toggle SW2=> got=%q ok=%t", got, ok)
	}
	if got, ok = toggleSwitchTexture("STARTAN3"); ok || got != "STARTAN3" {
		t.Fatalf("non-switch texture changed: got=%q ok=%t", got, ok)
	}
	if got, ok = toggleSwitchTexture("SW1FAKE"); ok || got != "SW1FAKE" {
		t.Fatalf("unknown switch texture changed: got=%q ok=%t", got, ok)
	}
}

func TestAnimateSwitchTextureRepeatReverts(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int32{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Mid: "SW1BRCOM"},
			},
		},
		delayedSwitchReverts: make([]delayedSwitchTexture, 0, 1),
	}
	g.animateSwitchTexture(0, 0, true)
	if got := g.m.Sidedefs[0].Mid; got != "SW2BRCOM" {
		t.Fatalf("mid=%q want=SW2BRCOM", got)
	}
	if len(g.delayedSwitchReverts) != 1 {
		t.Fatalf("delayed switch reverts=%d want=1", len(g.delayedSwitchReverts))
	}
	for i := 0; i < switchResetTics; i++ {
		g.tickDelayedSwitchReverts()
	}
	if got := g.m.Sidedefs[0].Mid; got != "SW1BRCOM" {
		t.Fatalf("mid=%q want reverted SW1BRCOM", got)
	}
	if len(g.delayedSwitchReverts) != 0 {
		t.Fatalf("delayed switch reverts=%d want=0", len(g.delayedSwitchReverts))
	}
}

func TestAnimateSwitchTexture_UsesFrontSidedefOnly(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int32{0, 1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Mid: "SW1BRCOM"},
				{Mid: "SW1LION"},
			},
		},
	}
	g.animateSwitchTexture(0, 1, false)
	if got := g.m.Sidedefs[0].Mid; got != "SW2BRCOM" {
		t.Fatalf("front mid=%q want=SW2BRCOM", got)
	}
	if got := g.m.Sidedefs[1].Mid; got != "SW1LION" {
		t.Fatalf("back mid=%q want unchanged SW1LION", got)
	}
}

func TestAnimateSwitchTextureRepeatKeepsSameLineTimer(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int32{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Mid: "SW1BRCOM"},
			},
		},
		delayedSwitchReverts: []delayedSwitchTexture{{
			line:    0,
			sidedef: 0,
			mid:     "SW1BRCOM",
			tics:    3,
		}},
	}
	g.animateSwitchTexture(0, 0, true)
	if len(g.delayedSwitchReverts) != 1 {
		t.Fatalf("delayed switch reverts=%d want=1", len(g.delayedSwitchReverts))
	}
	if got := g.delayedSwitchReverts[0].tics; got != 3 {
		t.Fatalf("revert timer=%d want=3", got)
	}
}

func TestAnimateSwitchTexture_StartsOneShotBlend(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode:             true,
			TextureAnimCrossfadeFrames: 5,
		},
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int32{0, -1}},
			},
			Sidedefs: []mapdata.Sidedef{
				{Mid: "SW1BRCOM"},
			},
		},
	}

	g.animateSwitchTexture(0, 0, false)
	got := g.switchTextureBlendFor(0, switchTextureSlotMid, "SW2BRCOM")
	if got.fromKey != "SW1BRCOM" || got.toKey != "SW2BRCOM" {
		t.Fatalf("switch blend=%+v want SW1BRCOM->SW2BRCOM", got)
	}
}

func TestTickDelayedSwitchReverts_StartsReturnBlend(t *testing.T) {
	g := &game{
		opts: Options{
			SourcePortMode:             true,
			TextureAnimCrossfadeFrames: 5,
		},
		worldTic: 10,
		m: &mapdata.Map{
			Sidedefs: []mapdata.Sidedef{
				{Mid: "SW2BRCOM"},
			},
		},
		delayedSwitchReverts: []delayedSwitchTexture{{
			sidedef: 0,
			mid:     "SW1BRCOM",
			tics:    1,
		}},
	}

	g.tickDelayedSwitchReverts()
	got := g.switchTextureBlendFor(0, switchTextureSlotMid, "SW1BRCOM")
	if got.fromKey != "SW2BRCOM" || got.toKey != "SW1BRCOM" {
		t.Fatalf("revert blend=%+v want SW2BRCOM->SW1BRCOM", got)
	}
}

func TestCustomSwitchOrderSlotsAndRestoration(t *testing.T) {
	g := &game{opts: Options{BoomSwitchPairs: [][2]string{{"A", "B"}, {"C", "D"}, {"A", "E"}}}, m: &mapdata.Map{Linedefs: []mapdata.Linedef{{SideNum: [2]int32{0, -1}}}, Sidedefs: []mapdata.Sidedef{{Top: "C", Mid: "A", Bottom: "A"}}}}
	g.animateSwitchTexture(0, 0, true)
	sd := &g.m.Sidedefs[0]
	if sd.Top != "C" || sd.Mid != "B" || sd.Bottom != "A" {
		t.Fatalf("must choose first pair then first slot: %+v", sd)
	}
	sd.Bottom = "OTHER"
	for i := 0; i < 10; i++ {
		g.tickDelayedSwitchReverts()
	}
	g.animateSwitchTexture(0, 0, true)
	if len(g.delayedSwitchReverts) != 1 || g.delayedSwitchReverts[0].tics != 25 || sd.Mid != "A" {
		t.Fatal("repeat must toggle without extending button timer")
	}
	sd.Top = "UNRELATED"
	for i := 0; i < 25; i++ {
		g.tickDelayedSwitchReverts()
	}
	if sd.Top != "UNRELATED" || sd.Mid != "A" || sd.Bottom != "OTHER" {
		t.Fatalf("restore changed unrelated slots: %+v", sd)
	}
	g.opts.BoomSwitchPairs = make([][2]string, 0)
	sd.Mid = "SW1BRCOM"
	g.animateSwitchTexture(0, 0, false)
	if sd.Mid != "SW1BRCOM" {
		t.Fatal("empty custom table fell back to Doom defaults")
	}
}
