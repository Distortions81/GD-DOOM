package doomruntime

const boomMoveFactor int64 = 2048

type boomFrictionThinker struct {
	order                int64
	sector               int
	friction, movefactor int64
}
type boomPusherThinker struct {
	order                       int64
	sector, source              int
	kind                        uint16
	xmag, ymag, magnitude, x, y int64
}

// Boom 2.02 uses ordered friction thinkers, unlike MBF's per-move lookup.
// Their output is consumed by the following player/body tic.
func (g *game) initBoomEnvironment() {
	g.boomVoodooBodies = nil
	g.boomFrictions = nil
	g.boomPushers = nil
	if g.opts.DemoScript != nil && g.opts.DemoScript.Header.Version < 201 {
		return
	}
	for _, l := range g.m.Linedefs {
		if l.Special != 223 {
			continue
		}
		dx, dy := g.boomLineDelta(l)
		value := approxDistance(dx, dy) >> fracBits
		f := boomFixed((0x1EB8*value)/0x80 + 0xD000)
		var move int64
		if f > friction {
			move = ((0x10092 - f) * 0x70) / 0x158
		} else {
			move = ((f - 0xDB34) * 0xA) / 0x80
		}
		for sec, s := range g.m.Sectors {
			if uint16(s.Tag) == l.Tag {
				g.boomFrictions = append(g.boomFrictions, boomFrictionThinker{g.allocThinkerOrder(), sec, f, move})
			}
		}
	}
	if len(g.boomFrictions) > 0 {
		g.thingFriction = make([]int64, len(g.m.Things))
		g.thingMoveFactor = make([]int64, len(g.m.Things))
	}
	for _, l := range g.m.Linedefs {
		if l.Special < 224 || l.Special > 226 {
			continue
		}
		dx, dy := g.boomLineDelta(l)
		xm, ym := dx>>fracBits, dy>>fracBits
		for sec, s := range g.m.Sectors {
			if uint16(s.Tag) != l.Tag {
				continue
			}
			source := -1
			if l.Special == 226 {
				// Sector thing links insert at the head: the last spawned point wins.
				for i := len(g.m.Things) - 1; i >= 0; i-- {
					th := g.m.Things[i]
					if (th.Type == 5001 || th.Type == 5002) && !g.thingCollected[i] && g.thingSectorCached(i, th) == sec {
						source = i
						break
					}
				}
				if source < 0 {
					continue
				}
			}
			p := boomPusherThinker{order: g.allocThinkerOrder(), sector: sec, source: source, kind: l.Special, xmag: xm, ymag: ym, magnitude: approxDistance(xm, ym)}
			if source >= 0 {
				p.x, p.y = g.thingPosFixed(source, g.m.Things[source])
			}
			g.boomPushers = append(g.boomPushers, p)
		}
	}
	if len(g.boomFrictions) > 0 || len(g.boomPushers) > 0 {
		for i, th := range g.m.Things {
			if isVoodooDollType(th.Type) {
				g.boomVoodooBodies = append(g.boomVoodooBodies, i)
			}
		}
	}

}

func storedBoomFriction(f int64) int64 {
	if f == 0 {
		return friction
	}
	return f
}

func (g *game) boomPlayerMoveFactor() int64 {
	if len(g.boomFrictions) == 0 || g.noClip {
		return boomMoveFactor
	}
	f := storedBoomFriction(g.p.friction)
	if f == friction {
		return boomMoveFactor
	}
	move := g.p.movefactor
	g.p.movefactor = boomMoveFactor
	if f < friction {
		momentum := approxDistance(g.p.momx, g.p.momy)
		switch {
		case momentum > 60000:
			move <<= 3
		case momentum > 30000:
			move <<= 2
		case momentum > 15000:
			move <<= 1
		}
	}
	return move
}

func (g *game) tickBoomFriction(f boomFrictionThinker) {
	if g.m.Sectors[f.sector].Special&0x100 == 0 {
		return
	}
	floor := g.sectorFloor[f.sector]
	applyPlayer := func() {
		if !g.noClip && g.p.z <= floor && g.boomActorTouchesSector(g.p.x, g.p.y, playerRadius, f.sector) && (storedBoomFriction(g.p.friction) == friction || f.friction < storedBoomFriction(g.p.friction)) {
			g.p.friction, g.p.movefactor = f.friction, f.movefactor
		}
	}
	if len(g.authorityPlayers) > 0 {
		for _, state := range g.authorityPlayers {
			g.withAuthoritativePlayer(state, applyPlayer)
		}
	} else {
		applyPlayer()
	}
	for _, i := range g.boomVoodooBodies {
		th := g.m.Things[i]
		if g.thingCollected[i] {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		z, _, _ := g.thingSupportState(i, th)
		if z <= floor && g.boomActorTouchesSector(x, y, playerRadius, f.sector) && (storedBoomFriction(g.thingFriction[i]) == friction || f.friction < storedBoomFriction(g.thingFriction[i])) {
			g.thingFriction[i], g.thingMoveFactor[i] = f.friction, f.movefactor
		}
	}
}

func (g *game) tickBoomPusher(p boomPusherThinker) {
	if g.m.Sectors[p.sector].Special&0x200 == 0 {
		return
	}
	force := func(x, y, z, floor, viewz, height, radius int64) (int64, int64) {
		if p.kind == 226 {
			distance := approxDistance(boomFixed(x-p.x), boomFixed(y-p.y))
			speed := boomFixed((p.magnitude - ((distance >> fracBits) >> 1)) << 8)
			if speed <= 0 {
				return 0, 0
			}
			th := g.m.Things[p.source]
			sx, sy := g.thingPosFixed(p.source, th)
			sz, _, _ := g.thingSupportState(p.source, th)
			if !g.boomPusherSight(x, y, z, height, sx, sy, sz) {
				return 0, 0
			}
			angle := doomPointToAngle2(x, y, p.x, p.y)
			if th.Type == 5001 {
				angle += doomAng180
			}
			return fixedMul(speed, doomFineCosine(angle)), fixedMul(speed, doomFineSineAtAngle(angle))
		}
		if !g.boomActorTouchesSector(x, y, radius, p.sector) {
			return 0, 0
		}
		mx, my := p.xmag, p.ymag
		control := g.heightControl(p.sector)
		if p.kind == 224 {
			if control < 0 {
				if z <= floor {
					mx >>= 1
					my >>= 1
				}
			} else {
				water := g.sectorFloor[control]
				if z <= water {
					if viewz < water {
						return 0, 0
					}
					mx >>= 1
					my >>= 1
				}
			}
		} else {
			ground := g.sectorFloor[p.sector]
			if control >= 0 {
				ground = g.sectorFloor[control]
			}
			if z > ground {
				return 0, 0
			}
		}
		return boomFixed(mx << 9), boomFixed(my << 9)
	}
	applyPlayer := func() {
		if !g.noClip {
			mx, my := force(g.p.x, g.p.y, g.p.z, g.p.floorz, g.playerViewZ, g.playerMobjHeight(), playerRadius)
			g.p.momx = boomFixed(g.p.momx + mx)
			g.p.momy = boomFixed(g.p.momy + my)
		}
	}
	if len(g.authorityPlayers) > 0 {
		for _, state := range g.authorityPlayers {
			g.withAuthoritativePlayer(state, applyPlayer)
		}
	} else {
		applyPlayer()
	}
	for _, i := range g.boomVoodooBodies {
		th := g.m.Things[i]
		if g.thingCollected[i] {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		z, floor, _ := g.thingSupportState(i, th)
		// Boom tests the shared player's viewz even for a voodoo body.
		mx, my := force(x, y, z, floor, g.playerViewZ, g.thingCurrentHeight(i, th), playerRadius)
		g.thingMomX[i] = boomFixed(g.thingMomX[i] + mx)
		g.thingMomY[i] = boomFixed(g.thingMomY[i] + my)
	}
}

func (g *game) boomPusherSight(x, y, z, height, sx, sy, sz int64) bool {
	const sourceHeight int64 = 8 * fracUnit
	a, b := g.heightControl(g.sectorAt(x, y)), g.heightControl(g.sectorAt(sx, sy))
	if a >= 0 && (z+height <= g.sectorFloor[a] && sz >= g.sectorFloor[a] || z >= g.sectorCeil[a] && sz+height <= g.sectorCeil[a]) {
		return false
	}
	if b >= 0 && (sz+sourceHeight <= g.sectorFloor[b] && z >= g.sectorFloor[b] || sz >= g.sectorCeil[b] && z+sourceHeight <= g.sectorCeil[b]) {
		return false
	}
	return g.actorHasLOS(x, y, z, height, sx, sy, sz, sourceHeight)
}

// Missing arrays are the default for snapshots predating friction support.
func validBoomFrictionState(s *gameSaveState, n int) bool {
	return len(s.ThingFriction) == len(s.ThingMoveFactor) && (len(s.ThingFriction) == 0 || len(s.ThingFriction) == n)
}
