package doomruntime

import "testing"

func TestE1M5ZombiemanNorthProbeBlockedByDropoff(t *testing.T) {
	g := mustLoadRealMapGame(t, "E1M5")

	const (
		thingIdx = 29
		typ      = int16(3004)

		curX = int64(-1408) * fracUnit
		curY = int64(672) * fracUnit
		curZ = int64(-180) * fracUnit

		tryX = int64(-1408) * fracUnit
		tryY = int64(720) * fracUnit
	)

	if got := g.m.Things[thingIdx].Type; got != typ {
		t.Fatalf("thing %d type=%d want=%d", thingIdx, got, typ)
	}

	g.setThingPosFixed(thingIdx, curX, curY)
	g.sectorFloor[19] = int64(-208) * fracUnit
	g.sectorCeil[19] = int64(72) * fracUnit
	g.sectorFloor[20] = curZ
	g.sectorCeil[20] = int64(144) * fracUnit
	g.setThingSupportState(thingIdx, curZ, curZ, int64(144)*fracUnit)

	if _, _, _, ok := g.tryMoveProbeMonster(thingIdx, typ, tryX, tryY); ok {
		t.Fatal("north probe succeeded, want blocked by dropoff like Doom")
	}
}
