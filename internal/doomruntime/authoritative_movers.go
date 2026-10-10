package doomruntime

import (
	"math"
	"time"
)

// A grounded rider shares the support plane's presentation timeline. Its body
// remains on the newest authoritative floor for collision and input replay.
// Do not assume the center sector supports a player touching a higher neighbor.
type authorityPlayerSupport struct {
	sector int
	floor  int64
	valid  bool
}

func (g *game) authorityPlayerSupport() authorityPlayerSupport {
	if g == nil || g.isDead || g.p.z != g.p.floorz {
		return authorityPlayerSupport{}
	}
	sec := g.playerSector()
	floor, _, ok := g.sectorHeightSnapshot(sec)
	if !ok || floor != g.p.floorz {
		return authorityPlayerSupport{}
	}
	return authorityPlayerSupport{sector: sec, floor: floor, valid: true}
}

func (g *game) authoritySupportEyeOffset() float64 {
	if g == nil || g.clientPrediction == nil {
		return 0
	}
	return g.authoritySupportOffset(g.authorityPlayerSupport())
}

func (g *game) authoritySupportOffset(support authorityPlayerSupport) float64 {
	if !support.valid {
		return 0
	}
	floor, _, ok := g.authoritySectorRenderHeights(support.sector)
	if !ok {
		return 0
	}
	return float64(floor-support.floor) / fracUnit
}

// Support changes get their own bounded decay, so stepping on/off a delayed
// floor neither jumps the eye nor restarts unrelated reconciliation smoothing.
type predictionSupportCorrection struct {
	z, pending float64
	started    time.Time
}

func (p *ClientPrediction) queueSupportTransition(from, to authorityPlayerSupport) {
	if from.valid == to.valid && (!from.valid || from.sector == to.sector) {
		return
	}
	p.supportCorrection.pending += p.g.authoritySupportOffset(from) - p.g.authoritySupportOffset(to)
}

func (p *ClientPrediction) prepareSupportCorrection(now time.Time) float64 {
	c := &p.supportCorrection
	remaining := 0.0
	if !c.started.IsZero() {
		remaining = math.Max(0, math.Min(1, 1-float64(now.Sub(c.started))/float64(predictionCorrectionDuration)))
	}
	z := c.z * remaining
	if c.pending != 0 {
		z += c.pending
		if math.Abs(z) > 32 {
			z = 0
		}
		*c = predictionSupportCorrection{z: z, started: now}
	}
	return z
}

func authoritySupportDisplacement(from, to authorityPlayerSupport) float64 {
	if !from.valid || !to.valid || from.sector != to.sector {
		return 0
	}
	return float64(to.floor-from.floor) / fracUnit
}
