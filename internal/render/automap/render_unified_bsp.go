package automap

import (
	"image/color"
	"math"
	"sort"

	"gddoom/internal/mapdata"

	"github.com/hajimehoshi/ebiten/v2"
)

type unifiedGatherContext struct {
	camX          float64
	camY          float64
	ca            float64
	sa            float64
	eyeZ          float64
	focal         float64
	near          float64
	tanHalfFOV    float64
	px            int64
	py            int64
	planesEnabled bool
	wallTop       []int
	wallBottom    []int
	maskedMids    []maskedMidSeg
	planeOrder    []*plane3DVisplane
	solid         []solidSpan
	ceilingClip   []int
	floorClip     []int
}

func (g *game) drawDoomUnifiedBSP3D(screen *ebiten.Image) {
	camX := g.renderPX
	camY := g.renderPY
	camAng := angleToRadians(g.renderAngle)
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()
	focal := doomFocalLength(g.viewW)
	near := 2.0
	tanHalfFOV := 1.0
	if focal > 0 {
		tanHalfFOV = (float64(g.viewW) * 0.5) / focal
	}
	g.beginSkyLayerFrame()

	ceilClr, floorClr := g.basicPlaneColors()
	g.ensureWallLayer()
	wallTop, wallBottom, ceilingClip, floorClip := g.ensure3DFrameBuffers()
	planesEnabled := len(g.opts.FlatBank) > 0
	planeOrder := g.beginPlane3DFrame(g.viewW)
	solid := g.beginSolid3DFrame()

	estMids := max(64, len(g.m.Segs)/2)
	ctx := unifiedGatherContext{
		camX:          camX,
		camY:          camY,
		ca:            ca,
		sa:            sa,
		eyeZ:          eyeZ,
		focal:         focal,
		near:          near,
		tanHalfFOV:    tanHalfFOV,
		px:            floatToFixed(camX),
		py:            floatToFixed(camY),
		planesEnabled: planesEnabled,
		wallTop:       wallTop,
		wallBottom:    wallBottom,
		maskedMids:    g.ensureMaskedMidSegScratch(estMids),
		planeOrder:    planeOrder,
		solid:         solid,
		ceilingClip:   ceilingClip,
		floorClip:     floorClip,
	}

	g.visibleEpoch++
	if g.visibleEpoch == 0 {
		g.visibleEpoch = 1
	}
	g.visibleBuf = g.visibleBuf[:0]
	g.beginUnifiedSubsectorSpanFrame()

	if len(g.m.Nodes) == 0 {
		if len(g.visibleSectorSeen) > 0 {
			for i := range g.visibleSectorSeen {
				g.visibleSectorSeen[i] = g.visibleEpoch
			}
		}
		if len(g.m.SubSectors) > 0 {
			for ss := range g.m.SubSectors {
				g.gatherUnifiedBSPSubsector(ss, &ctx)
			}
		} else {
			rootSolid := make([]solidSpan, 0, 64)
			for si := range g.m.Segs {
				g.gatherUnifiedBSPSeg(-1, si, &ctx, &rootSolid)
			}
			for _, sp := range rootSolid {
				ctx.solid = addSolidSpan(ctx.solid, sp.l, sp.r)
			}
		}
	} else {
		root := uint16(len(g.m.Nodes) - 1)
		g.traverseUnifiedBSP(root, &ctx)
	}

	g.maskedMidSegsScratch = ctx.maskedMids
	g.solid3DBuf = ctx.solid

	usedSkyLayer := false
	if planesEnabled && hasMarkedPlane3DData(ctx.planeOrder) {
		usedSkyLayer = g.drawDoomBasicTexturedPlanesVisplanePass(g.wallPix, camX, camY, ca, sa, eyeZ, focal, ceilClr, floorClr, ctx.planeOrder)
	}
	g.drawMaskedMidSegs(focal)
	if !g.depthOcclusionEnabled() {
		g.buildMaskedMidClipColumns(focal)
		g.billboardQueueCollect = true
		g.billboardQueueScratch = g.billboardQueueScratch[:0]
		g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
		g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
		g.billboardQueueCollect = false
		sort.Slice(g.billboardQueueScratch, func(i, j int) bool {
			return g.billboardQueueScratch[i].dist > g.billboardQueueScratch[j].dist
		})
		for _, qi := range g.billboardQueueScratch {
			g.billboardReplayActive = true
			g.billboardReplayKind = qi.kind
			g.billboardReplayIndex = qi.idx
			switch qi.kind {
			case billboardQueueProjectiles:
				g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
			case billboardQueueMonsters:
				g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
			case billboardQueueWorldThings:
				g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
			case billboardQueuePuffs:
				g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
			}
		}
		g.billboardReplayActive = false
		g.billboardQueueScratch = g.billboardQueueScratch[:0]
	} else {
		g.drawBillboardProjectilesToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardMonstersToBuffer(camX, camY, camAng, focal, near)
		g.drawBillboardWorldThingsToBuffer(camX, camY, camAng, focal, near)
		g.drawHitscanPuffsToBuffer(camX, camY, camAng, focal, near)
	}
	if g.opts.DepthBufferView && g.depthOcclusionEnabled() {
		g.drawDepthBufferView()
	}
	if g.lowDetailMode() {
		g.duplicateLowDetailColumns()
	}
	if usedSkyLayer {
		g.drawSkyLayerFrame(screen)
	}
	g.writePixelsTimed(g.wallLayer, g.wallPix)
	screen.DrawImage(g.wallLayer, nil)
}

func (g *game) traverseUnifiedBSP(child uint16, ctx *unifiedGatherContext) {
	if child&0x8000 != 0 {
		ss := int(child & 0x7fff)
		g.gatherUnifiedBSPSubsector(ss, ctx)
		return
	}
	ni := int(child)
	if ni < 0 || ni >= len(g.m.Nodes) {
		return
	}
	n := g.m.Nodes[ni]
	dl := divline{
		x:  int64(n.X) << fracBits,
		y:  int64(n.Y) << fracBits,
		dx: int64(n.DX) << fracBits,
		dy: int64(n.DY) << fracBits,
	}
	side := pointOnDivlineSide(ctx.px, ctx.py, dl)
	front := n.ChildID[side]
	back := n.ChildID[side^1]
	g.traverseUnifiedBSP(front, ctx)
	if !g.nodeChildBBoxMaybeVisible(n, side^1, ctx.px, ctx.py, ctx.ca, ctx.sa, ctx.near, ctx.tanHalfFOV) {
		return
	}
	if l, r, ok := g.nodeChildScreenRangeCached(ni, n, side^1, ctx.px, ctx.py, ctx.ca, ctx.sa, ctx.near, ctx.focal); ok && solidFullyCoveredFast(ctx.solid, l, r) {
		return
	}
	g.traverseUnifiedBSP(back, ctx)
}

func (g *game) gatherUnifiedBSPSubsector(ss int, ctx *unifiedGatherContext) {
	if ss < 0 || ss >= len(g.m.SubSectors) {
		return
	}
	if ss >= 0 && ss < len(g.visibleSubSectorSeen) && g.visibleSubSectorSeen[ss] == g.visibleEpoch {
		return
	}
	if ss >= 0 && ss < len(g.visibleSubSectorSeen) {
		g.visibleSubSectorSeen[ss] = g.visibleEpoch
	}
	if sec := g.sectorForSubSector(ss); sec >= 0 && sec < len(g.visibleSectorSeen) {
		g.visibleSectorSeen[sec] = g.visibleEpoch
	}
	sub := g.m.SubSectors[ss]
	// Keep solid-wall coverage local while traversing one subsector so adjacent
	// leaf edges do not clip each other by map seg order.
	subSolid := make([]solidSpan, 0, 8)
	for i := 0; i < int(sub.SegCount); i++ {
		si := int(sub.FirstSeg) + i
		g.visibleBuf = append(g.visibleBuf, si)
		g.gatherUnifiedBSPSeg(ss, si, ctx, &subSolid)
	}
	for _, sp := range subSolid {
		ctx.solid = addSolidSpan(ctx.solid, sp.l, sp.r)
	}
}

func (g *game) gatherUnifiedBSPSeg(ss, si int, ctx *unifiedGatherContext, subSolid *[]solidSpan) {
	pp := g.buildWallSegPrepassSingle(si, ctx.camX, ctx.camY, ctx.ca, ctx.sa, ctx.focal, ctx.near)
	if !pp.ok {
		if pp.logReason != "" {
			g.logWallCull(si, pp.logReason, pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
		}
		return
	}
	if solidFullyCoveredFast(ctx.solid, pp.minSX, pp.maxSX) {
		g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
		return
	}
	d := g.linedefDecisionPseudo3D(pp.ld)
	base, _ := g.decisionStyle(d)
	baseRGBA := color.RGBAModel.Convert(base).(color.RGBA)
	ld := pp.ld
	wallLightBias := doomWallLightBias(&ld, g.m.Vertexes)
	var frontSideDef *mapdata.Sidedef
	if pp.frontSideDefIdx >= 0 && pp.frontSideDefIdx < len(g.m.Sidedefs) {
		frontSideDef = &g.m.Sidedefs[pp.frontSideDefIdx]
	}
	front, back := g.segSectors(si)
	if front == nil {
		return
	}
	ws := classifyWallPortal(front, back, ctx.eyeZ)
	worldTop := ws.worldTop
	worldBottom := ws.worldBottom
	worldHigh := ws.worldHigh
	worldLow := ws.worldLow
	topWall := ws.topWall
	bottomWall := ws.bottomWall
	markCeiling := ws.markCeiling
	markFloor := ws.markFloor
	solidWall := ws.solidWall
	var midTex WallTexture
	var topTex WallTexture
	var botTex WallTexture
	hasMidTex := false
	hasTopTex := false
	hasBotTex := false
	midTexMid := 0.0
	topTexMid := 0.0
	botTexMid := 0.0
	texUOffset := wallSpecialScrollXOffset(ld.Special, g.worldTic)
	if frontSideDef != nil {
		texUOffset += float64(frontSideDef.TextureOffset)
		rowOffset := float64(frontSideDef.RowOffset)
		midTex, hasMidTex = g.wallTexture(frontSideDef.Mid)
		if hasMidTex {
			if (ld.Flags & mlDontPegBottom) != 0 {
				midTexMid = float64(front.FloorHeight) + float64(midTex.Height) - ctx.eyeZ
			} else {
				midTexMid = float64(front.CeilingHeight) - ctx.eyeZ
			}
			midTexMid += rowOffset
		}
		if topWall {
			topTex, hasTopTex = g.wallTexture(frontSideDef.Top)
			if hasTopTex {
				if (ld.Flags & mlDontPegTop) != 0 {
					topTexMid = float64(front.CeilingHeight) - ctx.eyeZ
				} else if back != nil {
					topTexMid = float64(back.CeilingHeight) + float64(topTex.Height) - ctx.eyeZ
				} else {
					topTexMid = float64(front.CeilingHeight) - ctx.eyeZ
				}
				topTexMid += rowOffset
			}
		}
		if bottomWall {
			botTex, hasBotTex = g.wallTexture(frontSideDef.Bottom)
			if hasBotTex {
				if (ld.Flags & mlDontPegBottom) != 0 {
					botTexMid = float64(front.CeilingHeight) - ctx.eyeZ
				} else if back != nil {
					botTexMid = float64(back.FloorHeight) - ctx.eyeZ
				} else {
					botTexMid = float64(front.FloorHeight) - ctx.eyeZ
				}
				botTexMid += rowOffset
			}
		}
	}

	var floorPlane *plane3DVisplane
	var ceilPlane *plane3DVisplane
	if ctx.planesEnabled {
		var created bool
		floorPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, true), pp.minSX, pp.maxSX, g.viewW)
		if created && floorPlane != nil {
			ctx.planeOrder = append(ctx.planeOrder, floorPlane)
		}
		ceilPlane, created = g.ensurePlane3DForRangeCached(g.plane3DKeyForSector(front, false), pp.minSX, pp.maxSX, g.viewW)
		if created && ceilPlane != nil {
			ctx.planeOrder = append(ctx.planeOrder, ceilPlane)
		}
	}

	visibleRanges := clipRangeAgainstSolidSpans(pp.minSX, pp.maxSX, ctx.solid, g.solidClipScratch[:0])
	g.solidClipScratch = visibleRanges
	if len(visibleRanges) == 0 {
		g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
		return
	}
	if !g.depthOcclusionEnabled() {
		allOcc := true
		for _, vis := range visibleRanges {
			visOcc := false
			if solidWall {
				visOcc = g.wallSliceRangeTriFullyOccludedByWallsOnly(pp, vis.l, vis.r, worldTop, worldBottom, ctx.focal)
			} else {
				topOcc := true
				botOcc := true
				hasSlice := false
				if topWall {
					hasSlice = true
					topOcc = g.wallSliceRangeTriFullyOccludedByWallsOnly(pp, vis.l, vis.r, worldTop, worldHigh, ctx.focal)
				}
				if bottomWall {
					hasSlice = true
					botOcc = g.wallSliceRangeTriFullyOccludedByWallsOnly(pp, vis.l, vis.r, worldLow, worldBottom, ctx.focal)
				}
				if hasSlice {
					visOcc = topOcc && botOcc
				}
			}
			if !visOcc {
				allOcc = false
				break
			}
		}
		if allOcc {
			g.logWallCull(si, "OCCLUDED", pp.logZ1, pp.logZ2, pp.logX1, pp.logX2)
			return
		}
	}
	g.markUnifiedSubsectorVisibleSpans(ss, visibleRanges)

	for _, vis := range visibleRanges {
		for x := vis.l; x <= vis.r; x++ {
			t := (float64(x) - pp.sx1) / (pp.sx2 - pp.sx1)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			invF := pp.invF1 + (pp.invF2-pp.invF1)*t
			if invF <= 0 {
				continue
			}
			f := 1.0 / invF
			if f <= 0 {
				continue
			}
			texU := (pp.uOverF1 + (pp.uOverF2-pp.uOverF1)*t) * f
			texU += texUOffset

			yl := int(math.Ceil(float64(g.viewH)/2 - (worldTop/f)*ctx.focal))
			if yl < ctx.ceilingClip[x]+1 {
				yl = ctx.ceilingClip[x] + 1
			}
			if markCeiling && ctx.planesEnabled && ceilPlane != nil {
				top := ctx.ceilingClip[x] + 1
				bottom := yl - 1
				if bottom >= ctx.floorClip[x] {
					bottom = ctx.floorClip[x] - 1
				}
				markPlane3DColumnRange(ceilPlane, x, top, bottom, ctx.ceilingClip, ctx.floorClip)
			}

			yh := int(math.Floor(float64(g.viewH)/2 - (worldBottom/f)*ctx.focal))
			if yh >= ctx.floorClip[x] {
				yh = ctx.floorClip[x] - 1
			}
			if markFloor && ctx.planesEnabled && floorPlane != nil {
				top := yh + 1
				bottom := ctx.floorClip[x] - 1
				if top <= ctx.ceilingClip[x] {
					top = ctx.ceilingClip[x] + 1
				}
				markPlane3DColumnRange(floorPlane, x, top, bottom, ctx.ceilingClip, ctx.floorClip)
			}
			if !solidWall {
				openTop := int(math.Ceil(float64(g.viewH)/2 - (worldHigh/f)*ctx.focal))
				openBottom := int(math.Floor(float64(g.viewH)/2 - (worldLow/f)*ctx.focal))
				if openTop < yl {
					openTop = yl
				}
				if openBottom > yh {
					openBottom = yh
				}
				g.appendSpritePortalColumnGap(x, openTop, openBottom, encodeDepthQ(f))
			}

			if solidWall {
				tex := midTex
				texMid := midTexMid
				useTex := hasMidTex
				if back != nil && !useTex {
					if topWall && hasTopTex {
						tex = topTex
						texMid = topTexMid
						useTex = true
					} else if bottomWall && hasBotTex {
						tex = botTex
						texMid = botTexMid
						useTex = true
					}
				}
				g.drawBasicWallColumn(ctx.wallTop, ctx.wallBottom, x, yl, yh, f, front.Light, wallLightBias, baseRGBA, texU, texMid, ctx.focal, tex, useTex)
				g.setWallDepthColumnClosedQ(x, encodeDepthQ(f))
				g.markSpriteClipColumnClosed(x, encodeDepthQ(f))
				ctx.ceilingClip[x] = g.viewH
				ctx.floorClip[x] = -1
				continue
			}

			if topWall {
				mid := int(math.Floor(float64(g.viewH)/2 - (worldHigh/f)*ctx.focal))
				if mid >= ctx.floorClip[x] {
					mid = ctx.floorClip[x] - 1
				}
				if mid >= yl {
					g.drawBasicWallColumn(ctx.wallTop, ctx.wallBottom, x, yl, mid, f, front.Light, wallLightBias, baseRGBA, texU, topTexMid, ctx.focal, topTex, hasTopTex)
					ctx.ceilingClip[x] = mid
				} else {
					ctx.ceilingClip[x] = yl - 1
				}
			} else if markCeiling {
				ctx.ceilingClip[x] = yl - 1
			}

			if bottomWall {
				mid := int(math.Ceil(float64(g.viewH)/2 - (worldLow/f)*ctx.focal))
				if mid <= ctx.ceilingClip[x] {
					mid = ctx.ceilingClip[x] + 1
				}
				if mid <= yh {
					g.drawBasicWallColumn(ctx.wallTop, ctx.wallBottom, x, mid, yh, f, front.Light, wallLightBias, baseRGBA, texU, botTexMid, ctx.focal, botTex, hasBotTex)
					ctx.floorClip[x] = mid
				} else {
					ctx.floorClip[x] = yh + 1
				}
			} else if markFloor {
				ctx.floorClip[x] = yh + 1
			}
		}
	}

	if back != nil && hasMidTex {
		for _, vis := range visibleRanges {
			if vis.l > vis.r {
				continue
			}
			dist := 0.0
			if pp.invF1+pp.invF2 > 0 {
				dist = 2.0 / (pp.invF1 + pp.invF2)
			}
			ctx.maskedMids = append(ctx.maskedMids, maskedMidSeg{
				dist:      dist,
				x0:        vis.l,
				x1:        vis.r,
				sx1:       pp.sx1,
				sx2:       pp.sx2,
				invF1:     pp.invF1,
				invF2:     pp.invF2,
				uOverF1:   pp.uOverF1,
				uOverF2:   pp.uOverF2,
				worldHigh: worldHigh,
				worldLow:  worldLow,
				texUOff:   texUOffset,
				texMid:    midTexMid,
				tex:       midTex,
				light:     front.Light,
				lightBias: wallLightBias,
			})
		}
	}

	if solidWall && subSolid != nil {
		out := *subSolid
		for _, vis := range visibleRanges {
			out = addSolidSpan(out, vis.l, vis.r)
		}
		*subSolid = out
	}
}

func (g *game) buildWallSegPrepassSingle(si int, camX, camY, ca, sa, focal, near float64) wallSegPrepass {
	pp := wallSegPrepass{
		segIdx:          si,
		frontSideDefIdx: -1,
	}
	cacheOK := si >= 0 && si < len(g.wallSegStaticCache) && g.wallSegStaticCache[si].valid
	var (
		ld                 mapdata.Linedef
		x1w, y1w, x2w, y2w float64
		u1, u2             float64
		hasTwoSidedMid     bool
		frontSectorIdx     = -1
		backSectorIdx      = -1
	)
	if cacheOK {
		c := g.wallSegStaticCache[si]
		ld = c.ld
		x1w, y1w, x2w, y2w = c.x1w, c.y1w, c.x2w, c.y2w
		pp.frontSideDefIdx = c.frontSideDefIdx
		u1 = c.uBase
		u2 = u1 + c.segLen
		if c.frontSide == 1 {
			u2 = u1 - c.segLen
		}
		hasTwoSidedMid = c.hasTwoSidedMidTex
		frontSectorIdx = c.frontSectorIdx
		backSectorIdx = c.backSectorIdx
	} else {
		if si < 0 || si >= len(g.m.Segs) {
			return pp
		}
		seg := g.m.Segs[si]
		li := int(seg.Linedef)
		if li < 0 || li >= len(g.m.Linedefs) {
			return pp
		}
		ld = g.m.Linedefs[li]
		var ok bool
		x1w, y1w, x2w, y2w, ok = g.segWorldEndpoints(si)
		if !ok {
			return pp
		}
		frontSide := int(seg.Direction)
		if frontSide < 0 || frontSide > 1 {
			frontSide = 0
		}
		backSide := frontSide ^ 1
		if sn := ld.SideNum[frontSide]; sn >= 0 && int(sn) < len(g.m.Sidedefs) {
			pp.frontSideDefIdx = int(sn)
		}
		segLen := math.Hypot(x2w-x1w, y2w-y1w)
		u1 = float64(seg.Offset)
		if pp.frontSideDefIdx >= 0 {
			u1 += float64(g.m.Sidedefs[pp.frontSideDefIdx].TextureOffset)
		}
		u2 = u1 + segLen
		if frontSide == 1 {
			u2 = u1 - segLen
		}
		hasTwoSidedMid = g.segHasTwoSidedMidTexture(si)
		frontSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[frontSide])
		backSectorIdx = g.sectorIndexFromSideNum(ld.SideNum[backSide])
	}
	pp.ld = ld
	d := g.linedefDecisionPseudo3D(ld)
	portalSplit := false
	if frontSectorIdx >= 0 && backSectorIdx >= 0 &&
		frontSectorIdx < len(g.m.Sectors) && backSectorIdx < len(g.m.Sectors) {
		front := &g.m.Sectors[frontSectorIdx]
		back := &g.m.Sectors[backSectorIdx]
		portalSplit = front.FloorHeight != back.FloorHeight ||
			front.CeilingHeight != back.CeilingHeight ||
			normalizeFlatName(front.FloorPic) != normalizeFlatName(back.FloorPic) ||
			normalizeFlatName(front.CeilingPic) != normalizeFlatName(back.CeilingPic) ||
			(front.Light != back.Light && doomSectorLighting)
	}
	if !d.visible && !hasTwoSidedMid && !portalSplit {
		return pp
	}
	x1 := x1w - camX
	y1 := y1w - camY
	x2 := x2w - camX
	y2 := y2w - camY
	f1 := x1*ca + y1*sa
	s1 := -x1*sa + y1*ca
	f2 := x2*ca + y2*sa
	s2 := -x2*sa + y2*ca
	origF1, origS1, origF2, origS2 := f1, s1, f2, s2
	preSX1 := float64(g.viewW) / 2
	preSX2 := float64(g.viewW) / 2
	if math.Abs(origF1) > 1e-9 {
		preSX1 -= (origS1 / origF1) * focal
	}
	if math.Abs(origF2) > 1e-9 {
		preSX2 -= (origS2 / origF2) * focal
	}
	var ok bool
	f1, s1, u1, f2, s2, u2, ok = clipSegmentToNearWithAttr(f1, s1, u1, f2, s2, u2, near)
	if !ok {
		pp.logReason = "BEHIND"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = origF1, origF2, preSX1, preSX2
		return pp
	}
	if f1*s2-s1*f2 >= 0 {
		pp.logReason = "BACKFACE"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, s1, s2
		return pp
	}
	sx1 := float64(g.viewW)/2 - (s1/f1)*focal
	sx2 := float64(g.viewW)/2 - (s2/f2)*focal
	if !isFinite(sx1) || !isFinite(sx2) {
		pp.logReason = "FLIPPED"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
		return pp
	}
	minSX := int(math.Floor(math.Min(sx1, sx2)))
	maxSX := int(math.Ceil(math.Max(sx1, sx2)))
	if minSX < 0 {
		minSX = 0
	}
	if maxSX >= g.viewW {
		maxSX = g.viewW - 1
	}
	if minSX > maxSX {
		pp.logReason = "OFFSCREEN"
		pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
		return pp
	}
	invF1 := 1.0 / f1
	invF2 := 1.0 / f2
	pp.sx1 = sx1
	pp.sx2 = sx2
	pp.minSX = minSX
	pp.maxSX = maxSX
	pp.invF1 = invF1
	pp.invF2 = invF2
	pp.uOverF1 = u1 * invF1
	pp.uOverF2 = u2 * invF2
	pp.logZ1, pp.logZ2, pp.logX1, pp.logX2 = f1, f2, sx1, sx2
	pp.ok = true
	return pp
}
