package doomruntime

import (
	"gddoom/internal/mapdata"
	"strings"
)

// GPU light tables must refresh even when two maps have the same row count.
var doomColormapGeneration uint64

type boomColormap struct {
	banks  [doomGammaLevels][]uint32
	rows   int
	shades []uint16
	raw    []byte
}

func (g *game) initBoomColormaps() {
	g.boomColormapSides = make(map[int]mapdata.Sidedef)
	g.boomColormapNames = make([][3]string, len(g.m.Sectors))
	for _, line := range g.m.Linedefs {
		if line.Special != 242 || line.SideNum[0] < 0 || int(line.SideNum[0]) >= len(g.m.Sidedefs) {
			continue
		}
		side := g.m.Sidedefs[line.SideNum[0]]
		if int(side.Sector) >= len(g.m.Sectors) {
			continue
		}
		names := [3]string{side.Bottom, side.Mid, side.Top}
		renderSide := side
		textures := [3]*string{&renderSide.Bottom, &renderSide.Mid, &renderSide.Top}
		for i, name := range names {
			name = strings.ToUpper(name)
			if name == "COLORMAP" || len(g.opts.BoomColorMaps[name]) >= 34*256 {
				*textures[i] = "-"
			}
			if name == "COLORMAP" || len(g.opts.BoomColorMaps[name]) < 34*256 {
				name = ""
			}
			names[i] = name
		}
		g.boomColormapNames[side.Sector] = names
		g.boomColormapSides[int(line.SideNum[0])] = renderSide
	}
	g.boomColormapCache = map[string]*boomColormap{"": {banks: doomColormapBanks, rows: doomColormapRows, shades: doomRowShadeMulLUT, raw: g.opts.DoomColorMap}}
	g.boomActiveColormap = ""
}

func (g *game) selectedBoomColormap(x, y, eye float64) string {
	control := g.heightControl(g.sectorAt(floorFixed(x), floorFixed(y)))
	if control < 0 || control >= len(g.boomColormapNames) {
		return ""
	}
	f, c, _ := g.sectorPhysicalRenderSnapshot(control)
	zone := 1
	if floorFixed(eye) < f {
		zone = 0
	} else if floorFixed(eye) > c {
		zone = 2
	}
	return g.boomColormapNames[control][zone]
}

func (g *game) activateBoomColormap(x, y, eye float64) {
	if len(g.boomColormapCache) == 0 || g.opts.DisableDoomLighting {
		return
	}
	name := g.selectedBoomColormap(x, y, eye)
	if name == g.boomActiveColormap {
		return
	}
	cm := g.boomColormapCache[name]
	if cm == nil {
		raw := g.opts.BoomColorMaps[name]
		if len(raw) < 34*256 || len(g.opts.DoomPaletteRGBA) < 1024 {
			return
		}
		cm = &boomColormap{raw: raw, rows: len(raw) / 256}
		cm.shades = buildDoomRowShadeMulLUT(g.opts.DoomPaletteRGBA, raw, cm.rows)
		for gamma := 0; gamma < doomGammaLevels; gamma++ {
			cm.banks[gamma] = make([]uint32, cm.rows*256)
			for i, index := range raw[:cm.rows*256] {
				p := g.opts.DoomPaletteRGBA[int(index)*4:]
				cm.banks[gamma][i] = packRGBA(doomGammaTables[gamma][p[0]], doomGammaTables[gamma][p[1]], doomGammaTables[gamma][p[2]])
			}
		}
		g.boomColormapCache[name] = cm
	}
	doomColormapBanks, doomColormapRows, doomRowShadeMulLUT = cm.banks, cm.rows, cm.shades
	doomColormapEnabled = name != "" || !g.opts.SourcePortMode
	g.boomActiveColormap = name
	doomColormapGeneration++
	applyActiveGammaLUTs()
}
func (g *game) renderColorMap() []byte {
	if cm := g.boomColormapCache[g.boomActiveColormap]; cm != nil {
		return cm.raw
	}
	return g.opts.DoomColorMap
}

// Colormap names on 242 setup sides are not wall textures. Unknown names
// retain their ordinary texture interpretation, as in P_LoadSideDefs2.
func (g *game) renderSidedef(index int) *mapdata.Sidedef {
	if side, ok := g.boomColormapSides[index]; ok {
		return &side
	}
	return &g.m.Sidedefs[index]
}
