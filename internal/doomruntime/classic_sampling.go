package doomruntime

import (
	"math"
	"sync"

	"gddoom/internal/render/scene"
)

// Immutable R_InitTextureMapping tables, shared by wall, sky and plane sampling.
// A cache also lets the parallel wall prepass read them without mutating game.
type classicProjection struct {
	toX           [4096]int
	angles        []uint32
	distanceScale []int64
}

var classicProjections sync.Map

func classicProjectionForWidth(width int) *classicProjection {
	if p, ok := classicProjections.Load(width); ok {
		return p.(*classicProjection)
	}
	p := &classicProjection{angles: make([]uint32, width+1), distanceScale: make([]int64, width+1)}
	center := int64(width/2) * fracUnit
	focal := fixedDiv(center, doomFineTangent[3072])
	var toX [4096]int
	for i, tangent := range doomFineTangent {
		x := int((center - fixedMul(tangent, focal) + fracUnit - 1) >> fracBits)
		if tangent > 2*fracUnit {
			x = -1
		} else if tangent < -2*fracUnit {
			x = width + 1
		}
		toX[i] = max(-1, min(width+1, x))
	}
	for x := 0; x <= width; x++ {
		i := 0
		for toX[i] > x {
			i++
		}
		p.angles[x] = uint32(i<<doomAngleToFineShift) - doomAng90
		p.distanceScale[x] = fixedDiv(fracUnit, abs64(doomFineCosine(p.angles[x])))
	}
	for i, x := range toX {
		p.toX[i] = max(0, min(width, x))
	}
	actual, _ := classicProjections.LoadOrStore(width, p)
	return actual.(*classicProjection)
}
func radiansToDoomAngle(a float64) uint32 {
	return uint32(int64(math.Round(a * (4294967296.0 / (2 * math.Pi)))))
}

// R_DrawColumn starts at an integer row and advances the reciprocal of a
// quantized 16.16 scale. Use the same start/step in every CPU and GPU wall path.
func (g *game) wallColumnTextureStep(y int, texMid, depth, focal float64) (int64, int64) {
	if g.opts.SourcePortMode {
		step := depth / focal
		return floorFixed(texMid - (float64(g.viewH)*0.5-float64(y)-0.5)*step), floorFixed(step)
	}
	scale := int64(max(256, min(64*fracUnit, int(math.Round(focal/depth*fracUnit)))))
	step := int64(uint64(0xffffffff) / uint64(scale))
	return floorFixed(texMid) + int64(y-g.viewH/2)*step, step
}

// R_StoreWallRange and R_ScaleFromGlobalAngle. The SEG angle and offset come
// from the WAD, including split segments and reversed back-side endpoints.
type classicWallSampler struct {
	projection           *classicProjection
	view, normal, center uint32
	distance, offset     int64
	scale, scaleStep     int64
	start                int
	focal                float64
}

func newClassicWallSampler(input scene.WallPrepassWorldInput, segAngle uint32, camX, camY, ca, sa float64, width int) classicWallSampler {
	vx, vy := floorFixed(camX), floorFixed(camY)
	x, y := floorFixed(input.X1W), floorFixed(input.Y1W)
	angle1 := doomPointToAngle2(vx, vy, x, y)
	normal := segAngle + doomAng90
	off := normal - angle1
	if off > doomAng180 {
		off = 0 - off
	}
	off = min(off, doomAng90)
	dx, dy := abs64(x-vx), abs64(y-vy)
	if dy > dx {
		dx, dy = dy, dx
	}
	hyp := int64(0)
	if dx != 0 {
		angle := doomTanToAngle[min(2048, int(fixedDiv(dy, dx)>>5))] + doomAng90
		hyp = fixedDiv(dx, doomFineSineAtAngle(angle))
	}
	offset := fixedMul(hyp, doomFineSineAtAngle(off))
	if normal-angle1 < doomAng180 {
		offset = -offset
	}
	view := radiansToDoomAngle(math.Atan2(sa, ca))
	return classicWallSampler{projection: classicProjectionForWidth(width), view: view, normal: normal, center: doomAng90 + view - normal,
		distance: fixedMul(hyp, doomFineSineAtAngle(doomAng90-off)), offset: offset + floorFixed(input.U1), focal: float64(width / 2)}
}
func (s classicWallSampler) scaleAt(x int) int64 {
	angle := s.view + s.projection.angles[x]
	num := fixedMul(floorFixed(s.focal), doomFineSineAtAngle(doomAng90+angle-s.normal))
	den := fixedMul(s.distance, doomFineSineAtAngle(doomAng90+angle-s.view))
	if den <= num>>16 {
		return 64 * fracUnit
	}
	return int64(max(256, min(64*fracUnit, int(fixedDiv(num, den)))))
}
func (s classicWallSampler) forRange(start, stop int) classicWallSampler {
	if s.projection == nil {
		return s
	}
	s.start = start
	s.scale = s.scaleAt(start)
	s.scaleStep = 0
	if stop > start {
		s.scaleStep = (s.scaleAt(stop) - s.scale) / int64(stop-start)
	}
	return s
}
func (s classicWallSampler) sample(x int, depth, u float64) (float64, float64) {
	if s.projection == nil {
		return depth, u
	}
	angle := (s.center + s.projection.angles[x]) >> doomAngleToFineShift
	// Only front-facing rays reach the tangent half-circle. Keep malformed
	// map geometry bounded rather than indexing outside the original table.
	angle &= 4095
	column := s.offset - fixedMul(doomFineTangent[angle], s.distance)
	scale := s.scale + int64(x-s.start)*s.scaleStep
	return s.focal * fracUnit / float64(max(256, int(scale))), float64(column) / fracUnit
}

func (s planeRowRenderState) textureStart(x int) (int64, int64) {
	if s.classicProjection == nil {
		return s.rowBaseWXFixed + int64(x)*s.stepWXFixed, s.rowBaseWYFixed + int64(x)*s.stepWYFixed
	}
	// fixed_t wraps to signed 32 bits before the subsequent trig multiply.
	length := int64(int32(fixedMul(s.classicDistance, s.classicProjection.distanceScale[x])))
	angle := s.classicAngle + s.classicProjection.angles[x]
	return int64(int32(s.classicViewX + fixedMul(doomFineCosine(angle), length))), int64(int32(-s.classicViewY - fixedMul(doomFineSineAtAngle(angle), length)))
}

// R_AddLine uses quantized angles and a half-open horizontal range. Expanding
// both endpoints with floor/ceil changes visplane span starts and thus their UVs.
func (p *classicProjection) wallBounds(input scene.WallPrepassWorldInput, camX, camY float64, view uint32) (int, int, bool) {
	a := doomPointToAngle2(floorFixed(camX), floorFixed(camY), floorFixed(input.X1W), floorFixed(input.Y1W))
	b := doomPointToAngle2(floorFixed(camX), floorFixed(camY), floorFixed(input.X2W), floorFixed(input.Y2W))
	span := a - b
	if span >= doomAng180 {
		return 0, 0, false
	}
	a -= view
	b -= view
	clip := p.angles[0]
	if t := a + clip; t > 2*clip {
		if t-2*clip >= span {
			return 0, 0, false
		}
		a = clip
	}
	if t := clip - b; t > 2*clip {
		if t-2*clip >= span {
			return 0, 0, false
		}
		b = 0 - clip
	}
	left := p.toX[(a+doomAng90)>>doomAngleToFineShift]
	right := p.toX[(b+doomAng90)>>doomAngleToFineShift] - 1
	return left, right, left <= right
}

// R_RenderSegLoop steps 20.12 wall edges; recomputing them as floating-point
// heights per column can move a boundary one row, including a plane's origin.
func (s classicWallSampler) edgeY(x, height int, worldHeight, focal, depth float64, roundUp bool) int {
	if s.projection == nil {
		y := float64(height)*0.5 - worldHeight*focal/depth
		if roundUp {
			return int(math.Ceil(y))
		}
		return int(math.Floor(y))
	}
	world := floorFixed(worldHeight*focal/s.focal) >> 4
	edge := (int64(height/2) * fracUnit >> 4) - fixedMul(world, s.scale) - int64(x-s.start)*fixedMul(world, s.scaleStep)
	if roundUp {
		edge += 4095
	}
	return int(edge >> 12)
}
