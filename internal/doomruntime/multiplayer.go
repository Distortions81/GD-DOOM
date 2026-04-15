package doomruntime

import (
	"math"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/scene"
)

const (
	// playerSpriteRadius is the Doom player collision radius in world units.
	playerSpriteRadius = 16 * fracUnit
	// playerWalkFrameTics is how many tics each walk animation frame lasts.
	playerWalkFrameTics = 8
)

// remotePlayer holds the simulated state for a peer co-op player.
type remotePlayer struct {
	slot int
	p    player
}

// spawnRemotePlayer creates a player struct for a remote peer at their map start.
func spawnRemotePlayer(m *mapdata.Map, slot int) player {
	starts := collectPlayerStarts(m)
	if s, ok := chooseSpawnStart(starts, slot); ok {
		return player{
			x: s.x, y: s.y, z: 0,
			floorz: 0, ceilz: 128 * fracUnit,
			subsector: -1, sector: -1,
			angle:      s.angle,
			viewHeight: playerViewHeight,
		}
	}
	b := mapBounds(m)
	return player{
		x:     int64(((b.minX + b.maxX) / 2) * fracUnit),
		y:     int64(((b.minY + b.maxY) / 2) * fracUnit),
		ceilz: 128 * fracUnit, subsector: -1, sector: -1,
		viewHeight: playerViewHeight,
	}
}

// stepRemotePlayer advances a remote player one tic using a received demo tic.
func (g *game) stepRemotePlayer(rp *remotePlayer, tc demo.Tic) {
	saved := g.p
	savedSlot := g.localSlot
	g.p = rp.p
	g.localSlot = rp.slot

	cmd, usePressed, fireHeld := demoTicCommand(DemoTic(tc))
	g.runGameplayTic(cmd, usePressed, fireHeld)

	rp.p = g.p
	g.p = saved
	g.localSlot = savedSlot
}

type playerStart struct {
	index int
	slot  int
	x     int64
	y     int64
	angle uint32
}

func collectPlayerStarts(m *mapdata.Map) []playerStart {
	starts := make([]playerStart, 0, 4)
	for i, t := range m.Things {
		slot := playerSlotFromThingType(t.Type)
		if slot == 0 {
			continue
		}
		starts = append(starts, playerStart{
			index: i,
			slot:  slot,
			x:     int64(t.X) << fracBits,
			y:     int64(t.Y) << fracBits,
			angle: thingDegToWorldAngle(t.Angle),
		})
	}
	return starts
}

func playerSlotFromThingType(typ int16) int {
	switch typ {
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 3
	case 4:
		return 4
	default:
		return 0
	}
}

func chooseSpawnStart(starts []playerStart, requestedSlot int) (playerStart, bool) {
	if requestedSlot >= 1 && requestedSlot <= 4 {
		for _, s := range starts {
			if s.slot == requestedSlot {
				return s, true
			}
		}
	}
	for _, s := range starts {
		if s.slot == 1 {
			return s, true
		}
	}
	if len(starts) > 0 {
		return starts[0], true
	}
	return playerStart{}, false
}

// remotePlayerSpriteName returns the PLAY walk frame name for a remote player
// given the current world tic and the rotation index (1-8) based on viewer angle.
func remotePlayerSpriteName(worldTic int, rot int) string {
	// Walk animation: frames A-D, 8 tics each.
	frame := byte('A' + (worldTic/playerWalkFrameTics)%4)
	// Prefer rotation-aware frame (e.g. PLAYA1), fall back to PLAYA0 (no rotation).
	return spriteFrameName("PLAY", frame, byte('0'+rot))
}

// appendRemotePlayerCutoutItems projects all remote players as PLAY-sprite
// billboards and appends them to the billboard queue.
func (g *game) appendRemotePlayerCutoutItems(camX, camY, camAng, focal, focalV, near float64) {
	if len(g.remotePlayers) == 0 {
		return
	}
	viewW := g.viewW
	viewH := g.viewH
	if len(g.wallPix32) != viewW*viewH {
		return
	}
	ca := math.Cos(camAng)
	sa := math.Sin(camAng)
	eyeZ := g.playerEyeZ()

	for _, rp := range g.remotePlayers {
		txFixed := rp.p.x
		tyFixed := rp.p.y
		tx := float64(txFixed)/fracUnit - camX
		ty := float64(tyFixed)/fracUnit - camY
		f := tx*ca + ty*sa
		s := -tx*sa + ty*ca
		if f <= near {
			continue
		}

		rot := monsterSpriteRotationIndexAt(rp.p.angle, float64(txFixed)/fracUnit, float64(tyFixed)/fracUnit, camX, camY)
		name := remotePlayerSpriteName(g.worldTic, rot)
		ref, ok := g.spriteRenderRef(name)
		if !ok {
			// Try no-rotation fallback PLAYA0.
			name = spriteFrameName("PLAY", byte('A'+(g.worldTic/playerWalkFrameTics)%4), '0')
			ref, ok = g.spriteRenderRef(name)
		}
		if !ok || ref == nil || ref.tex == nil || ref.tex.Height <= 0 || ref.tex.Width <= 0 {
			continue
		}

		clipTop, clipBottom, clipOK := g.spriteFootprintClipYBounds(txFixed, tyFixed, playerSpriteRadius, viewH, eyeZ, f, focalV)
		if !clipOK {
			continue
		}
		scale := focal / f
		scaleY := focalV / f
		if scale <= 0 {
			continue
		}
		clipBottom = spriteClipBottomWithPatchOverhang(clipBottom, ref.tex, scaleY, viewH)
		sx := float64(viewW)/2 - (s/f)*focal
		baseZ := float64(rp.p.z) / fracUnit
		sy := float64(viewH)/2 - ((baseZ-eyeZ)/f)*focalV
		w := float64(ref.tex.Width) * scale
		scaleY = g.spriteScaleYForAspect(ref.key, scale, scaleY)
		h := float64(ref.tex.Height) * scaleY
		dstX := sx - float64(ref.tex.OffsetX)*scale
		dstY := sy - float64(ref.tex.OffsetY)*scaleY
		x0, x1, y0, y1, boundsOK := scene.ClampedSpriteBounds(dstX, dstY, w, h, clipTop, clipBottom, viewW, viewH)
		if !boundsOK || h <= 0 || w <= 0 {
			continue
		}
		xPad := w/2 + 8
		yPad := h + 4
		if sx+xPad < 0 || sx-xPad > float64(viewW) || sy+yPad < 0 || sy-yPad > float64(viewH) {
			continue
		}
		sec := g.sectorAt(txFixed, tyFixed)
		lightMul := uint32(256)
		if sec >= 0 && sec < len(g.m.Sectors) {
			lightMul = g.sectorLightMulCached(sec)
		}
		depthQ := encodeDepthQ(f)
		shadeMul := g.cachedThingShadeMul(-1, false, lightMul, f, near)
		opaqueRectStart, opaqueRectCount := g.appendProjectedOpaqueRects(ref.opaque.rects, ref.tex.Width, false, dstX, dstY, scale, scaleY, clipTop, clipBottom, viewW, viewH)
		if opaqueRectCount > 0 && g.projectedOpaqueRectsFullyOccluded(g.projectedOpaqueRectScratch[opaqueRectStart:opaqueRectStart+opaqueRectCount], depthQ) {
			continue
		}
		if g.spriteWallClipQuadFullyOccluded(x0, x1, y0, y1, depthQ) {
			continue
		}
		g.billboardQueueScratch = append(g.billboardQueueScratch, cutoutItem{
			dist:            f,
			depthQ:          depthQ,
			kind:            billboardQueueMonsters,
			x0:              x0,
			x1:              x1,
			y0:              y0,
			y1:              y1,
			shadeMul:        shadeMul,
			tex:             ref.tex,
			flip:            false,
			shadow:          false,
			clipTop:         clipTop,
			clipBottom:      clipBottom,
			dstX:            dstX,
			dstY:            dstY,
			scale:           scale,
			scaleY:          scaleY,
			opaque:          ref.opaque,
			hasOpaque:       ref.hasOpaque,
			opaqueRectStart: opaqueRectStart,
			opaqueRectCount: opaqueRectCount,
			boundsOK:        true,
		})
	}
}

func nonLocalStarts(starts []playerStart, localSlot int) []playerStart {
	out := make([]playerStart, 0, len(starts))
	for _, s := range starts {
		if s.slot == localSlot {
			continue
		}
		out = append(out, s)
	}
	return out
}
