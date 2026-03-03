package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestLinedefDecisionPseudo3DIgnoresMappedGate(t *testing.T) {
	g := &game{
		parity: automapParityState{reveal: revealNormal, iddt: 0},
		m: &mapdata.Map{
			Sectors:  []mapdata.Sector{{FloorHeight: 0, CeilingHeight: 128}},
			Sidedefs: []mapdata.Sidedef{{Sector: 0}},
		},
	}
	ld := mapdata.Linedef{
		Flags:   0, // not mapped
		SideNum: [2]int16{0, -1},
	}
	d := g.linedefDecisionPseudo3D(ld)
	if !d.visible {
		t.Fatal("pseudo3d should not hide line due to automap mapped status")
	}
}
