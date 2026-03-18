package doomtex

import "gddoom/internal/wad"

func LoadPaletteRGBA(f *wad.File, palette int) ([]byte, error) {
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
	out := make([]byte, 256*4)
	pal := palettes[palette]
	for i := 0; i < 256; i++ {
		j := i * 4
		out[j+0] = pal[i][0]
		out[j+1] = pal[i][1]
		out[j+2] = pal[i][2]
		out[j+3] = 0xFF
	}
	return out, nil
}
