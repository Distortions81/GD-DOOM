package automap

import "gddoom/internal/doomrand"

// initMeltColumns mirrors Doom's wipe_initMelt setup and consumes M_Random.
func initMeltColumns(width int) []int {
	if width <= 0 {
		return nil
	}
	y := make([]int, width)
	y[0] = -(doomrand.MRandom() % 16)
	for i := 1; i < width; i++ {
		r := (doomrand.MRandom() % 3) - 1
		y[i] = y[i-1] + r
		if y[i] > 0 {
			y[i] = 0
		} else if y[i] == -16 {
			y[i] = -15
		}
	}
	return y
}

// initMeltColumnsScaled consumes M_Random multiple times and returns the last
// initialized column set. This lets source-port wipes scale RNG use with
// transition height while keeping Doom's per-init algorithm.
func initMeltColumnsScaled(width int, mult int) []int {
	if mult < 1 {
		mult = 1
	}
	var y []int
	for i := 0; i < mult; i++ {
		y = initMeltColumns(width)
	}
	return y
}

// stepMeltColumns advances the Doom wipe melt by ticks.
//
// This follows f_wipe.c behavior:
// - Uses y[] initialized from M_Random.
// - Advances once per tic.
// - Processes columns as 2-pixel pairs like Doom's short-buffer path.
func stepMeltColumns(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int) bool {
	if ticks <= 0 {
		return false
	}
	if width <= 0 || height <= 0 {
		return true
	}
	need := width * height * 4
	if len(fromPix) < need || len(toPix) < need || len(workPix) < need {
		return true
	}
	if len(y) < width {
		return true
	}

	pairW := width / 2
	if pairW <= 0 {
		copy(workPix[:need], toPix[:need])
		return true
	}

	for ; ticks > 0; ticks-- {
		done := true
		for i := 0; i < pairW; i++ {
			if y[i] < 0 {
				y[i]++
				done = false
				continue
			}
			if y[i] >= height {
				continue
			}

			dy := 8
			if y[i] < 16 {
				dy = y[i] + 1
			}
			if y[i]+dy >= height {
				dy = height - y[i]
			}

			x0 := i * 2
			for row := 0; row < dy; row++ {
				yPos := y[i] + row
				pixOff := (yPos*width + x0) * 4
				copy(workPix[pixOff:pixOff+8], toPix[pixOff:pixOff+8])
			}

			y[i] += dy
			for row := y[i]; row < height; row++ {
				srcY := row - y[i]
				srcOff := (srcY*width + x0) * 4
				dstOff := (row*width + x0) * 4
				copy(workPix[dstOff:dstOff+8], fromPix[srcOff:srcOff+8])
			}

			done = false
		}
		if done {
			copy(workPix[:need], toPix[:need])
			return true
		}
	}
	return false
}

// stepMeltSlices320 advances a Doom-like melt using fixed virtual slices.
// This is used for source-port presentation to preserve Doom-like RNG/motion
// while applying it across arbitrary screen widths.
func stepMeltSlices(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	if ticks <= 0 {
		return false
	}
	if width <= 0 || height <= 0 || slices <= 0 {
		return true
	}
	need := width * height * 4
	if len(fromPix) < need || len(toPix) < need || len(workPix) < need || len(y) < slices {
		return true
	}

	for ; ticks > 0; ticks-- {
		done := true
		for i := 0; i < slices; i++ {
			if y[i] < 0 {
				y[i]++
				done = false
				continue
			}
			if y[i] >= height {
				continue
			}

			dy := 8
			if y[i] < 16 {
				dy = y[i] + 1
			}
			if y[i]+dy >= height {
				dy = height - y[i]
			}

			x0 := (i * width) / slices
			x1 := ((i + 1) * width / slices) - 1
			if x1 < x0 {
				continue
			}
			colBytes := (x1 - x0 + 1) * 4

			for row := 0; row < dy; row++ {
				yPos := y[i] + row
				off := (yPos*width + x0) * 4
				copy(workPix[off:off+colBytes], toPix[off:off+colBytes])
			}

			y[i] += dy
			for row := y[i]; row < height; row++ {
				srcY := row - y[i]
				srcOff := (srcY*width + x0) * 4
				dstOff := (row*width + x0) * 4
				copy(workPix[dstOff:dstOff+colBytes], fromPix[srcOff:srcOff+colBytes])
			}
			done = false
		}
		if done {
			copy(workPix[:need], toPix[:need])
			return true
		}
	}
	return false
}

// stepMeltSlicesVirtual advances melt state in a virtual-height space and maps
// results to real pixels. This keeps melt motion profile stable across output
// resolutions while still applying wipe columns across the full real image.
func stepMeltSlicesVirtual(y []int, virtualH int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	if ticks <= 0 {
		return false
	}
	if width <= 0 || height <= 0 || slices <= 0 || virtualH <= 0 {
		return true
	}
	need := width * height * 4
	if len(fromPix) < need || len(toPix) < need || len(workPix) < need || len(y) < slices {
		return true
	}

	for ; ticks > 0; ticks-- {
		done := true
		for i := 0; i < slices; i++ {
			if y[i] < 0 {
				y[i]++
				done = false
				continue
			}
			if y[i] >= virtualH {
				continue
			}
			dy := 8
			if y[i] < 16 {
				dy = y[i] + 1
			}
			if y[i]+dy >= virtualH {
				dy = virtualH - y[i]
			}
			y[i] += dy
			done = false
		}

		composeMeltSlicesVirtual(y, virtualH, width, height, fromPix, toPix, workPix, slices)
		if done {
			copy(workPix[:need], toPix[:need])
			return true
		}
	}
	return false
}

func composeMeltSlicesVirtual(y []int, virtualH, width, height int, fromPix, toPix, workPix []byte, slices int) {
	for i := 0; i < slices; i++ {
		x0 := (i * width) / slices
		x1 := ((i + 1) * width / slices) - 1
		if x1 < x0 {
			continue
		}
		colBytes := (x1 - x0 + 1) * 4
		cut := y[i]
		if cut < 0 {
			cut = 0
		}
		if cut > virtualH {
			cut = virtualH
		}
		cutReal := (cut * height) / virtualH
		if cutReal < 0 {
			cutReal = 0
		}
		if cutReal > height {
			cutReal = height
		}
		for row := 0; row < height; row++ {
			dstOff := (row*width + x0) * 4
			vRow := (row * virtualH) / height
			if row < cutReal {
				srcRow := (vRow * height) / virtualH
				srcOff := (srcRow*width + x0) * 4
				copy(workPix[dstOff:dstOff+colBytes], toPix[srcOff:srcOff+colBytes])
				continue
			}
			srcV := vRow - cut
			if srcV < 0 {
				srcV = 0
			}
			srcRow := (srcV * height) / virtualH
			srcOff := (srcRow*width + x0) * 4
			copy(workPix[dstOff:dstOff+colBytes], fromPix[srcOff:srcOff+colBytes])
		}
	}
}
