package mapview

import "image/color"

type CachedLine struct {
	X1  float32
	Y1  float32
	X2  float32
	Y2  float32
	W   float32
	Clr color.RGBA
}

type LineCacheKey struct {
	CamX          float64
	CamY          float64
	Zoom          float64
	Angle         uint32
	RotateView    bool
	ViewW         int
	ViewH         int
	Reveal        int
	IDDT          int
	MappedRev     uint32
}

type LineCacheState struct {
	items []CachedLine
	key   LineCacheKey
	rev   uint32
	init  bool
}

func (s *LineCacheState) Revision() uint32 {
	if s == nil {
		return 0
	}
	return s.rev
}

func (s *LineCacheState) Touch() {
	if s == nil {
		return
	}
	s.rev++
}

func (s *LineCacheState) NeedsRebuild(key LineCacheKey) bool {
	return s == nil || !s.init || s.key != key
}

func (s *LineCacheState) Reset(draws []CachedLine, key LineCacheKey) {
	if s == nil {
		return
	}
	s.items = draws
	s.key = key
	s.init = true
}

func (s *LineCacheState) Reuse() []CachedLine {
	if s == nil {
		return nil
	}
	return s.items[:0]
}

func (s *LineCacheState) Items() []CachedLine {
	if s == nil {
		return nil
	}
	return s.items
}
