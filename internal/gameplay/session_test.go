package gameplay

import (
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

func TestApplyPersistentSettingsToOptions(t *testing.T) {
	got := ApplyPersistentSettingsToOptions(OptionState{}, PersistentSettings{
		MouseLook:        true,
		MouseLookSpeed:   2.5,
		MusicVolume:      2,
		OPLVolume:        9,
		SFXVolume:        -1,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		LineColorMode:    "doom",
		ThingRenderMode:  "sprites",
	}, 4)

	if !got.MouseLook || got.MouseLookSpeed != 2.5 {
		t.Fatal("mouse settings were not applied")
	}
	if got.MusicVolume != 1 || got.OPLVolume != 4 || got.SFXVolume != 0 {
		t.Fatalf("volume clamp mismatch: %+v", got)
	}
	if !got.AlwaysRun || got.AutoWeaponSwitch {
		t.Fatal("run/weapon-switch flags mismatch")
	}
	if got.LineColorMode != "doom" || got.ThingRenderMode != "sprites" {
		t.Fatalf("render settings mismatch: %+v", got)
	}
}

func TestCollectDemoRecord(t *testing.T) {
	accum := []demo.Tic{{Forward: 10}}
	pending := []demo.Tic{{Forward: 20}, {Forward: 30}}

	got, remaining := CollectDemoRecord(accum, pending)
	want := []demo.Tic{{Forward: 10}, {Forward: 20}, {Forward: 30}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("combined=%v want=%v", got, want)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining=%v want empty", remaining)
	}
}

func TestPrepareRebuild(t *testing.T) {
	state, remaining := PrepareRebuild(
		[]demo.Tic{{Forward: 10}},
		[]demo.Tic{{Forward: 20}},
		OptionState{LineColorMode: "custom"},
		PersistentSettings{
			MouseLook:       true,
			MusicVolume:     2,
			LineColorMode:   "doom",
			ThingRenderMode: "sprites",
		},
		4,
	)

	if len(remaining) != 0 {
		t.Fatalf("remaining=%v want empty", remaining)
	}
	if !reflect.DeepEqual(state.DemoRecord, []demo.Tic{{Forward: 10}, {Forward: 20}}) {
		t.Fatalf("demoRecord=%v", state.DemoRecord)
	}
	if !state.Options.MouseLook || state.Options.MusicVolume != 1 {
		t.Fatalf("options=%+v", state.Options)
	}
	if state.Options.LineColorMode != "doom" || state.Options.ThingRenderMode != "sprites" {
		t.Fatalf("options=%+v", state.Options)
	}
}

func TestBuildRuntime(t *testing.T) {
	type built struct{ name mapdata.MapName }
	got := BuildRuntime(func(m *mapdata.Map, _ int) built {
		return built{name: m.Name}
	}, &mapdata.Map{Name: "E1M1"}, 7)
	if got.name != "E1M1" {
		t.Fatalf("built=%+v", got)
	}
}
