package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestThingSpawnsForSkillBuckets(t *testing.T) {
	easyOnly := mapdata.Thing{Type: 2011, Flags: skillEasyBits}
	medOnly := mapdata.Thing{Type: 2011, Flags: skillMediumBits}
	hardOnly := mapdata.Thing{Type: 2011, Flags: skillHardBits}
	noSkillBits := mapdata.Thing{Type: 2011, Flags: 0}

	if !thingSpawnsForSkill(easyOnly, 1, false) || !thingSpawnsForSkill(easyOnly, 2, false) {
		t.Fatal("easy-only thing should spawn on skills 1/2")
	}
	if thingSpawnsForSkill(easyOnly, 3, false) || thingSpawnsForSkill(easyOnly, 4, false) {
		t.Fatal("easy-only thing should not spawn on skills 3/4")
	}
	if !thingSpawnsForSkill(medOnly, 3, false) {
		t.Fatal("medium-only thing should spawn on skill 3")
	}
	if !thingSpawnsForSkill(hardOnly, 4, false) || !thingSpawnsForSkill(hardOnly, 5, false) {
		t.Fatal("hard-only thing should spawn on skills 4/5")
	}
	if thingSpawnsForSkill(noSkillBits, 1, false) || thingSpawnsForSkill(noSkillBits, 5, false) {
		t.Fatal("thing with no skill bits should not spawn in vanilla Doom")
	}
	noSkillPickup := mapdata.Thing{Type: 2008, Flags: 0}
	if !thingSpawnsForSkill(noSkillPickup, 3, true) {
		t.Fatal("no-skill pickup should spawn when show-no-skill-items is enabled")
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

func TestThingSpawnsInSession_ShowAllItemsOverridesFiltersForPickupsOnly(t *testing.T) {
	pickup := mapdata.Thing{Type: 2008, Flags: thingFlagNotSingle}
	monster := mapdata.Thing{Type: 3004, Flags: thingFlagNotSingle}
	if !thingSpawnsInSession(pickup, 3, gameModeSingle, false, true) {
		t.Fatal("pickup should spawn when show-all-items is enabled")
	}
	if thingSpawnsInSession(monster, 3, gameModeSingle, false, true) {
		t.Fatal("monster should not bypass normal spawn filters when show-all-items is enabled")
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

func TestApplyThingSpawnFiltering_ShowNoSkillItemsPreservesPickups(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2008, Flags: 0},
				{Type: 3004, Flags: 0},
			},
		},
		opts: Options{SkillLevel: 3, GameMode: gameModeSingle, ShowNoSkillItems: true},
	}
	g.thingCollected = make([]bool, len(g.m.Things))
	g.applyThingSpawnFiltering()
	if g.thingCollected[0] {
		t.Fatal("no-skill pickup should remain visible when ShowNoSkillItems is enabled")
	}
	if !g.thingCollected[1] {
		t.Fatal("no-skill monster should still be filtered")
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

func TestFrontendMouseSensitivityScale(t *testing.T) {
	if got := clampFrontendMouseLookSpeed(0.1); got != 1.0/6.0 {
		t.Fatalf("clampFrontendMouseLookSpeed(0.1)=%.4f want=%.4f", got, 1.0/6.0)
	}
	if got := clampFrontendMouseLookSpeed(12); got != 1.5 {
		t.Fatalf("clampFrontendMouseLookSpeed(12)=%.2f want=1.5", got)
	}
	if got := frontendMouseSensitivityDot(0.5); got != 9 {
		t.Fatalf("frontendMouseSensitivityDot(0.5)=%d want 9", got)
	}
	if got := frontendMouseSensitivitySpeedForDot(0); got != 1.0/6.0 {
		t.Fatalf("frontendMouseSensitivitySpeedForDot(0)=%.4f want=%.4f", got, 1.0/6.0)
	}
	if got := frontendMouseSensitivitySpeedForDot(9); got != 0.5 {
		t.Fatalf("frontendMouseSensitivitySpeedForDot(9)=%.2f want=0.5", got)
	}
	if got := frontendMouseSensitivitySpeedForDot(18); got != 1.5 {
		t.Fatalf("frontendMouseSensitivitySpeedForDot(18)=%.2f want=1.5", got)
	}
	if got := frontendMouseSensitivityDot(1.0 / 6.0); got != 0 {
		t.Fatalf("frontendMouseSensitivityDot(min)=%d want 0", got)
	}
	if got := frontendMouseSensitivityDot(1.5); got != 18 {
		t.Fatalf("frontendMouseSensitivityDot(max)=%d want 18", got)
	}
	if got := frontendNextMouseSensitivity(1.0, -1); got >= 1.0 {
		t.Fatalf("frontendNextMouseSensitivity(1.0, -1)=%.4f want < 1.0", got)
	}
	if got := frontendNextMouseSensitivity(1.0, 1); got <= 1.0 {
		t.Fatalf("frontendNextMouseSensitivity(1.0, 1)=%.4f want > 1.0", got)
	}
}
