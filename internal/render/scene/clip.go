package scene

type MaskedClipSpan struct {
	Y0      int16
	Y1      int16
	OpenY0  int16
	OpenY1  int16
	DepthQ  uint16
	Closed  bool
	HasOpen bool
}

func MaskedClipColumnOccludesPoint(spans []MaskedClipSpan, y int, depthQ uint16) bool {
	for _, sp := range spans {
		if depthQ <= sp.DepthQ {
			continue
		}
		if sp.Closed {
			return true
		}
		if sp.HasOpen {
			if y < int(sp.OpenY0) || y > int(sp.OpenY1) {
				return true
			}
			continue
		}
		if y >= int(sp.Y0) && y <= int(sp.Y1) {
			return true
		}
	}
	return false
}

func MaskedClipColumnOccludesPointSorted(spans []MaskedClipSpan, y int, depthQ uint16) bool {
	if len(spans) == 0 {
		return false
	}
	if depthQ <= spans[0].DepthQ {
		return false
	}
	lo, hi := 0, len(spans)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if spans[mid].DepthQ < depthQ {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	limit := lo
	for i := limit - 1; i >= 0; i-- {
		sp := spans[i]
		if sp.Closed {
			return true
		}
		if sp.HasOpen {
			if y < int(sp.OpenY0) || y > int(sp.OpenY1) {
				return true
			}
			continue
		}
		if y >= int(sp.Y0) && y <= int(sp.Y1) {
			return true
		}
	}
	return false
}

func MaskedClipColumnHasAnyOccluder(spans []MaskedClipSpan, y0, y1 int, depthQ uint16) bool {
	for _, sp := range spans {
		if depthQ <= sp.DepthQ {
			continue
		}
		if sp.Closed {
			return true
		}
		if sp.HasOpen {
			if y0 < int(sp.OpenY0) || y1 > int(sp.OpenY1) {
				return true
			}
			continue
		}
		if y0 <= int(sp.Y1) && y1 >= int(sp.Y0) {
			return true
		}
	}
	return false
}
