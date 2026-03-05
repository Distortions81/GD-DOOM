package automap

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
