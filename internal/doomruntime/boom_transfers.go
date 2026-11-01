package doomruntime

import (
	"math"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/scene"
)

// These references are static map setup, rebuilt with the WAD on load/restart.
// Read light levels and sidedef offsets at render time so control-sector light
// thinkers, animation, and synchronized sidedef changes remain visible.
func (g *game) initBoomTransfers() {
	n := len(g.m.Sectors)
	g.boomFloorLight = make([]int, n)
	g.boomCeilingLight = make([]int, n)
	g.boomSkyLine = make([]int, n)
	for i := 0; i < n; i++ {
		g.boomFloorLight[i] = -1
		g.boomCeilingLight[i] = -1
	}
	for i, line := range g.m.Linedefs {
		if !mapdata.IsTransferSpecial(line.Special) {
			continue
		}
		control, ok := g.sectorIndexFromSidedef(line.SideNum[0])
		if !ok {
			continue
		}
		for sec, s := range g.m.Sectors {
			if uint16(s.Tag) != line.Tag {
				continue
			}
			switch line.Special {
			case 213:
				g.boomFloorLight[sec] = control
			case 261:
				g.boomCeilingLight[sec] = control
			case 271, 272:
				g.boomSkyLine[sec] = i + 1
			}
		}
	}
}
func (g *game) planeLightSource(sec int, floor bool) int {
	if g == nil || sec < 0 {
		return sec
	}
	refs := g.boomCeilingLight
	if floor {
		refs = g.boomFloorLight
	}
	if sec < len(refs) && refs[sec] >= 0 {
		return refs[sec]
	}
	return sec
}
func (g *game) planeLightForRender(sec int, s *mapdata.Sector, floor bool) int16 {
	source := g.planeLightSource(sec, floor)
	if g != nil && g.m != nil && source >= 0 && source < len(g.m.Sectors) {
		return g.sectorLightForRender(source, &g.m.Sectors[source])
	}
	return g.sectorLightForRender(sec, s)
}
func (g *game) skyLineForSector(sec int) int {
	if g != nil && sec >= 0 && sec < len(g.boomSkyLine) {
		return g.boomSkyLine[sec]
	}
	return 0
}
func (g *game) planeTransfersDiffer(a, b int, floor bool) bool {
	if g == nil || a < 0 || b < 0 {
		return false
	}
	refs := g.boomCeilingLight
	if floor {
		refs = g.boomFloorLight
	}
	ref := func(i int) int {
		if i < len(refs) {
			return refs[i]
		}
		return -1
	}
	return ref(a) != ref(b) || g.skyLineForSector(a) != g.skyLineForSector(b)
}

type boomSkyFrame struct {
	texture       *WallTexture
	columns, rows []int
	indexed       []byte
	rgba          []uint32
	gpu           gpuTexture
	ready         bool
}

// Keep sky identities in the visplane key: several transferred skies can be
// visible together, even when their heights, light levels and flats match.
func (g *game) prepareTransferredSkies(planes []*plane3DVisplane, camAngle, focal float64) map[int]*boomSkyFrame {
	frames := make(map[int]*boomSkyFrame)
	g.frameTransferSkies = g.frameTransferSkies[:0]
	for _, pl := range planes {
		lineID := pl.key.skyLine
		if !pl.key.sky || lineID == 0 || frames[lineID] != nil {
			continue
		}
		frame := g.transferredSkyFrame(lineID, camAngle, focal)
		if frame == nil {
			continue
		}
		if g.gpuFrame != nil {
			frame.gpu, frame.ready = g.gpuFrame.wallTexture(frame.texture)
		}
		frames[lineID] = frame
		g.frameTransferSkies = append(g.frameTransferSkies, frame)
	}
	return frames
}
func (g *game) transferredSkyFrame(lineID int, camAngle, focal float64) *boomSkyFrame {
	if lineID <= 0 || lineID > len(g.m.Linedefs) {
		return nil
	}
	line := g.m.Linedefs[lineID-1]
	side := line.SideNum[0]
	if side < 0 || int(side) >= len(g.m.Sidedefs) {
		return nil
	}
	s := g.m.Sidedefs[side]
	tex, ok := g.wallTexture(normalizeFlatName(s.Top))
	if !ok {
		return nil
	}
	// Boom skies use the declared texture height; the vanilla opaque profile
	// is always 128 rows and would truncate tall transferred skies.
	if tex.Width <= 0 || tex.Height <= 0 {
		return nil
	}
	frame := &boomSkyFrame{texture: tex, indexed: tex.Indexed, rgba: tex.RGBA32, columns: make([]int, g.viewW), rows: make([]int, g.viewH), ready: true}
	if len(frame.rgba) != tex.Width*tex.Height {
		frame.rgba = make([]uint32, tex.Width*tex.Height)
		for i := range frame.rgba {
			if i*4+3 < len(tex.RGBA) {
				frame.rgba[i] = packRGBA(tex.RGBA[i*4], tex.RGBA[i*4+1], tex.RGBA[i*4+2])
			}
		}
	}
	_, _, sampleW, sampleH := g.skyProjectionSize()
	// The horizontal offset is a signed 16.16 BAM offset, not map-unit degrees.
	angle := radiansToDoomAngle(camAngle) + uint32(int32(s.TextureOffset)*65536)
	for x := range frame.columns {
		sx := scene.ProjectedSampleIndex(x, g.viewW, sampleW)
		off := classicProjectionForWidth(sampleW).angles[sx]
		if g.opts.SourcePortMode {
			off = radiansToDoomAngle(math.Atan((float64(sampleW)*0.5 - (float64(sx) + 0.5)) / doomFocalLength(sampleW)))
		}
		col := angle + off
		if line.Special != 272 {
			col = ^col
		}
		frame.columns[x] = wrapIndex(int(col>>22), tex.Width)
	}
	iscale := int64(320*fracUnit) / int64(sampleW)
	mid := int64(s.RowOffset)*fracUnit - 28*fracUnit
	for y := range frame.rows {
		sy := scene.ProjectedSampleIndex(y, g.viewH, sampleH)
		frame.rows[y] = wrapIndex(int((mid+int64(sy-sampleH/2)*iscale)>>fracBits), tex.Height)
	}
	return frame
}
func (g *game) transferredSkyPixel(f *boomSkyFrame, i int) uint32 {
	if !g.opts.SourcePortMode && i < len(f.indexed) {
		return shadePaletteIndexPacked(f.indexed[i], 256)
	}
	return f.rgba[i]
}
