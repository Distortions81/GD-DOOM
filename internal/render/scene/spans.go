package scene

func SpriteColumnOccludesPoint(wall WallDepthColumn, masked []MaskedClipSpan, y int, depthQ uint16) bool {
	if WallDepthColumnOccludesPoint(wall, y, depthQ) {
		return true
	}
	return MaskedClipColumnOccludesPoint(masked, y, depthQ)
}

func SpriteColumnOccludesBBox(wall WallDepthColumn, y0, y1 int, depthQ uint16) bool {
	return WallDepthColumnOccludesBBox(wall, y0, y1, depthQ)
}

func SpriteColumnHasAnyOccluder(wall WallDepthColumn, masked []MaskedClipSpan, y0, y1 int, depthQ uint16) bool {
	if WallDepthColumnHasAnyOccluder(wall, y0, y1, depthQ) {
		return true
	}
	return MaskedClipColumnHasAnyOccluder(masked, y0, y1, depthQ)
}

func AppendVisibleRowSpans(x0, x1 int, clipCount int, clipAt func(i int) (int, int), columnOccluded func(x int) bool, appendSpan func(l, r int)) {
	appendVisible := func(l, r int) {
		if l > r {
			return
		}
		runStart := -1
		for x := l; x <= r; x++ {
			if columnOccluded(x) {
				if runStart >= 0 {
					appendSpan(runStart, x-1)
					runStart = -1
				}
				continue
			}
			if runStart < 0 {
				runStart = x
			}
		}
		if runStart >= 0 {
			appendSpan(runStart, r)
		}
	}

	if clipCount == 0 {
		appendVisible(x0, x1)
		return
	}
	for i := 0; i < clipCount; i++ {
		l, r := clipAt(i)
		if l < x0 {
			l = x0
		}
		if r > x1 {
			r = x1
		}
		appendVisible(l, r)
	}
}
