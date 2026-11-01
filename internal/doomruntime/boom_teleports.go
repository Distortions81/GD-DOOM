package doomruntime

import "gddoom/internal/mapdata"

// teleportLineGeometry retains the signed 16.16 linedef deltas used by Boom,
// including fractional split vertices and fixed_t wrapping.
func (g *game) teleportLineGeometry(index int) physLine {
	l := g.m.Linedefs[index]
	a, b := g.m.Vertexes[l.V1], g.m.Vertexes[l.V2]
	x1, y1 := boomFixed(int64(a.X*fracUnit)), boomFixed(int64(a.Y*fracUnit))
	x2, y2 := boomFixed(int64(b.X*fracUnit)), boomFixed(int64(b.Y*fracUnit))
	return physLine{x1: x1, y1: y1, x2: x2, y2: y2, dx: boomFixed(x2 - x1), dy: boomFixed(y2 - y1)}
}

func (g *game) activateSilentLineTeleport(lineIdx int, info mapdata.TeleportInfo, actorIdx int, isPlayer bool) bool {
	source := g.teleportLineGeometry(lineIdx)
	actorX, actorY, actorZ, actorFloor := g.p.x, g.p.y, g.p.z, g.p.floorz
	if !isPlayer {
		th := g.m.Things[actorIdx]
		actorX, actorY = g.thingPosFixed(actorIdx, th)
		actorZ, actorFloor, _ = g.thingSupportState(actorIdx, th)
	}
	for _, i := range g.silentLineTeleportExits(g.m.Linedefs[lineIdx].Tag) {
		line := g.m.Linedefs[i]
		if i == lineIdx {
			continue
		}
		front, okFront := g.sectorIndexFromSidedef(line.SideNum[0])
		back, okBack := g.sectorIndexFromSidedef(line.SideNum[1])
		if !okFront || !okBack {
			continue
		}
		exit := g.teleportLineGeometry(i)
		var pos int64
		if abs(source.dx) > abs(source.dy) {
			pos = fixedDiv(boomFixed(actorX-source.x1), source.dx)
		} else {
			pos = fixedDiv(boomFixed(actorY-source.y1), source.dy)
		}
		rotation := uint32(doomAng180)
		if info.Reverse {
			pos = boomFixed(fracUnit - pos)
			rotation = 0
		}
		rotation += doomPointToAngle2(0, 0, exit.dx, exit.dy) - doomPointToAngle2(0, 0, source.dx, source.dy)
		x, y := boomFixed(exit.x2-fixedMul(pos, exit.dx)), boomFixed(exit.y2-fixedMul(pos, exit.dy))
		stepdown := g.sectorFloor[front] < g.sectorFloor[back]
		exitSide := 0
		if info.Reverse || isPlayer && stepdown {
			exitSide = 1
		}
		// EV_SilentLineTeleport's FUDGEFACTOR is ten fixed_t units, not ten
		// map units. Preserve its axis choice and decrement-before-test ordering.
		for fudge := 10; pointOnLineSide(x, y, exit.x1, exit.y1, exit.dx, exit.dy) != exitSide; {
			fudge--
			if fudge < 0 {
				break
			}
			if abs(exit.dx) > abs(exit.dy) {
				if (exit.dx < 0) != (exitSide != 0) {
					y++
				} else {
					y--
				}
			} else {
				if (exit.dy < 0) != (exitSide != 0) {
					x--
				} else {
					x++
				}
			}
		}
		g.teleportMoveSerial++
		floor, ceil, ok := g.teleportDestinationHeights(x, y)
		if !ok {
			return false
		}
		radius := int64(playerRadius)
		if !isPlayer {
			radius = thingTypeRadius(g.m.Things[actorIdx].Type)
		}
		if !g.teleportStompDestinationThings(x, y, radius, actorIdx, isPlayer, actorX, actorY) {
			return false
		}
		highFloor := g.sectorFloor[front]
		if stepdown {
			highFloor = g.sectorFloor[back]
		}
		g.finishSilentTeleport(actorIdx, isPlayer, x, y, boomFixed(actorZ-actorFloor+highFloor), floor, ceil, rotation)
		return true
	}
	return false
}

// finishSilentTeleport changes neither vertical momentum nor reaction time.
// Like P_CalcHeight in Boom, view recalculation preserves step recovery speed.
func (g *game) finishSilentTeleport(actorIdx int, isPlayer bool, x, y, z, floor, ceil int64, rotation uint32) {
	c, s := doomFineCosine(rotation), doomFineSineAtAngle(rotation)
	rotate := func(mx, my int64) (int64, int64) {
		return boomFixed(fixedMul(mx, c) - fixedMul(my, s)), boomFixed(fixedMul(my, c) + fixedMul(mx, s))
	}
	if isPlayer {
		g.setPlayerPosFixed(x, y)
		g.p.z, g.p.floorz, g.p.ceilz = z, floor, ceil
		g.p.angle += rotation
		g.p.momx, g.p.momy = rotate(g.p.momx, g.p.momy)
		g.p.teleportedThisTic = true
		delta := g.p.deltaViewHeight
		g.p.deltaViewHeight = 0
		g.tickPlayerViewHeight()
		g.p.deltaViewHeight = delta
		if g.authorityRules != nil && g.localSlot >= 1 && g.localSlot <= 4 {
			g.authorityRules.Scores[g.localSlot].MovementEpoch++
		}
	} else {
		mx, my := rotate(g.thingMomX[actorIdx], g.thingMomY[actorIdx])
		mz := g.thingMomZ[actorIdx]
		g.setThingPosFixed(actorIdx, x, y)
		g.setThingSupportState(actorIdx, z, floor, ceil)
		g.setThingWorldAngle(actorIdx, g.thingWorldAngle(actorIdx, g.m.Things[actorIdx])+rotation)
		g.setThingMomentum(actorIdx, mx, my, mz)
		g.snapThingRenderState(actorIdx)
	}
}

// Boom disables vanilla ledge blocking for P_XYMovement's dropoff=true
// probes. AI P_Move still refuses ledges. Detect Boom map features for normal
// play, but retain the recorded demo's movement rules when playing a demo.
func boomMomentumDropoffEnabled(m *mapdata.Map, opts Options) bool {
	if opts.DemoScript != nil {
		v := opts.DemoScript.Header.Version
		return v == 201 || v == 202
	}
	for _, l := range m.Linedefs {
		if l.Special == 85 || l.Special >= 142 {
			return true
		}
	}
	for _, s := range m.Sectors {
		if s.Special >= 32 {
			return true
		}
	}
	return false
}

// Tagged exits retain map order. Resolve once per tag rather than scanning a
// large map on every actor crossing. These setup references rebuild on load.
func (g *game) silentLineTeleportExits(tag uint16) []int {
	if exits, ok := g.boomTeleportExits[tag]; ok {
		return exits
	}
	if g.boomTeleportExits == nil {
		g.boomTeleportExits = make(map[uint16][]int)
	}
	var exits []int
	for i, l := range g.m.Linedefs {
		if l.Tag != tag {
			continue
		}
		_, front := g.sectorIndexFromSidedef(l.SideNum[0])
		_, back := g.sectorIndexFromSidedef(l.SideNum[1])
		if front && back {
			exits = append(exits, i)
		}
	}
	g.boomTeleportExits[tag] = exits
	return exits
}
