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

// LoadWallTextureAnimSequences returns per-name cycles phased by absolute WAD
// texture index, starting at the translation used by leveltime zero. Before the
// first special update the renderer must use identity translation instead.
func LoadWallTextureAnimSequences(set *Set, defs []AnimDef) map[string][]string {
	if set == nil {
		return nil
	}
	return buildAnimSequences(set.TextureNames(), defs)
}

// LoadFlatAnimSequences uses classic flat indices, including marker lumps.
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
		// P_UpdateSpecials uses the absolute texture/flat index, not an
		// index relative to this animation. Store each alias's cycle beginning
		// at its translated frame for leveltime zero.
		for i, frame := range frames {
			phase := (startIdx + i) % len(frames)
			cycle := append([]string(nil), frames[phase:]...)
			cycle = append(cycle, frames[:phase]...)
			out[normalizeName(frame)] = cycle
		}
	}
	return out
}

func flatNamesInOrder(f *wad.File) []string {
	ranges := flatRanges(f.Lumps)
	if len(ranges) == 0 {
		return nil
	}
	// R_InitFlats numbers every lump between F_START and F_END, including
	// nested F1/F2 markers. Filtering those markers changes animation phase.
	out := make([]string, 0, 256)
	for _, r := range ranges {
		for i := r[0]; i < r[1]; i++ {
			out = append(out, f.Lumps[i].Name)
		}
	}
	return out
}
