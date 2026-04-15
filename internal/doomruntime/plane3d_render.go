package doomruntime

import (
	"math"
	"unsafe"
)

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
	pair := (uint64(packed) << 32) | uint64(packed)
	if count < 32 {
		ptr := unsafe.Pointer(&dst[dstI])
		for ; count >= 8; count -= 8 {
			*(*uint64)(ptr) = pair
			*(*uint64)(unsafe.Add(ptr, 8)) = pair
			*(*uint64)(unsafe.Add(ptr, 16)) = pair
			*(*uint64)(unsafe.Add(ptr, 24)) = pair
			ptr = unsafe.Add(ptr, 32)
		}
		for ; count >= 2; count -= 2 {
			*(*uint64)(ptr) = pair
			ptr = unsafe.Add(ptr, 8)
		}
		if count > 0 {
			*(*uint32)(ptr) = packed
		}
		return
	}
	run := dst[dstI : dstI+count]
	filled := 1
	if len(run) >= 2 {
		*(*uint64)(unsafe.Pointer(&run[0])) = pair
		filled = 2
	} else {
		run[0] = packed
	}
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
	total := count
	uStep := int(stepper.stepUFixed)
	vStep := int(stepper.stepVFixed)
	uFrac := int(stepper.uFixed & fracMask)
	vFrac := int(stepper.vFixed & fracMask)
	uTex := int(stepper.uFixed>>fracBits) & 63
	vTex := int(stepper.vFixed>>fracBits) & 63
	uTexDelta := 0
	if uStep > 0 {
		uTexDelta = 1
	} else if uStep < 0 {
		uTexDelta = -1
	}
	vTexDelta := 0
	if vStep > 0 {
		vTexDelta = 1
	} else if vStep < 0 {
		vTexDelta = -1
	}
	uRun := stepsUntilTexelChangeFrac(uFrac, uStep)
	vRun := stepsUntilTexelChangeFrac(vFrac, vStep)
	fillRun := func(run int, packed uint32) {
		switch run {
		case 1:
			dst[dstI] = packed
		case 2:
			*(*uint64)(unsafe.Pointer(&dst[dstI])) = (uint64(packed) << 32) | uint64(packed)
		case 3:
			*(*uint64)(unsafe.Pointer(&dst[dstI])) = (uint64(packed) << 32) | uint64(packed)
			dst[dstI+2] = packed
		default:
			fillPackedRun(dst, dstI, run, packed)
		}
	}
	if uStep == 0 {
		for remaining := count; remaining > 0; {
			run := vRun
			if run > remaining {
				run = remaining
			}
			fillRun(run, packedRow[texIndexed[(vTex<<6)+uTex]])
			dstI += run
			remaining -= run
			vFrac = (vFrac + run*vStep) & fracMask
			if remaining <= 0 {
				break
			}
			vTex = (vTex + vTexDelta) & 63
			vRun = stepsUntilTexelChangeFrac(vFrac, vStep)
		}
		stepper.advance(total)
		return stepper
	}
	if vStep == 0 {
		for remaining := count; remaining > 0; {
			run := uRun
			if run > remaining {
				run = remaining
			}
			fillRun(run, packedRow[texIndexed[(vTex<<6)+uTex]])
			dstI += run
			remaining -= run
			uFrac = (uFrac + run*uStep) & fracMask
			if remaining <= 0 {
				break
			}
			uTex = (uTex + uTexDelta) & 63
			uRun = stepsUntilTexelChangeFrac(uFrac, uStep)
		}
		stepper.advance(total)
		return stepper
	}
	for remaining := count; remaining > 0; {
		run := uRun
		if vRun < run {
			run = vRun
		}
		if run > remaining {
			run = remaining
		}
		fillRun(run, packedRow[texIndexed[(vTex<<6)+uTex]])
		dstI += run
		remaining -= run
		uFrac = (uFrac + run*uStep) & fracMask
		vFrac = (vFrac + run*vStep) & fracMask
		if remaining <= 0 {
			break
		}
		if uRun == run {
			uTex = (uTex + uTexDelta) & 63
			uRun = stepsUntilTexelChangeFrac(uFrac, uStep)
		} else {
			uRun -= run
		}
		if vRun == run {
			vTex = (vTex + vTexDelta) & 63
			vRun = stepsUntilTexelChangeFrac(vFrac, vStep)
		} else {
			vRun -= run
		}
	}
	stepper.advance(total)
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

func drawPlaneSpanIndexedPackedUnchecked(dst []uint32, dstI, count int, texIndexed []byte, packedRow []uint32, stepper planeTexStepper) planeTexStepper {
	if count <= 0 {
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

func planeSpanDepth(y int, key plane3DKey, eyeZ, focal, focalV, cy float64) (float64, uint16, bool) {
	den := cy - (float64(y) + 0.5)
	if math.Abs(den) < 1e-6 {
		return 0, 0, false
	}
	planeZ := float64(key.height)
	depth := ((planeZ - eyeZ) / den) * focalV
	if depth <= 0 {
		return 0, 0, false
	}
	return depth, encodeDepthQ(depth), true
}

func (g *game) planeRowRenderState(y int, key plane3DKey, eyeZ, camX, camY, ca, sa, focal, focalV, cx, cy float64) (planeRowRenderState, bool) {
	depth, depthQ, ok := planeSpanDepth(y, key, eyeZ, focal, focalV, cy)
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

func (g *game) drawPlaneTexturedSpanAtDepth(pix32 []uint32, rowPix, x1, x2 int, key plane3DKey, sample flatTextureBlendSample, state planeRowRenderState) {
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
	texIndexed := sample.fromIndexed
	if len(texIndexed) != 64*64 {
		return
	}
	if state.defaultRow >= doomNumColorMaps || doomColormapEnabled {
		if packedRow := doomColormapPackedRow(state.defaultRow); len(packedRow) == 256 {
			drawPlaneSpanIndexedPackedUnchecked(pix32, pixI, count, texIndexed, packedRow, stepper)
		} else {
			drawPlaneSpanIndexedDOOMRowScalar(pix32, pixI, count, texIndexed, state.defaultRow, stepper)
		}
		return
	}
	if fullbrightNoLighting {
		drawPlaneSpanIndexedPackedUnchecked(pix32, pixI, count, texIndexed, wallShadePackedLUT[256][:], stepper)
		return
	}
	if state.defaultShade > 256 {
		state.defaultShade = 256
	}
	drawPlaneSpanIndexedPackedUnchecked(pix32, pixI, count, texIndexed, wallShadePackedLUT[state.defaultShade][:], stepper)
}
