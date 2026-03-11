package sessiontransition

import "gddoom/internal/doomrand"

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

func InitMeltColumns(width int) []int {
	return initMeltColumns(width)
}

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

func InitMeltColumnsScaled(width int, mult int) []int {
	return initMeltColumnsScaled(width, mult)
}

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

func StepMeltColumns(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int) bool {
	return stepMeltColumns(y, width, height, fromPix, toPix, workPix, ticks)
}

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

func StepMeltSlicesVirtual(y []int, virtualH int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	return stepMeltSlicesVirtual(y, virtualH, width, height, fromPix, toPix, workPix, ticks, slices)
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
			if row < cutReal {
				srcOff := (row*width + x0) * 4
				copy(workPix[dstOff:dstOff+colBytes], toPix[srcOff:srcOff+colBytes])
				continue
			}
			srcY := row - cutReal
			if srcY < 0 {
				srcY = 0
			}
			srcOff := (srcY*width + x0) * 4
			copy(workPix[dstOff:dstOff+colBytes], fromPix[srcOff:srcOff+colBytes])
		}
	}
}
