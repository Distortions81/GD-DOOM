package doomruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gddoom/internal/doomrand"
)

type demoTraceWriter struct {
	path   string
	file   *os.File
	closed bool
}

const demoTraceWeaponNoChange = 10

type demoTracePlayer struct {
	PlayerState   int    `json:"playerstate"`
	Health        int    `json:"health"`
	ArmorPoints   int    `json:"armorpoints"`
	ArmorType     int    `json:"armortype"`
	ReadyWeapon   int    `json:"readyweapon"`
	PendingWeapon int    `json:"pendingweapon"`
	MO            int    `json:"mo"`
	X             int64  `json:"x"`
	Y             int64  `json:"y"`
	Z             int64  `json:"z"`
	Angle         uint32 `json:"angle"`
	MomX          int64  `json:"momx"`
	MomY          int64  `json:"momy"`
	MomZ          int64  `json:"momz"`
	MOHealth      int    `json:"mo_health"`
}

type demoTraceMobj struct {
	Type         int    `json:"type"`
	X            int64  `json:"x"`
	Y            int64  `json:"y"`
	Z            int64  `json:"z"`
	Angle        uint32 `json:"angle"`
	MomX         int64  `json:"momx"`
	MomY         int64  `json:"momy"`
	MomZ         int64  `json:"momz"`
	FloorZ       int64  `json:"floorz"`
	CeilingZ     int64  `json:"ceilingz"`
	Radius       int64  `json:"radius"`
	Height       int64  `json:"height"`
	Tics         int    `json:"tics"`
	State        int    `json:"state"`
	Flags        int    `json:"flags"`
	Health       int    `json:"health"`
	Movedir      int    `json:"movedir"`
	Movecount    int    `json:"movecount"`
	ReactionTime int    `json:"reactiontime"`
	Threshold    int    `json:"threshold"`
	LastLook     int    `json:"lastlook"`
	Subsector    int    `json:"subsector"`
	Sector       int    `json:"sector"`
	Player       int    `json:"player"`
	Target       int    `json:"target"`
	TargetType   int    `json:"target_type"`
	Tracer       int    `json:"tracer"`
	TracerType   int    `json:"tracer_type"`
	Kind         string `json:"kind,omitempty"`
	Dropped      int    `json:"dropped,omitempty"`
}

type demoTraceSpecial struct {
	Kind          string `json:"kind"`
	Sector        int    `json:"sector"`
	Type          int    `json:"type,omitempty"`
	Action        string `json:"action,omitempty"`
	TopHeight     int64  `json:"topheight,omitempty"`
	Speed         int64  `json:"speed,omitempty"`
	Direction     int    `json:"direction,omitempty"`
	TopWait       int    `json:"topwait,omitempty"`
	TopCountdown  int    `json:"topcountdown"`
	Crush         int    `json:"crush,omitempty"`
	NewSpecial    int16  `json:"newspecial,omitempty"`
	Texture       string `json:"texture,omitempty"`
	FloorDest     int64  `json:"floordestheight,omitempty"`
	Low           int64  `json:"low,omitempty"`
	High          int64  `json:"high,omitempty"`
	Wait          int    `json:"wait,omitempty"`
	Count         int    `json:"count,omitempty"`
	Status        int    `json:"status,omitempty"`
	OldStatus     int    `json:"oldstatus,omitempty"`
	Tag           int    `json:"tag,omitempty"`
	BottomHeight  int64  `json:"bottomheight,omitempty"`
	OldDirection  int    `json:"olddirection,omitempty"`
	FinishSpecial int16  `json:"finishspecial,omitempty"`
}

func newDemoTraceWriter(opts Options, mapName string) *demoTraceWriter {
	path := strings.TrimSpace(opts.DemoTracePath)
	if path == "" || opts.DemoScript == nil {
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("demo-trace-error path=%s err=%v\n", path, err)
		return nil
	}
	tw := &demoTraceWriter{path: path, file: f}
	tw.write(map[string]any{
		"kind":       "meta",
		"trace_path": path,
		"iwad":       opts.WADHash,
		"demo":       demoTraceLabel(opts.DemoScript),
		"gamemode":   opts.GameMode,
		"map":        mapName,
	})
	tw.write(map[string]any{
		"kind":          "demo",
		"demo":          demoTraceLabel(opts.DemoScript),
		"version":       opts.DemoScript.Header.Version,
		"skill":         opts.DemoScript.Header.Skill,
		"episode":       opts.DemoScript.Header.Episode,
		"map":           opts.DemoScript.Header.Map,
		"deathmatch":    boolToInt(opts.GameMode == "deathmatch"),
		"respawn":       boolToInt(opts.RespawnMonsters),
		"fast":          boolToInt(opts.FastMonsters),
		"nomonsters":    boolToInt(opts.NoMonsters),
		"consoleplayer": max(opts.PlayerSlot-1, 0),
		"playeringame": []int{
			boolToInt(opts.DemoScript.Header.PlayerInGame[0]),
			boolToInt(opts.DemoScript.Header.PlayerInGame[1]),
			boolToInt(opts.DemoScript.Header.PlayerInGame[2]),
			boolToInt(opts.DemoScript.Header.PlayerInGame[3]),
		},
	})
	return tw
}

func demoTraceLabel(script *DemoScript) string {
	if script == nil {
		return ""
	}
	if p := strings.TrimSpace(script.Path); p != "" {
		return p
	}
	return "demo"
}

func (tw *demoTraceWriter) write(v any) {
	if tw == nil || tw.file == nil || tw.closed {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("demo-trace-error path=%s err=%v\n", tw.path, err)
		return
	}
	_, _ = tw.file.Write(append(data, '\n'))
	_ = tw.file.Sync()
}

func (tw *demoTraceWriter) Close() {
	if tw == nil || tw.file == nil || tw.closed {
		return
	}
	tw.closed = true
	_ = tw.file.Close()
	tw.file = nil
}

func (g *game) writeDemoTraceTic() {
	if g == nil || g.demoTrace == nil {
		return
	}

	rndIndex, prndIndex := doomrand.State()
	pendingWeapon := demoTraceWeaponNoChange
	if g.inventory.PendingWeapon != 0 {
		pendingWeapon = demoTraceWeaponID(g.inventory.PendingWeapon)
	}
	player := demoTracePlayer{
		PlayerState:   boolToInt(g.isDead),
		Health:        g.stats.Health,
		ArmorPoints:   g.stats.Armor,
		ArmorType:     g.stats.ArmorType,
		ReadyWeapon:   demoTraceWeaponID(g.inventory.ReadyWeapon),
		PendingWeapon: pendingWeapon,
		MO:            1,
		X:             g.p.x,
		Y:             g.p.y,
		Z:             g.p.z,
		Angle:         g.p.angle,
		MomX:          g.p.momx,
		MomY:          g.p.momy,
		MomZ:          g.p.momz,
		MOHealth:      g.stats.Health,
	}

	mobjs := g.demoTraceMobjs()
	specials := g.demoTraceSpecials()
	g.demoTrace.write(map[string]any{
		"kind":            "tic",
		"gametic":         g.demoTick - 1,
		"rndindex":        rndIndex,
		"prndindex":       prndIndex,
		"gamestate":       0,
		"gamestate_name":  "GS_LEVEL",
		"gameaction":      0,
		"gameaction_name": "ga_nothing",
		"leveltime":       g.worldTic,
		"consoleplayer":   max(g.localSlot-1, 0),
		"displayplayer":   max(g.localSlot-1, 0),
		"playeringame": []int{
			1, 0, 0, 0,
		},
		"player":        player,
		"mobjs":         mobjs,
		"specials":      specials,
		"mobj_count":    len(mobjs),
		"special_count": len(specials),
	})
}

func (g *game) demoTraceMobjs() []demoTraceMobj {
	if g == nil {
		return nil
	}
	out := make([]demoTraceMobj, 0, 1+len(g.m.Things)+len(g.projectiles))
	out = append(out, demoTraceMobj{
		Type:         0,
		X:            g.p.x,
		Y:            g.p.y,
		Z:            g.p.z,
		Angle:        g.p.angle,
		MomX:         g.p.momx,
		MomY:         g.p.momy,
		MomZ:         g.p.momz,
		FloorZ:       g.p.floorz,
		CeilingZ:     g.p.ceilz,
		Radius:       playerRadius,
		Height:       playerHeight,
		Tics:         0,
		State:        0,
		Flags:        0,
		Health:       g.stats.Health,
		Movedir:      0,
		Movecount:    0,
		ReactionTime: 0,
		Threshold:    0,
		LastLook:     0,
		Subsector:    boolToInt(g.sectorAt(g.p.x, g.p.y) >= 0),
		Sector:       g.sectorAt(g.p.x, g.p.y),
		Player:       1,
		Target:       0,
		Tracer:       0,
	})
	for i, th := range g.m.Things {
		if playerSlotFromThingType(th.Type) != 0 {
			continue
		}
		if i >= 0 && i < len(g.thingCollected) && g.thingCollected[i] {
			continue
		}
		x, y := g.thingPosFixed(i, th)
		sec := g.thingSectorCached(i, th)
		z := g.thingFloorZCached(i, th)
		floorZ := z
		ceilZ := int64(0)
		if isMonster(th.Type) {
			z, floorZ, ceilZ = g.monsterSupportHeights(i, th)
		} else if thingTypeIsShootable(th.Type) {
			z, floorZ, ceilZ = g.thingSupportState(i, th)
		} else if sec >= 0 && sec < len(g.sectorCeil) {
			ceilZ = g.sectorCeil[sec]
		}
		radius, height := demoTraceThingBounds(th.Type)
		if thingTypeIsShootable(th.Type) {
			height = g.thingCurrentHeight(i, th)
		}
		target, targetType := demoTraceThingTarget(g, i)
		out = append(out, demoTraceMobj{
			Type:         demoTraceThingType(th.Type),
			X:            x,
			Y:            y,
			Z:            z,
			Angle:        g.thingWorldAngle(i, th),
			MomX:         demoTraceThingMomX(g, i),
			MomY:         demoTraceThingMomY(g, i),
			MomZ:         demoTraceThingMomZ(g, i),
			FloorZ:       floorZ,
			CeilingZ:     ceilZ,
			Radius:       radius,
			Height:       height,
			Tics:         demoTraceThingTics(g, i, th.Type),
			State:        demoTraceThingState(g, i, th.Type),
			Flags:        0,
			Health:       demoTraceThingHealth(g, i, th.Type),
			Movedir:      demoTraceThingMoveDir(g, i),
			Movecount:    demoTraceThingMoveCount(g, i),
			ReactionTime: demoTraceThingReaction(g, i),
			Threshold:    demoTraceThingThreshold(g, i),
			LastLook:     demoTraceThingLastLook(g, i),
			Subsector:    boolToInt(sec >= 0),
			Sector:       sec,
			Player:       0,
			Target:       target,
			TargetType:   targetType,
			Tracer:       0,
			Kind:         demoTraceThingKind(th.Type),
			Dropped:      boolToInt(i >= 0 && i < len(g.thingDropped) && g.thingDropped[i]),
		})
	}
	for _, p := range g.projectiles {
		sec := g.sectorAt(p.x, p.y)
		floorZ := g.thingFloorZ(p.x, p.y)
		ceilZ := int64(0)
		if sec >= 0 && sec < len(g.sectorCeil) {
			ceilZ = g.sectorCeil[sec]
		}
		out = append(out, demoTraceMobj{
			Type:         1000 + int(p.kind),
			X:            p.x,
			Y:            p.y,
			Z:            p.z,
			Angle:        0,
			MomX:         p.vx,
			MomY:         p.vy,
			MomZ:         p.vz,
			FloorZ:       floorZ,
			CeilingZ:     ceilZ,
			Radius:       p.radius,
			Height:       p.height,
			Tics:         p.ttl,
			State:        -1,
			Flags:        0,
			Health:       1000,
			Movedir:      0,
			Movecount:    0,
			ReactionTime: 0,
			Threshold:    0,
			LastLook:     0,
			Subsector:    boolToInt(sec >= 0),
			Sector:       sec,
			Player:       0,
			Target:       1,
			TargetType:   int(p.sourceType),
			Tracer:       0,
			Kind:         "projectile",
		})
	}
	for _, p := range g.hitscanPuffs {
		sec := g.sectorAt(p.x, p.y)
		floorZ := g.thingFloorZ(p.x, p.y)
		ceilZ := int64(0)
		if sec >= 0 && sec < len(g.sectorCeil) {
			ceilZ = g.sectorCeil[sec]
		}
		mobjType := 37
		flags := 528
		if p.kind == hitscanFxBlood {
			mobjType = 38
			flags = 16
		}
		out = append(out, demoTraceMobj{
			Type:         mobjType,
			X:            p.x,
			Y:            p.y,
			Z:            p.z,
			Angle:        0,
			MomX:         0,
			MomY:         0,
			MomZ:         p.momz,
			FloorZ:       floorZ,
			CeilingZ:     ceilZ,
			Radius:       20 * fracUnit,
			Height:       16 * fracUnit,
			Tics:         p.tics,
			State:        p.state,
			Flags:        flags,
			Health:       1000,
			Movedir:      0,
			Movecount:    0,
			ReactionTime: 8,
			Threshold:    0,
			LastLook:     0,
			Subsector:    boolToInt(sec >= 0),
			Sector:       sec,
			Player:       0,
			Target:       0,
			TargetType:   0,
			Tracer:       0,
			TracerType:   0,
		})
	}
	return out
}

func demoTraceThingTarget(g *game, i int) (target int, targetType int) {
	if g == nil || i < 0 {
		return 0, 0
	}
	if i < len(g.thingTargetPlayer) && g.thingTargetPlayer[i] {
		return 1, 0
	}
	if i < len(g.thingTargetIdx) {
		idx := g.thingTargetIdx[i]
		if idx >= 0 {
			// Demo traces serialize the player mobj first, then map things in map order.
			return idx + 2, demoTraceThingType(g.m.Things[idx].Type)
		}
	}
	return 0, 0
}

func (g *game) demoTraceSpecials() []map[string]any {
	if g == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(g.doors)+len(g.floors)+len(g.plats)+len(g.ceilings))

	doorKeys := sortedIntKeys(g.doors)
	for _, sec := range doorKeys {
		d := g.doors[sec]
		out = append(out, map[string]any{
			"kind":         "door",
			"sector":       sec,
			"type":         int(d.typ),
			"topheight":    d.topHeight,
			"speed":        d.speed,
			"direction":    d.direction,
			"topwait":      d.topWait,
			"topcountdown": d.topCountdown,
		})
	}
	floorKeys := sortedIntKeys(g.floors)
	for _, sec := range floorKeys {
		f := g.floors[sec]
		out = append(out, map[string]any{
			"kind":            "floor",
			"sector":          sec,
			"type":            f.direction,
			"speed":           f.speed,
			"direction":       f.direction,
			"floordestheight": f.destHeight,
			"texture":         f.finishFlat,
			"finishspecial":   int16(f.finishSpecial),
		})
	}
	platKeys := sortedIntKeys(g.plats)
	for _, sec := range platKeys {
		p := g.plats[sec]
		out = append(out, map[string]any{
			"kind":          "plat",
			"sector":        sec,
			"type":          int(p.typ),
			"speed":         p.speed,
			"low":           p.low,
			"high":          p.high,
			"wait":          p.wait,
			"count":         p.count,
			"status":        int(p.status),
			"oldstatus":     int(p.oldStatus),
			"finishspecial": int16(p.finishSpecial),
			"texture":       p.finishFlat,
		})
	}
	ceilingKeys := sortedIntKeys(g.ceilings)
	for _, sec := range ceilingKeys {
		c := g.ceilings[sec]
		out = append(out, map[string]any{
			"kind":         "ceiling",
			"sector":       sec,
			"action":       string(c.action),
			"speed":        c.speed,
			"direction":    c.direction,
			"topheight":    c.topHeight,
			"bottomheight": c.bottomHeight,
			"crush":        boolToInt(c.crush),
			"olddirection": c.oldDirection,
		})
	}
	return out
}

func demoTraceThingBounds(typ int16) (int64, int64) {
	if info, ok := demoTraceThingInfoForType(typ); ok {
		return info.radius, info.height
	}
	if isMonster(typ) {
		return monsterRadius(typ), monsterHeight(typ)
	}
	return 20 * fracUnit, 16 * fracUnit
}

func demoTraceThingHealth(g *game, i int, typ int16) int {
	if thingTypeIsShootable(typ) && i >= 0 && i < len(g.thingHP) {
		return g.thingHP[i]
	}
	if info, ok := demoTraceThingInfoForType(typ); ok {
		return info.health
	}
	return 1000
}

func demoTraceThingTics(g *game, i int, typ int16) int {
	if i < 0 {
		return 0
	}
	if i < len(g.thingStateTics) && g.thingStateTics[i] > 0 {
		return g.thingStateTics[i]
	}
	if i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
		if tics, ok := demoTraceMonsterPainStateTics(typ, g.thingPainTics[i]); ok {
			return tics
		}
		return g.thingPainTics[i]
	}
	if i < len(g.thingDeathTics) && g.thingDeathTics[i] > 0 {
		return g.thingDeathTics[i]
	}
	if i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
		return g.thingAttackTics[i]
	}
	if i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		return g.thingReactionTics[i]
	}
	return 0
}

func demoTraceThingState(g *game, i int, typ int16) int {
	if isBarrelThingType(typ) {
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
			phase := 0
			if i >= 0 && i < len(g.thingStatePhase) {
				phase = g.thingStatePhase[i]
			}
			if phase < 0 {
				phase = 0
			}
			if phase >= len(barrelDeathSprites) {
				phase = len(barrelDeathSprites) - 1
			}
			return barrelStateBEXP + phase
		}
		phase := 0
		if i >= 0 && i < len(g.thingStatePhase) {
			phase = g.thingStatePhase[i] & 1
		}
		return barrelStateBAR1 + phase
	}
	if i >= 0 && i < len(g.thingState) {
		switch g.thingState[i] {
		case monsterStateDeath:
			return 3
		case monsterStatePain:
			if i >= 0 && i < len(g.thingPainTics) {
				if state, ok := demoTraceMonsterPainState(typ, g.thingPainTics[i]); ok {
					return state
				}
			}
			return 2
		case monsterStateAttack:
			if i >= 0 && i < len(g.thingAttackPhase) {
				if state, ok := demoTraceMonsterAttackState(typ, g.thingAttackPhase[i]); ok {
					return state
				}
			}
			return 1
		case monsterStateSpawn:
			if i >= 0 && i < len(g.thingStatePhase) {
				if state, ok := demoTraceMonsterSpawnState(typ, g.thingStatePhase[i]); ok {
					return state
				}
			}
			return 4
		case monsterStateSee:
			if i >= 0 && i < len(g.thingStatePhase) {
				if state, ok := demoTraceMonsterSeeState(typ, g.thingStatePhase[i]); ok {
					return state
				}
			}
			return 5
		}
	}
	if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
		return 3
	}
	if i >= 0 && i < len(g.thingPainTics) && g.thingPainTics[i] > 0 {
		return 2
	}
	if i >= 0 && i < len(g.thingAttackTics) && g.thingAttackTics[i] > 0 {
		return 1
	}
	if isMonster(typ) {
		return 0
	}
	return -1
}

func demoTraceMonsterSpawnState(typ int16, phase int) (int, bool) {
	base := 0
	count := 0
	switch typ {
	case 3004:
		base, count = 174, 2
	case 9:
		base, count = 207, 2
	case 3001:
		base, count = 442, 2
	case 3002, 58:
		base, count = 475, 2
	case 3005:
		base, count = 502, 1
	case 3003:
		base, count = 527, 2
	case 69:
		base, count = 556, 2
	default:
		return 0, false
	}
	if phase < 0 || phase >= count {
		return 0, false
	}
	return base + phase, true
}

func demoTraceMonsterSeeState(typ int16, phase int) (int, bool) {
	base := 0
	count := 0
	switch typ {
	case 3004:
		base, count = 176, 8
	case 9:
		base, count = 209, 8
	case 3001:
		base, count = 444, 8
	case 3002, 58:
		base, count = 477, 8
	case 3005:
		base, count = 503, 1
	case 3003:
		base, count = 529, 8
	case 69:
		base, count = 558, 8
	default:
		return 0, false
	}
	if phase < 0 || phase >= count {
		return 0, false
	}
	return base + phase, true
}

func demoTraceMonsterPainState(typ int16, remaining int) (int, bool) {
	base := 0
	switch typ {
	case 3004:
		base = 187
	case 9:
		base = 220
	case 3001:
		base = 455
	case 3002, 58:
		base = 488
	case 3005:
		base = 507
	case 3003:
		base = 540
	case 69:
		base = 569
	case 3006:
		base = 593
	case 7:
		base = 619
	case 16:
		base = 690
	default:
		return 0, false
	}
	frameTics := monsterPainFrameTics(typ)
	if len(frameTics) == 0 || remaining <= 0 {
		return 0, false
	}
	total := 0
	for _, t := range frameTics {
		if t > 0 {
			total += t
		}
	}
	elapsed := total - remaining
	if elapsed < 0 {
		elapsed = 0
	}
	acc := 0
	for idx, t := range frameTics {
		if t <= 0 {
			continue
		}
		acc += t
		if elapsed < acc {
			return base + idx, true
		}
	}
	return base + len(frameTics) - 1, true
}

func demoTraceMonsterPainStateTics(typ int16, remaining int) (int, bool) {
	frameTics := monsterPainFrameTics(typ)
	if len(frameTics) == 0 || remaining <= 0 {
		return 0, false
	}
	total := 0
	for _, t := range frameTics {
		if t > 0 {
			total += t
		}
	}
	elapsed := total - remaining
	if elapsed < 0 {
		elapsed = 0
	}
	acc := 0
	for _, t := range frameTics {
		if t <= 0 {
			continue
		}
		acc += t
		if elapsed < acc {
			return acc - elapsed, true
		}
	}
	return 1, true
}

func demoTraceThingMoveDir(g *game, i int) int {
	if i >= 0 && i < len(g.thingMoveDir) {
		return int(g.thingMoveDir[i])
	}
	return 0
}

func demoTraceThingMoveCount(g *game, i int) int {
	if i >= 0 && i < len(g.thingMoveCount) {
		return g.thingMoveCount[i]
	}
	return 0
}

func demoTraceThingMomX(g *game, i int) int64 {
	if i >= 0 && i < len(g.thingMomX) {
		return g.thingMomX[i]
	}
	return 0
}

func demoTraceThingMomY(g *game, i int) int64 {
	if i >= 0 && i < len(g.thingMomY) {
		return g.thingMomY[i]
	}
	return 0
}

func demoTraceThingMomZ(g *game, i int) int64 {
	if i >= 0 && i < len(g.thingMomZ) {
		return g.thingMomZ[i]
	}
	return 0
}

func demoTraceThingReaction(g *game, i int) int {
	if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		return g.thingReactionTics[i]
	}
	return 0
}

func demoTraceThingThreshold(g *game, i int) int {
	if i >= 0 && i < len(g.thingThreshold) && g.thingThreshold[i] > 0 {
		return g.thingThreshold[i]
	}
	return 0
}

func demoTraceThingLastLook(g *game, i int) int {
	if i >= 0 && i < len(g.thingLastLook) {
		return g.thingLastLook[i]
	}
	return 0
}

func demoTraceThingKind(typ int16) string {
	switch {
	case isBarrelThingType(typ):
		return "barrel"
	case isMonster(typ):
		return "monster"
	case isPickupType(typ):
		return "pickup"
	default:
		return "thing"
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func sortedIntKeys[T any](m map[int]T) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
