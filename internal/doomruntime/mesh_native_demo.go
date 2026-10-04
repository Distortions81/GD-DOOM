package doomruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gddoom/internal/demo"
	"github.com/hajimehoshi/ebiten/v2"
)

// NativeDemoStatus describes playback without exposing a window implementation.
type NativeDemoStatus struct {
	Active, Done, Paused bool
	Tic, Total           int
}

func (c *NativeCampaign) DemoStatus() NativeDemoStatus {
	g := c.Game.g
	script := g.opts.DemoScript
	if script == nil {
		return NativeDemoStatus{}
	}
	return NativeDemoStatus{Active: true, Done: c.demoFinished, Paused: g.demoPaused, Tic: g.demoTick, Total: len(script.Tics)}
}
func (c *NativeCampaign) tickDemo() error {
	sg := c.session
	if c.demoFinished {
		return nil
	}
	// G_DoWorldDone runs before reading the next recorded command, keeping both
	// random streams, the continuous trace, command cursor and button latches.
	if sg.g.demoWorldDone {
		if err := c.finishLevel(); err != nil {
			return err
		}
	}
	g := c.Game.g
	if g.levelExitRequested && !g.demoIntermissionActive && !g.demoFinaleActive {
		return c.startLevelExit()
	}
	if err := g.advanceDemoTic(); err != nil {
		if errors.Is(err, ebiten.Termination) {
			c.demoFinished = true
			return nil
		}
		return err
	}
	switch c.Phase() {
	case NativeCampaignIntermission:
		sg.tickIntermission()
		if sg.finale.Active {
			c.queueMusic("D_READ_M")
		}
	case NativeCampaignFinale:
		if g.demoFinaleCommercial {
			sg.tickFinale()
		} else {
			sg.tickFinaleAdvance(false)
		}
	}
	return nil
}

func (c *NativeCampaign) RecordDemoFrame(duration time.Duration) {
	if !c.DemoStatus().Active {
		return
	}
	g := c.Game.g
	g.demoBenchDraws++
	g.recordDemoBenchFrame(duration)
}
func (c *NativeCampaign) CloseDemoTrace() {
	if c != nil && c.Game != nil && c.Game.g.demoTrace != nil {
		c.Game.g.demoTrace.Close()
		c.Game.g.demoTrace = nil
	}
}

// RecordedDemo keeps the starting map and recording options even after an exit
// freezes the stream. It uses the main host's format and stop-point policy.
func (c *NativeCampaign) RecordedDemo() (*demo.Script, string, error) {
	sg := c.session
	path := strings.TrimSpace(sg.opts.RecordDemoPath)
	if path == "" {
		path = strings.TrimSpace(sg.frozenDemoPath)
	}
	if path == "" {
		return nil, "", nil
	}
	tics := sg.effectiveDemoRecord()
	if len(tics) == 0 {
		return nil, path, nil
	}
	name := sg.demoRecordingMap
	if name == "" {
		name = sg.current
	}
	script, err := BuildRecordedDemo(name, c.recordingOptions, tics)
	return script, path, err
}
func (c *NativeCampaign) RecordingActive() bool {
	return strings.TrimSpace(c.session.opts.RecordDemoPath) != ""
}

// FlushDemoRecording atomically preserves a valid LMP on normal quit and at
// periodic checkpoints. A failed write keeps the previous checkpoint intact.
func (c *NativeCampaign) FlushDemoRecording() (int, error) {
	script, path, err := c.RecordedDemo()
	if err != nil || script == nil {
		return 0, err
	}
	if len(script.Tics) == c.session.demoFlushTics {
		return len(script.Tics), nil
	}
	data, err := demo.Format(script)
	if err != nil {
		return 0, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gddoom-demo-*.lmp")
	if err != nil {
		return 0, fmt.Errorf("write demo %s: %w", path, err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(0644); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporary, path)
	}
	if err != nil {
		return 0, fmt.Errorf("write demo %s: %w", path, err)
	}
	c.session.demoFlushTics = len(script.Tics)
	return len(script.Tics), nil
}
func (c *NativeCampaign) StopDemoRecording() error {
	c.session.freezeDemoRecord()
	_, err := c.FlushDemoRecording()
	return err
}
