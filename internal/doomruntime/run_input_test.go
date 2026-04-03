package doomruntime

import (
	"testing"

	"gddoom/internal/gameplay"
)

type demoActiveRuntime struct {
	layoutCountRuntime
	demoActive bool
}

func (r *demoActiveRuntime) sessionSignals() gameplay.SessionSignals {
	return gameplay.SessionSignals{DemoActive: r.demoActive}
}

func TestShouldSampleRuntimeInputDuringFrontendAttractDemo(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true},
		rt:       &demoActiveRuntime{demoActive: true},
	}
	if !sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = false, want true during frontend attract demo")
	}
}

func TestShouldNotSampleRuntimeInputDuringFrontendMenuWithoutDemo(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true},
		rt:       &demoActiveRuntime{demoActive: false},
	}
	if sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = true, want false for frontend without active demo")
	}
}

func TestShouldNotSampleRuntimeInputWhenAttractMenuIsOpen(t *testing.T) {
	sg := &sessionGame{
		frontend: frontendState{Active: true, MenuActive: true},
		rt:       &demoActiveRuntime{demoActive: true},
	}
	if sg.shouldSampleRuntimeInput() {
		t.Fatal("shouldSampleRuntimeInput() = true, want false when attract menu is already open")
	}
}

func TestOpenFrontendMenuFromSignalUsesAttractMenuForDemo(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{DemoActive: true})
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend attract menu to open")
	}
	if !sg.frontend.Attract {
		t.Fatal("expected attract mode for demo-triggered frontend menu")
	}
	if sg.frontend.InGame {
		t.Fatal("did not expect in-game pause menu for demo-triggered frontend menu")
	}
}

func TestOpenFrontendMenuFromSignalUsesPauseMenuForGameplay(t *testing.T) {
	sg := &sessionGame{}
	sg.openFrontendMenuFromSignal(gameplay.SessionSignals{DemoActive: false})
	if !sg.frontend.Active || !sg.frontend.MenuActive {
		t.Fatal("expected frontend menu to open")
	}
	if sg.frontend.Attract {
		t.Fatal("did not expect attract mode for gameplay frontend menu")
	}
	if !sg.frontend.InGame {
		t.Fatal("expected in-game pause menu for gameplay frontend menu")
	}
}
