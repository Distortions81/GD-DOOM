package automap

import (
	"strings"

	"gddoom/internal/mapdata"
)

const (
	mlSecret       = 0x0020
	lineSoundBlock = 0x0040
	lineNeverSee   = 0x0080
	mlMapped       = 0x0100
)

type lineAppearance int

const (
	lineAppearanceOneSided lineAppearance = iota
	lineAppearanceSecret
	lineAppearanceTeleporter
	lineAppearanceFloorChange
	lineAppearanceCeilChange
	lineAppearanceNoHeightDiff
	lineAppearanceUnrevealed
)

type lineDecision struct {
	visible    bool
	appearance lineAppearance
	width      float64
}

func parityLineDecision(ld mapdata.Linedef, front, back *mapdata.Sector, st automapParityState, lineColorMode string) lineDecision {
	isCheat := st.iddt > 0
	mapped := ld.Flags&mlMapped != 0
	neverSee := ld.Flags&lineNeverSee != 0
	strictParityColor := strings.EqualFold(strings.TrimSpace(lineColorMode), "parity")

	if !isCheat {
		if st.reveal == revealNormal {
			if !mapped || neverSee {
				return lineDecision{}
			}
		}
		if st.reveal == revealAllMap {
			if neverSee {
				return lineDecision{}
			}
			if strictParityColor && !mapped {
				return lineDecision{
					visible:    true,
					appearance: lineAppearanceUnrevealed,
					width:      lineTwoSidedWidth,
				}
			}
		}
	}

	if back == nil || ld.SideNum[1] < 0 || front == nil {
		return lineDecision{visible: true, appearance: lineAppearanceOneSided, width: lineOneSidedWidth}
	}

	if ld.Flags&mlSecret != 0 {
		if !isCheat {
			return lineDecision{visible: true, appearance: lineAppearanceOneSided, width: lineOneSidedWidth}
		}
		return lineDecision{visible: true, appearance: lineAppearanceSecret, width: lineTwoSidedWidth}
	}

	if ld.Special == 39 {
		return lineDecision{visible: true, appearance: lineAppearanceTeleporter, width: lineTwoSidedWidth}
	}

	if front.FloorHeight != back.FloorHeight {
		return lineDecision{visible: true, appearance: lineAppearanceFloorChange, width: lineTwoSidedWidth}
	}
	if front.CeilingHeight != back.CeilingHeight {
		return lineDecision{visible: true, appearance: lineAppearanceCeilChange, width: lineTwoSidedWidth}
	}
	if isCheat {
		return lineDecision{visible: true, appearance: lineAppearanceNoHeightDiff, width: 1}
	}
	return lineDecision{}
}
