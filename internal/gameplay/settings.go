package gameplay

type RuntimeSettings struct {
	DetailLevel      int
	GammaLevel       int
	MusicVolume      float64
	MUSPanMax        float64
	OPLVolume        float64
	SFXVolume        float64
	HUDMessages      bool
	MouseLook        bool
	AlwaysRun        bool
	AutoWeaponSwitch bool
	LineColorMode    string
	ThingRenderMode  string
	CRTEffect        bool
}

type MusicAction int

const (
	MusicActionNone MusicAction = iota
	MusicActionStop
	MusicActionRestart
	MusicActionUpdateVolume
)

type RuntimeSettingsResult struct {
	Settings    PersistentSettings
	MusicAction MusicAction
}

func ApplyRuntimeSettings(cur PersistentSettings, s RuntimeSettings, sourcePort bool, faithfulLevels, sourcePortLevels, gammaLevels int, maxOPLGain float64) RuntimeSettingsResult {
	next := cur
	next.DetailLevel = ClampDetailLevel(s.DetailLevel, sourcePort, faithfulLevels, sourcePortLevels)
	next.MouseLook = s.MouseLook
	next.MusicVolume = ClampVolume(s.MusicVolume)
	next.OPLVolume = ClampOPLVolume(s.OPLVolume, maxOPLGain)
	next.SFXVolume = ClampVolume(s.SFXVolume)
	next.HUDMessages = s.HUDMessages
	next.AlwaysRun = s.AlwaysRun
	next.AutoWeaponSwitch = s.AutoWeaponSwitch
	if !sourcePort {
		next.LineColorMode = "parity"
	} else {
		next.LineColorMode = s.LineColorMode
	}
	next.ThingRenderMode = s.ThingRenderMode
	next.GammaLevel = ClampGamma(s.GammaLevel, gammaLevels)
	next.CRTEnabled = s.CRTEffect
	result := RuntimeSettingsResult{Settings: next}
	switch {
	case next.MusicVolume <= 0:
		result.MusicAction = MusicActionStop
	case ClampVolume(cur.MusicVolume) <= 0:
		result.MusicAction = MusicActionRestart
	default:
		result.MusicAction = MusicActionUpdateVolume
	}
	return result
}
