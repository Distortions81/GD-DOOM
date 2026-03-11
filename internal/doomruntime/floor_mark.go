package doomruntime

func markFloorColumnRange(pl *floorVisplane, x, top, bottom int, floorclip, ceilingclip []int16) bool {
	if pl == nil || x < 0 || x >= len(floorclip) || x >= len(ceilingclip) {
		return false
	}
	ix := x + 1 // leave index 0 and len-1 for sentinels
	if ix < 0 || ix >= len(pl.top) || ix >= len(pl.bottom) {
		return false
	}

	t := top
	b := bottom
	clipTop := int(ceilingclip[x]) + 1
	clipBottom := int(floorclip[x]) - 1
	if t < clipTop {
		t = clipTop
	}
	if b > clipBottom {
		b = clipBottom
	}
	if t > b {
		return false
	}

	if pl.top[ix] == floorUnset || t < int(pl.top[ix]) {
		pl.top[ix] = int16(t)
	}
	if pl.bottom[ix] == floorUnset || b > int(pl.bottom[ix]) {
		pl.bottom[ix] = int16(b)
	}
	if x < pl.minX {
		pl.minX = x
	}
	if x > pl.maxX {
		pl.maxX = x
	}
	return true
}
