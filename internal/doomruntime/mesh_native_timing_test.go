package doomruntime

import (
	"fmt"
	"math"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/runtimecfg"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestNativeScaledLiveSimulationMatchesMainUpdates(t *testing.T) {
	for _, speed := range []float64{.1, .5, .9, 1, 1.1, 1.5, 2, 8} {
		t.Run(fmt.Sprint(speed), func(t *testing.T) {
			c := nativeCampaignFixture(t, "E1M1")
			opts := c.session.opts
			opts.SourcePortMode, opts.AlwaysRun = true, false
			opts.InputBindings = runtimecfg.DefaultInputBindings()
			c = NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(c.Game.g.restartTemplate), opts), opts, c.next)
			main := newGame(cloneMapForRestart(c.Game.g.restartTemplate), opts)
			c.SetSimulationSpeed(speed)
			main.setSimTickScale(speed)
			for update := 0; update < 80; update++ {
				menuRNG, playRNG := doomrand.State()
				main.input = gameInputSnapshot{pressedKeys: map[ebiten.Key]struct{}{ebiten.KeyW: {}}, justPressedKeys: map[ebiten.Key]struct{}{}}
				fire := update%9 < 3
				if fire {
					main.input.pressedKeys[ebiten.KeyControlLeft] = struct{}{}
				}
				if err := main.Update(); err != nil {
					t.Fatal(err)
				}
				afterMenu, afterPlay := doomrand.State()
				doomrand.SetState(menuRNG, playRNG)
				for range c.ConsumeSimulationTicks() {
					if err := c.Tick(NativeMeshInput{Forward: 1, Fire: fire}, false); err != nil {
						t.Fatal(err)
					}
				}
				if c.Game.g.SimChecksum() != main.SimChecksum() || c.Game.g.worldTic != main.worldTic {
					t.Fatalf("update %d diverged: native tic=%d checksum=%d main tic=%d checksum=%d", update, c.Game.g.worldTic, c.Game.g.SimChecksum(), main.worldTic, main.SimChecksum())
				}
				gotMenu, gotPlay := doomrand.State()
				if gotMenu != afterMenu || gotPlay != afterPlay {
					t.Fatalf("random sequence diverged at update %d", update)
				}
			}
		})
	}
}

func TestNativeSimulationSpeedBoundsAndRenderInterpolation(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M1")
	for _, tc := range []struct{ input, want float64 }{{0, .1}, {100, 8}, {1, 1}} {
		c.SetSimulationSpeed(tc.input)
		if c.SimulationSpeed() != tc.want || c.Game.g.useText != fmt.Sprintf("Game Speed: %.2fx", tc.want) {
			t.Fatal("speed limit or shared HUD feedback differs")
		}
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c.SetSimulationSpeed(invalid)
		if c.SimulationSpeed() != 1 {
			t.Fatal("invalid clock scale accepted")
		}
	}
	c.SetSimulationSpeed(.5)
	if c.ConsumeSimulationTicks() != 0 || c.SimulationRenderAlpha(.5) != .75 {
		t.Fatal("slow update lost its fractional simulation time")
	}
	if c.ConsumeSimulationTicks() != 1 || c.SimulationRenderAlpha(.5) != .25 {
		t.Fatal("slow interpolation did not start its next step")
	}
	c.SetSimulationSpeed(2)
	if c.ConsumeSimulationTicks() != 2 || c.SimulationRenderAlpha(.1) != 1 {
		t.Fatal("fast simulation did not draw latest state")
	}
	c.session.intermission.state.Active = true
	c.SetSimulationSpeed(.1)
	if c.SimulationSpeed() != 2 || c.ConsumeSimulationTicks() != 1 || c.SimulationRenderAlpha(.3) != .3 {
		t.Fatal("live speed changed intermission clock")
	}
}

func TestNativeSimulationSpeedCannotChangeDemoOrWatchClock(t *testing.T) {
	for _, watching := range []bool{false, true} {
		c := nativeCampaignFixture(t, "E1M1")
		if watching {
			c.session.opts.LiveTicSource = &testLiveTicSource{}
		} else {
			c.Game.g.opts.DemoScript = &DemoScript{}
		}
		c.SetSimulationSpeed(8)
		if c.SimulationSpeed() != 1 || c.ConsumeSimulationTicks() != 1 || c.SimulationRenderAlpha(.25) != .25 {
			t.Fatal("local shortcut changed authoritative demo/watch cadence")
		}
	}
}
