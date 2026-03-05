package doomtex

import (
	"fmt"
	"sort"

	"gddoom/internal/wad"
)

type PatchRef struct {
	OriginX   int
	OriginY   int
	PatchName string
}

type TextureDef struct {
	Name    string
	Width   int
	Height  int
	Patches []PatchRef
}

type Set struct {
	wad         *wad.File
	palettes    [][256][3]uint8
	textures    map[string]TextureDef
	patchByName map[string]wad.Lump
	patchCache  map[string]*decodedPatch
}

type decodedPatch struct {
	width      int
	height     int
	leftOffset int
	topOffset  int
	index      []uint8
	opaque     []bool
}

func (s *Set) TextureNames() []string {
	out := make([]string, 0, len(s.textures))
	for n := range s.textures {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (s *Set) TextureCount() int { return len(s.textures) }

func (s *Set) PaletteCount() int { return len(s.palettes) }

func (s *Set) PaletteRGBA(palette int) ([]byte, error) {
	if palette < 0 || palette >= len(s.palettes) {
		return nil, parseErrorf("palette out of range: %d", palette)
	}
	out := make([]byte, 256*4)
	pal := s.palettes[palette]
	for i := 0; i < 256; i++ {
		j := i * 4
		out[j+0] = pal[i][0]
		out[j+1] = pal[i][1]
		out[j+2] = pal[i][2]
		out[j+3] = 0xFF
	}
	return out, nil
}

func (s *Set) Texture(name string) (TextureDef, bool) {
	t, ok := s.textures[normalizeName(name)]
	return t, ok
}

func normalizeName(name string) string {
	return wadName(name)
}

func wadName(name string) string {
	out := make([]byte, 0, 8)
	for i := 0; i < len(name) && len(out) < 8; i++ {
		c := name[i]
		if c == 0 {
			break
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

func parseErrorf(format string, args ...any) error {
	return fmt.Errorf("doomtex: "+format, args...)
}
