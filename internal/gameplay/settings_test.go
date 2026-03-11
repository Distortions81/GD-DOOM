package gameplay

import "testing"

func TestApplyRuntimeSettingsFaithfulForcesParityLineMode(t *testing.T) {
	cur := PersistentSettings{LineColorMode: "doom", ThingRenderMode: "sprites"}
	got := ApplyRuntimeSettings(cur, RuntimeSettings{
		DetailLevel:      99,
		GammaLevel:       99,
		MusicVolume:      2,
		OPLVolume:        9,
		SFXVolume:        -1,
		MouseLook:        true,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		LineColorMode:    "custom",
		ThingRenderMode:  "glyphs",
		CRTEffect:        true,
	}, false, 3, 5, 7, 4)

	if got.DetailLevel != 2 {
		t.Fatalf("detail=%d want 2", got.DetailLevel)
	}
	if got.GammaLevel != 6 {
		t.Fatalf("gamma=%d want 6", got.GammaLevel)
	}
	if got.MusicVolume != 1 {
		t.Fatalf("music=%v want 1", got.MusicVolume)
	}
	if got.OPLVolume != 4 {
		t.Fatalf("opl=%v want 4", got.OPLVolume)
	}
	if got.SFXVolume != 0 {
		t.Fatalf("sfx=%v want 0", got.SFXVolume)
	}
	if got.LineColorMode != "parity" {
		t.Fatalf("lineColorMode=%q want parity", got.LineColorMode)
	}
	if got.ThingRenderMode != "glyphs" {
		t.Fatalf("thingRenderMode=%q want glyphs", got.ThingRenderMode)
	}
	if !got.MouseLook || !got.AlwaysRun || got.AutoWeaponSwitch || !got.CRTEnabled {
		t.Fatal("runtime settings did not persist expected flags")
	}
}

func TestApplyRuntimeSettingsSourcePortKeepsRequestedLineMode(t *testing.T) {
	got := ApplyRuntimeSettings(PersistentSettings{}, RuntimeSettings{
		LineColorMode: "doom",
	}, true, 3, 5, 7, 4)
	if got.LineColorMode != "doom" {
		t.Fatalf("lineColorMode=%q want doom", got.LineColorMode)
	}
}
