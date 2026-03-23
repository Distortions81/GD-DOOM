package doomruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
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

const (
	doomStatePlayerAttack1 = 154
	doomStatePlayerAttack2 = 155
	doomStatePlayerPain1   = 156
	doomStatePlayerPain2   = 157
)

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

func doomPlatType(t platType) int {
	switch t {
	case platTypePerpetualRaise:
		return 0
	case platTypeDownWaitUpStay:
		return 1
	case platTypeRaiseToNearestAndChange:
		return 3
	case platTypeBlazeDownWaitUpStay:
		return 4
	default:
		return int(t)
	}
}

func doomDoorType(t doorType) int {
	switch t {
	case doorNormal:
		return 0
	case doorClose30ThenOpen:
		return 1
	case doorClose:
		return 2
	case doorOpen:
		return 3
	case doorRaiseIn5Mins:
		return 4
	case doorBlazeRaise:
		return 5
	case doorBlazeOpen:
		return 6
	case doorBlazeClose:
		return 7
	default:
		return int(t)
	}
}

func doomPlatStatus(s platStatus) int {
	switch s {
	case platStatusInStasis:
		return 16
	default:
		return int(s)
	}
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
	readyWeapon := g.inventory.ReadyWeapon
	pendingWeaponID := g.inventory.PendingWeapon
	pendingWeapon := demoTraceWeaponNoChange
	if pendingWeaponID != 0 {
		pendingWeapon = demoTraceWeaponID(pendingWeaponID)
	}
	player := demoTracePlayer{
		PlayerState:   boolToInt(g.isDead),
		Health:        g.stats.Health,
		ArmorPoints:   g.stats.Armor,
		ArmorType:     g.stats.ArmorType,
		ReadyWeapon:   demoTraceWeaponID(readyWeapon),
		PendingWeapon: pendingWeapon,
		MO:            1,
		X:             g.p.x,
		Y:             g.p.y,
		Z:             g.p.z,
		Angle:         g.p.angle,
		MomX:          g.p.momx,
		MomY:          g.p.momy,
		MomZ:          g.p.momz,
		MOHealth:      g.playerMobjHealth,
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
	type orderedDemoTraceMobj struct {
		order int64
		idx   int
		mobj  demoTraceMobj
	}
	ordered := make([]orderedDemoTraceMobj, 0, 1+len(g.m.Things)+len(g.projectiles))
	playerState, playerTics := g.demoTracePlayerMobjState()
	ordered = append(ordered, orderedDemoTraceMobj{
		order: 0,
		idx:   -1,
		mobj: demoTraceMobj{
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
			Tics:         playerTics,
			State:        playerState,
			Flags:        0,
			Health:       g.playerMobjHealth,
			Movedir:      0,
			Movecount:    0,
			ReactionTime: 0,
			Threshold:    0,
			LastLook:     0,
			Subsector:    boolToInt(g.playerSubsector() >= 0),
			Sector:       g.playerSector(),
			Player:       1,
			Target:       0,
			Tracer:       0,
		}})
	for i, th := range g.m.Things {
		if playerSlotFromThingType(th.Type) != 0 || th.Type == 11 {
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
		} else if thingSpawnsOnCeiling(th.Type) {
			z, floorZ, ceilZ = g.thingSupportState(i, th)
		} else if thingTypeIsShootable(th.Type) {
			z, floorZ, ceilZ = g.thingSupportState(i, th)
		} else if sec >= 0 && sec < len(g.sectorCeil) {
			ceilZ = g.sectorCeil[sec]
		}
		radius, height := demoTraceThingBounds(th.Type)
		if i >= 0 && i < len(g.thingGibbed) && g.thingGibbed[i] {
			radius, height = 0, 0
		}
		if thingTypeIsShootable(th.Type) {
			height = g.thingCurrentHeight(i, th)
		}
		target, targetType := demoTraceThingTarget(g, i)
		order := int64(i + 1)
		if i >= 0 && i < len(g.thingThinkerOrder) && g.thingThinkerOrder[i] > 0 {
			order = g.thingThinkerOrder[i]
		}
		ordered = append(ordered, orderedDemoTraceMobj{
			order: order,
			idx:   i,
			mobj: demoTraceMobj{
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
				Flags:        demoTraceThingFlags(g, i, th),
				Health:       demoTraceThingHealth(g, i, th.Type),
				Movedir:      demoTraceThingMoveDir(g, i),
				Movecount:    demoTraceThingMoveCount(g, i),
				ReactionTime: demoTraceThingReaction(g, i, th.Type),
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
			}})
	}
	for _, p := range g.projectiles {
		ss := -1
		sec := -1
		if g.m != nil && len(g.m.SubSectors) > 0 {
			ss = g.subSectorAtFixed(p.x, p.y)
			if ss >= 0 {
				sec = g.sectorForSubSector(ss)
			}
		}
		if sec < 0 {
			sec = g.sectorAt(p.x, p.y)
		}
		target := 0
		targetType := 0
		if p.sourcePlayer {
			target = 1
			targetType = 0
		} else if p.sourceThing >= 0 && g.m != nil && p.sourceThing < len(g.m.Things) {
			target = 1
			targetType = demoTraceThingType(g.m.Things[p.sourceThing].Type)
		}
		ordered = append(ordered, orderedDemoTraceMobj{
			order: p.order,
			idx:   -1,
			mobj: demoTraceMobj{
				Type:         demoTraceProjectileType(p),
				X:            p.x,
				Y:            p.y,
				Z:            p.z,
				Angle:        p.angle,
				MomX:         p.vx,
				MomY:         p.vy,
				MomZ:         p.vz,
				FloorZ:       p.floorz,
				CeilingZ:     p.ceilz,
				Radius:       p.radius,
				Height:       p.height,
				Tics:         p.frameTics,
				State:        demoTraceProjectileState(p),
				Flags:        demoTraceProjectileFlags(p),
				Health:       1000,
				Movedir:      0,
				Movecount:    0,
				ReactionTime: 8,
				Threshold:    0,
				LastLook:     p.lastLook,
				Subsector:    boolToInt(ss >= 0),
				Sector:       sec,
				Player:       0,
				Target:       target,
				TargetType:   targetType,
				Tracer:       0,
				Kind:         "projectile",
			}})
	}
	for _, fx := range g.projectileImpacts {
		ss := -1
		sec := -1
		if g.m != nil && len(g.m.SubSectors) > 0 {
			ss = g.subSectorAtFixed(fx.x, fx.y)
			if ss >= 0 {
				sec = g.sectorForSubSector(ss)
			}
		}
		if sec < 0 {
			sec = g.sectorAt(fx.x, fx.y)
		}
		target := 0
		targetType := 0
		if fx.sourcePlayer {
			target = 1
			targetType = 0
		} else if fx.sourceThing >= 0 && g.m != nil && fx.sourceThing < len(g.m.Things) {
			target = 1
			targetType = demoTraceThingType(g.m.Things[fx.sourceThing].Type)
		}
		ordered = append(ordered, orderedDemoTraceMobj{
			order: fx.order,
			idx:   -1,
			mobj: demoTraceMobj{
				Type:         demoTraceProjectileImpactType(fx.kind),
				X:            fx.x,
				Y:            fx.y,
				Z:            fx.z,
				Angle:        fx.angle,
				MomX:         0,
				MomY:         0,
				MomZ:         0,
				FloorZ:       fx.floorz,
				CeilingZ:     fx.ceilz,
				Radius:       demoTraceProjectileImpactRadius(fx.kind),
				Height:       8 * fracUnit,
				Tics:         fx.phaseTics,
				State:        demoTraceProjectileImpactState(fx.kind, fx.phase),
				Flags:        1552,
				Health:       1000,
				Movedir:      0,
				Movecount:    0,
				ReactionTime: 8,
				Threshold:    0,
				LastLook:     fx.lastLook,
				Subsector:    boolToInt(ss >= 0),
				Sector:       sec,
				Player:       0,
				Target:       target,
				TargetType:   targetType,
				Tracer:       0,
			},
		})
	}
	for _, p := range g.hitscanPuffs {
		ss := -1
		sec := -1
		if g.m != nil && len(g.m.SubSectors) > 0 {
			ss = g.subSectorAtFixed(p.x, p.y)
			if ss >= 0 {
				sec = g.sectorForSubSector(ss)
			}
		}
		if sec < 0 {
			sec = g.sectorAt(p.x, p.y)
		}
		floorZ := p.floorz
		ceilZ := p.ceilz
		mobjType := 37
		flags := 528
		if p.kind == hitscanFxBlood {
			mobjType = 38
			flags = 16
		}
		ordered = append(ordered, orderedDemoTraceMobj{
			order: p.order,
			idx:   -1,
			mobj: demoTraceMobj{
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
				Subsector:    boolToInt(ss >= 0),
				Sector:       sec,
				Player:       0,
				Target:       0,
				TargetType:   0,
				Tracer:       0,
				TracerType:   0,
			}})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})
	out := make([]demoTraceMobj, 0, len(ordered))
	for _, item := range ordered {
		out = append(out, item.mobj)
	}
	if want := os.Getenv("GD_DEBUG_TRACE_MOBJ"); want != "" {
		var wantTic, wantOrdinal int
		if _, err := fmt.Sscanf(want, "%d:%d", &wantTic, &wantOrdinal); err == nil && (g.demoTick-1 == wantTic || g.worldTic == wantTic) {
			for idx, item := range ordered {
				if idx == wantOrdinal {
					fmt.Printf("trace-mobj-debug tic=%d ordinal=%d order=%d idx=%d type=%d x=%d y=%d sector=%d kind=%s target=%d target_type=%d\n",
						g.demoTick-1, idx, item.order, item.idx, item.mobj.Type, item.mobj.X, item.mobj.Y, item.mobj.Sector, item.mobj.Kind, item.mobj.Target, item.mobj.TargetType)
				}
			}
		}
	}
	return out
}

func demoTraceProjectileType(p projectile) int {
	switch p.kind {
	case projectileFireball:
		if p.sourceType == 3003 || p.sourceType == 69 {
			return 16 // MT_BRUISERSHOT
		}
		return 31 // MT_TROOPSHOT
	case projectilePlasmaBall:
		if p.sourceType == 68 {
			return 36 // MT_ARACHPLAZ
		}
		return 32 // MT_HEADSHOT
	case projectileBaronBall:
		return 16 // MT_BRUISERSHOT
	case projectileTracer:
		return 6 // MT_TRACER
	case projectileFatShot:
		return 9 // MT_FATSHOT
	case projectileRocket:
		return 33 // MT_ROCKET
	case projectilePlayerPlasma:
		return 34 // MT_PLASMA
	case projectileBFGBall:
		return 35 // MT_BFG
	default:
		return 0
	}
}

func demoTraceProjectileState(p projectile) int {
	switch p.kind {
	case projectileFireball:
		if p.sourceType == 3003 || p.sourceType == 69 {
			if p.frame&1 != 0 {
				return 523 // S_BRBALL2
			}
			return 522 // S_BRBALL1
		}
		if p.frame&1 != 0 {
			return 98 // S_TBALL2
		}
		return 97 // S_TBALL1
	case projectilePlasmaBall:
		if p.sourceType == 68 {
			if p.frame&1 != 0 {
				return 668 // S_ARACH_PLAZ2
			}
			return 667 // S_ARACH_PLAZ
		}
		if p.frame&1 != 0 {
			return 103 // S_RBALL2
		}
		return 102 // S_RBALL1
	case projectileBaronBall:
		if p.frame&1 != 0 {
			return 523 // S_BRBALL2
		}
		return 522 // S_BRBALL1
	case projectileTracer:
		if p.frame&1 != 0 {
			return 317 // S_TRACER2
		}
		return 316 // S_TRACER
	case projectileFatShot:
		if p.frame&1 != 0 {
			return 358 // S_FATSHOT2
		}
		return 357 // S_FATSHOT1
	case projectileRocket:
		return 114 // S_ROCKET
	case projectilePlayerPlasma:
		if p.frame&1 != 0 {
			return 108 // S_PLASBALL2
		}
		return 107 // S_PLASBALL
	case projectileBFGBall:
		if p.frame&1 != 0 {
			return 116 // S_BFGSHOT2
		}
		return 115 // S_BFGSHOT
	default:
		return -1
	}
}

func demoTraceProjectileFlags(_ projectile) int {
	return 67088
}

func demoTraceProjectileImpactType(kind projectileKind) int {
	switch kind {
	case projectileFireball:
		return 31
	case projectilePlasmaBall:
		return 32
	case projectileBaronBall:
		return 16
	case projectileTracer:
		return 6
	case projectileFatShot:
		return 9
	case projectileRocket:
		return 33
	case projectilePlayerPlasma:
		return 34
	case projectileBFGBall:
		return 35
	default:
		return 0
	}
}

func demoTraceProjectileImpactState(kind projectileKind, phase int) int {
	switch kind {
	case projectileFireball:
		return []int{99, 100, 101}[clampDemoPhase(phase, 3)]
	case projectilePlasmaBall:
		return []int{104, 105, 106}[clampDemoPhase(phase, 3)]
	case projectileBaronBall:
		return []int{524, 525, 526}[clampDemoPhase(phase, 3)]
	case projectileTracer:
		return []int{318, 319, 320}[clampDemoPhase(phase, 3)]
	case projectileFatShot:
		return []int{359, 360, 361}[clampDemoPhase(phase, 3)]
	case projectileRocket:
		return []int{127, 128, 129}[clampDemoPhase(phase, 3)]
	case projectilePlayerPlasma:
		return []int{109, 110, 111}[clampDemoPhase(phase, 3)]
	case projectileBFGBall:
		return []int{117, 118, 119, 120, 121, 122}[clampDemoPhase(phase, 6)]
	default:
		return -1
	}
}

func demoTraceProjectileImpactRadius(kind projectileKind) int64 {
	switch kind {
	case projectileRocket, projectileTracer:
		return 11 * fracUnit
	case projectilePlayerPlasma:
		return 13 * fracUnit
	case projectileBFGBall:
		return 13 * fracUnit
	default:
		return 6 * fracUnit
	}
}

func clampDemoPhase(phase, n int) int {
	if phase < 0 {
		return 0
	}
	if phase >= n {
		return n - 1
	}
	return phase
}

func (g *game) demoTracePlayerMobjState() (state int, tics int) {
	if g == nil {
		return 0, 0
	}
	return g.playerMobjState, g.playerMobjTics
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
			return 1, demoTraceThingType(g.m.Things[idx].Type)
		}
	}
	return 0, 0
}

func (g *game) demoTraceSpecials() []map[string]any {
	if g == nil {
		return nil
	}
	type orderedSpecial struct {
		order int64
		item  map[string]any
	}
	ordered := make([]orderedSpecial, 0, len(g.doors)+len(g.floors)+len(g.plats)+len(g.ceilings))

	doorKeys := sortedIntKeys(g.doors)
	for _, sec := range doorKeys {
		d := g.doors[sec]
		entry := map[string]any{
			"kind":         "door",
			"sector":       sec,
			"type":         doomDoorType(d.typ),
			"topheight":    d.topHeight,
			"speed":        d.speed,
			"direction":    d.direction,
			"topwait":      d.topWait,
			"topcountdown": d.topCountdown,
		}
		if os.Getenv("GD_TRACE_DEBUG_DOOR_HEIGHT") != "" && sec >= 0 && sec < len(g.sectorCeil) {
			entry["currentceil"] = g.sectorCeil[sec]
		}
		ordered = append(ordered, orderedSpecial{order: d.order, item: entry})
	}
	floorKeys := sortedIntKeys(g.floors)
	for _, sec := range floorKeys {
		f := g.floors[sec]
		ordered = append(ordered, orderedSpecial{order: f.order, item: map[string]any{
			"kind":            "floor",
			"sector":          sec,
			"type":            f.typ,
			"crush":           boolToInt(f.crush),
			"speed":           f.speed,
			"direction":       f.direction,
			"floordestheight": f.destHeight,
			"texture":         f.finishFlat,
			"newspecial":      int16(f.finishSpecial),
		}})
	}
	platKeys := sortedIntKeys(g.plats)
	for _, sec := range platKeys {
		p := g.plats[sec]
		tag := 0
		if g.m != nil && sec >= 0 && sec < len(g.m.Sectors) {
			tag = int(g.m.Sectors[sec].Tag)
		}
		item := map[string]any{
			"kind":      "plat",
			"sector":    sec,
			"type":      doomPlatType(p.typ),
			"speed":     p.speed,
			"low":       p.low,
			"high":      p.high,
			"wait":      p.wait,
			"count":     p.count,
			"status":    doomPlatStatus(p.status),
			"oldstatus": doomPlatStatus(p.oldStatus),
			"crush":     0,
			"tag":       tag,
		}
		if p.finishSpecial != 0 {
			item["finishspecial"] = int16(p.finishSpecial)
		}
		if p.finishFlat != "" {
			item["texture"] = p.finishFlat
		}
		ordered = append(ordered, orderedSpecial{order: p.order, item: item})
	}
	ceilingKeys := sortedIntKeys(g.ceilings)
	for _, sec := range ceilingKeys {
		c := g.ceilings[sec]
		ordered = append(ordered, orderedSpecial{order: c.order, item: map[string]any{
			"kind":         "ceiling",
			"sector":       sec,
			"action":       string(c.action),
			"speed":        c.speed,
			"direction":    c.direction,
			"topheight":    c.topHeight,
			"bottomheight": c.bottomHeight,
			"crush":        boolToInt(c.crush),
			"olddirection": c.oldDirection,
		}})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].order != ordered[j].order {
			return ordered[i].order < ordered[j].order
		}
		ik, _ := ordered[i].item["kind"].(string)
		jk, _ := ordered[j].item["kind"].(string)
		if ik != jk {
			return ik < jk
		}
		is, _ := ordered[i].item["sector"].(int)
		js, _ := ordered[j].item["sector"].(int)
		return is < js
	})
	out := make([]map[string]any, 0, len(ordered))
	for _, entry := range ordered {
		out = append(out, entry.item)
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

const demoTraceStateGibs = 895

const (
	demoTraceFlagSpecial   = 0x00000001
	demoTraceFlagSolid     = 0x00000002
	demoTraceFlagShootable = 0x00000004
	demoTraceFlagAmbush    = 0x00000020
	demoTraceFlagNoGravity = 0x00000200
	demoTraceFlagDropoff   = 0x00000400
	demoTraceFlagFloat     = 0x00004000
	demoTraceFlagDropped   = 0x00020000
	demoTraceFlagCorpse    = 0x00100000
	demoTraceFlagCountKill = 0x00400000
)

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
	if i < len(g.thingGibbed) && g.thingGibbed[i] {
		return -1
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
	if i >= 0 && i < len(g.thingGibbed) && g.thingGibbed[i] {
		return demoTraceStateGibs
	}
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
			if i >= 0 && i < len(g.thingStatePhase) {
				xdeath := i >= 0 && i < len(g.thingXDeath) && g.thingXDeath[i]
				if state, ok := demoTraceMonsterDeathState(typ, g.thingStatePhase[i], xdeath); ok {
					return state
				}
			}
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
		if i >= 0 && i < len(g.thingStatePhase) {
			xdeath := i >= 0 && i < len(g.thingXDeath) && g.thingXDeath[i]
			if state, ok := demoTraceMonsterDeathState(typ, g.thingStatePhase[i], xdeath); ok {
				return state
			}
		}
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

func demoTraceMonsterDeathState(typ int16, phase int, xdeath bool) (int, bool) {
	base := 0
	count := 0
	if xdeath {
		switch typ {
		case 3004:
			base, count = 194, 9
		case 9:
			base, count = 227, 9
		case 65:
			base, count = 429, 6
		case 84:
			base, count = 749, 9
		default:
			return 0, false
		}
	} else {
		switch typ {
		case 3004:
			base, count = 189, 5
		case 9:
			base, count = 222, 5
		case 3001:
			base, count = 457, 5
		case 3002, 58:
			base, count = 490, 6
		case 3005:
			base, count = 510, 6
		case 3003:
			base, count = 542, 7
		case 69:
			base, count = 571, 7
		case 3006:
			base, count = 595, 6
		case 7:
			base, count = 621, 10
		case 16:
			base, count = 691, 9
		default:
			return 0, false
		}
	}
	if phase < 0 || phase >= count {
		return 0, false
	}
	return base + phase, true
}

func demoTraceMonsterSpawnState(typ int16, phase int) (int, bool) {
	base := 0
	count := 0
	switch typ {
	case 3004:
		base, count = 174, 2
	case 9:
		base, count = 207, 2
	case 65:
		base, count = 582, 2
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
	case 65:
		base, count = 584, 8
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
	case 65:
		base = 596
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

func demoTraceThingFlags(g *game, i int, th mapdata.Thing) int {
	flags := 0
	if int(th.Flags)&thingFlagAmbush != 0 {
		flags |= demoTraceFlagAmbush
	}
	switch {
	case isMonster(th.Type):
		flags |= demoTraceFlagSolid | demoTraceFlagCountKill
		if i >= 0 && i < len(g.thingDead) && g.thingDead[i] {
			flags |= demoTraceFlagDropoff | demoTraceFlagCorpse
		} else {
			flags |= demoTraceFlagShootable
			if monsterCanFloat(th.Type) {
				flags |= demoTraceFlagFloat | demoTraceFlagNoGravity
			}
		}
	case isPickupType(th.Type):
		flags |= demoTraceFlagSpecial
		if i >= 0 && i < len(g.thingDropped) && g.thingDropped[i] {
			flags |= demoTraceFlagDropped
		}
	case isBarrelThingType(th.Type):
		flags |= demoTraceFlagSolid | demoTraceFlagShootable
	}
	return flags
}

func demoTraceThingReaction(g *game, i int, typ int16) int {
	if i >= 0 && i < len(g.thingReactionTics) && g.thingReactionTics[i] > 0 {
		return g.thingReactionTics[i]
	}
	_ = typ
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
