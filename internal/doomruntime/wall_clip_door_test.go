package doomruntime

import (
	"math"
	"testing"

	"gddoom/internal/mapdata"
)

func TestPartialThickDoorOpeningSweep_DoesNotFullyCloseFloorColumns(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 0, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
				{Sector: 1},
				{Sector: 2},
			},
			Linedefs: []mapdata.Linedef{
				{Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
				{Flags: mlTwoSided, SideNum: [2]int16{2, 3}},
			},
			Vertexes: []mapdata.Vertex{
				{X: 64, Y: 16},
				{X: 64, Y: -16},
				{X: 80, Y: 16},
				{X: 80, Y: -16},
			},
			Segs: []mapdata.Seg{
				{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: 0},
				{StartVertex: 2, EndVertex: 3, Linedef: 1, Direction: 0},
			},
		},
		viewW:       320,
		viewH:       200,
		renderPX:    0,
		renderPY:    0,
		renderAngle: 0,
		p:           player{z: 0},
		sectorFloor: []int64{0, 0, 0},
		sectorCeil:  []int64{128 * fracUnit, 0, 128 * fracUnit},
	}
	g.buildWallSegStaticCache()

	step := int16(vDoorSpeed / fracUnit)
	if step <= 0 {
		t.Fatal("invalid door step")
	}
	for h := step; h < 128; h += step {
		g.m.Sectors[1].CeilingHeight = h
		g.sectorCeil[1] = int64(h) * fracUnit

		ceilingClip := make([]int, g.viewW)
		floorClip := make([]int, g.viewW)
		for i := range floorClip {
			ceilingClip[i] = -1
			floorClip[i] = g.viewH
		}

		minX, maxX, changed := applyDoorSegClipForTest(g, 0, ceilingClip, floorClip)
		minX2, maxX2, changed2 := applyDoorSegClipForTest(g, 1, ceilingClip, floorClip)
		if !changed && !changed2 {
			t.Fatalf("door height=%d: expected at least one visible door face", h)
		}
		if minX2 < minX {
			minX = minX2
		}
		if maxX2 > maxX {
			maxX = maxX2
		}
		if minX < 0 || maxX < minX {
			t.Fatalf("door height=%d: invalid visible range %d..%d", h, minX, maxX)
		}

		for x := minX; x <= maxX; x++ {
			if ceilingClip[x] >= g.viewH || floorClip[x] < 0 {
				t.Fatalf("door height=%d col=%d: partial thick door should not fully close clip column (ceil=%d floor=%d)", h, x, ceilingClip[x], floorClip[x])
			}
			if floorClip[x] != g.viewH {
				t.Fatalf("door height=%d col=%d: partial thick door should not advance floor clip (got %d want %d)", h, x, floorClip[x], g.viewH)
			}
		}
	}
}

func TestE1M1SpawnDoorOpeningSweep_DoorFaceColumnsStayPartiallyOpen(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.p.x = int64(math.Round(1519.95 * fracUnit))
	g.p.y = int64(math.Round(-2508.87 * fracUnit))
	g.p.angle = doomAngleFromDegrees(2.0)
	g.playerViewZ = 41 * fracUnit
	g.syncRenderState()
	g.prepareRenderState()

	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 {
		t.Fatalf("expected spawn door use target, got line=%d trace=%v", lineIdx, tr)
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		t.Fatalf("target line %d is not a door special", lineIdx)
	}
	if !g.activateDoorLine(lineIdx, info, true) {
		t.Fatalf("failed to activate spawn door line %d", lineIdx)
	}

	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		t.Fatalf("door target sectors for line %d: %v", lineIdx, err)
	}
	doorSec := targets[0]
	if doorSec < 0 || doorSec >= len(g.m.Sectors) {
		t.Fatalf("invalid door sector %d", doorSec)
	}

	faceSegs := e1m1DoorFaceSegs(g, doorSec)
	if len(faceSegs) == 0 {
		t.Fatal("expected visible two-sided door face segs for spawn door")
	}

	sawPartial := false
	for tick := 0; tick < 128; tick++ {
		g.tickDoors()
		d := g.doors[doorSec]
		if d == nil {
			break
		}
		ceil := g.sectorCeil[doorSec]
		top := int64(d.topHeight)
		if ceil <= g.sectorFloor[doorSec] || ceil >= top {
			if ceil >= top {
				break
			}
			continue
		}
		sawPartial = true

		ceilingClip := make([]int, g.viewW)
		floorClip := make([]int, g.viewW)
		for i := range floorClip {
			ceilingClip[i] = -1
			floorClip[i] = g.viewH
		}

		changedAny := false
		minX := g.viewW
		maxX := -1
		for _, segIdx := range faceSegs {
			l, r, changed := applyDoorSegClipForTest(g, segIdx, ceilingClip, floorClip)
			if !changed {
				continue
			}
			changedAny = true
			if l < minX {
				minX = l
			}
			if r > maxX {
				maxX = r
			}
		}
		if !changedAny {
			continue
		}
		for x := minX; x <= maxX; x++ {
			if ceilingClip[x] >= g.viewH || floorClip[x] < 0 {
				t.Fatalf("tick=%d ceil=%d col=%d: partial real-map door face fully closed clip column (ceilClip=%d floorClip=%d)", tick, ceil/fracUnit, x, ceilingClip[x], floorClip[x])
			}
			if floorClip[x] != g.viewH {
				t.Fatalf("tick=%d ceil=%d col=%d: partial real-map door face advanced floor clip (got %d want %d)", tick, ceil/fracUnit, x, floorClip[x], g.viewH)
			}
		}
	}
	if !sawPartial {
		t.Fatal("expected to observe a partial-open door state")
	}
}

func TestE1M1SpawnDoorOpeningSweep_BackSectorFloorVisibleBeforeHalfOpen(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.p.x = int64(math.Round(1519.95 * fracUnit))
	g.p.y = int64(math.Round(-2508.87 * fracUnit))
	g.p.angle = doomAngleFromDegrees(2.0)
	g.playerViewZ = 41 * fracUnit
	g.syncRenderState()
	g.prepareRenderState()

	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 {
		t.Fatalf("expected spawn door use target, got line=%d trace=%v", lineIdx, tr)
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		t.Fatalf("target line %d is not a door special", lineIdx)
	}
	if !g.activateDoorLine(lineIdx, info, true) {
		t.Fatalf("failed to activate spawn door line %d", lineIdx)
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		t.Fatalf("door target sectors for line %d: %v", lineIdx, err)
	}
	doorSec := targets[0]
	playerSec := g.sectorAt(g.p.x, g.p.y)
	backSec := e1m1DoorBackSector(g, doorSec, playerSec)
	if backSec < 0 {
		t.Fatalf("failed to find back sector behind door=%d player=%d", doorSec, playerSec)
	}

	seenEarlyFloor := false
	for tick := 0; tick < 128; tick++ {
		g.tickDoors()
		d := g.doors[doorSec]
		if d == nil {
			break
		}
		ceil := g.sectorCeil[doorSec]
		top := int64(d.topHeight)
		if ceil <= g.sectorFloor[doorSec] || ceil >= top {
			if ceil >= top {
				break
			}
			continue
		}
		doorCols, ok := e1m1DoorColumns(g, doorSec)
		if !ok || doorCols.r < doorCols.l {
			continue
		}
		planes := collectBasic3DPlanesForTest(g)
		samples := countFloorPlaneSamplesForSectorInColumns(g, planes, backSec, doorCols.l, doorCols.r)
		if ceil/fracUnit < 64 && samples > 0 {
			seenEarlyFloor = true
			break
		}
	}
	if !seenEarlyFloor {
		t.Fatal("expected floor in the sector behind the spawn door to become visible before the door is half open")
	}
}

func e1m1DoorFaceSegs(g *game, doorSec int) []int {
	visible := g.visibleSegIndicesPseudo3D()
	out := make([]int, 0, 8)
	for _, segIdx := range visible {
		frontIdx, backIdx := g.segSectorIndices(segIdx)
		if frontIdx < 0 || backIdx < 0 {
			continue
		}
		if frontIdx != doorSec && backIdx != doorSec {
			continue
		}
		other := backIdx
		if frontIdx == doorSec {
			other = backIdx
		} else {
			other = frontIdx
		}
		if other < 0 || other >= len(g.m.Sectors) {
			continue
		}
		door := g.m.Sectors[doorSec]
		adj := g.m.Sectors[other]
		if door.FloorHeight != adj.FloorHeight {
			continue
		}
		if adj.CeilingHeight <= door.FloorHeight {
			continue
		}
		out = append(out, segIdx)
	}
	return out
}

func e1m1DoorBackSector(g *game, doorSec, playerSec int) int {
	for segIdx := range g.m.Segs {
		frontIdx, backIdx := g.segSectorIndices(segIdx)
		if frontIdx < 0 || backIdx < 0 {
			continue
		}
		if frontIdx != doorSec && backIdx != doorSec {
			continue
		}
		if frontIdx != doorSec && frontIdx != playerSec {
			return frontIdx
		}
		if backIdx != doorSec && backIdx != playerSec {
			return backIdx
		}
	}
	return -1
}

type columnRange struct {
	l int
	r int
}

func e1m1DoorColumns(g *game, doorSec int) (columnRange, bool) {
	faceSegs := e1m1DoorFaceSegs(g, doorSec)
	minX := g.viewW
	maxX := -1
	found := false
	for _, segIdx := range faceSegs {
		pp := g.buildWallSegPrepassSingle(segIdx, g.renderPX, g.renderPY, math.Cos(angleToRadians(g.renderAngle)), math.Sin(angleToRadians(g.renderAngle)), doomFocalLength(g.viewW), 2.0)
		if !pp.prepass.OK {
			continue
		}
		if pp.prepass.Projection.MinX < minX {
			minX = pp.prepass.Projection.MinX
		}
		if pp.prepass.Projection.MaxX > maxX {
			maxX = pp.prepass.Projection.MaxX
		}
		found = true
	}
	return columnRange{l: minX, r: maxX}, found
}

func doomAngleFromDegrees(deg float64) uint32 {
	turns := deg / 360.0
	if turns < 0 {
		turns = math.Mod(turns, 1.0) + 1.0
	}
	return uint32(math.Round(turns * 4294967296.0))
}

func collectBasic3DPlanesForTest(g *game) []*plane3DVisplane {
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	focal := doomFocalLength(g.viewW)
	near := 2.0

	_, _, ceilingClip, floorClip := g.ensure3DFrameBuffers()
	planeOrder := g.beginPlane3DFrame(g.viewW)
	solid := g.beginSolid3DFrame()
	prepass := g.buildWallSegPrepassParallel(g.visibleSegIndicesPseudo3D(), camX, camY, ca, sa, focal, near)

	for _, pp := range prepass {
		if !pp.prepass.OK || solidFullyCoveredFast(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX) {
			continue
		}
		front, back := g.segSectors(pp.segIdx)
		if front == nil {
			continue
		}
		frontIdx, backIdx := g.segSectorIndices(pp.segIdx)
		frontFloor := float64(front.FloorHeight)
		frontCeil := float64(front.CeilingHeight)
		if fz, cz, ok := g.sectorHeightRenderSnapshot(frontIdx); ok {
			frontFloor = float64(fz) / fracUnit
			frontCeil = float64(cz) / fracUnit
		}
		backFloor := 0.0
		backCeil := 0.0
		if back != nil {
			backFloor = float64(back.FloorHeight)
			backCeil = float64(back.CeilingHeight)
			if fz, cz, ok := g.sectorHeightRenderSnapshot(backIdx); ok {
				backFloor = float64(fz) / fracUnit
				backCeil = float64(cz) / fracUnit
			}
		}
		ws := classifyWallPortal(front, back, eyeZ, frontFloor, frontCeil, backFloor, backCeil)

		floorPlane, created := g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, true), pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, g.viewW)
		if created && floorPlane != nil {
			planeOrder = append(planeOrder, floorPlane)
		}
		ceilPlane, created := g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, false), pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, g.viewW)
		if created && ceilPlane != nil {
			planeOrder = append(planeOrder, ceilPlane)
		}

		visibleRanges := clipRangeAgainstSolidSpans(pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, solid, g.solidClipScratch[:0])
		g.solidClipScratch = visibleRanges
		if len(visibleRanges) == 0 {
			continue
		}
		for _, vis := range visibleRanges {
			for x := vis.L; x <= vis.R; x++ {
				t := (float64(x) - pp.prepass.Projection.SX1) / (pp.prepass.Projection.SX2 - pp.prepass.Projection.SX1)
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				invF := pp.prepass.Projection.InvDepth1 + (pp.prepass.Projection.InvDepth2-pp.prepass.Projection.InvDepth1)*t
				if invF <= 0 {
					continue
				}
				f := 1.0 / invF
				if f <= 0 {
					continue
				}
				yl := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldTop/f)*focal))
				if yl < ceilingClip[x]+1 {
					yl = ceilingClip[x] + 1
				}
				if ws.MarkCeiling && ceilPlane != nil {
					top := ceilingClip[x] + 1
					bottom := yl - 1
					if bottom >= floorClip[x] {
						bottom = floorClip[x] - 1
					}
					markPlane3DColumnRange(ceilPlane, x, top, bottom, ceilingClip, floorClip)
				}
				yh := int(math.Floor(float64(g.viewH)/2 - (ws.WorldBottom/f)*focal))
				if yh >= floorClip[x] {
					yh = floorClip[x] - 1
				}
				if ws.MarkFloor && floorPlane != nil {
					top := yh + 1
					bottom := floorClip[x] - 1
					if top <= ceilingClip[x] {
						top = ceilingClip[x] + 1
					}
					markPlane3DColumnRange(floorPlane, x, top, bottom, ceilingClip, floorClip)
				}
				if ws.SolidWall {
					ceilingClip[x] = g.viewH
					floorClip[x] = -1
					continue
				}
				if ws.TopWall {
					mid := int(math.Floor(float64(g.viewH)/2 - (ws.WorldHigh/f)*focal))
					if mid >= floorClip[x] {
						mid = floorClip[x] - 1
					}
					if mid >= yl {
						ceilingClip[x] = mid
					} else {
						ceilingClip[x] = yl - 1
					}
				} else if ws.MarkCeiling {
					ceilingClip[x] = yl - 1
				}
				if ws.BottomWall {
					mid := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldLow/f)*focal))
					if mid <= ceilingClip[x] {
						mid = ceilingClip[x] + 1
					}
					if mid <= yh {
						floorClip[x] = mid
					} else {
						floorClip[x] = yh + 1
					}
				} else if ws.MarkFloor {
					floorClip[x] = yh + 1
				}
			}
		}
		if ws.SolidWall {
			solid = addSolidSpan(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX)
		}
	}
	return planeOrder
}

func countFloorPlaneSamplesForSectorInColumns(g *game, planes []*plane3DVisplane, targetSec, minX, maxX int) int {
	spansByPlane, _, _, _ := g.buildPlaneSpansParallel(planes, g.viewH)
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	focal := doomFocalLength(g.viewW)
	cx := float64(g.viewW) * 0.5
	cy := float64(g.viewH) * 0.5

	count := 0
	for planeIdx, pl := range planes {
		if pl == nil || !pl.key.floor {
			continue
		}
		for _, sp := range spansByPlane[planeIdx] {
			if sp.y < 0 || sp.y >= g.viewH || float64(sp.y) <= cy {
				continue
			}
			x1 := sp.x1
			if x1 < minX {
				x1 = minX
			}
			x2 := sp.x2
			if x2 > maxX {
				x2 = maxX
			}
			if x2 < x1 {
				continue
			}
			den := cy - (float64(sp.y) + 0.5)
			if math.Abs(den) < 1e-6 {
				continue
			}
			planeZ := float64(pl.key.height)
			depth := ((planeZ - eyeZ) / den) * focal
			if depth <= 0 {
				continue
			}
			stepWX := (depth / focal) * sa
			stepWY := -(depth / focal) * ca
			rowBaseWX := camX + depth*ca - ((cx-0.5)*depth/focal)*sa
			rowBaseWY := camY + depth*sa + ((cx-0.5)*depth/focal)*ca
			wx := rowBaseWX + float64(x1)*stepWX
			wy := rowBaseWY + float64(x1)*stepWY
			for x := x1; x <= x2; x++ {
				if sec := g.sectorAt(int64(math.Round(wx*fracUnit)), int64(math.Round(wy*fracUnit))); sec == targetSec {
					count++
				}
				wx += stepWX
				wy += stepWY
			}
		}
	}
	return count
}

func applyDoorSegClipForTest(g *game, segIdx int, ceilingClip, floorClip []int) (int, int, bool) {
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	focal := doomFocalLength(g.viewW)
	near := 2.0
	eyeZ := g.playerEyeZ()

	pp := g.buildWallSegPrepassSingle(segIdx, camX, camY, ca, sa, focal, near)
	if !pp.prepass.OK {
		return -1, -1, false
	}
	front, back := g.segSectors(segIdx)
	if front == nil {
		return -1, -1, false
	}
	frontIdx, backIdx := g.segSectorIndices(segIdx)
	frontFloor := float64(front.FloorHeight)
	frontCeil := float64(front.CeilingHeight)
	if fz, cz, ok := g.sectorHeightRenderSnapshot(frontIdx); ok {
		frontFloor = float64(fz) / fracUnit
		frontCeil = float64(cz) / fracUnit
	}
	backFloor := 0.0
	backCeil := 0.0
	if back != nil {
		backFloor = float64(back.FloorHeight)
		backCeil = float64(back.CeilingHeight)
		if fz, cz, ok := g.sectorHeightRenderSnapshot(backIdx); ok {
			backFloor = float64(fz) / fracUnit
			backCeil = float64(cz) / fracUnit
		}
	}
	ws := classifyWallPortal(front, back, eyeZ, frontFloor, frontCeil, backFloor, backCeil)

	for x := pp.prepass.Projection.MinX; x <= pp.prepass.Projection.MaxX; x++ {
		t := (float64(x) - pp.prepass.Projection.SX1) / (pp.prepass.Projection.SX2 - pp.prepass.Projection.SX1)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		invF := pp.prepass.Projection.InvDepth1 + (pp.prepass.Projection.InvDepth2-pp.prepass.Projection.InvDepth1)*t
		if invF <= 0 {
			continue
		}
		f := 1.0 / invF
		if f <= 0 {
			continue
		}

		yl := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldTop/f)*focal))
		if yl < ceilingClip[x]+1 {
			yl = ceilingClip[x] + 1
		}
		yh := int(math.Floor(float64(g.viewH)/2 - (ws.WorldBottom/f)*focal))
		if yh >= floorClip[x] {
			yh = floorClip[x] - 1
		}

		if ws.SolidWall {
			ceilingClip[x] = g.viewH
			floorClip[x] = -1
			continue
		}
		if ws.TopWall {
			mid := int(math.Floor(float64(g.viewH)/2 - (ws.WorldHigh/f)*focal))
			if mid >= floorClip[x] {
				mid = floorClip[x] - 1
			}
			if mid >= yl {
				ceilingClip[x] = mid
			} else {
				ceilingClip[x] = yl - 1
			}
		} else if ws.MarkCeiling {
			ceilingClip[x] = yl - 1
		}
		if ws.BottomWall {
			mid := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldLow/f)*focal))
			if mid <= ceilingClip[x] {
				mid = ceilingClip[x] + 1
			}
			if mid <= yh {
				floorClip[x] = mid
			} else {
				floorClip[x] = yh + 1
			}
		} else if ws.MarkFloor {
			floorClip[x] = yh + 1
		}
	}
	return pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, true
}

func TestMovingDoorTopWallRevealThresholdIsMonotonic(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{
				{FloorHeight: 0, CeilingHeight: 128, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
				{FloorHeight: 0, CeilingHeight: 0, FloorPic: "FLOOR0_1", CeilingPic: "CEIL1_1", Light: 160},
			},
			Sidedefs: []mapdata.Sidedef{
				{Sector: 0},
				{Sector: 1},
			},
			Linedefs: []mapdata.Linedef{
				{Flags: mlTwoSided, SideNum: [2]int16{0, 1}},
			},
			Vertexes: []mapdata.Vertex{
				{X: 64, Y: 24},
				{X: 64, Y: -24},
			},
			Segs: []mapdata.Seg{
				{StartVertex: 0, EndVertex: 1, Linedef: 0, Direction: 0},
			},
		},
		viewW:       320,
		viewH:       200,
		renderPX:    0,
		renderPY:    0,
		renderAngle: 0,
		p:           player{z: 0},
		sectorFloor: []int64{0, 0},
		sectorCeil:  []int64{128 * fracUnit, 0},
	}
	g.buildWallSegStaticCache()

	type sample struct {
		h        int16
		topWall  bool
		yl       int
		mid      int
		revealed bool
	}
	var samples []sample
	step := int16(vDoorSpeed / fracUnit)
	for h := step; h < 128; h += step {
		g.m.Sectors[1].CeilingHeight = h
		g.sectorCeil[1] = int64(h) * fracUnit

		pp := g.buildWallSegPrepassSingle(0, g.renderPX, g.renderPY, 1, 0, doomFocalLength(g.viewW), 2)
		if !pp.prepass.OK {
			t.Fatalf("door height=%d: prepass failed", h)
		}
		x := (pp.prepass.Projection.MinX + pp.prepass.Projection.MaxX) / 2
		tu := (float64(x) - pp.prepass.Projection.SX1) / (pp.prepass.Projection.SX2 - pp.prepass.Projection.SX1)
		if tu < 0 {
			tu = 0
		}
		if tu > 1 {
			tu = 1
		}
		invF := pp.prepass.Projection.InvDepth1 + (pp.prepass.Projection.InvDepth2-pp.prepass.Projection.InvDepth1)*tu
		if invF <= 0 {
			t.Fatalf("door height=%d: invalid invF", h)
		}
		f := 1.0 / invF
		front, back := g.segSectors(0)
		if front == nil || back == nil {
			t.Fatal("expected two-sided door seg")
		}
		ws := classifyWallPortal(front, back, 41, 0, 128, 0, float64(h))
		yl := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldTop/f)*doomFocalLength(g.viewW)))
		mid := int(math.Floor(float64(g.viewH)/2 - (ws.WorldHigh/f)*doomFocalLength(g.viewW)))
		samples = append(samples, sample{
			h:        h,
			topWall:  ws.TopWall,
			yl:       yl,
			mid:      mid,
			revealed: ws.TopWall && mid >= yl,
		})
	}

	seenReveal := false
	for i, s := range samples {
		if !s.revealed {
			continue
		}
		seenReveal = true
		for j := i + 1; j < len(samples); j++ {
			if !samples[j].revealed {
				t.Fatalf("reveal regressed after door height %d: revealed at %d (yl=%d mid=%d), hidden at %d (yl=%d mid=%d)",
					s.h, s.h, s.yl, s.mid, samples[j].h, samples[j].yl, samples[j].mid)
			}
		}
		break
	}
	if !seenReveal {
		t.Fatal("expected moving door upper-wall reveal to become visible during the opening sweep")
	}
}

func TestE1M1SpawnDoorTopWallRevealStartsBeforeHalfOpen(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.p.x = int64(math.Round(1519.95 * fracUnit))
	g.p.y = int64(math.Round(-2508.87 * fracUnit))
	g.p.angle = doomAngleFromDegrees(2.0)
	g.playerViewZ = 41 * fracUnit
	g.syncRenderState()
	g.prepareRenderState()

	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 {
		t.Fatalf("expected spawn door use target, got line=%d trace=%v", lineIdx, tr)
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		t.Fatalf("target line %d is not a door special", lineIdx)
	}
	if !g.activateDoorLine(lineIdx, info, true) {
		t.Fatalf("failed to activate spawn door line %d", lineIdx)
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		t.Fatalf("door target sectors for line %d: %v", lineIdx, err)
	}
	doorSec := targets[0]
	faceSegs := e1m1DoorFaceSegs(g, doorSec)
	if len(faceSegs) == 0 {
		t.Fatal("expected visible two-sided door face segs for spawn door")
	}

	seenEarlyReveal := false
	for tick := 0; tick < 128; tick++ {
		g.tickDoors()
		d := g.doors[doorSec]
		if d == nil {
			break
		}
		ceil := g.sectorCeil[doorSec]
		top := int64(d.topHeight)
		if ceil <= g.sectorFloor[doorSec] || ceil >= top {
			if ceil >= top {
				break
			}
			continue
		}

		anyReveal := false
		for _, segIdx := range faceSegs {
			pp := g.buildWallSegPrepassSingle(segIdx, g.renderPX, g.renderPY, math.Cos(angleToRadians(g.renderAngle)), math.Sin(angleToRadians(g.renderAngle)), doomFocalLength(g.viewW), 2.0)
			if !pp.prepass.OK {
				continue
			}
			x := (pp.prepass.Projection.MinX + pp.prepass.Projection.MaxX) / 2
			tu := (float64(x) - pp.prepass.Projection.SX1) / (pp.prepass.Projection.SX2 - pp.prepass.Projection.SX1)
			if tu < 0 {
				tu = 0
			}
			if tu > 1 {
				tu = 1
			}
			invF := pp.prepass.Projection.InvDepth1 + (pp.prepass.Projection.InvDepth2-pp.prepass.Projection.InvDepth1)*tu
			if invF <= 0 {
				continue
			}
			f := 1.0 / invF
			if f <= 0 {
				continue
			}
			front, back := g.segSectors(segIdx)
			if front == nil || back == nil {
				continue
			}
			frontIdx, backIdx := g.segSectorIndices(segIdx)
			frontFloor := float64(front.FloorHeight)
			frontCeil := float64(front.CeilingHeight)
			if fz, cz, ok := g.sectorHeightRenderSnapshot(frontIdx); ok {
				frontFloor = float64(fz) / fracUnit
				frontCeil = float64(cz) / fracUnit
			}
			backFloor := float64(back.FloorHeight)
			backCeil := float64(back.CeilingHeight)
			if fz, cz, ok := g.sectorHeightRenderSnapshot(backIdx); ok {
				backFloor = float64(fz) / fracUnit
				backCeil = float64(cz) / fracUnit
			}
			ws := classifyWallPortal(front, back, g.playerEyeZ(), frontFloor, frontCeil, backFloor, backCeil)
			if !ws.TopWall {
				continue
			}
			yl := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldTop/f)*doomFocalLength(g.viewW)))
			mid := int(math.Floor(float64(g.viewH)/2 - (ws.WorldHigh/f)*doomFocalLength(g.viewW)))
			if mid >= yl {
				anyReveal = true
				break
			}
		}
		if ceil/fracUnit < 64 && anyReveal {
			seenEarlyReveal = true
			break
		}
	}
	if !seenEarlyReveal {
		t.Fatal("expected E1M1 spawn door top-wall reveal to start before the door is half open")
	}
}

func TestE1M1SpawnDoorColumns_NotClosedByUnrelatedSegBeforeHalfOpen(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.p.x = int64(math.Round(1519.95 * fracUnit))
	g.p.y = int64(math.Round(-2508.87 * fracUnit))
	g.p.angle = doomAngleFromDegrees(2.0)
	g.playerViewZ = 41 * fracUnit
	g.syncRenderState()
	g.prepareRenderState()

	lineIdx, tr := g.peekUseTargetLine()
	if tr != useTraceSpecial || lineIdx < 0 {
		t.Fatalf("expected spawn door use target, got line=%d trace=%v", lineIdx, tr)
	}
	info := mapdata.LookupLineSpecial(g.lineSpecial[lineIdx])
	if info.Door == nil {
		t.Fatalf("target line %d is not a door special", lineIdx)
	}
	if !g.activateDoorLine(lineIdx, info, true) {
		t.Fatalf("failed to activate spawn door line %d", lineIdx)
	}
	targets, err := g.m.DoorTargetSectors(lineIdx)
	if err != nil || len(targets) == 0 {
		t.Fatalf("door target sectors for line %d: %v", lineIdx, err)
	}
	doorSec := targets[0]
	faceSegs := e1m1DoorFaceSegs(g, doorSec)
	if len(faceSegs) == 0 {
		t.Fatal("expected visible two-sided door face segs for spawn door")
	}
	faceSegSet := make(map[int]struct{}, len(faceSegs))
	for _, segIdx := range faceSegs {
		faceSegSet[segIdx] = struct{}{}
	}

	for tick := 0; tick < 128; tick++ {
		g.tickDoors()
		d := g.doors[doorSec]
		if d == nil {
			break
		}
		ceil := g.sectorCeil[doorSec]
		top := int64(d.topHeight)
		if ceil <= g.sectorFloor[doorSec] || ceil >= top {
			if ceil >= top {
				break
			}
			continue
		}
		if ceil/fracUnit >= 64 {
			continue
		}

		camX := g.renderPX
		camY := g.renderPY
		camAng := angleToRadians(g.renderAngle)
		ca := math.Cos(camAng)
		sa := math.Sin(camAng)
		eyeZ := g.playerEyeZ()
		focal := doomFocalLength(g.viewW)
		near := 2.0

		ceilingClip := make([]int, g.viewW)
		floorClip := make([]int, g.viewW)
		firstCeilSeg := make([]int, g.viewW)
		firstFloorSeg := make([]int, g.viewW)
		doorFaceCol := make([]bool, g.viewW)
		doorFaceClipCol := make([]bool, g.viewW)
		for i := range floorClip {
			ceilingClip[i] = -1
			floorClip[i] = g.viewH
			firstCeilSeg[i] = -1
			firstFloorSeg[i] = -1
		}

		solid := g.beginSolid3DFrame()
		prepass := g.buildWallSegPrepassParallel(g.visibleSegIndicesPseudo3D(), camX, camY, ca, sa, focal, near)
		for _, pp := range prepass {
			if !pp.prepass.OK || solidFullyCoveredFast(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX) {
				continue
			}
			for _, faceSeg := range faceSegs {
				if pp.segIdx != faceSeg {
					continue
				}
				for x := pp.prepass.Projection.MinX; x <= pp.prepass.Projection.MaxX; x++ {
					if x >= 0 && x < len(doorFaceCol) {
						doorFaceCol[x] = true
					}
				}
				break
			}
			front, back := g.segSectors(pp.segIdx)
			if front == nil {
				continue
			}
			frontIdx, backIdx := g.segSectorIndices(pp.segIdx)
			frontFloor := float64(front.FloorHeight)
			frontCeil := float64(front.CeilingHeight)
			if fz, cz, ok := g.sectorHeightRenderSnapshot(frontIdx); ok {
				frontFloor = float64(fz) / fracUnit
				frontCeil = float64(cz) / fracUnit
			}
			backFloor := 0.0
			backCeil := 0.0
			if back != nil {
				backFloor = float64(back.FloorHeight)
				backCeil = float64(back.CeilingHeight)
				if fz, cz, ok := g.sectorHeightRenderSnapshot(backIdx); ok {
					backFloor = float64(fz) / fracUnit
					backCeil = float64(cz) / fracUnit
				}
			}
			ws := classifyWallPortal(front, back, eyeZ, frontFloor, frontCeil, backFloor, backCeil)
			visibleRanges := clipRangeAgainstSolidSpans(pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX, solid, g.solidClipScratch[:0])
			g.solidClipScratch = visibleRanges
			for _, vis := range visibleRanges {
				for x := vis.L; x <= vis.R; x++ {
					if x < 0 || x >= len(doorFaceCol) || !doorFaceCol[x] {
						continue
					}
					_, isDoorFaceSeg := faceSegSet[pp.segIdx]
					prevCeil := ceilingClip[x]
					prevFloor := floorClip[x]
					tu := (float64(x) - pp.prepass.Projection.SX1) / (pp.prepass.Projection.SX2 - pp.prepass.Projection.SX1)
					if tu < 0 {
						tu = 0
					}
					if tu > 1 {
						tu = 1
					}
					invF := pp.prepass.Projection.InvDepth1 + (pp.prepass.Projection.InvDepth2-pp.prepass.Projection.InvDepth1)*tu
					if invF <= 0 {
						continue
					}
					f := 1.0 / invF
					if f <= 0 {
						continue
					}
					yl := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldTop/f)*focal))
					if yl < ceilingClip[x]+1 {
						yl = ceilingClip[x] + 1
					}
					yh := int(math.Floor(float64(g.viewH)/2 - (ws.WorldBottom/f)*focal))
					if yh >= floorClip[x] {
						yh = floorClip[x] - 1
					}
					if ws.SolidWall {
						ceilingClip[x] = g.viewH
						floorClip[x] = -1
					} else {
						if ws.TopWall {
							mid := int(math.Floor(float64(g.viewH)/2 - (ws.WorldHigh/f)*focal))
							if mid >= floorClip[x] {
								mid = floorClip[x] - 1
							}
							if mid >= yl {
								ceilingClip[x] = mid
							} else {
								ceilingClip[x] = yl - 1
							}
						} else if ws.MarkCeiling {
							ceilingClip[x] = yl - 1
						}
						if ws.BottomWall {
							mid := int(math.Ceil(float64(g.viewH)/2 - (ws.WorldLow/f)*focal))
							if mid <= ceilingClip[x] {
								mid = ceilingClip[x] + 1
							}
							if mid <= yh {
								floorClip[x] = mid
							} else {
								floorClip[x] = yh + 1
							}
						} else if ws.MarkFloor {
							floorClip[x] = yh + 1
						}
					}
					if ceilingClip[x] != prevCeil && firstCeilSeg[x] < 0 {
						firstCeilSeg[x] = pp.segIdx
					}
					if floorClip[x] != prevFloor && firstFloorSeg[x] < 0 {
						firstFloorSeg[x] = pp.segIdx
					}
					if isDoorFaceSeg && (ceilingClip[x] != prevCeil || floorClip[x] != prevFloor) {
						doorFaceClipCol[x] = true
					}
				}
			}
			if ws.SolidWall {
				solid = addSolidSpan(solid, pp.prepass.Projection.MinX, pp.prepass.Projection.MaxX)
			}
		}

		for x := 0; x < g.viewW; x++ {
			if !doorFaceCol[x] {
				continue
			}
			if !doorFaceClipCol[x] {
				continue
			}
			if ceilingClip[x] >= g.viewH || floorClip[x] < 0 {
				_, ceilFromFace := faceSegSet[firstCeilSeg[x]]
				_, floorFromFace := faceSegSet[firstFloorSeg[x]]
				if ceilFromFace || floorFromFace {
					continue
				}
				t.Fatalf("tick=%d ceil=%d col=%d: firstCeilSeg=%d firstFloorSeg=%d column fully closed before half-open",
					tick, ceil/fracUnit, x, firstCeilSeg[x], firstFloorSeg[x])
			}
		}
	}
}
