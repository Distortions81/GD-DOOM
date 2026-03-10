package linepolicy

import (
	"testing"

	"gddoom/internal/mapdata"
)

const (
	mlSecret     = 0x0020
	lineNeverSee = 0x0080
	mlMapped     = 0x0100
)

func TestParityDecisionNormalHidesUnmapped(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealNormal, IDDT: 0}, "doom")
	if d.Visible {
		t.Fatalf("expected unmapped line hidden in normal mode")
	}
}

func TestParityDecisionNormalHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped | lineNeverSee}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealNormal, IDDT: 0}, "doom")
	if d.Visible {
		t.Fatalf("expected never-see line hidden in normal mode")
	}
}

func TestParityDecisionAllMapShowsUnmappedAsGrayInParityMode(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealAllMap, IDDT: 0}, "parity")
	if !d.Visible {
		t.Fatalf("expected unmapped line visible in allmap mode")
	}
	if d.Appearance != AppearanceUnrevealed {
		t.Fatalf("appearance=%v want %v", d.Appearance, AppearanceUnrevealed)
	}
	if d.Width != lineTwoSidedWidth {
		t.Fatalf("width=%v want %v", d.Width, lineTwoSidedWidth)
	}
}

func TestParityDecisionAllMapInDoomModeKeepsSemanticStyle(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0, SideNum: [2]int16{0, -1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := ParityDecision(ld, front, nil, State{Reveal: RevealAllMap, IDDT: 0}, "doom")
	if !d.Visible {
		t.Fatalf("expected line visible in allmap mode")
	}
	if d.Appearance != AppearanceOneSided {
		t.Fatalf("appearance=%v want %v", d.Appearance, AppearanceOneSided)
	}
	if d.Width != lineOneSidedWidth {
		t.Fatalf("width=%v want %v", d.Width, lineOneSidedWidth)
	}
}

func TestParityDecisionAllMapStillHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: lineNeverSee}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealAllMap, IDDT: 0}, "doom")
	if d.Visible {
		t.Fatalf("expected never-see line hidden in allmap mode")
	}
}

func TestParityDecisionIDDTRevealsNoHeightDiff(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 1}, "doom")
	if !d.Visible {
		t.Fatalf("expected line visible in iddt mode")
	}
	if d.Appearance != AppearanceNoHeightDiff {
		t.Fatalf("appearance=%v want %v", d.Appearance, AppearanceNoHeightDiff)
	}
	if d.Width != 1 {
		t.Fatalf("width=%v want %v", d.Width, 1)
	}
}

func TestParityDecisionSecretActsAsNormalWallWithoutCheat(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped | mlSecret, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 0}, "doom")
	if !d.Visible {
		t.Fatalf("expected secret line visible")
	}
	if d.Appearance != AppearanceOneSided {
		t.Fatalf("appearance=%v want %v", d.Appearance, AppearanceOneSided)
	}
	if d.Width != lineOneSidedWidth {
		t.Fatalf("width=%v want %v", d.Width, lineOneSidedWidth)
	}
}

func TestParityDecisionTeleporterStyle(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped, Special: 39, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 0}, "doom")
	if !d.Visible {
		t.Fatalf("expected teleporter line visible")
	}
	if d.Appearance != AppearanceTeleporter {
		t.Fatalf("appearance=%v want %v", d.Appearance, AppearanceTeleporter)
	}
	if d.Width != lineTwoSidedWidth {
		t.Fatalf("width=%v want %v", d.Width, lineTwoSidedWidth)
	}
}
