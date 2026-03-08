package automap

func demoTraceThingType(typ int16) int {
	if info, ok := demoTraceThingInfoByType[typ]; ok {
		return info.mobjType
	}
	return int(typ)
}

func demoTraceThingInfoForType(typ int16) (demoTraceThingInfo, bool) {
	info, ok := demoTraceThingInfoByType[typ]
	return info, ok
}
