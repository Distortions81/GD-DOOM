package doomruntime

// DetailSettings exposes the same manual divisor and AUTO state as Ebiten.
func (c *NativeCampaign) DetailSettings() (level int, auto bool) {
	return c.Game.g.detailLevel, c.Game.g.autoDetailEnabled
}

func (c *NativeCampaign) rememberDetailSettings() {
	g := c.Game.g
	for _, opts := range []*Options{&g.opts, &c.session.opts} {
		opts.InitialDetailLevel, opts.AutoDetail = g.detailLevel, g.autoDetailEnabled
	}
}

func (c *NativeCampaign) CycleDetail() {
	c.Game.g.cycleSourcePortDetailLevel()
	c.rememberDetailSettings()
}

// SceneSize matches the main session's integer-divisor source-port layout.
// Titles, menus and 3D overlays are still presented at the window resolution.
func (c *NativeCampaign) SceneSize(width, height int) (int, int) {
	div := c.Game.g.sourcePortDetailDivisor()
	return max(width/div, 1), max(height/div, 1)
}
