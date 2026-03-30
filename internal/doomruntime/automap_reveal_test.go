package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestAutomapSectorRevealedNormalRequiresMappedLine(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{}, {}},
			Linedefs: []mapdata.Linedef{{Flags: 0}, {Flags: mlMapped}, {Flags: lineNeverSee | mlMapped}},
		},
		sectorLineAdj: [][]automapSectorLine{
			{{line: 0}, {line: 2}},
			{{line: 1}},
		},
	}

	if g.automapSectorRevealed(0) {
		t.Fatalf("sector 0 should stay hidden in normal reveal until a non-neversee line is mapped")
	}
	if !g.automapSectorRevealed(1) {
		t.Fatalf("sector 1 should reveal when one of its lines is mapped")
	}
}

func TestAutomapSectorRevealedAllMapOverridesMappedState(t *testing.T) {
	g := &game{
		m: &mapdata.Map{
			Sectors: []mapdata.Sector{{}},
		},
		parity: automapParityState{reveal: revealAllMap},
	}

	if !g.automapSectorRevealed(0) {
		t.Fatalf("allmap reveal should expose the sector")
	}
}

func TestAutomapThingRevealedUsesThingSector(t *testing.T) {
	th := mapdata.Thing{}
	g := &game{
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{}, {}},
			Linedefs: []mapdata.Linedef{{Flags: mlMapped}},
		},
		thingSectorCache: []int{1},
		sectorLineAdj: [][]automapSectorLine{
			nil,
			{{line: 0}},
		},
	}

	if !g.automapThingRevealed(0, th) {
		t.Fatalf("thing in a revealed sector should be visible")
	}
	if g.automapThingRevealed(1, th) {
		t.Fatalf("thing without a valid sector should stay hidden in normal reveal")
	}
}
