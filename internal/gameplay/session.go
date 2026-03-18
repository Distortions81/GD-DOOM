package gameplay

import (
	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

type OptionState struct {
	MouseLook        bool
	MouseLookSpeed   float64
	MusicVolume      float64
	OPLVolume        float64
	SFXVolume        float64
	AlwaysRun        bool
	AutoWeaponSwitch bool
	LineColorMode    string
	ThingRenderMode  string
}

func ApplyPersistentSettingsToOptions(cur OptionState, s PersistentSettings, maxOPLGain float64) OptionState {
	cur.MouseLook = s.MouseLook
	cur.MouseLookSpeed = s.MouseLookSpeed
	cur.MusicVolume = ClampVolume(s.MusicVolume)
	cur.OPLVolume = ClampOPLVolume(s.OPLVolume, maxOPLGain)
	cur.SFXVolume = ClampVolume(s.SFXVolume)
	cur.AlwaysRun = s.AlwaysRun
	cur.AutoWeaponSwitch = s.AutoWeaponSwitch
	cur.LineColorMode = s.LineColorMode
	cur.ThingRenderMode = s.ThingRenderMode
	return cur
}

func CollectDemoRecord(accum []demo.Tic, pending []demo.Tic) (combined []demo.Tic, remaining []demo.Tic) {
	if len(pending) == 0 {
		return accum, pending
	}
	combined = append(accum, pending...)
	return combined, pending[:0]
}

type RebuildState struct {
	DemoRecord []demo.Tic
	Options    OptionState
}

type RebuildRequest[Opts any, T any] struct {
	Next           *mapdata.Map
	Current        T
	DemoRecord     []demo.Tic
	CurrentOptions OptionState
	Settings       PersistentSettings
	MaxOPLGain     float64

	PendingDemo      func(T) []demo.Tic
	SetPendingDemo   func(T, []demo.Tic)
	ClearBeforeBuild func(T)
	ApplyOptions     func(OptionState) Opts
	Build            func(*mapdata.Map, Opts) T
	ApplyPersistent  func(T)
}

type RebuildResult[Opts any, T any] struct {
	Runtime    T
	DemoRecord []demo.Tic
	Options    Opts
}

type RuntimeFactory[Opts any, T any] func(*mapdata.Map, Opts) T

type SessionSignals struct {
	DemoActive       bool
	FrontendMenu     bool
	SaveGame         bool
	LoadGame         bool
	NewGameMap       *mapdata.Map
	NewGameSkill     int
	QuitPrompt       bool
	ReadThis         bool
	MusicPlayer      bool
	LevelRestart     bool
	LevelExit        bool
	SecretLevelExit  bool
	MapName          mapdata.MapName
	SourcePortMode   bool
	ViewWidth        int
	ViewHeight       int
	LowDetail        bool
	HUDMessages      bool
	ShowPerf         bool
	MouseLookSpeed   float64
	MusicVolume      float64
	SFXVolume        float64
	PaletteLUT       bool
	GammaLevel       int
	CRTEnabled       bool
	WorldTic         int
	SourcePortDetail int
}

func BuildRuntime[Opts any, T any](factory RuntimeFactory[Opts, T], m *mapdata.Map, opts Opts) T {
	var zero T
	if factory == nil || m == nil {
		return zero
	}
	return factory(m, opts)
}

func PrepareRebuild(accum []demo.Tic, pending []demo.Tic, opts OptionState, settings PersistentSettings, maxOPLGain float64) (state RebuildState, remaining []demo.Tic) {
	state.DemoRecord, remaining = CollectDemoRecord(accum, pending)
	state.Options = ApplyPersistentSettingsToOptions(opts, settings, maxOPLGain)
	return state, remaining
}

func RebuildRuntime[Opts any, T any](req RebuildRequest[Opts, T]) RebuildResult[Opts, T] {
	var pending []demo.Tic
	if req.PendingDemo != nil {
		pending = req.PendingDemo(req.Current)
	}
	if req.ClearBeforeBuild != nil {
		req.ClearBeforeBuild(req.Current)
	}
	state, remaining := PrepareRebuild(req.DemoRecord, pending, req.CurrentOptions, req.Settings, req.MaxOPLGain)
	if req.SetPendingDemo != nil {
		req.SetPendingDemo(req.Current, remaining)
	}
	result := RebuildResult[Opts, T]{
		DemoRecord: state.DemoRecord,
	}
	if req.ApplyOptions != nil {
		result.Options = req.ApplyOptions(state.Options)
	}
	if req.Build != nil {
		result.Runtime = req.Build(req.Next, result.Options)
		if req.ApplyPersistent != nil {
			req.ApplyPersistent(result.Runtime)
		}
	}
	return result
}
