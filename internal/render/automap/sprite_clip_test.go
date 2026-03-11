package automap

import "testing"

func TestMonsterSpriteClipRadius_UsesActorRadius(t *testing.T) {
	if got := monsterSpriteClipRadius(3002); got != 30*fracUnit {
		t.Fatalf("monster clip radius=%d want %d", got, 30*fracUnit)
	}
	if got := monsterSpriteClipRadius(3006); got != 16*fracUnit {
		t.Fatalf("lost soul clip radius=%d want %d", got, 16*fracUnit)
	}
}

func TestWorldThingSpriteClipRadius_UsesThingRadius(t *testing.T) {
	if got := worldThingSpriteClipRadius(2035); got != 10*fracUnit {
		t.Fatalf("barrel clip radius=%d want %d", got, 10*fracUnit)
	}
	if got := worldThingSpriteClipRadius(2022); got != 20*fracUnit {
		t.Fatalf("invulnerability clip radius=%d want %d", got, 20*fracUnit)
	}
}

func TestFloorSpriteTop_AnchorsToBottom(t *testing.T) {
	if got := floorSpriteTop(16, 100); got != 84 {
		t.Fatalf("dstY=%f want 84", got)
	}
	if got := floorSpriteTop(8, 100); got != 92 {
		t.Fatalf("dstY=%f want 92", got)
	}
}
