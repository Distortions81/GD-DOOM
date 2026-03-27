package doomruntime

import "math"

const planeTexelChangeSentinel = int(^uint(0) >> 1)

type planeTexStepper struct {
	uFixed     int64
	vFixed     int64
	stepUFixed int64
	stepVFixed int64
}

type planeRowRenderState struct {
	depth          float64
	depthQ         uint16
	rowBaseWXFixed int64
	rowBaseWYFixed int64
	stepWXFixed    int64
	stepWYFixed    int64
	defaultShade   uint32
	defaultRow     int
}

func (s planeTexStepper) texelIndex() int {
	u := int(s.uFixed>>fracBits) & 63
	v := int(s.vFixed>>fracBits) & 63
	return (v << 6) + u
}

func (s *planeTexStepper) advance(n int) {
	step := int64(n)
	s.uFixed += step * s.stepUFixed
	s.vFixed += step * s.stepVFixed
}

func (s planeTexStepper) repeatEligible() bool {
	return abs64(s.stepUFixed) < fracUnit && abs64(s.stepVFixed) < fracUnit
}

func (s planeTexStepper) stepsUntilTexelChange() int {
	uRun := stepsUntilTexelChange(s.uFixed, s.stepUFixed)
	vRun := stepsUntilTexelChange(s.vFixed, s.stepVFixed)
	if uRun < vRun {
		return uRun
	}
	return vRun
}

func stepsUntilTexelChange(fixed, step int64) int {
	if step == 0 {
		return planeTexelChangeSentinel
	}
	cellStart := (fixed >> fracBits) << fracBits
	if step > 0 {
		nextBoundary := cellStart + fracUnit
		diff := nextBoundary - fixed
		if diff <= 0 {
			return 1
		}
		return int((diff + step - 1) / step)
	}
	return int((fixed-cellStart)/(-step)) + 1
}

func stepsUntilTexelChangeFrac(frac, step int) int {
	if step == 0 {
		return planeTexelChangeSentinel
	}
	if step > 0 {
		diff := fracUnit - frac
		if diff <= 0 {
			return 1
		}
		return (diff + step - 1) / step
	}
	return frac/(-step) + 1
}

func fillPackedRun(dst []uint32, dstI, count int, packed uint32) {
	if count <= 0 {
		return
	}
	if count < 8 {
		for ; count >= 4; count -= 4 {
			dst[dstI] = packed
			dst[dstI+1] = packed
			dst[dstI+2] = packed
			dst[dstI+3] = packed
			dstI += 4
		}
		for ; count > 0; count-- {
			dst[dstI] = packed
			dstI++
		}
		return
	}
	run := dst[dstI : dstI+count]
	run[0] = packed
	filled := 1
	for filled < len(run) {
		n := filled
		if remaining := len(run) - filled; n > remaining {
			n = remaining
		}
		copy(run[filled:filled+n], run[:n])
		filled += n
	}
}

func fillPackedRunStride(dst []uint32, dstI, stride, count int, packed uint32) {
	for ; count >= 4; count -= 4 {
		dst[dstI] = packed
		dst[dstI+stride] = packed
		dst[dstI+stride*2] = packed
		dst[dstI+stride*3] = packed
		dstI += stride * 4
	}
	for ; count > 0; count-- {
		dst[dstI] = packed
		dstI += stride
	}
}

func doomColormapPackedRow(row int) []uint32 {
	rows := doomColormapRowCount()
	if rows <= 0 || len(doomColormapRGBA) < rows*256 {
		return nil
	}
	if row < 0 {
		row = 0
	}
	if row >= rows {
		row = rows - 1
	}
	base := row * 256
	return doomColormapRGBA[base : base+256]
}

func drawPlaneSpanIndexedPackedFast(dst []uint32, dstI, count int, texIndexed []byte, packedRow []uint32, stepper planeTexStepper) planeTexStepper {
	const fracMask = fracUnit - 1
	uStep := int(stepper.stepUFixed)
	vStep := int(stepper.stepVFixed)
	uFrac := int(stepper.uFixed & fracMask)
	vFrac := int(stepper.vFixed & fracMask)
	uRun := stepsUntilTexelChangeFrac(uFrac, uStep)
	vRun := stepsUntilTexelChangeFrac(vFrac, vStep)
	for remaining := count; remaining > 0; {
		texIdx := stepper.texelIndex()
		run := uRun
		if vRun < run {
			run = vRun
		}
		if run > remaining {
			run = remaining
		}
		fillPackedRun(dst, dstI, run, packedRow[texIndexed[texIdx]])
		dstI += run
		remaining -= run
		stepper.advance(run)
		uFrac = (uFrac + run*uStep) & fracMask
		vFrac = (vFrac + run*vStep) & fracMask
		if remaining <= 0 {
			break
		}
		if uRun == run {
			uRun = stepsUntilTexelChangeFrac(uFrac, uStep)
		} else {
			uRun -= run
		}
		if vRun == run {
			vRun = stepsUntilTexelChangeFrac(vFrac, vStep)
		} else {
			vRun -= run
		}
	}
	return stepper
}

func drawPlaneSpanIndexedPackedScalar(dst []uint32, dstI, count int, texIndexed []byte, packedRow []uint32, stepper planeTexStepper) planeTexStepper {
	for ; count >= 4; count -= 4 {
		dst[dstI] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dst[dstI+1] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dst[dstI+2] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dst[dstI+3] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dstI += 4
	}
	for ; count > 0; count-- {
		dst[dstI] = packedRow[texIndexed[stepper.texelIndex()]]
		stepper.advance(1)
		dstI++
	}
	return stepper
}

func drawPlaneSpanIndexedPacked(dst []uint32, dstI, count int, texIndexed []byte, packedRow []uint32, stepper planeTexStepper) planeTexStepper {
	if count <= 0 || len(texIndexed) != 64*64 || len(packedRow) != 256 {
		return stepper
	}
	if stepper.repeatEligible() {
		return drawPlaneSpanIndexedPackedFast(dst, dstI, count, texIndexed, packedRow, stepper)
	}
	return drawPlaneSpanIndexedPackedScalar(dst, dstI, count, texIndexed, packedRow, stepper)
}

func drawPlaneSpanIndexedDOOMRowScalar(dst []uint32, dstI, count int, texIndexed []byte, row int, stepper planeTexStepper) planeTexStepper {
	for ; count >= 4; count -= 4 {
		dst[dstI] = shadePaletteIndexDOOMRow(texIndexed[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+1] = shadePaletteIndexDOOMRow(texIndexed[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+2] = shadePaletteIndexDOOMRow(texIndexed[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+3] = shadePaletteIndexDOOMRow(texIndexed[stepper.texelIndex()], row)
		stepper.advance(1)
		dstI += 4
	}
	for ; count > 0; count-- {
		dst[dstI] = shadePaletteIndexDOOMRow(texIndexed[stepper.texelIndex()], row)
		stepper.advance(1)
		dstI++
	}
	return stepper
}

func planeSpanDepth(y int, key plane3DKey, eyeZ, focal, cy float64) (float64, uint16, bool) {
	den := cy - (float64(y) + 0.5)
	if math.Abs(den) < 1e-6 {
		return 0, 0, false
	}
	planeZ := float64(key.height)
	depth := ((planeZ - eyeZ) / den) * focal
	if depth <= 0 {
		return 0, 0, false
	}
	return depth, encodeDepthQ(depth), true
}

func (g *game) planeRowRenderState(y int, key plane3DKey, eyeZ, camX, camY, ca, sa, focal, cx, cy float64) (planeRowRenderState, bool) {
	depth, depthQ, ok := planeSpanDepth(y, key, eyeZ, focal, cy)
	if !ok {
		return planeRowRenderState{}, false
	}
	stepWX := (depth / focal) * sa
	stepWY := -(depth / focal) * ca
	rowBaseWX := camX + depth*ca - ((cx-0.5)*depth/focal)*sa
	rowBaseWY := camY + depth*sa + ((cx-0.5)*depth/focal)*ca
	state := planeRowRenderState{
		depth:          depth,
		depthQ:         depthQ,
		rowBaseWXFixed: floorFixed(rowBaseWX),
		rowBaseWYFixed: floorFixed(rowBaseWY),
		stepWXFixed:    floorFixed(stepWX),
		stepWYFixed:    floorFixed(stepWY),
		defaultShade:   uint32(sectorLightMul(key.light)),
	}
	if doomLightingEnabled {
		state.defaultRow = doomPlaneLightRow(key.light, depth)
		if row, ok := g.playerFixedColormapRow(); ok {
			state.defaultRow = row
		}
		if !doomColormapEnabled {
			if g != nil && !g.opts.SourcePortMode {
				state.defaultShade = uint32(doomShadeMulFromRow(state.defaultRow))
			} else {
				state.defaultShade = uint32(doomShadeMulFromRowF(doomPlaneLightRowF(key.light, depth)))
			}
		}
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		state.defaultRow = row
	}
	return state, true
}

func (g *game) drawPlaneTexturedSpanAtDepth(pix32 []uint32, rowPix, x1, x2 int, key plane3DKey, tex32 []uint32, texIndexed []byte, state planeRowRenderState) {
	xOff := int64(x1)
	stepper := planeTexStepper{
		uFixed:     state.rowBaseWXFixed + xOff*state.stepWXFixed,
		vFixed:     state.rowBaseWYFixed + xOff*state.stepWYFixed,
		stepUFixed: state.stepWXFixed,
		stepVFixed: state.stepWYFixed,
	}
	count := x2 - x1 + 1
	pixI := rowPix + x1
	if !doomColormapEnabled && state.defaultShade == 0 {
		fillPackedRun(pix32, pixI, count, pixelOpaqueA)
		return
	}
	if state.defaultRow >= doomNumColorMaps || doomColormapEnabled {
		if len(texIndexed) == 64*64 {
			if packedRow := doomColormapPackedRow(state.defaultRow); len(packedRow) == 256 {
				drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, packedRow, stepper)
			} else {
				drawPlaneSpanIndexedDOOMRowScalar(pix32, pixI, count, texIndexed, state.defaultRow, stepper)
			}
		}
		return
	}
	if fullbrightNoLighting {
		if len(texIndexed) == 64*64 && wallShadePackedOK {
			drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, wallShadePackedLUT[256][:], stepper)
		}
		return
	}
	if len(texIndexed) == 64*64 && wallShadePackedOK {
		if state.defaultShade > 256 {
			state.defaultShade = 256
		}
		drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, wallShadePackedLUT[state.defaultShade][:], stepper)
	}
}
