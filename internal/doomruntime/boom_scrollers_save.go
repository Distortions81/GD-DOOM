package doomruntime

import "fmt"

func (w saveBinaryWriter) boomScrollerSlice(values []boomScrollerState) error {
	if err := w.u32(uint32(len(values))); err != nil {
		return err
	}
	for _, s := range values {
		if err := w.u8(s.Kind); err != nil {
			return err
		}
		if err := w.int(s.Target); err != nil {
			return err
		}
		if err := w.int(s.Control); err != nil {
			return err
		}
		if err := w.bool(s.Accel); err != nil {
			return err
		}
		for _, v := range []int64{s.Order, s.DX, s.DY, s.LastHeight, s.VX, s.VY, s.OffsetX, s.OffsetY} {
			if err := w.i64(v); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r saveBinaryReader) boomScrollerSlice() ([]boomScrollerState, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	if uint64(n) > uint64(r.remaining()/74) {
		return nil, fmt.Errorf("truncated Boom scroller state")
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]boomScrollerState, n)
	for i := range out {
		s := &out[i]
		if s.Kind, err = r.u8(); err != nil {
			return nil, err
		}
		if s.Target, err = r.int(); err != nil {
			return nil, err
		}
		if s.Control, err = r.int(); err != nil {
			return nil, err
		}
		if s.Accel, err = r.bool(); err != nil {
			return nil, err
		}
		for _, v := range []*int64{&s.Order, &s.DX, &s.DY, &s.LastHeight, &s.VX, &s.VY, &s.OffsetX, &s.OffsetY} {
			if *v, err = r.i64(); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
func (g *game) validBoomScrollerStates(states []boomScrollerState) bool {
	if len(states) != len(g.boomScrollers) {
		return false
	}
	for i, s := range states {
		base := g.boomScrollers[i]
		if s.Order != base.Order || s.Kind != base.Kind || s.Target != base.Target || s.Control != base.Control || s.DX != base.DX || s.DY != base.DY || s.Accel != base.Accel {
			return false
		}
		for _, v := range []int64{s.LastHeight, s.VX, s.VY, s.OffsetX, s.OffsetY} {
			if v != boomFixed(v) {
				return false
			}
		}
	}
	return true
}
