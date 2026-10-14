package doomruntime

import (
	"fmt"
	"gddoom/internal/runtimecfg"
)

func (c *NativeCampaign) Watching() bool {
	return c.session.opts.LiveTicSource != nil && c.session.opts.LiveTicSink == nil
}

// Tick retains the original command, snapshot and intermission wire formats.
// Only received commands advance a watcher; local movement never drives it.
func (c *NativeCampaign) Tick(in NativeMeshInput, skip bool) error {
	if err := c.PollNetwork(); err != nil {
		return err
	}
	if c.Watching() && c.Phase() == NativeCampaignPlaying {
		return c.tickWatch(in.Map)
	}
	if c.Watching() {
		skip = false
	}
	if c.Phase() == NativeCampaignIntermission {
		if c.Watching() {
			skip = false
			if source, ok := c.session.opts.LiveTicSource.(runtimecfg.LiveIntermissionAdvanceSource); ok {
				advance, err := source.PollIntermissionAdvance()
				if err != nil {
					return fmt.Errorf("watch intermission: %w", err)
				}
				skip = advance
			}
		} else if skip {
			if sink, ok := c.session.opts.LiveTicSink.(runtimecfg.LiveIntermissionAdvanceSink); ok {
				if err := sink.BroadcastIntermissionAdvance(); err != nil {
					return err
				}
			}
		}
	}
	if err := c.tickCampaign(in, skip); err != nil {
		return err
	}
	return c.emitPeriodicKeyframe()
}

// PollNetwork also runs while native menus are open, without advancing gameplay.
func (c *NativeCampaign) PollNetwork() error {
	if c.Watching() {
		if source, ok := c.session.opts.LiveTicSource.(runtimecfg.LiveRuntimeKeyframeSource); ok {
			for range 8 {
				kf, ready, err := source.PollRuntimeKeyframe()
				if err != nil {
					return fmt.Errorf("watch keyframe: %w", err)
				}
				if !ready {
					break
				}
				if kf.MandatoryApply {
					if err := c.LoadKeyframe(kf.Blob); err != nil {
						return fmt.Errorf("apply watch keyframe: %w", err)
					}
				}
			}
		}
	}
	if err := c.Game.g.pollChatMessages(); err != nil {
		return fmt.Errorf("chat stream: %w", err)
	}
	return nil
}

func (c *NativeCampaign) tickWatch(mapInput *NativeMapInput) error {
	g := c.Game.g
	if mapInput != nil {
		view := c.Game.updateMap(*mapInput)
		g.State.Pan(view.PanDX, view.PanDY)
	}
	budget := 1
	if source, ok := g.opts.LiveTicSource.(runtimecfg.LiveTicBufferedSource); ok && g.worldTic > 0 {
		budget = max(budget, min(watchMaxCatchUpTics, source.PendingTics()-watchTargetBufferedTics))
	}
	for range budget {
		tc, ready, err := g.opts.LiveTicSource.PollTic()
		if err != nil {
			return fmt.Errorf("watch stream: %w", err)
		}
		if !ready {
			break
		}
		g.capturePrevState()
		g.stepGameplayFromDemoTic(tc)
		if g.levelExitRequested {
			return c.startLevelExit()
		}
	}
	return nil
}

func (c *NativeCampaign) BroadcastMandatoryKeyframe() error {
	return c.session.broadcastMandatoryRuntimeKeyframe()
}

func (c *NativeCampaign) BroadcastInitialKeyframe() error {
	sink, ok := c.session.opts.LiveTicSink.(saveKeyframeSink)
	if !ok {
		return nil
	}
	blob, err := c.CaptureKeyframe()
	if err != nil {
		return err
	}
	return sink.BroadcastKeyframe(uint32(c.Game.g.worldTic), blob)
}

func (c *NativeCampaign) emitPeriodicKeyframe() error {
	sink, ok := c.session.opts.LiveTicSink.(saveKeyframeSink)
	tic := uint32(c.Game.g.worldTic)
	if !ok || tic == 0 || tic%175 != 0 || tic == c.lastNetworkKeyframe {
		return nil
	}
	blob, err := c.CaptureKeyframe()
	if err != nil {
		return err
	}
	if err := sink.BroadcastKeyframe(tic, blob); err != nil {
		return fmt.Errorf("broadcast keyframe: %w", err)
	}
	c.lastNetworkKeyframe = tic
	return nil
}
