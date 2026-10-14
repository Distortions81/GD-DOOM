package doomruntime

import (
	"strings"

	"gddoom/internal/music"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
)

// NativeFrontendStatus describes the shared title/credit/demo sequence. The
// native host draws the named page or the campaign's current demo scene.
type NativeFrontendStatus struct {
	Active             bool
	Page               string
	Sequence, PageTics int
}

func (c *NativeCampaign) FrontendStatus() NativeFrontendStatus {
	f := c.session.frontend
	return NativeFrontendStatus{f.Active, f.AttractPage, f.AttractSeq, f.AttractPageTic}
}

func (c *NativeCampaign) StartFrontend() {
	sg := c.session
	if sg.bootMap == nil {
		sg.bootMap = cloneMapForRestart(c.Map())
	}
	sg.frontend = sessionflow.StartFrontend()
	c.complete, c.demoFinished = false, false
	c.advanceNativeAttract()
}

// TickFrontend uses the main host's 35-Hz page timer and attract sequence.
// Attract playback continues behind menus, which capture user input.
func (c *NativeCampaign) TickFrontend() error {
	sg := c.session
	if !sg.frontend.Active {
		return nil
	}
	if c.DemoStatus().Active {
		if err := c.tickDemo(); err != nil {
			return err
		}
		if c.demoFinished {
			c.advanceNativeAttract()
		}
	}
	var advance bool
	sg.frontend, advance = sessionflow.AdvanceFrontendFrame(sg.frontend, menuSkullBlinkTics)
	if advance {
		c.advanceNativeAttract()
	}
	return nil
}

func (c *NativeCampaign) advanceNativeAttract() {
	sg := c.session
	seq := sg.frontendAttractSequence()
	commercial := strings.HasPrefix(strings.ToUpper(string(sg.bootMap.Name)), "MAP")
	for range seq {
		state, action, ok := sessionflow.AdvanceAttract(sg.frontend, seq, commercial, attractPageTitleCommercial, attractPageTitleNonCommercial, attractPageInfo)
		if !ok {
			return
		}
		sg.frontend = state
		if action.Kind == sessionflow.AttractActionDemo {
			if sg.opts.DemoMapLoader == nil {
				continue
			}
			for _, script := range sg.opts.AttractDemos {
				if script == nil || !strings.EqualFold(script.Path, action.Name) {
					continue
				}
				m, err := sg.opts.DemoMapLoader(script)
				if err != nil || m == nil {
					continue
				}
				c.CloseDemoTrace()
				opts := runtimecfg.PrepareDemoPlaybackOptions(sg.opts, script)
				opts.DemoScript = script
				opts.DemoQuitOnComplete = false
				opts.RecordDemoPath, opts.DemoTracePath = "", ""
				opts.DemoExitOnDeath, opts.DemoStopAfterTics = false, 0
				gamma := c.Game.GammaLevel()
				c.Game = NewNativeMeshGame(cloneMapForRestart(m), opts)
				c.Game.SetGammaLevel(gamma)
				sg.g, sg.current = c.Game.g, m.Name
				sg.intermission, sg.finale, sg.levelCarryover = sessionIntermission{}, sessionFinale{}, nil
				c.complete, c.demoFinished = false, false
				lump, _ := music.MapLumpName(string(m.Name))
				c.queueMusic(lump)
				return
			}
			continue
		}
		c.CloseDemoTrace()
		sg.g.opts.DemoScript = nil
		c.demoFinished = false
		if action.PlayTitle {
			lump := "D_INTRO"
			if commercial {
				lump = "D_DM2TTL"
			}
			c.queueMusic(lump)
		}
		return
	}
}
