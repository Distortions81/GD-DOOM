package doomruntime

import (
	"fmt"
	"strings"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
)

// Authority drives a headless Doom world from numbered player slots. All calls
// belong to one simulation owner goroutine. The current runtime has global RNG
// and compatibility state, so run only one match per process.
//
// Snapshot produces client replication baselines for rendering and movement
// correction. PlayerStates is diagnostic data; neither format is a server
// rollback checkpoint. Networking and the fixed simulation clock live outside
// this facade so transport arrival order never drives world stepping.
type Authority struct {
	g       *game
	players [5]*authoritativePlayerState
}

// AuthorityPlayerState is an owned, read-only diagnostic view of a player. Body
// coordinates, momentum, and floor/ceiling heights are signed 16.16 map units;
// Angle uses Doom's unsigned binary angle, where a full turn is 2^32 units.
// This intentionally omits world and internal thinker state and cannot restore
// a simulation or safely reconcile predicted commands.
type AuthorityPlayerState struct {
	ID                              byte
	X, Y, Z                         int64
	MomentumX, MomentumY, MomentumZ int64
	FloorZ, CeilingZ                int64
	Angle                           uint32
	Health, Armor                   int
	Bullets, Shells, Rockets, Cells int
	Dead                            bool
	ReadyWeapon                     string
	Weapons                         []string
}

// NewAuthority creates an owned map instance without a window or audio device.
// No slot is active until AddPlayer succeeds. Network connection objects and
// local demo controls are excluded so the caller alone controls simulation time.
func NewAuthority(m *mapdata.Map, opts Options) (*Authority, error) {
	if m == nil || len(m.Sectors) == 0 {
		return nil, fmt.Errorf("authority requires a map with sectors")
	}
	if opts.DemoScript != nil || opts.LiveTicSource != nil || opts.LiveTicSink != nil || opts.CoopPeers != nil || opts.AuthorityClient != nil {
		return nil, fmt.Errorf("authority cannot use demo or peer-driven input")
	}
	if opts.AllCheats || opts.CheatLevel != 0 || opts.Invulnerable {
		return nil, fmt.Errorf("authority cannot use local cheats")
	}
	mode := strings.ToLower(strings.TrimSpace(opts.GameMode))
	if mode == "" {
		mode = gameModeCoop
	}
	if mode != gameModeCoop && mode != gameModeDeathmatch {
		return nil, fmt.Errorf("authority requires coop or deathmatch game mode")
	}
	opts.GameMode = mode
	opts.Headless = true
	opts.RecordDemoPath = ""
	opts.DemoTracePath = ""
	opts.PlayerSlot = 1
	a := &Authority{g: newGame(cloneMapForRestart(m), opts)}
	a.g.authorityEvents = &authorityEventLog{}
	a.g.authorityRules = &authorityRulesState{Config: AuthorityRules{RespawnDelayTics: doomTicsPerSecond}}
	a.selectAnchor()
	return a, nil
}

func (a *Authority) Tic() uint32 {
	return uint32(a.g.worldTic)
}

// AddPlayer activates a fresh player with independent inventory and input state.
// Co-op uses a free player start; deathmatch uses a free deathmatch start.
// Occupied starts are rejected without killing an existing player.
func (a *Authority) AddPlayer(id byte) error {
	if id < 1 || id > 4 {
		return fmt.Errorf("player slot %d is outside 1..4", id)
	}
	if a.players[id] != nil {
		return fmt.Errorf("player slot %d is already active", id)
	}
	return a.spawnAuthoritativePlayer(id, false)
}

func (a *Authority) RemovePlayer(id byte) {
	if id >= 1 && id <= 4 {
		a.g.clearAuthoritativePlayerTargets(int(id))
		a.players[id] = nil
		a.selectAnchor()
	}
}

func (a *Authority) selectAnchor() {
	a.g.authorityPlayers = a.g.authorityPlayers[:0]
	anchor := false
	for id := 1; id <= 4; id++ {
		if p := a.players[id]; p != nil {
			a.g.authorityPlayers = append(a.g.authorityPlayers, p)
			if !anchor {
				a.g.applyAuthoritativePlayer(*p)
				anchor = true
			}
		}
	}
}

// Step advances exactly one world tic. Missing player commands are neutral;
// deadline expiry and brief movement holding are decisions for the match owner.
// An empty match is paused and returns an error rather than advancing an unseen
// local player. Commands for inactive slots are rejected before any simulation.
func (a *Authority) Step(commands map[byte]demo.Tic) error {
	if a.g.authorityRules != nil && a.g.authorityRules.Ended {
		return ErrAuthorityMatchEnded
	}
	roster := make([]*authoritativePlayerState, 0, 4)
	for id := 1; id <= 4; id++ {
		if p := a.players[id]; p != nil {
			roster = append(roster, p)
		}
	}
	if len(roster) == 0 {
		return fmt.Errorf("authority has no active players")
	}
	input := make(map[int]DemoTic, len(commands))
	for id, cmd := range commands {
		if id < 1 || id > 4 || a.players[id] == nil {
			return fmt.Errorf("command for inactive player slot %d", id)
		}
		if err := netgame.ValidateInputCommand(cmd); err != nil {
			return fmt.Errorf("invalid command for player slot %d: %w", id, err)
		}
		input[int(id)] = cmd
	}
	a.selectAnchor()
	a.prepareAuthoritativeRespawns(input)
	if err := a.g.runAuthoritativePlayersTic(roster, input); err != nil {
		return err
	}
	// Shared queues must be processed once, independent of player count. Keep
	// vanilla sound bookkeeping even though headless playback owns no device.
	g := a.g
	g.tickDelayedSounds()
	g.flushSoundEvents()
	for _, p := range roster {
		g.withAuthoritativePlayer(p, func() {
			g.tickStatusWidgets()
			if g.useFlash > 0 {
				g.useFlash--
			}
			if g.damageFlashTic > 0 {
				g.damageFlashTic--
			}
			if g.bonusFlashTic > 0 {
				g.bonusFlashTic--
			}
		})
	}
	g.tickDelayedSwitchReverts()
	a.finishAuthoritativeRulesTic()
	return nil
}

// PlayerStates returns player diagnostics in slot order without sharing mutable
// inventory storage with the simulation.
func (a *Authority) PlayerStates() []AuthorityPlayerState {
	states := make([]AuthorityPlayerState, 0, 4)
	for id := 1; id <= 4; id++ {
		p := a.players[id]
		if p == nil {
			continue
		}
		s := AuthorityPlayerState{
			ID: byte(id), X: p.p.x, Y: p.p.y, Z: p.p.z,
			MomentumX: p.p.momx, MomentumY: p.p.momy, MomentumZ: p.p.momz,
			FloorZ: p.p.floorz, CeilingZ: p.p.ceilz, Angle: p.p.angle,
			Health: p.stats.Health, Armor: p.stats.Armor, Dead: p.isDead,
			Bullets: p.stats.Bullets, Shells: p.stats.Shells,
			Rockets: p.stats.Rockets, Cells: p.stats.Cells,
			ReadyWeapon: weaponInfo(p.inventory.ReadyWeapon).name,
		}
		// weaponOwned also knows the implicit fist and pistol; use an isolated
		// inventory view instead of swapping the live world's active player.
		view := game{inventory: p.inventory}
		for weapon := weaponFist; weapon <= weaponChainsaw; weapon++ {
			if view.weaponOwned(weapon) {
				s.Weapons = append(s.Weapons, weaponInfo(weapon).name)
			}
		}
		states = append(states, s)
	}
	return states
}
