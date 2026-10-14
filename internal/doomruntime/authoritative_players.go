package doomruntime

import (
	"fmt"
	"slices"

	"gddoom/internal/mapdata"
)

// authoritativePlayerState is the player-owned part of game. The legacy
// simulation still addresses these values through g; activating a player must
// swap all of them together, never only the physical body. World actors, RNG,
// collected items, discovered secret sectors, sector movers, and the world tic
// remain shared. This is an internal migration boundary, not a wire snapshot.
//
// Presentation state with gameplay consequences (attacker facing, weapon
// psprites, view height) is retained alongside inventory and input latches.
type authoritativePlayerState struct {
	p                           player
	currentMoveCmd              moveCmd
	lastAttackRange             int64
	localSlot                   int
	localPlayerThingIndex       int
	playerBlockOrder            int64
	authorityPlayerThinkerOrder int64
	turnHeld                    int
	cheatLevel                  int
	invulnerable                bool
	noClip                      bool
	inventory                   playerInventory
	alwaysRun                   bool
	autoWeaponSwitch            bool
	weaponRefire                bool
	weaponAttackDown            bool
	useButtonDown               bool
	prevWeaponState             weaponPspriteState
	prevWeaponFlashState        weaponPspriteState
	prevWeaponPSpriteY          int
	weaponState                 weaponPspriteState
	weaponStateTics             int
	weaponFlashState            weaponPspriteState
	weaponFlashTics             int
	weaponPSpriteY              int
	stats                       playerStats
	playerKillCount             int
	playerItemCount             int
	playerViewZ                 int64
	secretsFound                int
	isDead                      bool
	playerReborn                bool
	playerMobjHealth            int
	damageFlashTic              int
	bonusFlashTic               int
	statusFaceIndex             int
	statusFaceCount             int
	statusFacePriority          int
	statusOldHealth             int
	statusRandom                int
	statusLastAttack            int
	statusAttackDown            bool
	statusAttackerX             int64
	statusAttackerY             int64
	statusAttackerThing         int
	statusHasAttacker           bool
	statusOldWeapons            [9]bool
	statusDamageCount           int
	statusBonusCount            int
	playerMobjState             int
	playerMobjTics              int
	useFlash                    int
	useText                     string
	prevPX                      int64
	prevPY                      int64
	prevAngle                   uint32
	prevPrevAngle               uint32
}

// captureAuthoritativePlayer makes an owned snapshot: inventory maps must not
// alias another player's inventory or a pending correction/history snapshot.
func (g *game) captureAuthoritativePlayer() authoritativePlayerState {
	state := authoritativePlayerState{
		p:                           g.p,
		currentMoveCmd:              g.currentMoveCmd,
		lastAttackRange:             g.lastAttackRange,
		localSlot:                   g.localSlot,
		localPlayerThingIndex:       g.localPlayerThingIndex,
		playerBlockOrder:            g.playerBlockOrder,
		authorityPlayerThinkerOrder: g.authorityPlayerThinkerOrder,
		turnHeld:                    g.turnHeld,
		cheatLevel:                  g.cheatLevel,
		invulnerable:                g.invulnerable,
		noClip:                      g.noClip,
		inventory:                   g.inventory,
		alwaysRun:                   g.alwaysRun,
		autoWeaponSwitch:            g.autoWeaponSwitch,
		weaponRefire:                g.weaponRefire,
		weaponAttackDown:            g.weaponAttackDown,
		useButtonDown:               g.useButtonDown,
		prevWeaponState:             g.prevWeaponState,
		prevWeaponFlashState:        g.prevWeaponFlashState,
		prevWeaponPSpriteY:          g.prevWeaponPSpriteY,
		weaponState:                 g.weaponState,
		weaponStateTics:             g.weaponStateTics,
		weaponFlashState:            g.weaponFlashState,
		weaponFlashTics:             g.weaponFlashTics,
		weaponPSpriteY:              g.weaponPSpriteY,
		stats:                       g.stats,
		playerKillCount:             g.playerKillCount,
		playerItemCount:             g.playerItemCount,
		playerViewZ:                 g.playerViewZ,
		secretsFound:                g.secretsFound,
		isDead:                      g.isDead,
		playerReborn:                g.playerReborn,
		playerMobjHealth:            g.playerMobjHealth,
		damageFlashTic:              g.damageFlashTic,
		bonusFlashTic:               g.bonusFlashTic,
		statusFaceIndex:             g.statusFaceIndex,
		statusFaceCount:             g.statusFaceCount,
		statusFacePriority:          g.statusFacePriority,
		statusOldHealth:             g.statusOldHealth,
		statusRandom:                g.statusRandom,
		statusLastAttack:            g.statusLastAttack,
		statusAttackDown:            g.statusAttackDown,
		statusAttackerX:             g.statusAttackerX,
		statusAttackerY:             g.statusAttackerY,
		statusAttackerThing:         g.statusAttackerThing,
		statusHasAttacker:           g.statusHasAttacker,
		statusOldWeapons:            g.statusOldWeapons,
		statusDamageCount:           g.statusDamageCount,
		statusBonusCount:            g.statusBonusCount,
		playerMobjState:             g.playerMobjState,
		playerMobjTics:              g.playerMobjTics,
		useFlash:                    g.useFlash,
		useText:                     g.useText,
		prevPX:                      g.prevPX,
		prevPY:                      g.prevPY,
		prevAngle:                   g.prevAngle,
		prevPrevAngle:               g.prevPrevAngle,
	}
	state.inventory.Weapons = cloneWeaponInventory(g.inventory.Weapons)
	return state
}

func (g *game) applyAuthoritativePlayer(state authoritativePlayerState) {
	g.p = state.p
	g.currentMoveCmd = state.currentMoveCmd
	g.lastAttackRange = state.lastAttackRange
	g.localSlot = state.localSlot
	g.localPlayerThingIndex = state.localPlayerThingIndex
	g.playerBlockOrder = state.playerBlockOrder
	g.authorityPlayerThinkerOrder = state.authorityPlayerThinkerOrder
	g.turnHeld = state.turnHeld
	g.cheatLevel = state.cheatLevel
	g.invulnerable = state.invulnerable
	g.noClip = state.noClip
	g.inventory = state.inventory
	g.alwaysRun = state.alwaysRun
	g.autoWeaponSwitch = state.autoWeaponSwitch
	g.weaponRefire = state.weaponRefire
	g.weaponAttackDown = state.weaponAttackDown
	g.useButtonDown = state.useButtonDown
	g.prevWeaponState = state.prevWeaponState
	g.prevWeaponFlashState = state.prevWeaponFlashState
	g.prevWeaponPSpriteY = state.prevWeaponPSpriteY
	g.weaponState = state.weaponState
	g.weaponStateTics = state.weaponStateTics
	g.weaponFlashState = state.weaponFlashState
	g.weaponFlashTics = state.weaponFlashTics
	g.weaponPSpriteY = state.weaponPSpriteY
	g.stats = state.stats
	g.playerKillCount = state.playerKillCount
	g.playerItemCount = state.playerItemCount
	g.playerViewZ = state.playerViewZ
	g.secretsFound = state.secretsFound
	g.isDead = state.isDead
	g.playerReborn = state.playerReborn
	g.playerMobjHealth = state.playerMobjHealth
	g.damageFlashTic = state.damageFlashTic
	g.bonusFlashTic = state.bonusFlashTic
	g.statusFaceIndex = state.statusFaceIndex
	g.statusFaceCount = state.statusFaceCount
	g.statusFacePriority = state.statusFacePriority
	g.statusOldHealth = state.statusOldHealth
	g.statusRandom = state.statusRandom
	g.statusLastAttack = state.statusLastAttack
	g.statusAttackDown = state.statusAttackDown
	g.statusAttackerX = state.statusAttackerX
	g.statusAttackerY = state.statusAttackerY
	g.statusAttackerThing = state.statusAttackerThing
	g.statusHasAttacker = state.statusHasAttacker
	g.statusOldWeapons = state.statusOldWeapons
	g.statusDamageCount = state.statusDamageCount
	g.statusBonusCount = state.statusBonusCount
	g.playerMobjState = state.playerMobjState
	g.playerMobjTics = state.playerMobjTics
	g.useFlash = state.useFlash
	g.useText = state.useText
	g.prevPX = state.prevPX
	g.prevPY = state.prevPY
	g.prevAngle = state.prevAngle
	g.prevPrevAngle = state.prevPrevAngle

	g.inventory.Weapons = cloneWeaponInventory(state.inventory.Weapons)
}

func (state *authoritativePlayerState) thinkerOrder(m *mapdata.Map) int64 {
	if state.authorityPlayerThinkerOrder > 0 {
		return state.authorityPlayerThinkerOrder
	}
	i := state.localPlayerThingIndex
	if m == nil || i < 0 || i >= len(m.Things) || !isPlayerStart(m.Things[i].Type) {
		return 0
	}
	return int64(i + 1)
}

// withAuthoritativePlayer keeps mutations to the active player even if world
// logic already changed that player after its initial input phase. In particular,
// reloading a stale saved state for the anchor would undo damage from a monster.
func (g *game) withAuthoritativePlayer(state *authoritativePlayerState, step func()) {
	if g.localSlot == state.localSlot {
		step()
		*state = g.captureAuthoritativePlayer()
		return
	}
	saved := g.captureAuthoritativePlayer()
	previous := g.authoritativePlayerForSlot(saved.localSlot)
	if previous != nil {
		*previous = saved
	}
	g.applyAuthoritativePlayer(*state)
	step()
	*state = g.captureAuthoritativePlayer()
	if previous != nil {
		// Nested world effects may activate and change the previous player.
		// Restore that latest state, not the stale value from before step.
		saved = *previous
	}
	g.applyAuthoritativePlayer(saved)
}

func (g *game) authoritativePlayerForSlot(slot int) *authoritativePlayerState {
	for _, p := range g.authorityPlayers {
		if p.localSlot == slot {
			return p
		}
	}
	return nil
}

// authoritativePlayerBody reads the live active context because it may have
// changed since the last checkpoint of its roster entry.
func (g *game) authoritativePlayerBody(state *authoritativePlayerState) (player, bool, int64) {
	if state.localSlot == g.localSlot {
		return g.p, g.isDead, g.playerBlockOrder
	}
	return state.p, state.isDead, state.playerBlockOrder
}

// runAuthoritativePlayersTic consumes one command per player in slot order,
// advances every player body at its map thinker position, and advances the
// shared world exactly once. Missing commands are neutral; the session layer
// decides whether a missing command should hold movement before calling here.
//
// World thinkers retain a selected player context for legacy code, while
// authority-aware targeting and damage activate the explicit owning/victim slot.
func (g *game) runAuthoritativePlayersTic(players []*authoritativePlayerState, commands map[int]DemoTic) error {
	if g == nil || len(players) == 0 {
		return fmt.Errorf("authoritative tic requires a world and at least one player")
	}
	roster := slices.Clone(players)
	slots := make(map[int]bool, len(roster))
	orders := make(map[int64]bool, len(roster))
	for _, p := range roster {
		if p == nil || p.localSlot < 1 || p.localSlot > 4 || slots[p.localSlot] {
			return fmt.Errorf("authoritative tic has an invalid or duplicate player slot")
		}
		slots[p.localSlot] = true
		if order := p.thinkerOrder(g.m); order != 0 {
			if orders[order] {
				return fmt.Errorf("authoritative players share thinker order %d", order)
			}
			orders[order] = true
		}
	}
	for slot := range commands {
		if !slots[slot] {
			return fmt.Errorf("command for inactive player slot %d", slot)
		}
	}
	slices.SortFunc(roster, func(a, b *authoritativePlayerState) int { return a.localSlot - b.localSlot })
	previousRoster := g.authorityPlayers
	g.authorityPlayers = roster
	defer func() { g.authorityPlayers = previousRoster }()

	saved := g.captureAuthoritativePlayer()
	g.applyAuthoritativePlayer(*roster[0])
	g.platTickedThisTic = false
	for _, p := range roster {
		g.withAuthoritativePlayer(p, func() {
			cmd, use, fire := demoTicCommand(commands[p.localSlot])
			g.runPlayerTic(cmd, use, fire)
		})
	}
	g.platTickedThisTic = false
	g.beginWorldTic()
	for _, p := range roster {
		if p.thinkerOrder(g.m) == 0 {
			g.withAuthoritativePlayer(p, g.tickPlayerBody)
		}
	}
	g.runOrderedWorldThinkersForPlayers(roster)
	g.tickProjectiles()
	g.tickDeferredProjectiles()
	g.tickHitscanPuffs()
	g.finishWorldTic()
	*roster[0] = g.captureAuthoritativePlayer()

	// Preserve the selected client's viewpoint, or its previous context when
	// the selected player is not participating in this authoritative roster.
	for _, p := range roster {
		if p.localSlot == saved.localSlot {
			saved = *p
			break
		}
	}
	g.applyAuthoritativePlayer(saved)
	return nil
}
