package hud

// FlashPaletteIndex matches ST_doPaletteStuff, including its +1 red/gold
// palette offsets (even the faintest flash selects the second colored row).
func FlashPaletteIndex(damage, bonus, strength, suit int) int {
	count := damage
	if strength > 0 {
		count = max(count, 12-(strength>>6))
	}
	if count > 0 {
		return 1 + min(7, (count+7)>>3)
	}
	if bonus > 0 {
		return 9 + min(3, (bonus+7)>>3)
	}
	if suit > 128 || suit&8 != 0 {
		return 13
	}
	return 0
}

// PaletteChannels preserves PLAYPAL's exact integer channel values. Doom's
// flash palettes transform each RGB channel independently. Interpolate between
// known inputs for modern blended pixels that aren't original palette colors.
func PaletteChannels(base, target []byte) []byte {
	if len(base) != 1024 || len(target) != 1024 {
		return nil
	}
	lut := make([]byte, 1024)
	for c := 0; c < 3; c++ {
		var known [256]bool
		for i := 0; i < 256; i++ {
			x, y := int(base[i*4+c]), target[i*4+c]
			if known[x] && lut[x*4+c] != y {
				return nil
			}
			known[x] = true
			lut[x*4+c] = y
		}
		for x := 0; x < 256; x++ {
			if known[x] {
				continue
			}
			lo, hi := x, x
			for lo > 0 && !known[lo] {
				lo--
			}
			for hi < 255 && !known[hi] {
				hi++
			}
			if !known[lo] {
				lo = hi
			}
			if !known[hi] {
				hi = lo
			}
			if lo == hi {
				lut[x*4+c] = lut[lo*4+c]
			} else {
				lut[x*4+c] = byte((int(lut[lo*4+c])*(hi-x) + int(lut[hi*4+c])*(x-lo) + (hi-lo)/2) / (hi - lo))
			}
		}
	}
	for i := 0; i < 256; i++ {
		lut[i*4+3] = 255
	}
	return lut
}
