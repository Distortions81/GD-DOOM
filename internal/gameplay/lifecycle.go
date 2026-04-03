package gameplay

import "gddoom/internal/mapdata"

type PersistentSettings struct {
	DetailLevel      int
	AutoDetail       bool
	RotateView       bool
	MouseLook        bool
	MouseInvert      bool
	MouseLookSpeed   float64
	MusicVolume      float64
	OPLVolume        float64
	SFXVolume        float64
	HUDMessages      bool
	AlwaysRun        bool
	AutoWeaponSwitch bool
	ThingRenderMode  string
	ShowLegend       bool
	PaletteLUT       bool
	GammaLevel       int
	CRTEnabled       bool
	Reveal           int
	IDDT             int
}

type AppliedPersistentSettings struct {
	DetailLevel      int
	AutoDetail       bool
	RotateView       bool
	MouseLook        bool
	MouseInvert      bool
	MouseLookSpeed   float64
	MusicVolume      float64
	OPLVolume        float64
	SFXVolume        float64
	HUDMessages      bool
	AlwaysRun        bool
	AutoWeaponSwitch bool
	ThingRenderMode  string
	ShowLegend       bool
	PaletteLUT       bool
	GammaLevel       int
	CRTEnabled       bool
	Reveal           int
	IDDT             int
}

func ApplyPersistentSettings(s PersistentSettings, sourcePort bool, faithfulLevels, sourcePortLevels, gammaLevels int, maxOPLGain float64, kageShader, hasPalette bool, normalReveal, allMapReveal int) AppliedPersistentSettings {
	return AppliedPersistentSettings{
		DetailLevel:      ClampDetailLevel(s.DetailLevel, sourcePort, faithfulLevels, sourcePortLevels),
		AutoDetail:       s.AutoDetail,
		RotateView:       s.RotateView,
		MouseLook:        s.MouseLook,
		MouseInvert:      s.MouseInvert,
		MouseLookSpeed:   s.MouseLookSpeed,
		MusicVolume:      ClampVolume(s.MusicVolume),
		OPLVolume:        ClampOPLVolume(s.OPLVolume, maxOPLGain),
		SFXVolume:        ClampVolume(s.SFXVolume),
		HUDMessages:      s.HUDMessages,
		AlwaysRun:        s.AlwaysRun,
		AutoWeaponSwitch: s.AutoWeaponSwitch,
		ThingRenderMode:  s.ThingRenderMode,
		ShowLegend:       s.ShowLegend,
		PaletteLUT:       s.PaletteLUT && kageShader && hasPalette,
		GammaLevel:       ClampGamma(s.GammaLevel, gammaLevels),
		CRTEnabled:       s.CRTEnabled && kageShader,
		Reveal:           NormalizeReveal(s.Reveal, sourcePort, normalReveal, allMapReveal),
		IDDT:             ClampIDDT(s.IDDT),
	}
}

func CloneMapForRestart(src *mapdata.Map) *mapdata.Map {
	if src == nil {
		return nil
	}
	dup := *src
	dup.Things = append([]mapdata.Thing(nil), src.Things...)
	dup.Vertexes = append([]mapdata.Vertex(nil), src.Vertexes...)
	dup.Linedefs = append([]mapdata.Linedef(nil), src.Linedefs...)
	dup.Sidedefs = append([]mapdata.Sidedef(nil), src.Sidedefs...)
	dup.Sectors = append([]mapdata.Sector(nil), src.Sectors...)
	dup.Segs = append([]mapdata.Seg(nil), src.Segs...)
	dup.SubSectors = append([]mapdata.SubSector(nil), src.SubSectors...)
	dup.Nodes = append([]mapdata.Node(nil), src.Nodes...)
	dup.Reject = append([]byte(nil), src.Reject...)
	dup.Blockmap = append([]int16(nil), src.Blockmap...)
	if src.RejectMatrix != nil {
		rm := *src.RejectMatrix
		rm.Data = append([]byte(nil), src.RejectMatrix.Data...)
		dup.RejectMatrix = &rm
	}
	if src.BlockMap != nil {
		bm := *src.BlockMap
		bm.Offsets = append([]uint16(nil), src.BlockMap.Offsets...)
		bm.Cells = make([][]int16, len(src.BlockMap.Cells))
		for i, cell := range src.BlockMap.Cells {
			bm.Cells[i] = append([]int16(nil), cell...)
		}
		dup.BlockMap = &bm
	}
	return &dup
}

func ClampDetailLevel(level int, sourcePort bool, faithfulLevels, sourcePortLevels int) int {
	limit := faithfulLevels
	if sourcePort {
		limit = sourcePortLevels
	}
	if limit <= 0 {
		return 0
	}
	if level < 0 {
		return 0
	}
	maxLevel := limit - 1
	if level > maxLevel {
		return maxLevel
	}
	return level
}

func NormalizeReveal(mode int, sourcePort bool, normalMode, allMapMode int) int {
	switch mode {
	case normalMode, allMapMode:
		return mode
	default:
		if sourcePort {
			return allMapMode
		}
		return normalMode
	}
}

func ClampIDDT(v int) int {
	if v < 0 {
		return 0
	}
	if v > 2 {
		return 2
	}
	return v
}

func ClampGamma(level, gammaLevels int) int {
	if level < 0 || gammaLevels <= 0 {
		return 0
	}
	maxLevel := gammaLevels - 1
	if level > maxLevel {
		return maxLevel
	}
	return level
}

func ClampVolume(v float64) float64 {
	if v != v {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func ClampOPLVolume(v, maxGain float64) float64 {
	if v != v {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > maxGain {
		return maxGain
	}
	return v
}

func RestartMapForRespawn(current, pristine *mapdata.Map, singlePlayer bool) *mapdata.Map {
	if !singlePlayer {
		return current
	}
	return CloneMapForRestart(pristine)
}
