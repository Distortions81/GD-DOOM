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
				{SideNum: [2]int16{0, -1}},
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
				{SideNum: [2]int16{0, 1}},
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

func TestAnimateSwitchTextureRepeatRefreshesSameLineEntry(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Linedefs: []mapdata.Linedef{
				{SideNum: [2]int16{0, -1}},
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
	if got := g.delayedSwitchReverts[0].tics; got != switchResetTics {
		t.Fatalf("revert timer=%d want=%d", got, switchResetTics)
	}
}
