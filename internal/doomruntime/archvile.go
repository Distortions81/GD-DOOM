package doomruntime

// A_VileTarget creates a normal ordered mobj thinker. Its first state lasts
// two tics without randomization; P_SpawnMobj consumes only the lastlook draw.
func (g *game) spawnArchVileFire(source int) {
	x, _, z, _, _, ok := g.monsterAttackTargetPos(source)
	if !ok {
		return
	}
	fx := projectileImpact{
		x: x, y: x, z: z, // Preserve A_VileTarget's original target->x typo.
		kind: projectileArchVileFire, order: g.allocThinkerOrder(),
		sourceThing: source, sourceType: 64,
		lastLook: doomPRandomN(4), tics: 60, totalTics: 60, phaseTics: 2,
	}
	fx.subsector = g.subSectorAtFixed(fx.x, fx.y) + 1
	sec := g.sectorAt(fx.x, fx.y)
	if sec >= 0 && sec < len(g.sectorFloor) && sec < len(g.sectorCeil) {
		fx.floorz, fx.ceilz = g.sectorFloor[sec], g.sectorCeil[sec]
	}
	if target, ok := g.monsterTargetThingIdx(source); ok {
		fx.fireTargetThing = target + 1
	} else {
		fx.fireTargetPlayer = true
	}
	if len(g.thingTracerFireOrder) != len(g.m.Things) {
		old := g.thingTracerFireOrder
		g.thingTracerFireOrder = make([]int64, len(g.m.Things))
		copy(g.thingTracerFireOrder, old)
	}
	g.thingTracerFireOrder[source] = fx.order
	g.followArchVileFire(&fx)
	g.projectileImpacts = append(g.projectileImpacts, fx)
}

// A_Fire keeps the captured victim even if the arch-vile changes targets.
// Relinking its position does not recompute its cached support heights.
func (g *game) followArchVileFire(fx *projectileImpact) {
	if fx == nil || g.m == nil || fx.sourceThing < 0 || fx.sourceThing >= len(g.m.Things) {
		return
	}
	x, y, z, height, angle := g.p.x, g.p.y, g.p.z, g.playerMobjHeight(), g.p.angle
	if !fx.fireTargetPlayer {
		i := fx.fireTargetThing - 1
		if i < 0 || i >= len(g.m.Things) {
			return
		}
		th := g.m.Things[i]
		x, y = g.thingPosFixed(i, th)
		z, _, _ = g.thingSupportState(i, th)
		height, angle = g.thingCurrentHeight(i, th), g.thingWorldAngle(i, th)
	}
	source := g.m.Things[fx.sourceThing]
	sx, sy := g.thingPosFixed(fx.sourceThing, source)
	sz, _, _ := g.thingSupportState(fx.sourceThing, source)
	if !g.actorHasLOS(sx, sy, sz, g.thingCurrentHeight(fx.sourceThing, source), x, y, z, height) {
		return
	}
	fx.x = x + fixedMul(24*fracUnit, doomFineCosine(angle))
	fx.y = y + fixedMul(24*fracUnit, doomFineSineAtAngle(angle))
	fx.z = z
	// A_Fire calls P_SetThingPosition after moving the fire. A_VileAttack
	// later changes XY directly and must retain this last linked subsector.
	fx.subsector = g.subSectorAtFixed(fx.x, fx.y) + 1
}

func (g *game) archVileFireForSource(source int) *projectileImpact {
	for i := len(g.projectileImpacts) - 1; i >= 0; i-- {
		fx := &g.projectileImpacts[i]
		if fx.kind == projectileArchVileFire && fx.sourceThing == source {
			return fx
		}
	}
	return nil
}

func (g *game) archVileBlast(source int, sx, sy int64) bool {
	if !g.monsterHasLOSTarget(source, 64, sx, sy) {
		return false
	}
	g.damageMonsterTarget(source, 20, "Arch-Vile blast", sx, sy)
	if target, ok := g.monsterTargetThingIdx(source); ok {
		g.thingMomZ[target] = 1000 * fracUnit / int64(thingTypeMass(g.m.Things[target].Type))
	} else {
		g.p.momz = 10 * fracUnit
	}
	fx := g.archVileFireForSource(source)
	if fx == nil {
		return true
	}
	x, y, _, _, _, ok := g.monsterAttackTargetPos(source)
	if !ok {
		return true
	}
	angle := g.thingWorldAngle(source, g.m.Things[source])
	fx.x = x - fixedMul(24*fracUnit, doomFineCosine(angle))
	fx.y = y - fixedMul(24*fracUnit, doomFineSineAtAngle(angle))
	g.radiusAttackAt(fx.x, fx.y, fx.z, 16*fracUnit, -1, 70, "Arch-Vile blast", false, source)
	return true
}
