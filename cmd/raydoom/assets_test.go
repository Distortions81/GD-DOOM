//go:build raylib && cgo && !js

package main

import (
	"testing"

	"gddoom/internal/wad"
)

func TestNativeAssetsKeepStimpackInSpriteBank(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"STIMA0", "PISGA0", "PISFA0", "TROOA2A8", "BAR1A0"} {
		if p, ok := opts.SpritePatchBank[name]; !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 {
			t.Fatalf("native sprite missing: %s", name)
		}
	}
	for _, name := range []string{"STBAR", "STTNUM0", "STFST00", "STKEYS0"} {
		if _, ok := opts.StatusPatchBank[name]; !ok {
			t.Fatalf("native HUD patch missing: %s", name)
		}
	}
	if _, ok := opts.StatusPatchBank["STIMA0"]; ok {
		t.Fatal("stimpack misclassified as status-bar artwork")
	}
}
