package doomruntime

func demoTraceThingType(typ int16) int {
	if info, ok := demoTraceThingInfoByType[typ]; ok {
		return info.mobjType
	}
	return int(typ)
}

func demoTraceThingInfoForType(typ int16) (demoTraceThingInfo, bool) {
	if typ == 5001 || typ == 5002 {
		return demoTraceThingInfo{mobjType: 137 + int(typ-5001), health: 1000, radius: 8 * fracUnit, height: 8 * fracUnit, spawnTics: -1}, true
	}
	if isVoodooDollType(typ) {
		return demoTraceThingInfo{mobjType: 0, health: 100, radius: playerRadius, height: playerHeight, spawnTics: -1}, true
	}
	info, ok := demoTraceThingInfoByType[typ]
	return info, ok
}
