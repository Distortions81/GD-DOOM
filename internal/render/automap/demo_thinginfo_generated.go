package automap

type demoTraceThingInfo struct {
	mobjType  int
	health    int
	reaction  int
	radius    int64
	height    int64
	spawnTics int
}

var demoTraceThingInfoByType = map[int16]demoTraceThingInfo{
	5:    {mobjType: 47, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC4
	6:    {mobjType: 49, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC6
	7:    {mobjType: 19, health: 3000, reaction: 8, radius: 128 * fracUnit, height: 100 * fracUnit, spawnTics: 10}, // MT_SPIDER
	8:    {mobjType: 71, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC24
	9:    {mobjType: 2, health: 30, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},      // MT_SHOTGUY
	10:   {mobjType: 118, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC68
	12:   {mobjType: 119, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC69
	13:   {mobjType: 48, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC5
	14:   {mobjType: 41, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_TELEPORTMAN
	15:   {mobjType: 112, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC62
	16:   {mobjType: 21, health: 4000, reaction: 8, radius: 40 * fracUnit, height: 110 * fracUnit, spawnTics: 10},  // MT_CYBORG
	17:   {mobjType: 68, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC21
	18:   {mobjType: 113, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC63
	19:   {mobjType: 117, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC67
	20:   {mobjType: 116, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC66
	21:   {mobjType: 114, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC64
	22:   {mobjType: 111, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC61
	23:   {mobjType: 115, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},   // MT_MISC65
	24:   {mobjType: 121, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC71
	25:   {mobjType: 124, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC74
	26:   {mobjType: 125, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 6},   // MT_MISC75
	27:   {mobjType: 122, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC72
	28:   {mobjType: 120, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC70
	29:   {mobjType: 123, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 6},   // MT_MISC73
	30:   {mobjType: 82, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC32
	31:   {mobjType: 83, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC33
	32:   {mobjType: 84, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC34
	33:   {mobjType: 85, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC35
	34:   {mobjType: 99, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC49
	35:   {mobjType: 100, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC50
	36:   {mobjType: 87, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 14},   // MT_MISC37
	37:   {mobjType: 86, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC36
	38:   {mobjType: 51, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC8
	39:   {mobjType: 50, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC7
	40:   {mobjType: 52, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 10},   // MT_MISC9
	41:   {mobjType: 88, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC38
	42:   {mobjType: 89, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC39
	43:   {mobjType: 90, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC40
	44:   {mobjType: 91, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC41
	45:   {mobjType: 92, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC42
	46:   {mobjType: 93, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC43
	47:   {mobjType: 97, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC47
	48:   {mobjType: 98, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC48
	49:   {mobjType: 101, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 68 * fracUnit, spawnTics: 10},  // MT_MISC51
	50:   {mobjType: 102, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 84 * fracUnit, spawnTics: -1},  // MT_MISC52
	51:   {mobjType: 103, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 84 * fracUnit, spawnTics: -1},  // MT_MISC53
	52:   {mobjType: 104, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 68 * fracUnit, spawnTics: -1},  // MT_MISC54
	53:   {mobjType: 105, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 52 * fracUnit, spawnTics: -1},  // MT_MISC55
	54:   {mobjType: 126, health: 1000, reaction: 8, radius: 32 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC76
	55:   {mobjType: 94, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC44
	56:   {mobjType: 95, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC45
	57:   {mobjType: 96, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC46
	58:   {mobjType: 13, health: 150, reaction: 8, radius: 30 * fracUnit, height: 56 * fracUnit, spawnTics: 10},    // MT_SHADOWS
	59:   {mobjType: 106, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 84 * fracUnit, spawnTics: -1},  // MT_MISC56
	60:   {mobjType: 107, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 68 * fracUnit, spawnTics: -1},  // MT_MISC57
	61:   {mobjType: 108, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 52 * fracUnit, spawnTics: -1},  // MT_MISC58
	62:   {mobjType: 109, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 52 * fracUnit, spawnTics: -1},  // MT_MISC59
	63:   {mobjType: 110, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 68 * fracUnit, spawnTics: 10},  // MT_MISC60
	64:   {mobjType: 3, health: 700, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},     // MT_VILE
	65:   {mobjType: 10, health: 70, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},     // MT_CHAINGUY
	66:   {mobjType: 5, health: 300, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},     // MT_UNDEAD
	67:   {mobjType: 8, health: 600, reaction: 8, radius: 48 * fracUnit, height: 64 * fracUnit, spawnTics: 15},     // MT_FATSO
	68:   {mobjType: 20, health: 500, reaction: 8, radius: 64 * fracUnit, height: 64 * fracUnit, spawnTics: 10},    // MT_BABY
	69:   {mobjType: 17, health: 500, reaction: 8, radius: 24 * fracUnit, height: 64 * fracUnit, spawnTics: 10},    // MT_KNIGHT
	70:   {mobjType: 127, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},   // MT_MISC77
	71:   {mobjType: 22, health: 400, reaction: 8, radius: 31 * fracUnit, height: 56 * fracUnit, spawnTics: 10},    // MT_PAIN
	72:   {mobjType: 24, health: 100, reaction: 8, radius: 16 * fracUnit, height: 72 * fracUnit, spawnTics: -1},    // MT_KEEN
	73:   {mobjType: 128, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 88 * fracUnit, spawnTics: -1},  // MT_MISC78
	74:   {mobjType: 129, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 88 * fracUnit, spawnTics: -1},  // MT_MISC79
	75:   {mobjType: 130, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 64 * fracUnit, spawnTics: -1},  // MT_MISC80
	76:   {mobjType: 131, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 64 * fracUnit, spawnTics: -1},  // MT_MISC81
	77:   {mobjType: 132, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 64 * fracUnit, spawnTics: -1},  // MT_MISC82
	78:   {mobjType: 133, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 64 * fracUnit, spawnTics: -1},  // MT_MISC83
	79:   {mobjType: 134, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC84
	80:   {mobjType: 135, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC85
	81:   {mobjType: 136, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},  // MT_MISC86
	82:   {mobjType: 78, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_SUPERSHOTGUN
	83:   {mobjType: 62, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MEGA
	84:   {mobjType: 23, health: 50, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},     // MT_WOLFSS
	85:   {mobjType: 79, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC29
	86:   {mobjType: 80, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: 4},    // MT_MISC30
	87:   {mobjType: 27, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 32 * fracUnit, spawnTics: -1},   // MT_BOSSTARGET
	88:   {mobjType: 25, health: 250, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},    // MT_BOSSBRAIN
	89:   {mobjType: 26, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 32 * fracUnit, spawnTics: 10},   // MT_BOSSSPIT
	2001: {mobjType: 77, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_SHOTGUN
	2002: {mobjType: 73, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_CHAINGUN
	2003: {mobjType: 75, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC27
	2004: {mobjType: 76, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC28
	2005: {mobjType: 74, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC26
	2006: {mobjType: 72, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC25
	2007: {mobjType: 63, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_CLIP
	2008: {mobjType: 69, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC22
	2010: {mobjType: 65, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC18
	2011: {mobjType: 53, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC10
	2012: {mobjType: 54, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC11
	2013: {mobjType: 55, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC12
	2014: {mobjType: 45, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC2
	2015: {mobjType: 46, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC3
	2018: {mobjType: 43, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC0
	2019: {mobjType: 44, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC1
	2022: {mobjType: 56, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_INV
	2023: {mobjType: 57, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC13
	2024: {mobjType: 58, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_INS
	2025: {mobjType: 59, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC14
	2026: {mobjType: 60, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC15
	2028: {mobjType: 81, health: 1000, reaction: 8, radius: 16 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC31
	2035: {mobjType: 30, health: 20, reaction: 8, radius: 10 * fracUnit, height: 42 * fracUnit, spawnTics: 6},      // MT_BARREL
	2045: {mobjType: 61, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: 6},    // MT_MISC16
	2046: {mobjType: 66, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC19
	2047: {mobjType: 67, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC20
	2048: {mobjType: 64, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC17
	2049: {mobjType: 70, health: 1000, reaction: 8, radius: 20 * fracUnit, height: 16 * fracUnit, spawnTics: -1},   // MT_MISC23
	3001: {mobjType: 11, health: 60, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},     // MT_TROOP
	3002: {mobjType: 12, health: 150, reaction: 8, radius: 30 * fracUnit, height: 56 * fracUnit, spawnTics: 10},    // MT_SERGEANT
	3003: {mobjType: 15, health: 1000, reaction: 8, radius: 24 * fracUnit, height: 64 * fracUnit, spawnTics: 10},   // MT_BRUISER
	3004: {mobjType: 1, health: 20, reaction: 8, radius: 20 * fracUnit, height: 56 * fracUnit, spawnTics: 10},      // MT_POSSESSED
	3005: {mobjType: 14, health: 400, reaction: 8, radius: 31 * fracUnit, height: 56 * fracUnit, spawnTics: 10},    // MT_HEAD
	3006: {mobjType: 18, health: 100, reaction: 8, radius: 16 * fracUnit, height: 56 * fracUnit, spawnTics: 10},    // MT_SKULL
}
