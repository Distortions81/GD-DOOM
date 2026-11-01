package doomruntime

func demoTraceThingType(typ int16) int {
	if info, ok := demoTraceThingInfoByType[typ]; ok {
		return info.mobjType
	}
	return int(typ)
}

func demoTraceThingInfoForType(typ int16) (demoTraceThingInfo, bool) {
	if isVoodooDollType(typ) {
		return demoTraceThingInfo{mobjType: 0, health: 100, radius: playerRadius, height: playerHeight, spawnTics: -1}, true
	}
	info, ok := demoTraceThingInfoByType[typ]
	return info, ok
}
