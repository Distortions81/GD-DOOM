package doomruntime

func demoTraceWeaponID(id weaponID) int {
	switch id {
	case weaponFist:
		return 0
	case weaponPistol:
		return 1
	case weaponShotgun:
		return 2
	case weaponChaingun:
		return 3
	case weaponRocketLauncher:
		return 4
	case weaponPlasma:
		return 5
	case weaponBFG:
		return 6
	case weaponChainsaw:
		return 7
	case weaponSuperShotgun:
		return 8
	default:
		return 0
	}
}
