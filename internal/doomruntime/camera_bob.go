package doomruntime

import "math"

const (
	playerBobPhaseStep = doomFineAngles / 20
	playerMaxBob       = 0x100000
)

// Presentation-only inputs captured at P_CalcHeight, before the thinker phase
// changes momentum/body height. Reconstructing them from the post-tic player
// would use the wrong amplitude and base height.
type cameraBobFrame struct {
	tic                      int
	base, amplitude, ceiling float64
	eye                      int64
	valid                    bool
}

func (g *game) recordCameraBob(amplitude int64, grounded bool) {
	base := float64(g.p.z+g.p.viewHeight) / fracUnit
	// Airborne P_CalcHeight does not clip the eye to the ceiling. Eight units
	// above the base is a finite, nonbinding limit (MAXBOB/2), which also lets
	// the limit interpolate continuously on takeoff/landing.
	ceiling := base + float64(playerMaxBob/2)/fracUnit
	if grounded {
		ceiling = float64(g.p.ceilz-4*fracUnit) / fracUnit
	}
	g.cameraBob = cameraBobFrame{g.worldTic, base, float64(amplitude) / fracUnit, ceiling, g.playerViewZ, true}
}

func (g *game) cameraBobRenderFrame() cameraBobFrame {
	if g.cameraBob.valid && g.cameraBob.eye == g.playerViewZ && !g.p.teleportedThisTic {
		return g.cameraBob
	}
	// A fresh/cut camera has no bob history. worldTic has already advanced
	// past the last P_CalcHeight call, whose phase was worldTic-1.
	z := g.playerBaseEyeZ()
	return cameraBobFrame{g.worldTic - 1, z, 0, z + float64(playerMaxBob/2)/fracUnit, g.playerViewZ, true}
}

func (g *game) renderCameraEyeZ() float64 {
	if g.opts.SourcePortMode && g.opts.DisableCameraBob && g.authorityRules == nil && g.clientPrediction == nil && !g.predictionMovement {
		if g.cameraBob.valid && g.prevCameraBob.valid && !g.p.teleportedThisTic && g.cameraBob.eye == g.playerViewZ {
			alpha := math.Max(0, math.Min(1, g.renderAlpha))
			return min(lerp(g.prevCameraBob.base, g.cameraBob.base, alpha), lerp(g.prevCameraBob.ceiling, g.cameraBob.ceiling, alpha))
		}
		if g.p.viewHeight != 0 {
			base := float64(g.p.z+g.p.viewHeight) / fracUnit
			if g.p.z <= g.p.floorz {
				base = min(base, float64(g.p.ceilz-4*fracUnit)/fracUnit)
			}
			return base
		}
	}

	if !g.opts.SourcePortMode || g.authorityRules != nil || g.clientPrediction != nil || g.predictionMovement ||
		!g.cameraBob.valid || !g.prevCameraBob.valid || g.p.teleportedThisTic || g.cameraBob.eye != g.playerViewZ {
		return g.playerBaseEyeZ()
	}
	prev, curr := g.prevCameraBob, g.cameraBob
	alpha := math.Max(0, math.Min(1, g.renderAlpha))
	base := lerp(prev.base, curr.base, alpha)
	amplitude := lerp(prev.amplitude, curr.amplitude, alpha)
	ceiling := lerp(prev.ceiling, curr.ceiling, alpha)
	// Classic advances by integer 8192/20 = 409 fine angles per tic, not
	// exactly one revolution per 20 tics. Its sine table samples i+0.5.
	// Unwrap the interval before interpolating; masking both endpoints would
	// send the phase backwards at the wrap. Reduce the starting phase first
	// so long-running games retain fractional-frame precision.
	phase := float64((prev.tic*playerBobPhaseStep)&doomFineMask) + 0.5 +
		float64(curr.tic-prev.tic)*playerBobPhaseStep*alpha
	bob := amplitude * math.Sin(phase*(2*math.Pi/doomFineAngles))
	return min(base+bob, ceiling)
}
