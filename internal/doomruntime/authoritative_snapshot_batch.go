package doomruntime

import (
	"fmt"
	"maps"
)

// SnapshotBatch captures the shared world once for every requested viewer. Like
// Step and Snapshot, it belongs to the simulation owner; the returned payloads
// are independent immutable buffers, while temporary capture arrays stay local.
func (a *Authority) SnapshotBatch(viewers []byte) (map[byte][]byte, error) {
	states := make(map[byte][]byte, min(len(viewers), 4))
	if len(viewers) == 0 {
		return states, nil
	}
	for _, viewer := range viewers {
		if a == nil || viewer < 1 || viewer > 4 || a.players[viewer] == nil {
			return nil, fmt.Errorf("snapshot viewer %d is inactive", viewer)
		}
	}
	r, err := a.captureSharedAuthorityReplica()
	if err != nil {
		return nil, err
	}
	for _, viewer := range viewers {
		if _, exists := states[viewer]; exists {
			continue
		}
		r.Viewer = viewer
		for _, p := range r.Players {
			if p.LocalSlot == int(viewer) {
				applyReplicaViewerToSave(&r.Game, p)
				break
			}
		}
		r.SoundCursor, r.Sounds = a.snapshotSoundEvents(viewer)
		if err := validateAuthorityReplica(a.g, r); err != nil {
			return nil, fmt.Errorf("capture authority baseline: %w", err)
		}
		data, err := encodeAuthorityReplica(r)
		if err != nil {
			return nil, err
		}
		states[viewer] = data
	}
	return states, nil
}

func (a *Authority) captureSharedAuthorityReplica() (authorityReplica, error) {
	g := a.g
	mapHash, err := authorityMapHash(g)
	if err != nil {
		return authorityReplica{}, err
	}
	r := authorityReplica{
		Version: authorityReplicaVersion, Map: g.m.Name, MapHash: mapHash,
		WADHash: g.opts.WADHash, Tic: a.Tic(), Game: captureGameSaveState(g),
		ThingBlockOrder:       append([]int64(nil), g.thingBlockOrder...),
		ThingTelefragTick:     append([]int(nil), g.thingTelefragTick...),
		ThingTargetPlayerSlot: append([]int(nil), g.thingTargetPlayerSlot...),
		SectorSoundPlayerSlot: append([]int(nil), g.sectorSoundPlayerSlot...),
		BarrelSources:         maps.Clone(g.authorityBarrelSources),
		Players:               make([]authorityReplicaPlayer, 0, len(a.players)),
	}
	for id := 1; id <= 4; id++ {
		if p := a.players[id]; p != nil {
			r.Players = append(r.Players, captureReplicaPlayer(*p))
		}
	}
	for _, p := range g.projectiles {
		r.ProjectilePlayers = append(r.ProjectilePlayers, replicaProjectilePlayers{p.sourcePlayerSlot, p.tracerPlayerSlot, p.sourcePlayerGeneration, p.tracerPlayerGeneration})
	}
	for _, p := range g.projectileImpacts {
		r.ImpactPlayers = append(r.ImpactPlayers, replicaImpactPlayers{p.sourcePlayerSlot, p.fireTargetPlayerSlot, p.sourcePlayerGeneration, p.fireTargetPlayerGeneration})
	}
	if g.authorityRules != nil {
		rules := *g.authorityRules
		r.Rules = &rules
	}
	return r, nil
}

// Match the player-owned fields that applyAuthoritativePlayer followed by
// captureGameSaveState would produce, without changing the live game or copying
// its world arrays again. Inventory is already owned by the captured roster.
func applyReplicaViewerToSave(s *gameSaveState, p authorityReplicaPlayer) {
	s.Session.PlayerSlot = p.LocalSlot
	s.Player = p.P
	s.UseFlash, s.UseText = p.UseFlash, p.UseText
	s.PrevPX, s.PrevPY, s.PrevAngle = p.PrevPX, p.PrevPY, p.PrevAngle
	s.PlayerViewZ = p.PlayerViewZ
	s.CheatLevel, s.Invulnerable, s.NoClip = p.CheatLevel, p.Invulnerable, p.NoClip
	s.Inventory = p.Inventory
	s.LastAttackRange = p.LastAttackRange
	s.AlwaysRun, s.AutoWeaponSwitch = p.AlwaysRun, p.AutoWeaponSwitch
	s.WeaponRefire, s.WeaponAttackDown = p.WeaponRefire, p.WeaponAttackDown
	s.WeaponState, s.WeaponStateTics = int(p.WeaponState), p.WeaponStateTics
	s.WeaponFlashState, s.WeaponFlashTics = int(p.WeaponFlashState), p.WeaponFlashTics
	s.WeaponPSpriteY = p.WeaponPSpriteY
	s.Stats = p.Stats
	s.PlayerKillCount, s.PlayerItemCount = p.PlayerKillCount, p.PlayerItemCount
	s.PlayerBlockOrder = p.PlayerBlockOrder
	s.SecretsFound = p.SecretsFound
	s.IsDead, s.PlayerMobjHealth = p.IsDead, p.PlayerMobjHealth
	s.DamageFlashTic, s.BonusFlashTic = p.DamageFlashTic, p.BonusFlashTic
}
