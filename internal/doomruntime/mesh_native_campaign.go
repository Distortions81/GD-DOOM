package doomruntime

import (
	"fmt"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/render/levelmesh"
)

type NativeCampaignPhase uint8

const (
	NativeCampaignPlaying NativeCampaignPhase = iota
	NativeCampaignIntermission
	NativeCampaignFinale
	NativeCampaignComplete
)

// NativeCampaign owns the map lifecycle while sharing the main host's
// intermission/finale state machine, layout and player carryover rules.
type NativeCampaign struct {
	Game                    *NativeMeshGame
	session                 *sessionGame
	next                    NextMapFunc
	patches                 []levelmesh.Patch
	chatPatches             []levelmesh.Patch
	perfPatches             []levelmesh.Patch
	messagePatches          []levelmesh.Patch
	deathPatches            []levelmesh.Patch
	recordingPatches        []levelmesh.Patch
	pausePatches            []levelmesh.Patch
	complete                bool
	demoFinished            bool
	recordingOptions        Options
	musicLump, musicPending string
	lastNetworkKeyframe     uint32
}

func NewNativeCampaign(game *NativeMeshGame, opts Options, next NextMapFunc) *NativeCampaign {
	c := &NativeCampaign{Game: game, session: &sessionGame{g: game.g, current: game.g.m.Name, opts: opts}, next: next, recordingOptions: opts}
	c.musicLump, _ = music.MapLumpName(string(game.g.m.Name))
	return c
}
func (c *NativeCampaign) Phase() NativeCampaignPhase {
	if c.complete {
		return NativeCampaignComplete
	}
	if c.session.finale.Active {
		return NativeCampaignFinale
	}
	if c.session.intermission.state.Active {
		return NativeCampaignIntermission
	}
	return NativeCampaignPlaying
}
func (c *NativeCampaign) Map() *mapdata.Map      { return c.Game.g.m }
func (c *NativeCampaign) MusicLump() string      { return c.musicLump }
func (c *NativeCampaign) queueMusic(lump string) { c.musicLump, c.musicPending = lump, lump }
func (c *NativeCampaign) TakeMusicRequest() string {
	lump := c.musicPending
	c.musicPending = ""
	return lump
}

// Tick advances one 35-Hz command tic. Skip is an edge-triggered press sampled
// by the native host; the shared state machine enforces its input delay.
func (c *NativeCampaign) tickCampaign(in NativeMeshInput, skip bool) error {
	sg := c.session
	if sg.g.opts.DemoScript != nil {
		return c.tickDemo()
	}
	switch c.Phase() {
	case NativeCampaignComplete:
		return nil
	case NativeCampaignFinale:
		if sg.tickFinaleAdvance(skip) {
			if sg.finale.Commercial {
				return c.finishLevel()
			}
			c.complete = true
		}
		return nil
	case NativeCampaignIntermission:
		if sg.tickIntermissionAdvance(skip) {
			if sg.startCommercialFinale() {
				c.queueMusic("D_READ_M")
				return nil
			}
			return c.finishLevel()
		}
		return nil
	}
	c.Game.Tick(in)
	if !sg.g.levelExitRequested {
		return nil
	}
	return c.startLevelExit()
}

func (c *NativeCampaign) startLevelExit() error {
	sg := c.session
	if sg.startEpisodeFinale(sg.current, sg.g.secretLevelExit) {
		sg.freezeDemoRecord()
		c.queueMusic("D_VICTOR")
		return nil
	}
	if c.next == nil {
		return fmt.Errorf("campaign has no next-map loader")
	}
	next, name, err := c.next(sg.current, sg.g.secretLevelExit)
	if err != nil {
		return err
	}
	if next == nil {
		return fmt.Errorf("campaign loader returned no map for %s", name)
	}
	sg.startIntermission(next, name, sg.g.secretLevelExit)
	if sg.intermission.state.Commercial {
		c.queueMusic("D_DM2INT")
	} else {
		c.queueMusic("D_INTER")
	}
	return nil
}

func (c *NativeCampaign) finishLevel() error {
	sg := c.session
	next := sg.intermission.nextMap
	if next == nil {
		return fmt.Errorf("campaign transition has no next map")
	}
	old := sg.g
	continuing := old.opts.DemoScript != nil
	opts := sg.opts
	if continuing {
		opts.DemoScript = old.opts.DemoScript
		opts.DemoQuitOnComplete = old.opts.DemoQuitOnComplete
		opts.DemoTracePath = ""
	}
	game := newNativeMeshGame(next, opts, !continuing)
	game.SetGammaLevel(old.gammaLevel)
	game.g.lastAttackRange = old.lastAttackRange
	if continuing {
		game.g.opts.DemoTracePath = old.opts.DemoTracePath
		game.g.inheritDemoPlayback(old)
		game.g.weaponAttackDown, game.g.useButtonDown = old.weaponAttackDown, old.useButtonDown
	}
	if sg.levelCarryover != nil {
		game.g.applyLevelCarryover(*sg.levelCarryover)
		if continuing {
			game.g.bringUpWeapon()
		}
		game.g.syncRenderState()
	}
	c.Game = game
	sg.g = game.g
	sg.current = next.Name
	c.lastNetworkKeyframe = 0
	sg.levelCarryover = nil
	sg.intermission = sessionIntermission{}
	sg.finale = sessionFinale{}
	lump, _ := music.MapLumpName(string(next.Name))
	c.queueMusic(lump)
	return nil
}

// Patches borrows logical 320x200 commands from the main intermission renderer
// without creating Ebiten images or drawing to an Ebiten surface.
func (c *NativeCampaign) Patches() []levelmesh.Patch {
	c.patches = c.patches[:0]
	sg := c.session
	sg.nativePatches = &c.patches
	defer func() { sg.nativePatches = nil }()
	switch c.Phase() {
	case NativeCampaignFinale:
		sg.drawFinaleContents(nil, 1, 0, 0)
	case NativeCampaignIntermission:
		if sg.intermission.state.Screen == intermissionScreenStats {
			sg.drawIntermissionStatsScreen(nil, 1, 0, 0, &sg.intermission)
		} else {
			sg.drawIntermissionMapScreen(nil, 1, 0, 0, &sg.intermission)
		}
	}
	return c.patches
}

// Restart resets the current map and player while retaining campaign history,
// as the main session does on death/restart. New Game creates a new campaign.
func (c *NativeCampaign) Restart(fresh *mapdata.Map) error {
	if fresh == nil || fresh.Name != c.session.current {
		return fmt.Errorf("restart needs current map %s", c.session.current)
	}
	sg := c.session
	sg.freezeDemoRecord()
	sg.g.clearPendingSoundState()
	gamma := c.Game.GammaLevel()
	c.Game = NewNativeMeshGame(fresh, sg.opts)
	c.Game.SetGammaLevel(gamma)
	sg.g = c.Game.g
	sg.levelCarryover = nil
	sg.intermission = sessionIntermission{}
	sg.finale = sessionFinale{}
	c.complete = false
	c.lastNetworkKeyframe = 0
	lump, _ := music.MapLumpName(string(fresh.Name))
	c.queueMusic(lump)
	return c.BroadcastMandatoryKeyframe()
}
