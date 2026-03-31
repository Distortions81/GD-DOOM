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
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealNormal, IDDT: 0})
	if d.Visible {
		t.Fatalf("expected unmapped line hidden in normal mode")
	}
}

func TestStateForAutomap(t *testing.T) {
	tests := []struct {
		name         string
		revealAllMap bool
		iddt         int
		want         State
	}{
		{
			name:         "normal reveal",
			revealAllMap: false,
			iddt:         0,
			want:         State{Reveal: RevealNormal, IDDT: 0},
		},
		{
			name:         "allmap reveal",
			revealAllMap: true,
			iddt:         2,
			want:         State{Reveal: RevealAllMap, IDDT: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateForAutomap(tt.revealAllMap, tt.iddt)
			if got != tt.want {
				t.Fatalf("StateForAutomap(%v, %d)=%+v want %+v", tt.revealAllMap, tt.iddt, got, tt.want)
			}
		})
	}
}

func TestPseudo3DStateFromAutomapOverridesRevealAndIDDT(t *testing.T) {
	got := Pseudo3DStateFromAutomap(false, 0)
	want := State{Reveal: RevealAllMap, IDDT: 1}
	if got != want {
		t.Fatalf("Pseudo3DStateFromAutomap(false, 0)=%+v want %+v", got, want)
	}

	got = Pseudo3DStateFromAutomap(true, 3)
	if got != want {
		t.Fatalf("Pseudo3DStateFromAutomap(true, 3)=%+v want %+v", got, want)
	}
}

func TestParityDecisionNormalHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: mlMapped | lineNeverSee}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealNormal, IDDT: 0})
	if d.Visible {
		t.Fatalf("expected never-see line hidden in normal mode")
	}
}

func TestParityDecisionAllMapShowsUnmappedAsGray(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealAllMap, IDDT: 0})
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

func TestParityDecisionAllMapStillHidesNeverSee(t *testing.T) {
	ld := mapdata.Linedef{Flags: lineNeverSee}
	d := ParityDecision(ld, nil, nil, State{Reveal: RevealAllMap, IDDT: 0})
	if d.Visible {
		t.Fatalf("expected never-see line hidden in allmap mode")
	}
}

func TestParityDecisionIDDTRevealsNoHeightDiff(t *testing.T) {
	ld := mapdata.Linedef{Flags: 0, SideNum: [2]int16{0, 1}}
	front := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	back := &mapdata.Sector{FloorHeight: 0, CeilingHeight: 128}
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 1})
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
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 0})
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
	d := ParityDecision(ld, front, back, State{Reveal: RevealNormal, IDDT: 0})
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
