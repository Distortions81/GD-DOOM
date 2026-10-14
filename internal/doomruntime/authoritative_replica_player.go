package doomruntime

// authorityReplicaPlayer explicitly captures the activation state of every player.
// World data is shared once by authorityReplica; fields here never alias live maps.
type authorityReplicaPlayer struct {
	AuthorityPlayerThinkerOrder int64
	TeleportedThisTic           bool
	P                           playerSaveState
	CurrentMoveCmd              replicaMoveCommand
	LastAttackRange             int64
	LocalSlot                   int
	LocalPlayerThingIndex       int
	PlayerBlockOrder            int64
	TurnHeld                    int
	CheatLevel                  int
	Invulnerable                bool
	NoClip                      bool
	Inventory                   playerInventorySaveState
	AlwaysRun                   bool
	AutoWeaponSwitch            bool
	WeaponRefire                bool
	WeaponAttackDown            bool
	UseButtonDown               bool
	PrevWeaponState             weaponPspriteState
	PrevWeaponFlashState        weaponPspriteState
	PrevWeaponPSpriteY          int
	WeaponState                 weaponPspriteState
	WeaponStateTics             int
	WeaponFlashState            weaponPspriteState
	WeaponFlashTics             int
	WeaponPSpriteY              int
	Stats                       playerStats
	PlayerKillCount             int
	PlayerItemCount             int
	PlayerViewZ                 int64
	SecretsFound                int
	IsDead                      bool
	PlayerReborn                bool
	PlayerMobjHealth            int
	DamageFlashTic              int
	BonusFlashTic               int
	StatusFaceIndex             int
	StatusFaceCount             int
	StatusFacePriority          int
	StatusOldHealth             int
	StatusRandom                int
	StatusLastAttack            int
	StatusAttackDown            bool
	StatusAttackerX             int64
	StatusAttackerY             int64
	StatusAttackerThing         int
	StatusHasAttacker           bool
	StatusOldWeapons            [9]bool
	StatusDamageCount           int
	StatusBonusCount            int
	PlayerMobjState             int
	PlayerMobjTics              int
	UseFlash                    int
	UseText                     string
	PrevPX                      int64
	PrevPY                      int64
	PrevAngle                   uint32
	PrevPrevAngle               uint32
}

type replicaMoveCommand struct {
	Forward, Side, TurnRaw int64
	Turn, WeaponSlot       int
	Run                    bool
}

func captureReplicaPlayer(p authoritativePlayerState) authorityReplicaPlayer {
	return authorityReplicaPlayer{
		AuthorityPlayerThinkerOrder: p.authorityPlayerThinkerOrder,
		TeleportedThisTic:           p.p.teleportedThisTic,
		P:                           capturePlayerSaveState(p.p),
		CurrentMoveCmd:              replicaMoveCommand{Forward: p.currentMoveCmd.forward, Side: p.currentMoveCmd.side, TurnRaw: p.currentMoveCmd.turnRaw, Turn: p.currentMoveCmd.turn, WeaponSlot: p.currentMoveCmd.weaponSlot, Run: p.currentMoveCmd.run},
		LastAttackRange:             p.lastAttackRange,
		LocalSlot:                   p.localSlot,
		LocalPlayerThingIndex:       p.localPlayerThingIndex,
		PlayerBlockOrder:            p.playerBlockOrder,
		TurnHeld:                    p.turnHeld,
		CheatLevel:                  p.cheatLevel,
		Invulnerable:                p.invulnerable,
		NoClip:                      p.noClip,
		Inventory:                   capturePlayerInventorySaveState(p.inventory),
		AlwaysRun:                   p.alwaysRun,
		AutoWeaponSwitch:            p.autoWeaponSwitch,
		WeaponRefire:                p.weaponRefire,
		WeaponAttackDown:            p.weaponAttackDown,
		UseButtonDown:               p.useButtonDown,
		PrevWeaponState:             p.prevWeaponState,
		PrevWeaponFlashState:        p.prevWeaponFlashState,
		PrevWeaponPSpriteY:          p.prevWeaponPSpriteY,
		WeaponState:                 p.weaponState,
		WeaponStateTics:             p.weaponStateTics,
		WeaponFlashState:            p.weaponFlashState,
		WeaponFlashTics:             p.weaponFlashTics,
		WeaponPSpriteY:              p.weaponPSpriteY,
		Stats:                       p.stats,
		PlayerKillCount:             p.playerKillCount,
		PlayerItemCount:             p.playerItemCount,
		PlayerViewZ:                 p.playerViewZ,
		SecretsFound:                p.secretsFound,
		IsDead:                      p.isDead,
		PlayerReborn:                p.playerReborn,
		PlayerMobjHealth:            p.playerMobjHealth,
		DamageFlashTic:              p.damageFlashTic,
		BonusFlashTic:               p.bonusFlashTic,
		StatusFaceIndex:             p.statusFaceIndex,
		StatusFaceCount:             p.statusFaceCount,
		StatusFacePriority:          p.statusFacePriority,
		StatusOldHealth:             p.statusOldHealth,
		StatusRandom:                p.statusRandom,
		StatusLastAttack:            p.statusLastAttack,
		StatusAttackDown:            p.statusAttackDown,
		StatusAttackerX:             p.statusAttackerX,
		StatusAttackerY:             p.statusAttackerY,
		StatusAttackerThing:         p.statusAttackerThing,
		StatusHasAttacker:           p.statusHasAttacker,
		StatusOldWeapons:            p.statusOldWeapons,
		StatusDamageCount:           p.statusDamageCount,
		StatusBonusCount:            p.statusBonusCount,
		PlayerMobjState:             p.playerMobjState,
		PlayerMobjTics:              p.playerMobjTics,
		UseFlash:                    p.useFlash,
		UseText:                     p.useText,
		PrevPX:                      p.prevPX,
		PrevPY:                      p.prevPY,
		PrevAngle:                   p.prevAngle,
		PrevPrevAngle:               p.prevPrevAngle,
	}
}

func restoreReplicaPlayer(p authorityReplicaPlayer) authoritativePlayerState {
	state := authoritativePlayerState{
		authorityPlayerThinkerOrder: p.AuthorityPlayerThinkerOrder,
		p:                           restorePlayerSaveState(p.P),
		currentMoveCmd:              moveCmd{forward: p.CurrentMoveCmd.Forward, side: p.CurrentMoveCmd.Side, turnRaw: p.CurrentMoveCmd.TurnRaw, turn: p.CurrentMoveCmd.Turn, weaponSlot: p.CurrentMoveCmd.WeaponSlot, run: p.CurrentMoveCmd.Run},
		lastAttackRange:             p.LastAttackRange,
		localSlot:                   p.LocalSlot,
		localPlayerThingIndex:       p.LocalPlayerThingIndex,
		playerBlockOrder:            p.PlayerBlockOrder,
		turnHeld:                    p.TurnHeld,
		cheatLevel:                  p.CheatLevel,
		invulnerable:                p.Invulnerable,
		noClip:                      p.NoClip,
		inventory:                   restorePlayerInventorySaveState(p.Inventory),
		alwaysRun:                   p.AlwaysRun,
		autoWeaponSwitch:            p.AutoWeaponSwitch,
		weaponRefire:                p.WeaponRefire,
		weaponAttackDown:            p.WeaponAttackDown,
		useButtonDown:               p.UseButtonDown,
		prevWeaponState:             p.PrevWeaponState,
		prevWeaponFlashState:        p.PrevWeaponFlashState,
		prevWeaponPSpriteY:          p.PrevWeaponPSpriteY,
		weaponState:                 p.WeaponState,
		weaponStateTics:             p.WeaponStateTics,
		weaponFlashState:            p.WeaponFlashState,
		weaponFlashTics:             p.WeaponFlashTics,
		weaponPSpriteY:              p.WeaponPSpriteY,
		stats:                       p.Stats,
		playerKillCount:             p.PlayerKillCount,
		playerItemCount:             p.PlayerItemCount,
		playerViewZ:                 p.PlayerViewZ,
		secretsFound:                p.SecretsFound,
		isDead:                      p.IsDead,
		playerReborn:                p.PlayerReborn,
		playerMobjHealth:            p.PlayerMobjHealth,
		damageFlashTic:              p.DamageFlashTic,
		bonusFlashTic:               p.BonusFlashTic,
		statusFaceIndex:             p.StatusFaceIndex,
		statusFaceCount:             p.StatusFaceCount,
		statusFacePriority:          p.StatusFacePriority,
		statusOldHealth:             p.StatusOldHealth,
		statusRandom:                p.StatusRandom,
		statusLastAttack:            p.StatusLastAttack,
		statusAttackDown:            p.StatusAttackDown,
		statusAttackerX:             p.StatusAttackerX,
		statusAttackerY:             p.StatusAttackerY,
		statusAttackerThing:         p.StatusAttackerThing,
		statusHasAttacker:           p.StatusHasAttacker,
		statusOldWeapons:            p.StatusOldWeapons,
		statusDamageCount:           p.StatusDamageCount,
		statusBonusCount:            p.StatusBonusCount,
		playerMobjState:             p.PlayerMobjState,
		playerMobjTics:              p.PlayerMobjTics,
		useFlash:                    p.UseFlash,
		useText:                     p.UseText,
		prevPX:                      p.PrevPX,
		prevPY:                      p.PrevPY,
		prevAngle:                   p.PrevAngle,
		prevPrevAngle:               p.PrevPrevAngle,
	}
	state.p.teleportedThisTic = p.TeleportedThisTic
	return state
}
