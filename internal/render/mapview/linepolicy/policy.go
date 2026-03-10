package linepolicy

import (
	"image/color"
	"strings"

	"gddoom/internal/mapdata"
)

const (
	lineOneSidedWidth = 1.8
	lineTwoSidedWidth = 1.2
)

type RevealMode int

const (
	RevealNormal RevealMode = iota
	RevealAllMap
)

type State struct {
	Reveal RevealMode
	IDDT   int
}

type Appearance int

const (
	AppearanceOneSided Appearance = iota
	AppearanceSecret
	AppearanceTeleporter
	AppearanceFloorChange
	AppearanceCeilChange
	AppearanceNoHeightDiff
	AppearanceUnrevealed
)

type Decision struct {
	Visible    bool
	Appearance Appearance
	Width      float64
}

type Palette struct {
	OneSided     color.RGBA
	Secret       color.RGBA
	Teleporter   color.RGBA
	FloorChange  color.RGBA
	CeilChange   color.RGBA
	NoHeightDiff color.RGBA
	Unrevealed   color.RGBA
}

type Style struct {
	Color color.RGBA
	Width float64
}

func ParityDecision(ld mapdata.Linedef, front, back *mapdata.Sector, st State, lineColorMode string) Decision {
	isCheat := st.IDDT > 0
	mapped := ld.Flags&0x0100 != 0
	neverSee := ld.Flags&0x0080 != 0
	strictParityColor := strings.EqualFold(strings.TrimSpace(lineColorMode), "parity")

	if !isCheat {
		if st.Reveal == RevealNormal {
			if !mapped || neverSee {
				return Decision{}
			}
		}
		if st.Reveal == RevealAllMap {
			if neverSee {
				return Decision{}
			}
			if strictParityColor && !mapped {
				return Decision{
					Visible:    true,
					Appearance: AppearanceUnrevealed,
					Width:      lineTwoSidedWidth,
				}
			}
		}
	}

	if back == nil || ld.SideNum[1] < 0 || front == nil {
		return Decision{Visible: true, Appearance: AppearanceOneSided, Width: lineOneSidedWidth}
	}

	if ld.Flags&0x0020 != 0 {
		if !isCheat {
			return Decision{Visible: true, Appearance: AppearanceOneSided, Width: lineOneSidedWidth}
		}
		return Decision{Visible: true, Appearance: AppearanceSecret, Width: lineTwoSidedWidth}
	}

	if ld.Special == 39 {
		return Decision{Visible: true, Appearance: AppearanceTeleporter, Width: lineTwoSidedWidth}
	}

	if front.FloorHeight != back.FloorHeight {
		return Decision{Visible: true, Appearance: AppearanceFloorChange, Width: lineTwoSidedWidth}
	}
	if front.CeilingHeight != back.CeilingHeight {
		return Decision{Visible: true, Appearance: AppearanceCeilChange, Width: lineTwoSidedWidth}
	}
	if isCheat {
		return Decision{Visible: true, Appearance: AppearanceNoHeightDiff, Width: 1}
	}
	return Decision{}
}

func (d Decision) Style(p Palette) Style {
	switch d.Appearance {
	case AppearanceOneSided:
		return Style{Color: p.OneSided, Width: d.Width}
	case AppearanceSecret:
		return Style{Color: p.Secret, Width: d.Width}
	case AppearanceTeleporter:
		return Style{Color: p.Teleporter, Width: d.Width}
	case AppearanceFloorChange:
		return Style{Color: p.FloorChange, Width: d.Width}
	case AppearanceCeilChange:
		return Style{Color: p.CeilChange, Width: d.Width}
	case AppearanceNoHeightDiff:
		return Style{Color: p.NoHeightDiff, Width: d.Width}
	case AppearanceUnrevealed:
		return Style{Color: p.Unrevealed, Width: d.Width}
	default:
		return Style{Color: p.NoHeightDiff, Width: d.Width}
	}
}
