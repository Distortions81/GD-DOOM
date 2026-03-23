package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func mustLoadRealMapGame(t *testing.T, mapName string) *game {
	t.Helper()

	wadPath := findDOOM1WAD(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	m, err := mapdata.LoadMap(wf, mapdata.MapName(mapName))
	if err != nil {
		t.Fatalf("load %s: %v", mapName, err)
	}
	return newGame(m, Options{
		Width:          320,
		Height:         200,
		SourcePortMode: true,
	})
}

func TestE1M5RocketSplashLOSMatchesReferenceTrace(t *testing.T) {
	g := mustLoadRealMapGame(t, "E1M5")

	const (
		playerX = int64(3411314)
		playerY = int64(2661588)
		playerZ = int64(0)

		rocketX = int64(2558710)
		rocketY = int64(8499075)
		rocketZ = int64(1677402)
	)

	if got, want := g.sectorAt(playerX, playerY), 63; got != want {
		t.Fatalf("player sector=%d want=%d", got, want)
	}
	if got, want := g.sectorAt(rocketX, rocketY), 63; got != want {
		t.Fatalf("rocket sector=%d want=%d", got, want)
	}
	if !g.actorHasLOS(playerX, playerY, playerZ, playerHeight, rocketX, rocketY, rocketZ, 8*fracUnit) {
		t.Fatal("actorHasLOS(player, rocket impact) = false, want true from reference splash damage trace")
	}
}
