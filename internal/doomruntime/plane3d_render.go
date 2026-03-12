package doomruntime

import "math"

const planeTexelChangeSentinel = int(^uint(0) >> 1)

type planeTexStepper struct {
	uFixed     int64
	vFixed     int64
	stepUFixed int64
	stepVFixed int64
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

func fillPackedRun(dst []uint32, dstI, count int, packed uint32) {
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
}

func doomColormapPackedRow(row int) []uint32 {
	rows := doomShadeRows()
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
	for remaining := count; remaining > 0; {
		texIdx := stepper.texelIndex()
		run := stepper.stepsUntilTexelChange()
		if run > remaining {
			run = remaining
		}
		fillPackedRun(dst, dstI, run, packedRow[texIndexed[texIdx]])
		dstI += run
		remaining -= run
		stepper.advance(run)
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

func drawPlaneSpanRGBADOOMRow(dst []uint32, dstI, count int, tex32 []uint32, row int, stepper planeTexStepper) planeTexStepper {
	for ; count >= 4; count -= 4 {
		dst[dstI] = shadePackedDOOMColormapRow(tex32[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+1] = shadePackedDOOMColormapRow(tex32[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+2] = shadePackedDOOMColormapRow(tex32[stepper.texelIndex()], row)
		stepper.advance(1)
		dst[dstI+3] = shadePackedDOOMColormapRow(tex32[stepper.texelIndex()], row)
		stepper.advance(1)
		dstI += 4
	}
	for ; count > 0; count-- {
		dst[dstI] = shadePackedDOOMColormapRow(tex32[stepper.texelIndex()], row)
		stepper.advance(1)
		dstI++
	}
	return stepper
}

func drawPlaneSpanRGBA(dst []uint32, dstI, count int, tex32 []uint32, shadeMul uint32, stepper planeTexStepper) planeTexStepper {
	if shadeMul > 256 {
		shadeMul = 256
	}
	if shadeMul == 256 {
		for ; count >= 4; count -= 4 {
			dst[dstI] = tex32[stepper.texelIndex()]
			stepper.advance(1)
			dst[dstI+1] = tex32[stepper.texelIndex()]
			stepper.advance(1)
			dst[dstI+2] = tex32[stepper.texelIndex()]
			stepper.advance(1)
			dst[dstI+3] = tex32[stepper.texelIndex()]
			stepper.advance(1)
			dstI += 4
		}
		for ; count > 0; count-- {
			dst[dstI] = tex32[stepper.texelIndex()]
			stepper.advance(1)
			dstI++
		}
		return stepper
	}
	for ; count >= 4; count -= 4 {
		dst[dstI] = shadePackedRGBA(tex32[stepper.texelIndex()], shadeMul)
		stepper.advance(1)
		dst[dstI+1] = shadePackedRGBA(tex32[stepper.texelIndex()], shadeMul)
		stepper.advance(1)
		dst[dstI+2] = shadePackedRGBA(tex32[stepper.texelIndex()], shadeMul)
		stepper.advance(1)
		dst[dstI+3] = shadePackedRGBA(tex32[stepper.texelIndex()], shadeMul)
		stepper.advance(1)
		dstI += 4
	}
	for ; count > 0; count-- {
		dst[dstI] = shadePackedRGBA(tex32[stepper.texelIndex()], shadeMul)
		stepper.advance(1)
		dstI++
	}
	return stepper
}

func planeFallbackPacked(fbPacked uint32, defaultShade uint32, defaultRow int) uint32 {
	if doomColormapEnabled {
		return shadePackedDOOMColormapRow(fbPacked, defaultRow)
	}
	if fullbrightNoLighting {
		return fbPacked
	}
	return shadePackedRGBA(fbPacked, defaultShade)
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

func (g *game) drawPlaneTexturedSpanAtDepth(pix32 []uint32, rowPix, x1, x2 int, key plane3DKey, fbPacked uint32, tex32 []uint32, texIndexed []byte, flatTexReady bool, camX, camY, ca, sa, focal, cx float64, depth float64) {
	stepWX := (depth / focal) * sa
	stepWY := -(depth / focal) * ca
	rowBaseWX := camX + depth*ca - ((cx-0.5)*depth/focal)*sa
	rowBaseWY := camY + depth*sa + ((cx-0.5)*depth/focal)*ca
	rowBaseWXFixed := floorFixed(rowBaseWX)
	rowBaseWYFixed := floorFixed(rowBaseWY)
	stepWXFixed := floorFixed(stepWX)
	stepWYFixed := floorFixed(stepWY)
	xOff := int64(x1)
	stepper := planeTexStepper{
		uFixed:     rowBaseWXFixed + xOff*stepWXFixed,
		vFixed:     rowBaseWYFixed + xOff*stepWYFixed,
		stepUFixed: stepWXFixed,
		stepVFixed: stepWYFixed,
	}
	defaultShade := uint32(sectorLightMul(key.light))
	defaultRow := 0
	if doomLightingEnabled {
		defaultRow = doomPlaneLightRow(key.light, depth)
		if !doomColormapEnabled {
			defaultShade = uint32(doomShadeMulFromRowF(doomPlaneLightRowF(key.light, depth)))
		}
	}
	count := x2 - x1 + 1
	pixI := rowPix + x1
	if !flatTexReady {
		fillPackedRun(pix32, pixI, count, planeFallbackPacked(fbPacked, defaultShade, defaultRow))
		return
	}
	if doomColormapEnabled {
		if len(texIndexed) == 64*64 {
			if packedRow := doomColormapPackedRow(defaultRow); len(packedRow) == 256 {
				drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, packedRow, stepper)
			} else {
				drawPlaneSpanIndexedDOOMRowScalar(pix32, pixI, count, texIndexed, defaultRow, stepper)
			}
			return
		}
		drawPlaneSpanRGBADOOMRow(pix32, pixI, count, tex32, defaultRow, stepper)
		return
	}
	if fullbrightNoLighting {
		if len(texIndexed) == 64*64 && wallShadePackedOK {
			drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, wallShadePackedLUT[256][:], stepper)
			return
		}
		drawPlaneSpanRGBA(pix32, pixI, count, tex32, 256, stepper)
		return
	}
	if len(texIndexed) == 64*64 && wallShadePackedOK {
		if defaultShade > 256 {
			defaultShade = 256
		}
		drawPlaneSpanIndexedPacked(pix32, pixI, count, texIndexed, wallShadePackedLUT[defaultShade][:], stepper)
		return
	}
	drawPlaneSpanRGBA(pix32, pixI, count, tex32, defaultShade, stepper)
}
