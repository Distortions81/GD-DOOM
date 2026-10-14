package doomruntime

// UpdateHostFrame runs on the host frames between ordinary 35 Hz session
// updates. The authoritative client owns its own bounded command clock, so
// gating it at 35 Hz as well would produce alternating empty/catch-up updates
// whenever the two clocks drift. Local gameplay and menus keep their old rate.
func (sg *sessionGame) UpdateHostFrame() error {
	if sg == nil || sg.g == nil || sg.opts.AuthorityClient == nil {
		return nil
	}
	// Reliable map/resume control must precede snapshots even between session
	// tics. This also pumps neutral controls while a local menu owns input.
	err := sg.updateAuthoritySession()
	if err == nil && sg.shouldSampleRuntimeInput() {
		err = sg.g.Update()
	} else {
		sg.g.clearSampledInput()
	}
	if err != nil {
		sg.err = err
		sg.g.setAuthorityConnectionFailure(err)
	}
	// The regular session update still consumes menu requests and owns UI
	// input/touch latches. A failed connection remains available to its menu.
	return nil
}
