package doomtex

import (
	"encoding/binary"
	"fmt"

	"gddoom/internal/wad"
)

// BoomTables are immutable WAD resources. Non-nil empty tables explicitly
// disable the built-in definitions, as a terminator-only Boom lump does.
type BoomTables struct {
	Walls, Flats           map[string][]string
	WallSpeeds, FlatSpeeds map[string]int
	Switches               [][2]string
}

// LoadBoomTables resolves the effective (last) ANIMATED and SWITCHES lumps.
// Absent lumps retain Doom defaults. Missing start frames and switch textures
// are allowed for tables shared by different game episodes.
func LoadBoomTables(f *wad.File, set *Set) (BoomTables, error) {
	var out BoomTables
	if f == nil {
		return out, nil
	}
	walls, flats := DoomWallAnimDefs, DoomFlatAnimDefs
	if l, ok := f.LumpByName("ANIMATED"); ok {
		data, err := f.LumpDataView(l)
		if err != nil {
			return out, err
		}
		walls, flats, err = parseAnimated(data)
		if err != nil {
			return out, err
		}
	}
	_, customSwitches := f.LumpByName("SWITCHES")
	_, customAnimated := f.LumpByName("ANIMATED")
	if set == nil && (customSwitches || customAnimated) {
		var err error
		set, err = LoadFromWAD(f)
		if err != nil {
			return out, err
		}
	}
	var names []string
	if set != nil {
		names = set.TextureNames()
	}
	var err error
	out.Walls, out.WallSpeeds, err = resolveBoomAnims(names, walls)
	if err != nil {
		return out, err
	}
	out.Flats, out.FlatSpeeds, err = resolveBoomAnims(flatNamesInOrder(f), flats)
	if err != nil {
		return out, err
	}
	if l, ok := f.LumpByName("SWITCHES"); ok {
		data, err := f.LumpDataView(l)
		if err != nil {
			return out, err
		}
		episode := 1
		if _, ok := f.LumpByName("E3M1"); ok {
			episode = 2
		}
		if _, ok := f.LumpByName("MAP01"); ok {
			episode = 3
		}
		out.Switches, err = parseSwitches(data, names, episode)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func parseAnimated(data []byte) ([]AnimDef, []AnimDef, error) {
	walls, flats := []AnimDef{}, []AnimDef{}
	for pos := 0; ; pos += 23 {
		if pos < len(data) && data[pos] == 255 {
			return walls, flats, nil
		}
		if len(data)-pos < 23 {
			return nil, nil, fmt.Errorf("ANIMATED: missing terminator or truncated entry at %d", pos)
		}
		speed := int(int32(binary.LittleEndian.Uint32(data[pos+19 : pos+23])))
		def := AnimDef{EndName: normalizeName(string(data[pos+1 : pos+10])), StartName: normalizeName(string(data[pos+10 : pos+19])), Speed: speed}
		if data[pos] > 1 || speed <= 0 || def.StartName == "" || def.EndName == "" {
			return nil, nil, fmt.Errorf("ANIMATED: invalid entry at %d", pos)
		}
		if data[pos] == 1 {
			walls = append(walls, def)
		} else {
			flats = append(flats, def)
		}
	}
}

func resolveBoomAnims(names []string, defs []AnimDef) (map[string][]string, map[string]int, error) {
	seqs, speeds := map[string][]string{}, map[string]int{}
	index := map[string]int{}
	for i, n := range names {
		if _, ok := index[n]; !ok {
			index[n] = i
		}
	}
	for _, d := range defs {
		start, exists := index[d.StartName]
		if !exists {
			continue
		}
		end, exists := index[d.EndName]
		if !exists || end <= start {
			return nil, nil, fmt.Errorf("ANIMATED: invalid range %s to %s", d.StartName, d.EndName)
		}
		speed := d.Speed
		if speed == 0 {
			speed = 8
		}
		for name, cycle := range buildAnimSequences(names, []AnimDef{d}) {
			seqs[name] = cycle
			speeds[name] = speed
		}
	}
	return seqs, speeds, nil
}

func parseSwitches(data []byte, names []string, episode int) ([][2]string, error) {
	out := make([][2]string, 0)
	available := map[string]bool{}
	for _, n := range names {
		available[n] = true
	}
	for pos := 0; ; pos += 20 {
		if len(data)-pos < 20 {
			return nil, fmt.Errorf("SWITCHES: missing terminator or truncated entry at %d", pos)
		}
		ep := int(int16(binary.LittleEndian.Uint16(data[pos+18 : pos+20])))
		if ep == 0 {
			return out, nil
		}
		a, b := normalizeName(string(data[pos:pos+9])), normalizeName(string(data[pos+9:pos+18]))
		if ep < 0 || a == "" || b == "" {
			return nil, fmt.Errorf("SWITCHES: invalid entry at %d", pos)
		}
		if ep <= episode && available[a] && available[b] {
			out = append(out, [2]string{a, b})
		}
	}
}
