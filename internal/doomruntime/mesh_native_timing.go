package doomruntime

import "math"

// Simulation speed is a local Source Port live-play control. Multiplayer,
// demos and intermissions retain their own 35-Hz clock.
func (c *NativeCampaign) canScaleSimulation() bool {
	return c.Phase() == NativeCampaignPlaying && c.Game.g.canScaleLiveSimulation() && !c.Watching()
}

func (c *NativeCampaign) SetSimulationSpeed(speed float64) {
	if c.canScaleSimulation() && !math.IsNaN(speed) && !math.IsInf(speed, 0) {
		c.Game.g.setSimTickScale(speed)
	}
}

func (c *NativeCampaign) SimulationSpeed() float64 { return c.Game.g.simTickScale }

// ConsumeSimulationTicks is called once per unpaused host update. Tick still
// advances one command tic, preserving demos, recording and relay formats.
func (c *NativeCampaign) ConsumeSimulationTicks() int {
	if !c.canScaleSimulation() {
		return 1
	}
	return c.Game.g.consumeSimTicks()
}

// SimulationRenderAlpha converts the fractional host update to the fraction of
// a simulation step. Fast simulation draws the latest state, as in Ebiten.
func (c *NativeCampaign) SimulationRenderAlpha(updateFraction float64) float64 {
	if !c.canScaleSimulation() {
		return math.Max(0, math.Min(1, updateFraction))
	}
	g := c.Game.g
	if g.simTickScale > 1 {
		return 1
	}
	return math.Max(0, math.Min(1, g.simTickAccum+updateFraction*g.simTickScale))
}
