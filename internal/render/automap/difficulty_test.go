package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestThingSpawnsForSkillBuckets(t *testing.T) {
	easyOnly := mapdata.Thing{Type: 2011, Flags: skillEasyBits}
	medOnly := mapdata.Thing{Type: 2011, Flags: skillMediumBits}
	hardOnly := mapdata.Thing{Type: 2011, Flags: skillHardBits}
	all := mapdata.Thing{Type: 2011, Flags: 0}

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
	if !thingSpawnsForSkill(all, 1) || !thingSpawnsForSkill(all, 5) {
		t.Fatal("thing with no skill bits should spawn on all skills")
	}
}

func TestApplySkillThingFilteringMarksUnavailableThings(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Things: []mapdata.Thing{
				{Type: 2011, Flags: skillEasyBits},
				{Type: 2011, Flags: skillHardBits},
				{Type: 1, Flags: 0}, // player start always available
			},
		},
		opts: Options{SkillLevel: 1},
	}
	g.thingCollected = make([]bool, len(g.m.Things))
	g.applySkillThingFiltering()
	if g.thingCollected[0] {
		t.Fatal("easy thing should remain active at skill 1")
	}
	if !g.thingCollected[1] {
		t.Fatal("hard-only thing should be filtered at skill 1")
	}
	if g.thingCollected[2] {
		t.Fatal("player start should not be filtered")
	}
}
