package mapdata

// Initial Boom subset: extended triggers for existing Doom actions. Keep these
// aliases separate from the vanilla tables; generalized linedefs need their own
// parameter decoder. Numbers follow Boom 2.02 p_spec.c / p_switch.c, rather than
// assuming that similarly named actions in the editing reference are identical.
// https://boomref.soulsphere.org/
var boomLineAliases = map[uint16]struct {
	base    uint16
	trigger TriggerType
	repeat  bool
}{
	142: {140, TriggerWalk, false}, // raise floor 512
	143: {15, TriggerWalk, false},  // raise and change 24 (platform)
	144: {14, TriggerWalk, false},  // raise and change 32 (platform)
	145: {41, TriggerWalk, false},  // lower ceiling to floor
	147: {140, TriggerWalk, true},
	148: {15, TriggerWalk, true},
	149: {14, TriggerWalk, true},
	152: {41, TriggerWalk, true},
	156: {17, TriggerWalk, true},  // start strobing
	157: {104, TriggerWalk, true}, // lights off
	158: {30, TriggerUse, false},  // raise to shortest texture
	159: {37, TriggerUse, false},  // lower and change
	160: {59, TriggerUse, false},  // raise 24 and change (floor)
	161: {58, TriggerUse, false},  // raise 24
	162: {53, TriggerUse, false},  // perpetual platform
	163: {54, TriggerUse, false},  // stop platform
	164: {6, TriggerUse, false},   // fast crusher
	167: {44, TriggerUse, false},  // lower ceiling and crush
	168: {57, TriggerUse, false},  // stop crusher
	169: {12, TriggerUse, false},  // brightest neighboring light
	170: {35, TriggerUse, false},  // light 35
	171: {13, TriggerUse, false},  // light 255
	172: {17, TriggerUse, false},
	173: {104, TriggerUse, false},
	174: {39, TriggerUse, false}, // normal teleport
	175: {16, TriggerUse, false}, // close, wait 30 seconds, open
	176: {30, TriggerUse, true},
	177: {37, TriggerUse, true},
	178: {140, TriggerUse, true},
	179: {59, TriggerUse, true},
	180: {58, TriggerUse, true},
	181: {53, TriggerUse, true},
	182: {54, TriggerUse, true},
	183: {6, TriggerUse, true},
	184: {25, TriggerUse, true}, // slow crusher
	187: {44, TriggerUse, true},
	188: {57, TriggerUse, true},
	192: {12, TriggerUse, true},
	193: {17, TriggerUse, true},
	194: {104, TriggerUse, true},
	195: {39, TriggerUse, true},
	196: {16, TriggerUse, true},
}

const (
	SectorTypeMask   int16 = 31
	SectorDamageMask int16 = 0x60
	SectorSecretMask int16 = 0x80
)

func SectorIsSecret(special int16) bool {
	return special == 9 || special&SectorSecretMask != 0
}

// ClearSectorSecret preserves independent damage, friction and push bits.
// Once no extended bits remain, low type bits must not become a vanilla hazard.
func ClearSectorSecret(special int16) int16 {
	special &^= SectorSecretMask
	if special < 32 {
		return 0
	}
	return special
}

// SectorSpecialSupported describes this initial subset, not all valid Boom
// sectors. Friction, pushers and later extensions remain audit findings.
func SectorSpecialSupported(special int16) bool {
	if special < 0 || special&^int16(0xff) != 0 {
		return false
	}
	switch special & SectorTypeMask {
	case 0, 1, 2, 3, 4, 5, 7, 8, 9, 10, 11, 12, 13, 14, 16, 17:
		return true
	}
	return false
}

func IsScrollingWallSpecial(special uint16) bool {
	return special == 48 || special == 85 || special >= 250 && special <= 253
}
