package gameplay

import "testing"

func TestApplyRuntimeSettingsPreservesThingMode(t *testing.T) {
	cur := PersistentSettings{ThingRenderMode: "sprites", MusicVolume: 0.5}
	got := ApplyRuntimeSettings(cur, RuntimeSettings{
		DetailLevel:      99,
		AutoDetail:       true,
		GammaLevel:       99,
		MusicVolume:      2,
		OPLVolume:        9,
		SFXVolume:        -1,
		MouseLook:        true,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		ThingRenderMode:  "glyphs",
		CRTEffect:        true,
	}, false, 3, 5, 7, 4)

	if got.Settings.DetailLevel != 2 {
		t.Fatalf("detail=%d want 2", got.Settings.DetailLevel)
	}
	if !got.Settings.AutoDetail {
		t.Fatal("autoDetail should be preserved")
	}
	if got.Settings.GammaLevel != 6 {
		t.Fatalf("gamma=%d want 6", got.Settings.GammaLevel)
	}
	if got.Settings.MusicVolume != 1 {
		t.Fatalf("music=%v want 1", got.Settings.MusicVolume)
	}
	if got.Settings.OPLVolume != 4 {
		t.Fatalf("opl=%v want 4", got.Settings.OPLVolume)
	}
	if got.Settings.SFXVolume != 0 {
		t.Fatalf("sfx=%v want 0", got.Settings.SFXVolume)
	}
	if got.Settings.ThingRenderMode != "glyphs" {
		t.Fatalf("thingRenderMode=%q want glyphs", got.Settings.ThingRenderMode)
	}
	if !got.Settings.MouseLook || !got.Settings.AlwaysRun || got.Settings.AutoWeaponSwitch || !got.Settings.CRTEnabled {
		t.Fatal("runtime settings did not persist expected flags")
	}
	if got.MusicAction != MusicActionUpdateVolume {
		t.Fatalf("musicAction=%v want update-volume", got.MusicAction)
	}
}

func TestApplyRuntimeSettingsSourcePortKeepsRequestedThingMode(t *testing.T) {
	got := ApplyRuntimeSettings(PersistentSettings{}, RuntimeSettings{
		ThingRenderMode: "sprites",
	}, true, 3, 5, 7, 4)
	if got.Settings.ThingRenderMode != "sprites" {
		t.Fatalf("thingRenderMode=%q want sprites", got.Settings.ThingRenderMode)
	}
}

func TestApplyRuntimeSettingsMusicActionRestartWhenTurningMusicBackOn(t *testing.T) {
	got := ApplyRuntimeSettings(PersistentSettings{MusicVolume: 0}, RuntimeSettings{
		MusicVolume: 0.5,
	}, true, 3, 5, 7, 4)
	if got.MusicAction != MusicActionRestart {
		t.Fatalf("musicAction=%v want restart", got.MusicAction)
	}
}

func TestApplyRuntimeSettingsPreservesHUDMessages(t *testing.T) {
	got := ApplyRuntimeSettings(PersistentSettings{HUDMessages: false}, RuntimeSettings{
		HUDMessages: true,
	}, true, 3, 5, 7, 4)
	if !got.Settings.HUDMessages {
		t.Fatal("HUDMessages should be updated from runtime settings")
	}
}
