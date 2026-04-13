package doomruntime

import (
	"fmt"
	"strings"
)

const wasmSpawnPrewarmMonsterRadius = 100 * playerRadius

func (s *soundSystem) prewarmEvent(ev soundEvent) bool {
	if s == nil || s.player == nil {
		return false
	}
	sample, ok := s.sampleForEvent(ev)
	if !ok || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return false
	}
	return s.player.PrewarmSample(sample)
}

func (s *soundSystem) prewarmRemaining() int {
	if s == nil || s.player == nil {
		return 0
	}
	return s.player.PrewarmRemaining()
}

func (g *game) prewarmMapStartSounds() {
	if g == nil || g.snd == nil {
		return
	}
	loaded, full := g.wasmMapStartPrewarmEvents()
	if len(loaded) == 0 {
		return
	}
	parts := make([]string, 0, len(loaded))
	for _, ev := range loaded {
		parts = append(parts, prewarmEventLabel(ev))
	}
	status := ""
	if full {
		status = " full"
	}
	fmt.Printf("wasm-prewarm map=%s count=%d%s events=%s\n", strings.TrimSpace(string(g.m.Name)), len(loaded), status, strings.Join(parts, ","))
}

func (g *game) wasmMapStartPrewarmEvents() ([]soundEvent, bool) {
	if g == nil || g.m == nil || g.snd == nil {
		return nil, false
	}
	loaded := make([]soundEvent, 0, 32)
	seen := make(map[soundEvent]struct{}, 32)
	tryAdd := func(ev soundEvent) bool {
		if ev < 0 {
			return false
		}
		if _, ok := seen[ev]; ok {
			return false
		}
		if g.snd.prewarmRemaining() <= 0 {
			return true
		}
		if !g.snd.prewarmEvent(ev) {
			return false
		}
		seen[ev] = struct{}{}
		loaded = append(loaded, ev)
		return false
	}
	runBucket := func(events []soundEvent) bool {
		for _, ev := range events {
			if tryAdd(ev) {
				return true
			}
		}
		return false
	}

	world, projectile, monsters := g.wasmMapStartPrewarmBuckets()
	if runBucket(world) {
		return loaded, true
	}
	if runBucket(projectile) {
		return loaded, true
	}
	if runBucket(monsters) {
		return loaded, true
	}
	return loaded, false
}

func (g *game) wasmMapStartPrewarmBuckets() ([]soundEvent, []soundEvent, []soundEvent) {
	if g == nil || g.m == nil {
		return nil, nil, nil
	}
	world := make([]soundEvent, 0, 24)
	projectile := make([]soundEvent, 0, 24)
	monsters := make([]soundEvent, 0, 24)
	worldSeen := make(map[soundEvent]struct{}, 24)
	projectileSeen := make(map[soundEvent]struct{}, 24)
	monsterSeen := make(map[soundEvent]struct{}, 24)
	addWorld := func(ev soundEvent) {
		if ev < 0 {
			return
		}
		if _, ok := worldSeen[ev]; ok {
			return
		}
		worldSeen[ev] = struct{}{}
		world = append(world, ev)
	}
	addProjectile := func(ev soundEvent) {
		if ev < 0 {
			return
		}
		if _, ok := projectileSeen[ev]; ok {
			return
		}
		projectileSeen[ev] = struct{}{}
		projectile = append(projectile, ev)
	}
	addMonster := func(ev soundEvent) {
		if ev < 0 {
			return
		}
		if _, ok := monsterSeen[ev]; ok {
			return
		}
		monsterSeen[ev] = struct{}{}
		monsters = append(monsters, ev)
	}

	for _, ev := range []soundEvent{
		soundEventItemUp,
		soundEventWeaponUp,
		soundEventPowerUp,
		soundEventSwitchOn,
		soundEventSwitchExit,
		soundEventNoWay,
		soundEventTeleport,
	} {
		addWorld(ev)
	}

	for _, id := range weaponCycleOrder() {
		if !g.weaponOwned(id) {
			continue
		}
		for _, ev := range weaponWorldPrewarmEvents(id) {
			addWorld(ev)
		}
		for _, ev := range weaponProjectilePrewarmEvents(id) {
			addProjectile(ev)
		}
	}

	for _, th := range g.m.Things {
		switch th.Type {
		case 88:
			addProjectile(soundEventBossBrainAwake)
			addProjectile(soundEventBossBrainSpit)
			addProjectile(soundEventBossBrainCube)
		}
		for _, ev := range projectilePrewarmEventsForThingType(th.Type) {
			addProjectile(ev)
		}
		if !isMonster(th.Type) {
			continue
		}
		tx := int64(th.X) << fracBits
		ty := int64(th.Y) << fracBits
		if doomApproxDistance(tx-g.p.x, ty-g.p.y) > wasmSpawnPrewarmMonsterRadius {
			continue
		}
		for _, ev := range monsterSeePrewarmEvents(th.Type) {
			addMonster(ev)
		}
		addMonster(monsterActiveSoundEvent(th.Type))
		addMonster(monsterPainSoundEvent(th.Type))
		for _, ev := range monsterAttackPrewarmEvents(th.Type) {
			addMonster(ev)
		}
	}

	return world, projectile, monsters
}

func weaponWorldPrewarmEvents(id weaponID) []soundEvent {
	switch id {
	case weaponFist:
		return []soundEvent{soundEventPunch}
	case weaponPistol, weaponChaingun:
		return []soundEvent{soundEventShootPistol}
	case weaponShotgun:
		return []soundEvent{soundEventShootShotgun}
	case weaponSuperShotgun:
		return []soundEvent{
			soundEventShootSuperShotgun,
			soundEventShotgunOpen,
			soundEventShotgunLoad,
			soundEventShotgunClose,
		}
	case weaponChainsaw:
		return []soundEvent{
			soundEventSawUp,
			soundEventSawIdle,
			soundEventSawFull,
			soundEventSawHit,
		}
	default:
		return nil
	}
}

func weaponProjectilePrewarmEvents(id weaponID) []soundEvent {
	switch id {
	case weaponRocketLauncher:
		return []soundEvent{
			soundEventShootRocket,
			soundEventBarrelExplode,
		}
	case weaponPlasma:
		return []soundEvent{
			soundEventShootPlasma,
			soundEventImpactFire,
		}
	case weaponBFG:
		return []soundEvent{
			soundEventShootBFG,
			soundEventImpactRocket,
		}
	default:
		return nil
	}
}

func projectilePrewarmEventsForThingType(typ int16) []soundEvent {
	switch {
	case usesMonsterProjectile(typ):
		return []soundEvent{
			projectileLaunchSoundEvent(typ),
			projectileImpactSoundEvent(monsterProjectileKind(typ)),
		}
	case typ == 88:
		return []soundEvent{
			soundEventBossBrainSpit,
			soundEventBossBrainCube,
		}
	default:
		return nil
	}
}

func monsterSeePrewarmEvents(typ int16) []soundEvent {
	switch typ {
	case 3004, 9, 65:
		return []soundEvent{
			soundEventMonsterSeePosit1,
			soundEventMonsterSeePosit2,
			soundEventMonsterSeePosit3,
		}
	case 3001:
		return []soundEvent{
			soundEventMonsterSeeImp1,
			soundEventMonsterSeeImp2,
		}
	case 3002, 58:
		return []soundEvent{soundEventMonsterSeeDemon}
	case 3005:
		return []soundEvent{soundEventMonsterSeeCaco}
	case 3003:
		return []soundEvent{soundEventMonsterSeeBaron}
	case 69:
		return []soundEvent{soundEventMonsterSeeKnight}
	case 7:
		return []soundEvent{soundEventMonsterSeeSpider}
	case 68:
		return []soundEvent{soundEventMonsterSeeArachnotron}
	case 16:
		return []soundEvent{soundEventMonsterSeeCyber}
	case 71:
		return []soundEvent{soundEventMonsterSeePainElemental}
	case 84:
		return []soundEvent{soundEventMonsterSeeWolfSS}
	case 64:
		return []soundEvent{soundEventMonsterSeeArchvile}
	case 66:
		return []soundEvent{soundEventMonsterSeeRevenant}
	default:
		return nil
	}
}

func monsterAttackPrewarmEvents(typ int16) []soundEvent {
	out := make([]soundEvent, 0, 2)
	if ev := monsterMeleeAttackSoundEvent(typ); ev >= 0 {
		out = append(out, ev)
	}
	if ev := monsterAttackStateEntrySoundEvent(typ); ev >= 0 {
		out = append(out, ev)
	}
	return out
}

func prewarmEventLabel(ev soundEvent) string {
	if dsName, ok := soundEventDSName(ev); ok {
		return dsName
	}
	return fmt.Sprintf("sound:%d", ev)
}
