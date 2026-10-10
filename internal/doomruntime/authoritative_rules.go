package doomruntime

import (
	"errors"
	"fmt"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
)

var (
	ErrAuthoritySpawnBlocked = errors.New("all player spawn locations are blocked")
	ErrAuthorityMatchEnded   = errors.New("authoritative match has ended")
)

// AuthorityRules are server-owned match settings. Limits of zero disable the
// corresponding limit. RespawnDelayTics is measured from the confirmed death;
// a dead player requests respawn with use. Co-op shares keys and preserves them
// through death. Deathmatch grants all keys and always allows player damage.
type AuthorityRules struct {
	FragLimit        int
	TimeLimitTics    uint32
	RespawnDelayTics uint32
	FriendlyFire     bool
}

type authorityScoreState struct {
	Generation     uint32
	MovementEpoch  uint32 // changes on teleport so sparse snapshots retain the discontinuity
	Frags          int
	Deaths         int
	DeathRecorded  bool
	DeathTic       uint32
	RespawnPending bool
}

type authorityRulesState struct {
	Config      AuthorityRules
	Scores      [5]authorityScoreState
	SpawnCursor int
	StartedTic  uint32
	Ended       bool
	EndReason   string
	WinnerID    byte
	TeamKeys    mapdata.KeyRing
}

type AuthorityScore struct {
	ID         byte
	Active     bool
	Generation uint32
	Frags      int
	Deaths     int
}

type AuthorityMatchState struct {
	Mode     string
	Tic      uint32
	Ended    bool
	Reason   string
	WinnerID byte // zero is a tie or a co-op completion
	Scores   []AuthorityScore
}

// SetRules changes server-owned settings. It never restarts a world or changes
// an already recorded result. If a newly reduced limit has already been reached,
// the match ends immediately with the current scores.
func (a *Authority) SetRules(config AuthorityRules) error {
	if config.FragLimit < 0 {
		return fmt.Errorf("frag limit cannot be negative")
	}
	if a.g.authorityRules.Ended {
		return ErrAuthorityMatchEnded
	}
	a.g.authorityRules.Config = config
	a.checkAuthoritativeMatchLimits()
	return nil
}

func (a *Authority) MatchState() AuthorityMatchState {
	rules := a.g.authorityRules
	out := AuthorityMatchState{Mode: a.g.opts.GameMode, Tic: a.Tic(), Ended: rules.Ended, Reason: rules.EndReason, WinnerID: rules.WinnerID}
	for id := byte(1); id <= 4; id++ {
		score := rules.Scores[id]
		if score.Generation != 0 {
			out.Scores = append(out.Scores, AuthorityScore{
				ID: id, Active: a.players[id] != nil, Generation: score.Generation,
				Frags: score.Frags, Deaths: score.Deaths,
			})
		}
	}
	return out
}

func (g *game) authorityPlayerGeneration(slot int) uint32 {
	if g.authorityRules == nil || slot < 1 || slot > 4 {
		return 0
	}
	return g.authorityRules.Scores[slot].Generation
}

func (g *game) authorityPlayerDamageAllowed(sourceID, targetID int) bool {
	if g.authorityRules == nil || sourceID == 0 || sourceID == targetID {
		return true
	}
	return g.opts.GameMode == gameModeDeathmatch || g.authorityRules.Config.FriendlyFire
}

// authorityPlayerKilled is called at the authoritative damage transition, not
// when a client reports a hit or receives a snapshot. The per-incarnation latch
// prevents duplicate death/frag events. Zero identifies an environmental or
// monster kill; in deathmatch those and suicides subtract one victim frag.
func (g *game) authorityPlayerKilled(victimID, killerID int) {
	if g.authorityRules == nil || victimID < 1 || victimID > 4 {
		return
	}
	score := &g.authorityRules.Scores[victimID]
	if score.Generation == 0 || score.DeathRecorded {
		return
	}
	score.DeathRecorded = true
	score.DeathTic = uint32(g.worldTic)
	score.Deaths++
	if g.opts.GameMode != gameModeDeathmatch {
		return
	}
	if killerID >= 1 && killerID <= 4 && killerID != victimID && g.authoritativePlayerForSlot(killerID) != nil {
		g.authorityRules.Scores[killerID].Frags++
	} else {
		score.Frags--
	}
}

func (a *Authority) prepareAuthoritativeRespawns(commands map[int]DemoTic) {
	rules := a.g.authorityRules
	for id := byte(1); id <= 4; id++ {
		p := a.players[id]
		if p == nil || !p.isDead {
			continue
		}
		if !rules.Scores[id].DeathRecorded {
			a.g.authorityPlayerKilled(int(id), 0)
		}
		score := &rules.Scores[id]
		if commands[int(id)].Buttons&demo.ButtonUse != 0 {
			score.RespawnPending = true
		}
		if score.RespawnPending && a.Tic()-score.DeathTic >= rules.Config.RespawnDelayTics {
			// A blocked spawn remains queued while everyone else keeps playing.
			// No telefrag, full-world restart, or extra simulation time is needed.
			_ = a.spawnAuthoritativePlayer(id, true)
		}
	}
}

func (a *Authority) finishAuthoritativeRulesTic() {
	rules := a.g.authorityRules
	a.g.tickAuthoritativeItemRespawns()
	for id := byte(1); id <= 4; id++ {
		if p := a.players[id]; p != nil && p.isDead && !rules.Scores[id].DeathRecorded {
			a.g.authorityPlayerKilled(int(id), 0)
		}
	}
	if a.g.opts.GameMode == gameModeCoop {
		a.shareAuthoritativeCoopKeys()
	}
	a.checkAuthoritativeMatchLimits()
}

func (a *Authority) shareAuthoritativeCoopKeys() {
	keys := &a.g.authorityRules.TeamKeys
	for _, p := range a.players {
		if p != nil {
			keys.Blue = keys.Blue || p.inventory.BlueKey
			keys.Red = keys.Red || p.inventory.RedKey
			keys.Yellow = keys.Yellow || p.inventory.YellowKey
		}
	}
	for _, p := range a.players {
		if p != nil {
			p.inventory.BlueKey, p.inventory.RedKey, p.inventory.YellowKey = keys.Blue, keys.Red, keys.Yellow
		}
	}
	a.selectAnchor()
}

func (a *Authority) checkAuthoritativeMatchLimits() {
	rules := a.g.authorityRules
	if rules.Ended {
		return
	}
	reason := ""
	if a.g.levelExitRequested {
		reason = "map_exit"
	} else if rules.Config.TimeLimitTics != 0 && a.Tic()-rules.StartedTic >= rules.Config.TimeLimitTics {
		reason = "time_limit"
	} else if a.g.opts.GameMode == gameModeDeathmatch && rules.Config.FragLimit > 0 {
		for id := 1; id <= 4; id++ {
			if a.players[id] != nil && rules.Scores[id].Frags >= rules.Config.FragLimit {
				reason = "frag_limit"
				break
			}
		}
	}
	if reason == "" {
		return
	}
	rules.Ended, rules.EndReason = true, reason
	if a.g.opts.GameMode == gameModeDeathmatch {
		best, haveBest := 0, false
		for id := byte(1); id <= 4; id++ {
			if a.players[id] == nil {
				continue
			}
			frags := rules.Scores[id].Frags
			if !haveBest || frags > best {
				best, haveBest, rules.WinnerID = frags, true, id
			} else if frags == best {
				rules.WinnerID = 0
			}
		}
	}
}
