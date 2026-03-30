package doomtex

import "gddoom/internal/wad"

type AnimDef struct {
	EndName   string
	StartName string
}

var DoomWallAnimDefs = []AnimDef{
	{EndName: "BLODGR4", StartName: "BLODGR1"},
	{EndName: "SLADRIP3", StartName: "SLADRIP1"},
	{EndName: "BLODRIP4", StartName: "BLODRIP1"},
	{EndName: "FIREWALL", StartName: "FIREWALA"},
	{EndName: "GSTFONT3", StartName: "GSTFONT1"},
	{EndName: "FIRELAVA", StartName: "FIRELAV3"},
	{EndName: "FIREMAG3", StartName: "FIREMAG1"},
	{EndName: "FIREBLU2", StartName: "FIREBLU1"},
	{EndName: "ROCKRED3", StartName: "ROCKRED1"},
	{EndName: "BFALL4", StartName: "BFALL1"},
	{EndName: "SFALL4", StartName: "SFALL1"},
	{EndName: "WFALL4", StartName: "WFALL1"},
	{EndName: "DBRAIN4", StartName: "DBRAIN1"},
}

var DoomFlatAnimDefs = []AnimDef{
	{EndName: "NUKAGE3", StartName: "NUKAGE1"},
	{EndName: "FWATER4", StartName: "FWATER1"},
	{EndName: "SWATER4", StartName: "SWATER1"},
	{EndName: "LAVA4", StartName: "LAVA1"},
	{EndName: "BLOOD3", StartName: "BLOOD1"},
	{EndName: "RROCK08", StartName: "RROCK05"},
	{EndName: "SLIME04", StartName: "SLIME01"},
	{EndName: "SLIME08", StartName: "SLIME05"},
	{EndName: "SLIME12", StartName: "SLIME09"},
}

func LoadWallTextureAnimSequences(set *Set, defs []AnimDef) map[string][]string {
	if set == nil {
		return nil
	}
	return buildAnimSequences(set.TextureNames(), defs)
}

func LoadFlatAnimSequences(f *wad.File, defs []AnimDef) map[string][]string {
	if f == nil {
		return nil
	}
	return buildAnimSequences(flatNamesInOrder(f), defs)
}

func buildAnimSequences(order []string, defs []AnimDef) map[string][]string {
	if len(order) == 0 || len(defs) == 0 {
		return nil
	}
	index := make(map[string]int, len(order))
	for i, name := range order {
		key := normalizeName(name)
		if key == "" {
			continue
		}
		if _, ok := index[key]; !ok {
			index[key] = i
		}
	}
	out := make(map[string][]string, len(defs)*4)
	for _, def := range defs {
		start := normalizeName(def.StartName)
		end := normalizeName(def.EndName)
		if start == "" || end == "" {
			continue
		}
		startIdx, okStart := index[start]
		endIdx, okEnd := index[end]
		if !okStart || !okEnd || endIdx < startIdx {
			continue
		}
		frames := append([]string(nil), order[startIdx:endIdx+1]...)
		if len(frames) < 2 {
			continue
		}
		for _, frame := range frames {
			out[normalizeName(frame)] = frames
		}
	}
	return out
}

func flatNamesInOrder(f *wad.File) []string {
	ranges := flatRanges(f.Lumps)
	if len(ranges) == 0 {
		return nil
	}
	out := make([]string, 0, 256)
	seen := make(map[string]struct{}, 256)
	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			l := f.Lumps[i]
			if l.Name == "" || l.Size != doomFlatSize {
				continue
			}
			if _, ok := seen[l.Name]; ok {
				continue
			}
			seen[l.Name] = struct{}{}
			out = append(out, l.Name)
		}
	}
	return out
}
