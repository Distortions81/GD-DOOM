package doomtex

import (
	"encoding/binary"

	"gddoom/internal/wad"
)

func LoadFromWAD(f *wad.File) (*Set, error) {
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

	pnamesLump, ok := f.LumpByName("PNAMES")
	if !ok {
		return nil, parseErrorf("missing PNAMES")
	}
	pnamesData, err := f.LumpData(pnamesLump)
	if err != nil {
		return nil, err
	}
	patchNames, err := parsePNames(pnamesData)
	if err != nil {
		return nil, err
	}

	textures := make(map[string]TextureDef)
	if l, ok := f.LumpByName("TEXTURE1"); ok {
		data, err := f.LumpData(l)
		if err != nil {
			return nil, err
		}
		list, err := parseTextureLump(data, patchNames)
		if err != nil {
			return nil, err
		}
		for _, td := range list {
			textures[td.Name] = td
		}
	}
	if l, ok := f.LumpByName("TEXTURE2"); ok {
		data, err := f.LumpData(l)
		if err != nil {
			return nil, err
		}
		list, err := parseTextureLump(data, patchNames)
		if err != nil {
			return nil, err
		}
		for _, td := range list {
			textures[td.Name] = td
		}
	}
	if len(textures) == 0 {
		return nil, parseErrorf("missing TEXTURE1/TEXTURE2 definitions")
	}

	patchByName := make(map[string]wad.Lump)
	for _, l := range f.Lumps {
		if l.Name == "" {
			continue
		}
		patchByName[l.Name] = l
	}

	return &Set{
		wad:         f,
		palettes:    palettes,
		textures:    textures,
		patchByName: patchByName,
		patchCache:  make(map[string]*decodedPatch),
	}, nil
}

func parsePlaypal(data []byte) ([][256][3]uint8, error) {
	const paletteSize = 256 * 3
	if len(data) < paletteSize {
		return nil, parseErrorf("PLAYPAL too short: %d", len(data))
	}
	count := len(data) / paletteSize
	out := make([][256][3]uint8, 0, count)
	for p := 0; p < count; p++ {
		ofs := p * paletteSize
		var pal [256][3]uint8
		for i := 0; i < 256; i++ {
			pal[i][0] = data[ofs+i*3+0]
			pal[i][1] = data[ofs+i*3+1]
			pal[i][2] = data[ofs+i*3+2]
		}
		out = append(out, pal)
	}
	return out, nil
}

func parsePNames(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, parseErrorf("PNAMES too short")
	}
	count := int(int32(binary.LittleEndian.Uint32(data[0:4])))
	if count < 0 {
		return nil, parseErrorf("PNAMES negative count")
	}
	need := 4 + count*8
	if need > len(data) {
		return nil, parseErrorf("PNAMES truncated: need=%d have=%d", need, len(data))
	}
	out := make([]string, count)
	for i := 0; i < count; i++ {
		start := 4 + i*8
		out[i] = parseName8(data[start : start+8])
	}
	return out, nil
}

func parseTextureLump(data []byte, patchNames []string) ([]TextureDef, error) {
	if len(data) < 4 {
		return nil, parseErrorf("TEXTURE lump too short")
	}
	count := int(int32(binary.LittleEndian.Uint32(data[0:4])))
	if count < 0 {
		return nil, parseErrorf("TEXTURE negative count")
	}
	if len(data) < 4+count*4 {
		return nil, parseErrorf("TEXTURE offset table truncated")
	}
	out := make([]TextureDef, 0, count)
	for i := 0; i < count; i++ {
		ofs := int(int32(binary.LittleEndian.Uint32(data[4+i*4 : 8+i*4])))
		if ofs < 0 || ofs+22 > len(data) {
			return nil, parseErrorf("TEXTURE[%d] invalid offset %d", i, ofs)
		}
		name := parseName8(data[ofs : ofs+8])
		width := int(int16(binary.LittleEndian.Uint16(data[ofs+12 : ofs+14])))
		height := int(int16(binary.LittleEndian.Uint16(data[ofs+14 : ofs+16])))
		patchCount := int(int16(binary.LittleEndian.Uint16(data[ofs+20 : ofs+22])))
		if width <= 0 || height <= 0 || patchCount < 0 {
			return nil, parseErrorf("TEXTURE[%s] invalid dimensions/patch count", name)
		}
		entrySize := 10
		need := ofs + 22 + patchCount*entrySize
		if need > len(data) {
			return nil, parseErrorf("TEXTURE[%s] patch table truncated", name)
		}
		refs := make([]PatchRef, 0, patchCount)
		for j := 0; j < patchCount; j++ {
			po := ofs + 22 + j*entrySize
			ox := int(int16(binary.LittleEndian.Uint16(data[po : po+2])))
			oy := int(int16(binary.LittleEndian.Uint16(data[po+2 : po+4])))
			pi := int(int16(binary.LittleEndian.Uint16(data[po+4 : po+6])))
			if pi < 0 || pi >= len(patchNames) {
				return nil, parseErrorf("TEXTURE[%s] patch index out of range: %d", name, pi)
			}
			refs = append(refs, PatchRef{OriginX: ox, OriginY: oy, PatchName: patchNames[pi]})
		}
		out = append(out, TextureDef{Name: name, Width: width, Height: height, Patches: refs})
	}
	return out, nil
}

func parseName8(b []byte) string {
	n := len(b)
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			n = i
			break
		}
	}
	for n > 0 && b[n-1] == ' ' {
		n--
	}
	if n == 0 {
		return ""
	}
	return wadName(string(b[:n]))
}
