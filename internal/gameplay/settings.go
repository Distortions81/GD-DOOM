package gameplay

type RuntimeSettings struct {
	DetailLevel      int
	GammaLevel       int
	MusicVolume      float64
	MUSPanMax        float64
	OPLVolume        float64
	SFXVolume        float64
	MouseLook        bool
	AlwaysRun        bool
	AutoWeaponSwitch bool
	LineColorMode    string
	ThingRenderMode  string
	CRTEffect        bool
}

func ApplyRuntimeSettings(cur PersistentSettings, s RuntimeSettings, sourcePort bool, faithfulLevels, sourcePortLevels, gammaLevels int, maxOPLGain float64) PersistentSettings {
	next := cur
	next.DetailLevel = ClampDetailLevel(s.DetailLevel, sourcePort, faithfulLevels, sourcePortLevels)
	next.MouseLook = s.MouseLook
	next.MusicVolume = ClampVolume(s.MusicVolume)
	next.OPLVolume = ClampOPLVolume(s.OPLVolume, maxOPLGain)
	next.SFXVolume = ClampVolume(s.SFXVolume)
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
	return next
}
