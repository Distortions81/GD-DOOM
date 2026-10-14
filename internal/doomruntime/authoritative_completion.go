package doomruntime

// MatchCompletion exposes ordinary match completion to the transport without
// coupling its session owner to Doom's error types or game rules.
func (a *Authority) MatchCompletion() (bool, string) {
	if a == nil || a.g == nil || a.g.authorityRules == nil {
		return false, ""
	}
	return a.g.authorityRules.Ended, a.g.authorityRules.EndReason
}
