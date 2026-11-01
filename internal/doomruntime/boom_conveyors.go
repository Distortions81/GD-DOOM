package doomruntime

type boomScrollVector struct{ x, y int64 }
type boomCarryThinker struct {
	order  int64
	sector int
	vector boomScrollVector
}

// Static scrollers have no mutable controller state. Texture phase derives
// from the saved world tic, while carry impulses run in thinker-list order.
func (g *game) initBoomScrollers() {
	g.boomCarries = nil
	g.boomFloorScroll = make([]boomScrollVector, len(g.m.Sectors))
	g.boomCeilingScroll = make([]boomScrollVector, len(g.m.Sectors))
	for _, line := range g.m.Linedefs {
		if line.Special < 250 || line.Special > 253 {
			continue
		}
		a, b := g.m.Vertexes[line.V1], g.m.Vertexes[line.V2]
		dx, dy := (int64(b.X)-int64(a.X))*fracUnit>>5, (int64(b.Y)-int64(a.Y))*fracUnit>>5
		targets := g.taggedSectorsForTag(line.Tag)
		for _, sec := range targets {
			if line.Special == 250 {
				g.boomCeilingScroll[sec].x -= dx
				g.boomCeilingScroll[sec].y += dy
			}
			if line.Special == 251 || line.Special == 253 {
				g.boomFloorScroll[sec].x -= dx
				g.boomFloorScroll[sec].y += dy
			}
		}
		if line.Special == 252 || line.Special == 253 {
			for _, sec := range targets {
				g.boomCarries = append(g.boomCarries, boomCarryThinker{g.allocThinkerOrder(), sec, boomScrollVector{fixedMul(dx, 6144), fixedMul(dy, 6144)}})
			}
		}
	}
}

func (g *game) boomActorTouchesSector(x, y, radius int64, sec int) bool {
	if g.sectorAt(x, y) == sec {
		return true
	}
	box := [4]int64{y + radius, y - radius, x + radius, x - radius}
	if sec >= len(g.sectorLineAdj) {
		return false
	}
	for _, edge := range g.sectorLineAdj[sec] {
		if edge.line < 0 || edge.line >= len(g.physForLine) {
			continue
		}
		idx := g.physForLine[edge.line]
		if idx < 0 || idx >= len(g.lines) {
			continue
		}
		line := g.lines[idx]
		if box[2] <= line.bbox[3] || box[3] >= line.bbox[2] || box[0] <= line.bbox[1] || box[1] >= line.bbox[0] {
			continue
		}
		if g.boxOnLineSide(box, line) == -1 {
			return true
		}
	}
	return false
}

func (g *game) tickBoomCarry(c boomCarryThinker) {
	floor := g.sectorFloor[c.sector]
	carryPlayer := func() {
		if !g.noClip && g.p.z <= floor && g.boomActorTouchesSector(g.p.x, g.p.y, playerRadius, c.sector) {
			g.p.momx += c.vector.x
			g.p.momy += c.vector.y
		}
	}
	if len(g.authorityPlayers) > 0 {
		for _, state := range g.authorityPlayers {
			g.withAuthoritativePlayer(state, carryPlayer)
		}
	} else {
		carryPlayer()
	}
	for i, th := range g.m.Things {
		if !thingTypeUsesBlockmap(th.Type) || g.thingCollected[i] || monsterCanFloat(th.Type) || thingSpawnsOnCeiling(th.Type) {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		z, _, _ := g.thingSupportState(i, th)
		if z <= floor && g.boomActorTouchesSector(x, y, g.thingCurrentRadius(i, th), c.sector) {
			g.thingMomX[i] += c.vector.x
			g.thingMomY[i] += c.vector.y
		}
	}
}

// Earlier starts become physical actors; the last start retains the camera.
// Editor numbers stay unchanged in restartTemplate. Internal actor types are
// serialized with the ordinary thing arrays, including momentum and health.
const voodooThingType int16 = -32001

func isVoodooDollType(typ int16) bool { return typ == voodooThingType }

func (g *game) initVoodooDolls() {
	if g.opts.GameMode != gameModeSingle {
		return
	}
	for i, th := range g.m.Things {
		if i != g.localPlayerThingIndex && playerSlotFromThingType(th.Type) == g.localSlot {
			g.m.Things[i].Type = voodooThingType
		}
	}
}

func (g *game) damageVoodooDoll(i, amount int, x, y, z int64, hasInflictor bool, sourceThing int) {
	main := g.p
	th := g.m.Things[i]
	px, py := g.thingPosFixed(i, th)
	pz, floor, ceil := g.thingSupportState(i, th)
	g.p = player{x: px, y: py, z: pz, floorz: floor, ceilz: ceil, momx: g.thingMomX[i], momy: g.thingMomY[i], momz: g.thingMomZ[i], subsector: -1, sector: -1, viewHeight: playerViewHeight}
	g.damagePlayerFromWithInflictorZ(amount, "Ouch!", x, y, hasInflictor, sourceThing, z)
	g.thingHP[i] = g.stats.Health
	g.setThingMomentum(i, g.p.momx, g.p.momy, g.p.momz)
	if g.stats.Health <= 0 {
		g.thingDead[i] = true
	}
	g.p = main
}
