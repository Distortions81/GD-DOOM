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

type RuntimeFactory[Opts any, T any] func(*mapdata.Map, Opts) T

type SessionSignals struct {
	DemoActive       bool
	NewGameMap       *mapdata.Map
	NewGameSkill     int
	QuitPrompt       bool
	ReadThis         bool
	LevelRestart     bool
	LevelExit        bool
	SecretLevelExit  bool
	MapName          mapdata.MapName
	SourcePortMode   bool
	ViewWidth        int
	ViewHeight       int
	LowDetail        bool
	HUDMessages      bool
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
