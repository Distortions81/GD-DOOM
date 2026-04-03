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
		MouseInvert:      true,
		MouseLookSpeed:   2.5,
		MusicVolume:      2,
		OPLVolume:        9,
		SFXVolume:        -1,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		ThingRenderMode:  "sprites",
	}, 4)

	if !got.MouseLook || !got.MouseInvert || got.MouseLookSpeed != 2.5 {
		t.Fatal("mouse settings were not applied")
	}
	if got.MusicVolume != 1 || got.OPLVolume != 4 || got.SFXVolume != 0 {
		t.Fatalf("volume clamp mismatch: %+v", got)
	}
	if !got.AlwaysRun || got.AutoWeaponSwitch {
		t.Fatal("run/weapon-switch flags mismatch")
	}
	if got.ThingRenderMode != "sprites" {
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
		OptionState{},
		PersistentSettings{
			MouseLook:       true,
			MusicVolume:     2,
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
	if state.Options.ThingRenderMode != "sprites" {
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

func TestRebuildRuntime(t *testing.T) {
	type runtime struct {
		pending []demo.Tic
		cleared bool
		applied bool
		name    mapdata.MapName
		opts    int
	}

	current := &runtime{pending: []demo.Tic{{Forward: 20}}}
	result := RebuildRuntime(RebuildRequest[int, *runtime]{
		Next:           &mapdata.Map{Name: "E1M2"},
		Current:        current,
		DemoRecord:     []demo.Tic{{Forward: 10}},
		CurrentOptions: OptionState{MusicVolume: 0.25},
		Settings:       PersistentSettings{MusicVolume: 2},
		MaxOPLGain:     4,
		PendingDemo: func(r *runtime) []demo.Tic {
			return r.pending
		},
		SetPendingDemo: func(r *runtime, remaining []demo.Tic) {
			r.pending = remaining
		},
		ClearBeforeBuild: func(r *runtime) {
			r.cleared = true
		},
		ApplyOptions: func(state OptionState) int {
			if state.MusicVolume != 1 {
				t.Fatalf("musicVolume=%v want 1", state.MusicVolume)
			}
			return 42
		},
		Build: func(m *mapdata.Map, opts int) *runtime {
			return &runtime{name: m.Name, opts: opts}
		},
		ApplyPersistent: func(r *runtime) {
			r.applied = true
		},
	})

	if !current.cleared {
		t.Fatal("expected current runtime to be cleared before rebuild")
	}
	if len(current.pending) != 0 {
		t.Fatalf("pending=%v want empty", current.pending)
	}
	if !reflect.DeepEqual(result.DemoRecord, []demo.Tic{{Forward: 10}, {Forward: 20}}) {
		t.Fatalf("demoRecord=%v", result.DemoRecord)
	}
	if result.Options != 42 {
		t.Fatalf("options=%d want 42", result.Options)
	}
	if result.Runtime == nil || result.Runtime.name != "E1M2" || result.Runtime.opts != 42 || !result.Runtime.applied {
		t.Fatalf("runtime=%+v", result.Runtime)
	}
}

func TestApplyPersistentSettingsNormalizesForRuntime(t *testing.T) {
	got := ApplyPersistentSettings(PersistentSettings{
		DetailLevel:      99,
		MusicVolume:      2,
		OPLVolume:        9,
		SFXVolume:        -1,
		ThingRenderMode:  "glyphs",
		PaletteLUT:       true,
		GammaLevel:       99,
		CRTEnabled:       true,
		Reveal:           99,
		IDDT:             99,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
	}, true, 3, 5, 7, 4, false, false, 0, 1)

	if got.DetailLevel != 4 {
		t.Fatalf("detailLevel=%d want 4", got.DetailLevel)
	}
	if got.MusicVolume != 1 || got.OPLVolume != 4 || got.SFXVolume != 0 {
		t.Fatalf("volume normalization mismatch: %+v", got)
	}
	if got.PaletteLUT || got.CRTEnabled {
		t.Fatalf("expected palette LUT and CRT to be disabled: %+v", got)
	}
	if got.GammaLevel != 6 {
		t.Fatalf("gammaLevel=%d want 6", got.GammaLevel)
	}
	if got.Reveal != 1 || got.IDDT != 2 {
		t.Fatalf("reveal/iddt=%d/%d want 1/2", got.Reveal, got.IDDT)
	}
	if !got.AlwaysRun || got.AutoWeaponSwitch {
		t.Fatalf("run flags mismatch: %+v", got)
	}
}
