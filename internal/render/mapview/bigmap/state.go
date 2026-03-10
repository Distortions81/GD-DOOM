package bigmap

import "gddoom/internal/render/mapview/viewstate"

type State struct {
	enabled bool
}

func (s *State) Enabled() bool {
	return s != nil && s.enabled
}

func (s *State) Toggle(view *viewstate.State, centerX, centerY float64) bool {
	if s == nil || view == nil {
		return false
	}
	s.enabled = view.ToggleBigMap(centerX, centerY)
	return s.enabled
}
