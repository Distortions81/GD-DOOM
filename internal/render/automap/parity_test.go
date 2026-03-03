package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestParityLineDecisionNormalHidesUnmapped(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0}
	d := parityLineDecision(ld, nil, nil, automapParityState{reveal: revealNormal, iddt: 0}, "doom")
	if d.visible {
		t.Fatalf("expected unmapped line hidden in normal mode")
	}
}

func TestParityLineDecisionNormalHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped | lineNeverSee}
	d := parityLineDecision(ld, nil, nil, automapParityState{reveal: revealNormal, iddt: 0}, "doom")
	if d.visible {
		t.Fatalf("expected never-see line hidden in normal mode")
	}
}

func TestParityLineDecisionAllMapShowsUnmappedAsGrayInParityMode(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0}
	d := parityLineDecision(ld, nil, nil, automapParityState{reveal: revealAllMap, iddt: 0}, "parity")
	if !d.visible {
		t.Fatalf("expected unmapped line visible in allmap mode")
	}
	if d.appearance != lineAppearanceUnrevealed {
		t.Fatalf("appearance=%v want %v", d.appearance, lineAppearanceUnrevealed)
	}
}

func TestParityLineDecisionAllMapInDoomModeKeepsSemanticStyle(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0, SideNum: [2]int16{0, -1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := parityLineDecision(ld, front, nil, automapParityState{reveal: revealAllMap, iddt: 0}, "doom")
	if !d.visible {
		t.Fatalf("expected line visible in allmap mode")
	}
	if d.appearance != lineAppearanceOneSided {
		t.Fatalf("appearance=%v want %v", d.appearance, lineAppearanceOneSided)
	}
}

func TestParityLineDecisionAllMapStillHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: lineNeverSee}
	d := parityLineDecision(ld, nil, nil, automapParityState{reveal: revealAllMap, iddt: 0}, "doom")
	if d.visible {
		t.Fatalf("expected never-see line hidden in allmap mode")
	}
}

func TestParityLineDecisionIDDTRevealsNoHeightDiff(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := parityLineDecision(ld, front, back, automapParityState{reveal: revealNormal, iddt: 1}, "doom")
	if !d.visible {
		t.Fatalf("expected line visible in iddt mode")
	}
	if d.appearance != lineAppearanceNoHeightDiff {
		t.Fatalf("appearance=%v want %v", d.appearance, lineAppearanceNoHeightDiff)
	}
}

func TestParityLineDecisionSecretActsAsNormalWallWithoutCheat(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped | mlSecret, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := parityLineDecision(ld, front, back, automapParityState{reveal: revealNormal, iddt: 0}, "doom")
	if !d.visible {
		t.Fatalf("expected secret line visible")
	}
	if d.appearance != lineAppearanceOneSided {
		t.Fatalf("appearance=%v want %v", d.appearance, lineAppearanceOneSided)
	}
}

func TestParityLineDecisionTeleporterStyle(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped, Special: 39, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := parityLineDecision(ld, front, back, automapParityState{reveal: revealNormal, iddt: 0}, "doom")
	if !d.visible {
		t.Fatalf("expected teleporter line visible")
	}
	if d.appearance != lineAppearanceTeleporter {
		t.Fatalf("appearance=%v want %v", d.appearance, lineAppearanceTeleporter)
	}
}
