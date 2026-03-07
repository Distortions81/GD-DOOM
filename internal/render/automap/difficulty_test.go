package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestThingSpawnsForSkillBuckets(t *testing.T) {
	easyOnly := mapdata.Thing{Type: 2011, Flags: skillEasyBits}
	medOnly := mapdata.Thing{Type: 2011, Flags: skillMediumBits}
	hardOnly := mapdata.Thing{Type: 2011, Flags: skillHardBits}
	noSkillBits := mapdata.Thing{Type: 2011, Flags: 0}

	if !thingSpawnsForSkill(easyOnly, 1) || !thingSpawnsForSkill(easyOnly, 2) {
		t.Fatal("easy-only thing should spawn on skills 1/2")
	}
	if thingSpawnsForSkill(easyOnly, 3) || thingSpawnsForSkill(easyOnly, 4) {
		t.Fatal("easy-only thing should not spawn on skills 3/4")
	}
	if !thingSpawnsForSkill(medOnly, 3) {
		t.Fatal("medium-only thing should spawn on skill 3")
	}
	if !thingSpawnsForSkill(hardOnly, 4) || !thingSpawnsForSkill(hardOnly, 5) {
		t.Fatal("hard-only thing should spawn on skills 4/5")
	}
	if thingSpawnsForSkill(noSkillBits, 1) || thingSpawnsForSkill(noSkillBits, 5) {
		t.Fatal("thing with no skill bits should not spawn in vanilla Doom")
	}
}

func TestThingSpawnsForGameMode(t *testing.T) {
	singleOnly := mapdata.Thing{Type: 2011, Flags: thingFlagNotCoop | thingFlagNotDM}
	coopOnly := mapdata.Thing{Type: 2011, Flags: thingFlagNotSingle | thingFlagNotDM}
	dmOnly := mapdata.Thing{Type: 2011, Flags: thingFlagNotSingle | thingFlagNotCoop}
	allModes := mapdata.Thing{Type: 2011, Flags: 0}

	if !thingSpawnsForGameMode(singleOnly, gameModeSingle) {
		t.Fatal("single-only thing should spawn in single mode")
	}
	if thingSpawnsForGameMode(singleOnly, gameModeCoop) || thingSpawnsForGameMode(singleOnly, gameModeDeathmatch) {
		t.Fatal("single-only thing should not spawn in multiplayer modes")
	}
	if !thingSpawnsForGameMode(coopOnly, gameModeCoop) {
		t.Fatal("coop-only thing should spawn in coop mode")
	}
	if !thingSpawnsForGameMode(dmOnly, gameModeDeathmatch) {
		t.Fatal("dm-only thing should spawn in deathmatch mode")
	}
	if !thingSpawnsForGameMode(allModes, gameModeSingle) || !thingSpawnsForGameMode(allModes, gameModeCoop) || !thingSpawnsForGameMode(allModes, gameModeDeathmatch) {
		t.Fatal("thing with no multiplayer exclusion bits should spawn in all modes")
	}
}

func TestApplyThingSpawnFilteringMarksUnavailableThings(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2011, Flags: skillEasyBits},
				{Type: 2011, Flags: skillHardBits | thingFlagNotSingle},
				{Type: 1, Flags: 0}, // player start always available
			},
		},
		opts: Options{SkillLevel: 1, GameMode: gameModeSingle},
	}
	g.thingCollected = make([]bool, len(g.m.Things))
	g.applyThingSpawnFiltering()
	if g.thingCollected[0] {
		t.Fatal("easy thing should remain active at skill 1")
	}
	if !g.thingCollected[1] {
		t.Fatal("hard-only and not-single thing should be filtered")
	}
	if g.thingCollected[2] {
		t.Fatal("player start should not be filtered")
	}
}

func TestNormalizeKeyboardTurnSpeed(t *testing.T) {
	if got := normalizeKeyboardTurnSpeed(0); got != 1.0 {
		t.Fatalf("normalizeKeyboardTurnSpeed(0)=%.2f want=1.0", got)
	}
	if got := normalizeKeyboardTurnSpeed(0.5); got != 0.5 {
		t.Fatalf("normalizeKeyboardTurnSpeed(0.5)=%.2f want=0.5", got)
	}
	if got := normalizeKeyboardTurnSpeed(9); got != 4.0 {
		t.Fatalf("normalizeKeyboardTurnSpeed(9)=%.2f want=4.0", got)
	}
}

func TestNormalizeMouseLookSpeed(t *testing.T) {
	if got := normalizeMouseLookSpeed(0); got != 1.0 {
		t.Fatalf("normalizeMouseLookSpeed(0)=%.2f want=1.0", got)
	}
	if got := normalizeMouseLookSpeed(0.35); got != 0.35 {
		t.Fatalf("normalizeMouseLookSpeed(0.35)=%.2f want=0.35", got)
	}
	if got := normalizeMouseLookSpeed(12); got != 8.0 {
		t.Fatalf("normalizeMouseLookSpeed(12)=%.2f want=8.0", got)
	}
}
