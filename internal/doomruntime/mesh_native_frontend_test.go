package doomruntime

import (
	"errors"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestNativeAttractSequenceMatchesMainPagesAndSimulation(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	base := nativeCampaignFixture(t, "E1M1")
	opts := base.session.opts
	opts.SourcePortMode = true
	opts.AttractDemos = launchcatalog.LoadBuiltInDemos(wf)
	for i, script := range opts.AttractDemos {
		copy := *script
		copy.Tics = append([]demo.Tic(nil), script.Tics[:8]...)
		opts.AttractDemos[i] = &copy
	}
	opts.DemoMapLoader = func(script *DemoScript) (*mapdata.Map, error) {
		name, err := launchcatalog.ResolveDemoStartMap(wf, script, "E1M1")
		if err != nil {
			return nil, err
		}
		return mapdata.LoadMap(wf, name)
	}
	type state struct {
		page                    string
		seq, remaining, demoTic int
		mapName                 mapdata.MapName
		checksum                uint32
	}
	traces := make([][]state, 0, 2)
	for _, native := range []bool{false, true} {
		m, err := mapdata.LoadMap(wf, "E1M1")
		if err != nil {
			t.Fatal(err)
		}
		var c *NativeCampaign
		var sg *sessionGame
		if native {
			c = NewNativeCampaign(NewNativeMeshGame(m, opts), opts, base.next)
			c.StartFrontend()
			sg = c.session
		} else {
			sg = &sessionGame{bootMap: cloneMapForRestart(m), opts: opts, g: newGame(m, opts)}
			sg.startFrontend()
		}
		trace := make([]state, 0, 900)
		for tic := 0; tic < 900; tic++ {
			if native {
				if err := c.TickFrontend(); err != nil {
					t.Fatal(err)
				}
			} else {
				if sg.g.opts.DemoScript != nil {
					if err := sg.g.advanceDemoTic(); errors.Is(err, ebiten.Termination) {
						sg.advanceFrontendAttract()
					} else if err != nil {
						t.Fatal(err)
					}
				}
				var advance bool
				sg.frontend, advance = sessionflow.AdvanceFrontendFrame(sg.frontend, menuSkullBlinkTics)
				if advance {
					sg.advanceFrontendAttract()
				}
			}
			f := sg.frontend
			s := state{page: f.AttractPage, seq: f.AttractSeq, remaining: f.AttractPageTic}
			if sg.g.opts.DemoScript != nil {
				s.demoTic, s.mapName, s.checksum = sg.g.demoTick, sg.g.m.Name, sg.g.SimChecksum()
			}
			trace = append(trace, s)
		}
		traces = append(traces, trace)
	}
	if !reflect.DeepEqual(traces[0], traces[1]) {
		for i, got := range traces[1] {
			if want := traces[0][i]; got != want {
				t.Fatalf("attract tic %d native=%+v main=%+v", i, got, want)
			}
		}
	}
	seen := map[int]bool{}
	for _, s := range traces[1] {
		seen[s.seq] = true
	}
	if len(seen) != 6 {
		t.Fatalf("did not exercise complete title loop: %v", seen)
	}
	doomrand.Clear()
}

func TestNativeFrontendSkipsUnavailableDemosAndPreservesSaveLoad(t *testing.T) {
	c := nativeSaveFixture(t)
	data, err := c.SaveData("playable save")
	if err != nil {
		t.Fatal(err)
	}
	c.StartFrontend()
	if c.FrontendStatus().Page != "TITLEPIC" {
		t.Fatal("frontend did not start on title")
	}
	if _, err := c.SaveData("hidden game"); err == nil {
		t.Fatal("frontend saved a hidden game")
	}
	for i := 0; i < attractPageTitleNonCommercial; i++ {
		if err := c.TickFrontend(); err != nil {
			t.Fatal(err)
		}
	}
	if c.FrontendStatus().Page != "CREDIT" {
		t.Fatalf("missing demo did not advance to credits: %+v", c.FrontendStatus())
	}
	if err := c.LoadData([]byte("invalid")); err == nil || !c.FrontendStatus().Active {
		t.Fatal("failed load left the title loop")
	}
	if err := c.LoadData(data); err != nil {
		t.Fatal(err)
	}
	if c.FrontendStatus().Active || c.DemoStatus().Active || c.Phase() != NativeCampaignPlaying {
		t.Fatal("loaded game retained attract mode")
	}
}
