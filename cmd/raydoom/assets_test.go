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
		if p, ok := opts.SpritePatchBank[name]; !ok || p.Width <= 0 || p.Height <= 0 || len(p.RGBA) != p.Width*p.Height*4 || len(p.Indexed) != p.Width*p.Height {
			t.Fatalf("native sprite missing: %s", name)
		}
	}
	for _, name := range []string{"STBAR", "STTNUM0", "STFST00", "STKEYS0"} {
		if _, ok := opts.StatusPatchBank[name]; !ok {
			t.Fatalf("native HUD patch missing: %s", name)
		}
	}
	for _, name := range []string{"TITLEPIC", "M_DOOM", "M_SKULL1", "M_SKULL2", "M_NGAME", "M_OPTION", "M_QUITG"} {
		if _, ok := opts.MenuPatchBank[name]; !ok {
			t.Fatalf("native menu patch missing: %s", name)
		}
	}
	if tex, ok := opts.MessageFontBank['A']; !ok || tex.Width <= 0 {
		t.Fatal("Doom message/menu font missing")
	}
	if _, ok := opts.SpritePatchBank["STCFN065"]; ok {
		t.Fatal("font glyph misclassified as a world sprite")
	}
	if _, ok := opts.StatusPatchBank["STIMA0"]; ok {
		t.Fatal("stimpack misclassified as status-bar artwork")
	}
}

func TestNativeAssetsLoadIntermissionAndEndingArt(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WIMAP0", "WILV00", "WINUM0", "WIOSTK", "WISCRT2", "WIURH0", "CREDIT"} {
		if p, ok := opts.IntermissionPatchBank[name]; !ok || p.Width <= 0 || p.Height <= 0 {
			t.Fatalf("missing intermission art %s", name)
		}
		if _, ok := opts.SpritePatchBank[name]; ok {
			t.Fatalf("intermission art %s misclassified as world sprite", name)
		}
	}
}
