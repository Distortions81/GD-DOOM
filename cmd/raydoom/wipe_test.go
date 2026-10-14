//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessiontransition"
	"gddoom/internal/wad"
	"testing"
)

func TestNativeWipeFreezesPendingAndMovingCampaignTics(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	opts := doomruntime.Options{NoMonsters: true, SkillLevel: 3}
	c := doomruntime.NewNativeCampaign(doomruntime.NewNativeMeshGame(m, opts), opts, nil)
	w := &nativeWipe{active: true}
	in := doomruntime.NativeMeshInput{Forward: 1, Fire: true, Use: true, WeaponCycle: 1}
	for range 10 {
		if err := advanceNativeCampaign(c, w, in, true); err != nil {
			t.Fatal(err)
		}
		if c.Game.Frame(1).WorldTic != 0 {
			t.Fatal("pending wipe advanced simulation")
		}
	}
	w.ready = true
	w.positions = make([]int, sessiontransition.SourcePortMeltColumns)
	tics := 0
	for w.Active() && tics < 100 {
		if err := advanceNativeCampaign(c, w, in, true); err != nil {
			t.Fatal(err)
		}
		if c.Game.Frame(1).WorldTic != 0 {
			t.Fatal("moving wipe advanced simulation")
		}
		tics++
	}
	if w.Active() || tics <= 1 {
		t.Fatal("wipe did not complete its command tics")
	}
	if err := advanceNativeCampaign(c, w, doomruntime.NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.Game.Frame(1).WorldTic != 1 {
		t.Fatal("gameplay did not resume after the finishing transition tic")
	}
	w.Clear()
	if w.Active() || w.Ready() || len(w.positions) > 0 {
		t.Fatal("loading/clearing retained a wipe")
	}
	w.Queue()
	if w.Active() {
		t.Fatal("queued a wipe with no previous snapshot")
	}
}
