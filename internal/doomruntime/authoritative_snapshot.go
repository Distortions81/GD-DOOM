package doomruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"strings"

	"gddoom/internal/mapdata"

	"github.com/zeebo/blake3"
)

const authorityReplicaVersion = 1

// MaxAuthoritySnapshotBytes includes framing, JSON payload, and checksum.
const MaxAuthoritySnapshotBytes = 8 << 20

var authorityReplicaMagic = []byte("GDDOOMAUTH\x00")

// authorityReplica is a complete current-state baseline for presentation and
// local movement correction. Clients must NOT tick AI, weapons, or world movers
// from it. It deliberately excludes server-only transient execution state,
// queues and RNG, so it is not a server rollback/restart snapshot.
//
// Independent full baselines can supersede lost snapshots. The match protocol
// owns sequence numbers, input acknowledgements, and epoch validation.
type authorityReplica struct {
	Version               int
	Map                   mapdata.MapName
	MapHash               [32]byte
	WADHash               string
	Viewer                byte
	Tic                   uint32
	Game                  gameSaveState
	Players               []authorityReplicaPlayer
	ThingBlockOrder       []int64
	ThingTelefragTick     []int
	ThingTargetPlayerSlot []int
	SectorSoundPlayerSlot []int
	ProjectilePlayers     []replicaProjectilePlayers
	ImpactPlayers         []replicaImpactPlayers
	BarrelSources         map[int]authorityBarrelSource
	SoundCursor           uint64
	Sounds                []authoritySoundEvent
	Rules                 *authorityRulesState
}

type replicaProjectilePlayers struct {
	SourceSlot, TracerSlot             int
	SourceGeneration, TracerGeneration uint32
}

type replicaImpactPlayers struct {
	SourceSlot, FireTargetSlot             int
	SourceGeneration, FireTargetGeneration uint32
}

// Snapshot creates an immutable, bounded baseline for an active player's view.
// Capture must run on the same simulation owner goroutine as Step.
func (a *Authority) Snapshot(viewer byte) ([]byte, error) {
	if viewer < 1 || viewer > 4 || a.players[viewer] == nil {
		return nil, fmt.Errorf("snapshot viewer %d is inactive", viewer)
	}
	g := a.g
	mapHash, err := authorityMapHash(g)
	if err != nil {
		return nil, err
	}
	saved := g.captureAuthoritativePlayer()
	g.applyAuthoritativePlayer(*a.players[viewer])
	common := captureGameSaveState(g)
	g.applyAuthoritativePlayer(saved)
	common.Session.PlayerSlot = int(viewer)
	r := authorityReplica{
		Version: authorityReplicaVersion, Map: g.m.Name, MapHash: mapHash,
		WADHash: g.opts.WADHash, Viewer: viewer, Tic: a.Tic(), Game: common,
		ThingBlockOrder:       append([]int64(nil), g.thingBlockOrder...),
		ThingTelefragTick:     append([]int(nil), g.thingTelefragTick...),
		ThingTargetPlayerSlot: append([]int(nil), g.thingTargetPlayerSlot...),
		SectorSoundPlayerSlot: append([]int(nil), g.sectorSoundPlayerSlot...),
		BarrelSources:         maps.Clone(g.authorityBarrelSources),
	}
	r.SoundCursor, r.Sounds = a.snapshotSoundEvents(viewer)
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
	if err := validateAuthorityReplica(g, r); err != nil {
		return nil, fmt.Errorf("capture authority baseline: %w", err)
	}
	return encodeAuthorityReplica(r)
}

func authorityMapHash(g *game) ([32]byte, error) {
	m := g.restartTemplate
	if m == nil {
		m = g.m
	}
	data, err := json.Marshal(m)
	if err != nil {
		return [32]byte{}, fmt.Errorf("encode static map: %w", err)
	}
	return blake3.Sum256(data), nil
}

func encodeAuthorityReplica(r authorityReplica) ([]byte, error) {
	payload, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if len(payload)+len(authorityReplicaMagic)+32 > MaxAuthoritySnapshotBytes {
		return nil, fmt.Errorf("authority baseline exceeds %d bytes", MaxAuthoritySnapshotBytes)
	}
	data := make([]byte, 0, len(authorityReplicaMagic)+len(payload)+32)
	data = append(data, authorityReplicaMagic...)
	data = append(data, payload...)
	sum := blake3.Sum256(data)
	return append(data, sum[:]...), nil
}

func decodeAuthorityReplica(data []byte) (authorityReplica, error) {
	var r authorityReplica
	if len(data) < len(authorityReplicaMagic)+32 || len(data) > MaxAuthoritySnapshotBytes || !bytes.HasPrefix(data, authorityReplicaMagic) {
		return r, fmt.Errorf("invalid authority baseline framing or size")
	}
	end := len(data) - 32
	sum := blake3.Sum256(data[:end])
	if !bytes.Equal(sum[:], data[end:]) {
		return r, fmt.Errorf("authority baseline checksum mismatch")
	}
	d := json.NewDecoder(bytes.NewReader(data[len(authorityReplicaMagic):end]))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return r, fmt.Errorf("decode authority baseline: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return r, fmt.Errorf("unexpected trailing authority baseline data")
	}
	if r.Version != authorityReplicaVersion {
		return r, fmt.Errorf("unsupported authority baseline version %d", r.Version)
	}
	return r, nil
}

func validateAuthorityReplica(g *game, r authorityReplica) error {
	if g == nil || g.m == nil || r.Version != authorityReplicaVersion || r.Map != g.m.Name || r.WADHash != g.opts.WADHash {
		return fmt.Errorf("authority baseline does not match loaded content")
	}
	hash, err := authorityMapHash(g)
	if err != nil || hash != r.MapHash {
		return fmt.Errorf("authority baseline static map mismatch")
	}
	s := r.Game
	if s.WorldTic < 0 || uint64(s.WorldTic) != uint64(r.Tic) || s.Session.PlayerSlot != int(r.Viewer) {
		return fmt.Errorf("authority baseline tic/viewer mismatch")
	}
	if (s.Session.GameMode != gameModeCoop && s.Session.GameMode != gameModeDeathmatch) || s.Session.SkillLevel < 1 || s.Session.SkillLevel > 5 {
		return fmt.Errorf("authority baseline has invalid session rules")
	}
	n, sectors := len(s.Things), len(g.m.Sectors)
	base := g.restartTemplate
	if base == nil {
		base = g.m
	}
	if n < len(base.Things) || n > 65536 || len(s.Sectors) != sectors || len(s.Sidedefs) != len(g.m.Sidedefs) || len(s.LineSpecial) != len(g.m.Linedefs) {
		return fmt.Errorf("authority baseline map shape mismatch")
	}
	for _, length := range []int{len(s.SectorFloor), len(s.SectorCeil), len(s.SecretFound), len(s.SectorSoundTarget)} {
		if length != sectors {
			return fmt.Errorf("authority baseline sector array mismatch")
		}
	}
	for _, values := range [][]int64{s.SectorFloor, s.SectorCeil, s.ThingX, s.ThingY, s.ThingZState, s.ThingFloorState, s.ThingCeilState} {
		for _, value := range values {
			if value < math.MinInt32 || value > math.MaxInt32 {
				return fmt.Errorf("authority baseline world coordinate out of range")
			}
		}
	}
	// Thing state is stored in parallel arrays. Some lazy AI arrays can be empty
	// before their first tick; nonempty arrays must always cover the whole world.
	v, typ := reflect.ValueOf(s), reflect.TypeOf(s)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if strings.HasPrefix(typ.Field(i).Name, "Thing") && field.Kind() == reflect.Slice && field.Len() != 0 && field.Len() != n {
			return fmt.Errorf("authority baseline %s shape mismatch", typ.Field(i).Name)
		}
	}
	for _, length := range []int{len(s.ThingX), len(s.ThingY), len(s.ThingZState), len(s.ThingFloorState), len(s.ThingCeilState), len(s.ThingHP), len(s.ThingDead), len(s.ThingCollected), len(s.ThingDropped), len(r.ThingBlockOrder)} {
		if length != n {
			return fmt.Errorf("authority baseline collision array mismatch")
		}
	}
	for _, pair := range [][2]int{{len(r.ThingTelefragTick), n}, {len(r.ThingTargetPlayerSlot), n}, {len(r.SectorSoundPlayerSlot), sectors}, {len(s.SectorLightFx), sectors}} {
		if pair[0] != 0 && pair[0] != pair[1] {
			return fmt.Errorf("authority baseline supplemental array mismatch")
		}
	}
	for i, sd := range s.Sidedefs {
		if sd.Sector != g.m.Sidedefs[i].Sector {
			return fmt.Errorf("authority baseline changed static sector connectivity")
		}
	}
	for _, slot := range append(append([]int(nil), r.ThingTargetPlayerSlot...), r.SectorSoundPlayerSlot...) {
		if slot < 0 || slot > 4 {
			return fmt.Errorf("authority baseline has invalid actor player target")
		}
	}
	if len(r.ProjectilePlayers) != len(s.Projectiles) || len(r.ImpactPlayers) != len(s.ProjectileImpacts) {
		return fmt.Errorf("authority baseline projectile identity mismatch")
	}
	for _, p := range r.ProjectilePlayers {
		if p.SourceSlot < 0 || p.SourceSlot > 4 || p.TracerSlot < 0 || p.TracerSlot > 4 {
			return fmt.Errorf("authority baseline projectile identity invalid")
		}
	}
	for _, p := range r.ImpactPlayers {
		if p.SourceSlot < 0 || p.SourceSlot > 4 || p.FireTargetSlot < 0 || p.FireTargetSlot > 4 {
			return fmt.Errorf("authority baseline impact identity invalid")
		}
	}
	for index, owner := range r.BarrelSources {
		if index < 0 || index >= n || s.Things[index].Type != barrelThingType || owner.PlayerSlot < 0 || owner.PlayerSlot > 4 || owner.Thing < -1 || owner.Thing >= n {
			return fmt.Errorf("authority baseline barrel identity invalid")
		}
	}
	if len(r.Players) < 1 || len(r.Players) > 4 || r.Viewer < 1 || r.Viewer > 4 {
		return fmt.Errorf("authority baseline player roster is invalid")
	}
	if len(r.Sounds) > authorityEventCapacity {
		return fmt.Errorf("authority baseline sound history exceeds limit")
	}
	var lastSoundID uint64
	for _, event := range r.Sounds {
		if event.ID <= lastSoundID || event.ID > r.SoundCursor || event.Tick > r.Tic || event.Kind < soundEventDoorOpen || event.Kind > soundEventMonsterRaise || event.LocalPlayer > 4 || event.Audience > 4 || event.Audience != 0 && event.Audience != r.Viewer || event.X < math.MinInt32 || event.X > math.MaxInt32 || event.Y < math.MinInt32 || event.Y > math.MaxInt32 {
			return fmt.Errorf("authority baseline sound event invalid")
		}
		lastSoundID = event.ID
	}
	if r.Rules == nil || r.Rules.WinnerID > 4 || r.Rules.Config.FragLimit < 0 || r.Rules.SpawnCursor < 0 || r.Rules.StartedTic > r.Tic || len(r.Rules.EndReason) > 128 {
		return fmt.Errorf("authority baseline match rules are invalid")
	}
	found := false
	last := 0
	for _, p := range r.Players {
		if p.LocalSlot <= last || p.LocalSlot > 4 || p.LocalPlayerThingIndex < -1 || p.LocalPlayerThingIndex >= n || p.P.Sector < -1 || p.P.Sector >= sectors || p.P.Subsector < -1 || p.P.Subsector >= len(g.m.SubSectors) {
			return fmt.Errorf("authority baseline player identity/sector is invalid")
		}
		_, validWeapon := weaponPspriteDefs[p.WeaponState]
		_, validFlash := weaponPspriteDefs[p.WeaponFlashState]
		if p.CheatLevel != 0 || p.Invulnerable || p.NoClip || p.Inventory.ReadyWeapon < int(weaponFist) || p.Inventory.ReadyWeapon > int(weaponChainsaw) || (!validWeapon && p.WeaponState != weaponStateNone) || (!validFlash && p.WeaponFlashState != weaponStateNone) {
			return fmt.Errorf("authority baseline player state is invalid")
		}
		for _, value := range []int64{p.P.X, p.P.Y, p.P.Z, p.P.FloorZ, p.P.CeilZ, p.P.MomX, p.P.MomY, p.P.MomZ} {
			if value < math.MinInt32 || value > math.MaxInt32 {
				return fmt.Errorf("authority baseline player coordinate out of range")
			}
		}
		last = p.LocalSlot
		if r.Rules.Scores[p.LocalSlot].Generation == 0 || r.Rules.Scores[p.LocalSlot].Deaths < 0 || r.Rules.Scores[p.LocalSlot].DeathTic > r.Tic {
			return fmt.Errorf("authority baseline player generation or score is invalid")
		}
		if p.LocalSlot == int(r.Viewer) {
			found = true
			if s.Player != p.P || s.Stats != p.Stats || !reflect.DeepEqual(s.Inventory, p.Inventory) || s.WeaponState != int(p.WeaponState) || s.WeaponFlashState != int(p.WeaponFlashState) || s.CheatLevel != p.CheatLevel || s.Invulnerable != p.Invulnerable || s.NoClip != p.NoClip {
				return fmt.Errorf("authority baseline viewer state is inconsistent")
			}
		}
	}
	if !found {
		return fmt.Errorf("authority baseline viewer is not in roster")
	}
	return nil
}

// applyAuthoritySnapshot validates the entire untrusted baseline before changing
// the game. It preserves local presentation settings and never consumes or
// restores gameplay RNG. Callers must reject stale sequence/epoch envelopes.
func (g *game) applyAuthoritySnapshot(data []byte) error {
	r, err := decodeAuthorityReplica(data)
	if err != nil {
		return err
	}
	if err := validateAuthorityReplica(g, r); err != nil {
		return err
	}
	g.applyValidatedAuthorityReplica(r)
	return nil
}

func (g *game) applyValidatedAuthorityReplica(r authorityReplica) {
	s := r.Game
	alwaysRun := g.alwaysRun
	// Save-file snapshots include local UI choices; the authoritative server has
	// no authority over the client's renderer, automap, gamma, or HUD preferences.
	s.View, s.Mode, s.RotateView = g.State, int(g.mode), g.rotateView
	s.ParityReveal, s.ParityIDDT = int(g.parity.reveal), g.parity.iddt
	s.ShowGrid, s.ShowLegend = g.showGrid, g.showLegend
	s.PaletteLUTEnabled, s.GammaLevel, s.CRTEnabled = g.paletteLUTEnabled, g.gammaLevel, g.crtEnabled
	s.HUDMessagesEnabled = g.hudMessagesEnabled
	g.thingBlockOrder = append([]int64(nil), r.ThingBlockOrder...)
	restoreGameSaveState(g, s)
	g.thingTelefragTick = append([]int(nil), r.ThingTelefragTick...)
	g.thingTargetPlayerSlot = append([]int(nil), r.ThingTargetPlayerSlot...)
	g.sectorSoundPlayerSlot = append([]int(nil), r.SectorSoundPlayerSlot...)
	g.authorityBarrelSources = maps.Clone(r.BarrelSources)
	g.authorityRules = nil
	if r.Rules != nil {
		rules := *r.Rules
		g.authorityRules = &rules
	}
	for i, p := range r.ProjectilePlayers {
		g.projectiles[i].sourcePlayerSlot, g.projectiles[i].sourcePlayerGeneration = p.SourceSlot, p.SourceGeneration
		g.projectiles[i].tracerPlayerSlot, g.projectiles[i].tracerPlayerGeneration = p.TracerSlot, p.TracerGeneration
	}
	for i, p := range r.ImpactPlayers {
		g.projectileImpacts[i].sourcePlayerSlot, g.projectileImpacts[i].sourcePlayerGeneration = p.SourceSlot, p.SourceGeneration
		g.projectileImpacts[i].fireTargetPlayerSlot = p.FireTargetSlot
		g.projectileImpacts[i].fireTargetPlayerGeneration = p.FireTargetGeneration
	}
	g.authorityPlayers = make([]*authoritativePlayerState, 0, len(r.Players))
	g.remotePlayers = make(map[int]*remotePlayer, len(r.Players)-1)
	for _, wire := range r.Players {
		state := restoreReplicaPlayer(wire)
		g.authorityPlayers = append(g.authorityPlayers, &state)
		if state.localSlot == int(r.Viewer) {
			g.applyAuthoritativePlayer(state)
		} else {
			g.remotePlayers[state.localSlot] = &remotePlayer{slot: state.localSlot, p: state.p}
		}
	}
	// Run mode controls how this client builds commands. It is not simulation
	// state: the server receives the already encoded movement speed.
	g.alwaysRun = alwaysRun
	// Do not retain effects or caches from a superseded predicted timeline.
	g.soundQueue, g.soundQueueOrigin, g.delayedSfx = nil, nil, nil
	g.switchTextureBlends = nil
	g.extraDoors, g.recycledDoors = nil, nil
	g.initThingRenderState()
	g.sectorPlaneCache, g.sectorPlaneTris = nil, nil
	g.sectorLightCacheValid = false
	g.refreshPlayerSubsectorCache(g.p.x, g.p.y)
	// Discovery is local presentation. The client bypasses the ordinary
	// gameplay update, so reveal its confirmed surroundings explicitly (also
	// after a late join, deathmatch spawn or spectator camera switch).
	g.discoverLinesAroundPlayer()
	g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
	g.syncRenderState()
}
