package doomruntime

import "gddoom/internal/mapdata"

// A frame-local R_FakeFlat view. Geometry remains 16.16, including the one
// fixed-unit separation at a water surface; mapdata.Sector retains int16
// heights for the physical map and must never be mutated for rendering.
type boomHeightView struct {
	sector                              mapdata.Sector
	floor, ceiling                      int64
	wallLight, floorLight, ceilingLight int16
	floorOffset, ceilingOffset          boomScrollVector
}

func (g *game) heightControl(sec int) int {
	if g != nil && sec >= 0 && sec < len(g.boomHeightControl) {
		return g.boomHeightControl[sec]
	}
	return -1
}
func (g *game) heightPair(a, b int) bool { return g.heightControl(a) >= 0 || g.heightControl(b) >= 0 }
func (g *game) heightView(sec int, back bool) *boomHeightView {
	if g == nil || !g.boomHeightFrameValid || g.heightControl(sec) < 0 {
		return nil
	}
	if back {
		return &g.boomHeightBack[sec]
	}
	return &g.boomHeightFront[sec]
}
func (g *game) renderSector(sec int, back bool) *mapdata.Sector {
	if g == nil || g.m == nil || sec < 0 || sec >= len(g.m.Sectors) {
		return nil
	}
	if v := g.heightView(sec, back); v != nil {
		return &v.sector
	}
	return &g.m.Sectors[sec]
}
func (g *game) renderViewForPointer(sec int, s *mapdata.Sector) *boomHeightView {
	if v := g.heightView(sec, false); v != nil && s == &v.sector {
		return v
	}
	if v := g.heightView(sec, true); v != nil && s == &v.sector {
		return v
	}
	return nil
}
func (g *game) sectorBackRenderHeights(sec int) (int64, int64, bool) {
	if v := g.heightView(sec, true); v != nil {
		return v.floor, v.ceiling, true
	}
	return g.sectorPhysicalRenderSnapshot(sec)
}
func (g *game) boomPlaneOffset(sec int, floor bool) boomScrollVector {
	vectors := g.boomCeilingScroll
	if floor {
		vectors = g.boomFloorScroll
	}
	if sec < 0 || sec >= len(vectors) {
		return boomScrollVector{}
	}
	v := vectors[sec]
	offsets := g.boomCeilingOffset
	if floor {
		offsets = g.boomFloorOffset
	}
	off := boomScrollVector{}
	if sec < len(offsets) {
		off = offsets[sec]
	}
	return boomScrollVector{x: boomFixed(v.x*int64(g.worldTic) + off.x), y: boomFixed(v.y*int64(g.worldTic) + off.y)}
}
func (g *game) prepareHeightTransfers(x, y, eyeZ float64) {
	g.boomHeightFrameValid = false
	if !g.boomHasHeightTransfers {
		return
	}
	n := len(g.m.Sectors)
	if len(g.boomHeightFront) != n {
		g.boomHeightFront = make([]boomHeightView, n)
		g.boomHeightBack = make([]boomHeightView, n)
	}
	cameraSector := g.sectorAt(floorFixed(x), floorFixed(y))
	cameraControl := g.heightControl(cameraSector)
	underwater, above := false, false
	if cameraControl >= 0 {
		f, c, _ := g.sectorPhysicalRenderSnapshot(cameraControl)
		z := floorFixed(eyeZ)
		underwater = z <= f
		above = z >= c
	}
	for sec := range g.m.Sectors {
		control := g.heightControl(sec)
		if control < 0 {
			continue
		}
		realFloor, realCeil, _ := g.sectorPhysicalRenderSnapshot(sec)
		controlFloor, controlCeil, _ := g.sectorPhysicalRenderSnapshot(control)
		s := &g.m.Sectors[sec]
		model := &g.m.Sectors[control]
		base := boomHeightView{sector: *s, floor: controlFloor, ceiling: controlCeil, wallLight: g.sectorLightForRender(sec, s), floorLight: g.planeLightForRender(sec, s, true), ceilingLight: g.planeLightForRender(sec, s, false), floorOffset: g.boomPlaneOffset(sec, true), ceilingOffset: g.boomPlaneOffset(sec, false)}
		for back := 0; back < 2; back++ {
			v := base
			// R_FakeFlat performs these assignments for the back sector too, but
			// only substitutes textures/light for a front sector under water.
			if underwater {
				v.floor = realFloor
				v.ceiling = controlFloor - 1
				if back == 0 {
					v.sector.FloorPic = model.FloorPic
					v.floorOffset = g.boomPlaneOffset(control, true)
					v.sector.CeilingPic = model.CeilingPic
					v.ceilingOffset = g.boomPlaneOffset(control, false)
					if isSkyFlatName(model.CeilingPic) {
						v.floor = v.ceiling + 1
						v.sector.CeilingPic = v.sector.FloorPic
						v.ceilingOffset = v.floorOffset
					}
					v.wallLight = g.sectorLightForRender(control, model)
					v.floorLight = g.planeLightForRender(control, model, true)
					v.ceilingLight = g.planeLightForRender(control, model, false)
				}
			}
			// In R_FakeFlat the underwater back-side assignment can fall
			// through to the above-ceiling case for an inverted model.
			if (!underwater || back != 0) && above && realCeil > controlCeil {
				v.ceiling = controlCeil
				v.floor = controlCeil + 1
				v.sector.FloorPic = model.CeilingPic
				v.sector.CeilingPic = model.CeilingPic
				v.floorOffset = g.boomPlaneOffset(control, false)
				v.ceilingOffset = v.floorOffset
				if !isSkyFlatName(model.FloorPic) {
					v.ceiling = realCeil
					v.sector.FloorPic = model.FloorPic
					v.floorOffset = g.boomPlaneOffset(control, true)
				}
				v.wallLight = g.sectorLightForRender(control, model)
				v.floorLight = g.planeLightForRender(control, model, true)
				v.ceilingLight = g.planeLightForRender(control, model, false)
			}
			v.sector.FloorHeight = int16(v.floor >> fracBits)
			v.sector.CeilingHeight = int16(v.ceiling >> fracBits)
			v.sector.Light = v.wallLight
			if back == 0 {
				g.boomHeightFront[sec] = v
			} else {
				g.boomHeightBack[sec] = v
			}
		}
	}
	g.boomHeightFrameValid = true
}

func (g *game) forceHeightPlane(sec int, floor bool) bool {
	control := g.heightControl(sec)
	if control < 0 || !g.boomHeightFrameValid {
		return false
	}
	if floor {
		return isSkyFlatName(g.m.Sectors[control].CeilingPic)
	}
	return isSkyFlatName(g.m.Sectors[control].FloorPic)
}
