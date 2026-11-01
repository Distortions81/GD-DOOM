package doomtex

import (
	"gddoom/internal/wad"
	"strings"
)

// LoadBoomColormaps loads complete light tables only from Boom's C namespace.
// Later namespace entries override earlier ones, as with other WAD resources.
func LoadBoomColormaps(w *wad.File) map[string][]byte {
	maps := make(map[string][]byte)
	inside := false
	for _, lump := range w.Lumps {
		name := strings.ToUpper(lump.Name)
		switch name {
		case "C_START":
			inside = true
			continue
		case "C_END":
			inside = false
			continue
		}
		if !inside || name == "COLORMAP" {
			continue
		}
		// 32 light rows, the invulnerability row, and the reserved row.
		if lump.Size < 34*256 {
			delete(maps, name)
			continue
		}
		if data, err := w.LumpDataView(lump); err == nil {
			maps[name] = data[:34*256]
		}
	}
	return maps
}
