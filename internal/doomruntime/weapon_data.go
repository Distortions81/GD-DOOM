package doomruntime

type ammoKind uint8

const (
	ammoKindNone ammoKind = iota
	ammoKindBullets
	ammoKindShells
	ammoKindRockets
	ammoKindCells
)

type weaponDef struct {
	id            weaponID
	name          string
	ammo          ammoKind
	minAmmo       int
	pickupType    int16
	readystate    weaponPspriteState
	upstate       weaponPspriteState
	downstate     weaponPspriteState
	atkstate      weaponPspriteState
	flashstate    weaponPspriteState
	nonAutoRefire bool
	commercial    bool
}

var weaponDefs = map[weaponID]weaponDef{
	weaponFist: {
		id: weaponFist, name: "fist", ammo: ammoKindNone, minAmmo: 0,
		readystate: weaponStateFistReady, upstate: weaponStateFistUp, downstate: weaponStateFistDown,
		atkstate: weaponStateFistAtk1,
	},
	weaponPistol: {
		id: weaponPistol, name: "pistol", ammo: ammoKindBullets, minAmmo: 1,
		readystate: weaponStatePistolReady, upstate: weaponStatePistolUp, downstate: weaponStatePistolDown,
		atkstate: weaponStatePistolAtk1, flashstate: weaponStatePistolFlash,
	},
	weaponShotgun: {
		id: weaponShotgun, name: "shotgun", ammo: ammoKindShells, minAmmo: 1, pickupType: 2001,
		readystate: weaponStateShotgunReady, upstate: weaponStateShotgunUp, downstate: weaponStateShotgunDown,
		atkstate: weaponStateShotgunAtk1, flashstate: weaponStateShotgunFlash1,
	},
	weaponChaingun: {
		id: weaponChaingun, name: "chaingun", ammo: ammoKindBullets, minAmmo: 1, pickupType: 2002,
		readystate: weaponStateChaingunReady, upstate: weaponStateChaingunUp, downstate: weaponStateChaingunDown,
		atkstate: weaponStateChaingunAtk1, flashstate: weaponStateChaingunFlash1,
	},
	weaponRocketLauncher: {
		id: weaponRocketLauncher, name: "rocket", ammo: ammoKindRockets, minAmmo: 1, pickupType: 2003,
		readystate: weaponStateRocketReady, upstate: weaponStateRocketUp, downstate: weaponStateRocketDown,
		atkstate: weaponStateRocketAtk1, flashstate: weaponStateRocketFlash1, nonAutoRefire: true,
	},
	weaponPlasma: {
		id: weaponPlasma, name: "plasma", ammo: ammoKindCells, minAmmo: 1, pickupType: 2004,
		readystate: weaponStatePlasmaReady, upstate: weaponStatePlasmaUp, downstate: weaponStatePlasmaDown,
		atkstate: weaponStatePlasmaAtk1, flashstate: weaponStatePlasmaFlash1, commercial: true,
	},
	weaponBFG: {
		id: weaponBFG, name: "bfg", ammo: ammoKindCells, minAmmo: 40, pickupType: 2006,
		readystate: weaponStateBFGReady, upstate: weaponStateBFGUp, downstate: weaponStateBFGDown,
		atkstate: weaponStateBFGAtk1, flashstate: weaponStateBFGFlash1, nonAutoRefire: true, commercial: true,
	},
	weaponChainsaw: {
		id: weaponChainsaw, name: "chainsaw", ammo: ammoKindNone, minAmmo: 0, pickupType: 2005,
		readystate: weaponStateSawReady, upstate: weaponStateSawUp, downstate: weaponStateSawDown,
		atkstate: weaponStateSawAtk1,
	},
	weaponSuperShotgun: {
		id: weaponSuperShotgun, name: "supershotgun", ammo: ammoKindShells, minAmmo: 2, pickupType: 82,
		readystate: weaponStateSuperShotgunReady, upstate: weaponStateSuperShotgunUp, downstate: weaponStateSuperShotgunDown,
		atkstate: weaponStateSuperShotgunAtk1, flashstate: weaponStateSuperShotgunFlash1, commercial: true,
	},
}

func weaponInfo(id weaponID) weaponDef {
	if def, ok := weaponDefs[id]; ok {
		return def
	}
	return weaponDefs[weaponPistol]
}

func weaponAmmoCount(stats playerStats, ammo ammoKind) int {
	switch ammo {
	case ammoKindBullets:
		return stats.Bullets
	case ammoKindShells:
		return stats.Shells
	case ammoKindRockets:
		return stats.Rockets
	case ammoKindCells:
		return stats.Cells
	default:
		return 0
	}
}
