package doomtex

import "gddoom/internal/wad"

const doomFlatSize = 64 * 64

func LoadFlatsIndexed(f *wad.File) (map[string][]byte, error) {
	if f == nil {
		return nil, parseErrorf("nil wad")
	}
	start, end, ok := flatRange(f.Lumps)
	if !ok {
		return nil, parseErrorf("flat range marker not found")
	}
	out := make(map[string][]byte)
	for i := start; i < end; i++ {
		l := f.Lumps[i]
		if l.Name == "" || l.Size != doomFlatSize {
			continue
		}
		data, err := f.LumpData(l)
		if err != nil || len(data) != doomFlatSize {
			continue
		}
		flat := make([]byte, doomFlatSize)
		copy(flat, data)
		out[l.Name] = flat
	}
	return out, nil
}

func LoadFlatsRGBA(f *wad.File, palette int) (map[string][]byte, error) {
	if f == nil {
		return nil, parseErrorf("nil wad")
	}
	playpal, ok := f.LumpByName("PLAYPAL")
	if !ok {
		return nil, parseErrorf("missing PLAYPAL")
	}
	playpalData, err := f.LumpData(playpal)
	if err != nil {
		return nil, err
	}
	palettes, err := parsePlaypal(playpalData)
	if err != nil {
		return nil, err
	}
	if palette < 0 || palette >= len(palettes) {
		return nil, parseErrorf("palette out of range: %d", palette)
	}
	pal := palettes[palette]

	start, end, ok := flatRange(f.Lumps)
	if !ok {
		return nil, parseErrorf("flat range marker not found")
	}
	out := make(map[string][]byte)
	for i := start; i < end; i++ {
		l := f.Lumps[i]
		if l.Name == "" || l.Size != doomFlatSize {
			continue
		}
		data, err := f.LumpData(l)
		if err != nil {
			continue
		}
		rgba := make([]byte, doomFlatSize*4)
		for p := 0; p < doomFlatSize; p++ {
			c := pal[data[p]]
			o := p * 4
			rgba[o+0] = c[0]
			rgba[o+1] = c[1]
			rgba[o+2] = c[2]
			rgba[o+3] = 0xFF
		}
		out[l.Name] = rgba
	}
	return out, nil
}

func flatRange(lumps []wad.Lump) (int, int, bool) {
	start := -1
	end := -1
	for i, l := range lumps {
		switch l.Name {
		case "F_START", "FF_START":
			if start < 0 {
				start = i + 1
			}
		case "F_END", "FF_END":
			if start >= 0 {
				end = i
				return start, end, true
			}
		}
	}
	return 0, 0, false
}
