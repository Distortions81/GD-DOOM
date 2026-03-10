package linecache

import "image/color"

type Draw struct {
	X1  float32
	Y1  float32
	X2  float32
	Y2  float32
	W   float32
	Clr color.RGBA
}

type Key struct {
	CamX          float64
	CamY          float64
	Zoom          float64
	Angle         uint32
	RotateView    bool
	ViewW         int
	ViewH         int
	Reveal        int
	IDDT          int
	LineColorMode string
	MappedRev     uint32
}

type State struct {
	items []Draw
	key   Key
	rev   uint32
	init  bool
}

func (s *State) Revision() uint32 {
	if s == nil {
		return 0
	}
	return s.rev
}

func (s *State) Touch() {
	if s == nil {
		return
	}
	s.rev++
}

func (s *State) NeedsRebuild(key Key) bool {
	return s == nil || !s.init || s.key != key
}

func (s *State) Reset(draws []Draw, key Key) {
	if s == nil {
		return
	}
	s.items = draws
	s.key = key
	s.init = true
}

func (s *State) Reuse() []Draw {
	if s == nil {
		return nil
	}
	return s.items[:0]
}

func (s *State) Items() []Draw {
	if s == nil {
		return nil
	}
	return s.items
}
