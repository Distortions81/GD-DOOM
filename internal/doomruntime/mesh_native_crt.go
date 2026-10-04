package doomruntime

func (n *NativeMeshGame) CRTEnabled() bool { return n.g.crtEnabled }

func (c *NativeCampaign) SetCRTEnabled(enabled bool) {
	c.Game.g.crtEnabled = enabled
	c.Game.g.opts.CRTEffect = enabled
	c.session.opts.CRTEffect = enabled
}

func (n *NativeMeshGame) ToggleCRT() {
	n.g.toggleCRT()
	n.g.opts.CRTEffect = n.g.crtEnabled
}
