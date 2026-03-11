package sessiontransition

func initMeltColumns(width int) []int {
	if width <= 0 {
		return nil
	}
	y := make([]int, width)
	y[0] = -(meltRand()%16 + 1)
	for i := 1; i < width; i++ {
		r := (meltRand() % 3) - 1
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
	y := initMeltColumns(width)
	if mult == 1 {
		return y
	}
	for i := range y {
		y[i] *= mult
	}
	return y
}

func InitMeltColumnsScaled(width int, mult int) []int {
	return initMeltColumnsScaled(width, mult)
}

func stepMeltColumns(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int) bool {
	if width <= 0 || height <= 0 || len(y) != width {
		return true
	}
	if len(fromPix) < width*height*4 || len(toPix) < width*height*4 || len(workPix) < width*height*4 {
		return true
	}
	for t := 0; t < ticks; t++ {
		done := true
		for x := 0; x < width; x++ {
			yy := y[x]
			if yy < 0 {
				y[x]++
				done = false
				continue
			}
			if yy >= height {
				continue
			}
			done = false
			dy := 8
			if yy < 16 {
				dy = yy + 1
			}
			if yy+dy > height {
				dy = height - yy
			}
			copyMeltColumn(workPix, toPix, width, x, 0, yy)
			copyMeltColumn(workPix, fromPix, width, x, yy, height-yy-dy)
			copyMeltColumn(workPix, toPix, width, x, yy+height-yy-dy, dy)
			y[x] += dy
		}
		if done {
			return true
		}
	}
	return false
}

func StepMeltColumns(y []int, width, height int, fromPix, toPix, workPix []byte, ticks int) bool {
	return stepMeltColumns(y, width, height, fromPix, toPix, workPix, ticks)
}

func copyMeltColumn(dst, src []byte, width, x, y0, count int) {
	if count <= 0 {
		return
	}
	for y := 0; y < count; y++ {
		si := ((y0+y)*width + x) * 4
		copy(dst[si:si+4], src[si:si+4])
	}
}

func stepMeltSlicesVirtual(y []int, virtualH int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	if width <= 0 || height <= 0 || virtualH <= 0 || slices <= 0 || len(y) != slices {
		return true
	}
	if len(fromPix) < width*height*4 || len(toPix) < width*height*4 || len(workPix) < width*height*4 {
		return true
	}
	sliceW := width / slices
	if sliceW < 1 {
		sliceW = 1
	}
	for t := 0; t < ticks; t++ {
		done := true
		for i := 0; i < slices; i++ {
			yy := y[i]
			if yy < 0 {
				y[i]++
				done = false
				continue
			}
			if yy >= virtualH {
				continue
			}
			done = false
			dy := 8
			if yy < 16 {
				dy = yy + 1
			}
			if yy+dy > virtualH {
				dy = virtualH - yy
			}
			x0 := i * sliceW
			x1 := x0 + sliceW
			if i == slices-1 || x1 > width {
				x1 = width
			}
			copyMeltSliceVirtual(workPix, toPix, width, height, x0, x1, 0, yy, virtualH)
			copyMeltSliceVirtual(workPix, fromPix, width, height, x0, x1, yy, virtualH-dy-yy, virtualH)
			copyMeltSliceVirtual(workPix, toPix, width, height, x0, x1, yy+(virtualH-dy-yy), dy, virtualH)
			y[i] += dy
		}
		if done {
			return true
		}
	}
	return false
}

func StepMeltSlicesVirtual(y []int, virtualH int, width, height int, fromPix, toPix, workPix []byte, ticks int, slices int) bool {
	return stepMeltSlicesVirtual(y, virtualH, width, height, fromPix, toPix, workPix, ticks, slices)
}

func copyMeltSliceVirtual(dst, src []byte, width, height, x0, x1, y0, count, virtualH int) {
	if count <= 0 || x0 >= x1 {
		return
	}
	for vy := 0; vy < count; vy++ {
		sy := ((y0 + vy) * height) / virtualH
		dy := sy
		if sy < 0 || sy >= height || dy < 0 || dy >= height {
			continue
		}
		for x := x0; x < x1; x++ {
			si := (sy*width + x) * 4
			di := (dy*width + x) * 4
			copy(dst[di:di+4], src[si:si+4])
		}
	}
}

var meltSeed uint32 = 1

func meltRand() int {
	meltSeed = meltSeed*1664525 + 1013904223
	return int((meltSeed >> 16) & 0x7fff)
}
