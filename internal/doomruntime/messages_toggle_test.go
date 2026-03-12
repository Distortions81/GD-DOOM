package doomruntime

import (
	"testing"

	"gddoom/internal/gameplay"
)

func TestFrontendChangeMessagesTogglesRuntimeAndSettings(t *testing.T) {
	g := &game{hudMessagesEnabled: true}
	sg := &sessionGame{
		g:        g,
		rt:       g,
		settings: gameplay.PersistentSettings{HUDMessages: true},
	}
	sg.frontendChangeMessages()
	if g.hudMessagesEnabled {
		t.Fatal("expected HUD messages to toggle off")
	}
	if sg.settings.HUDMessages {
		t.Fatal("expected frontend settings copy to toggle off")
	}
	sg.frontendChangeMessages()
	if !g.hudMessagesEnabled {
		t.Fatal("expected HUD messages to toggle back on")
	}
}

func TestFrontendChangeMessagesWorksWithoutSessionRuntime(t *testing.T) {
	g := &game{hudMessagesEnabled: true}
	sg := &sessionGame{
		g:        g,
		settings: gameplay.PersistentSettings{HUDMessages: true},
	}
	sg.frontendChangeMessages()
	if g.hudMessagesEnabled {
		t.Fatal("expected direct game toggle path to turn HUD messages off")
	}
	if sg.settings.HUDMessages {
		t.Fatal("expected settings copy to follow direct game toggle path")
	}
}

func TestPauseMessagesOptionTogglesOnAdjustAndCycle(t *testing.T) {
	g := &game{hudMessagesEnabled: true}
	g.pauseMenuOptionsOn = 0
	g.adjustPauseOption(1)
	if g.hudMessagesEnabled {
		t.Fatal("expected pause left/right adjustment to toggle messages off")
	}
	g.cyclePauseOption()
	if !g.hudMessagesEnabled {
		t.Fatal("expected pause enter cycle to toggle messages back on")
	}
}
