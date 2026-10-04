package doomruntime

// GammaTable is the main renderer's exact per-channel correction table.
// Native world shaders apply it after lighting; UI and sky retain source colors.
func (n *NativeMeshGame) GammaTable() [256]uint8 {
	return doomGammaTables[n.GammaLevel()]
}

func (n *NativeMeshGame) GammaLevel() int { return clampGamma(n.g.gammaLevel) }

func (n *NativeMeshGame) SetGammaLevel(level int) {
	level = clampGamma(level)
	if n.g.gammaLevel != level || activeGammaLevel != level {
		n.g.setGammaLevel(level)
	}
	n.g.opts.InitialGammaLevel = level
}

func (n *NativeMeshGame) CycleGammaLevel() {
	n.g.cycleGammaLevel()
	n.g.opts.InitialGammaLevel = n.GammaLevel()
}

func (c *NativeCampaign) SetGammaLevel(level int) {
	c.Game.SetGammaLevel(level)
	c.session.opts.InitialGammaLevel = c.Game.GammaLevel()
}
