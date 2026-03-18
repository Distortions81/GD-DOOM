package mapview

type BigMapState struct {
	enabled bool
}

func (s *BigMapState) Enabled() bool {
	return s != nil && s.enabled
}

func (s *BigMapState) Toggle(view *ViewState, centerX, centerY float64) bool {
	if s == nil || view == nil {
		return false
	}
	s.enabled = view.ToggleBigMap(centerX, centerY)
	return s.enabled
}
