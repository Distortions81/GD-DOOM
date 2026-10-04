package doomruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomrand"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/runtimehost"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestNativeDemoFullTracesMatchMain(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range launchcatalog.LoadBuiltInDemos(wf) {
		t.Run(script.Path, func(t *testing.T) {
			name, err := launchcatalog.ResolveDemoStartMap(wf, script, "")
			if err != nil {
				t.Fatal(err)
			}
			fixture := loadMeshExperimentMap(t, name)
			opts := fixture.opts
			opts.SourcePortMode = true
			opts.DemoScript = script
			opts.DemoQuitOnComplete = true
			opts.Invulnerable = true
			opts.NoMonsters = true // The demo header must win.
			next := func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
				name, err := mapdata.NextMapName(wf, current, secret)
				if err != nil {
					return nil, "", err
				}
				m, err := mapdata.LoadMap(wf, name)
				return m, name, err
			}
			traces := make([][]byte, 0, 2)
			for _, native := range []bool{false, true} {
				opts.DemoTracePath = filepath.Join(t.TempDir(), fmt.Sprintf("trace-%t.jsonl", native))
				m, err := mapdata.LoadMap(wf, name)
				if err != nil {
					t.Fatal(err)
				}
				if native {
					c := NewNativeCampaign(NewNativeMeshGame(m, opts), opts, next)
					for tic := 0; tic < len(script.Tics)+3000 && !c.DemoStatus().Done; tic++ {
						if err := c.Tick(NativeMeshInput{Forward: -1, Fire: true, WeaponSlot: 7}, true); err != nil {
							t.Fatal(err)
						}
						if tic%17 == 0 {
							c.Game.Frame(.5)
						}
					}
					if !c.DemoStatus().Done || c.DemoStatus().Tic != len(script.Tics) {
						t.Fatalf("native did not consume complete demo: %+v", c.DemoStatus())
					}
					c.CloseDemoTrace()
				} else {
					runtime, meta := NewRuntime(m, opts, next)
					finished := false
					for tic := 0; tic < len(script.Tics)+3000; tic++ {
						err := runtime.Update()
						if errors.Is(err, ebiten.Termination) || errors.Is(err, runtimehost.ErrTerminate) {
							finished = true
							break
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					if !finished {
						t.Fatal("main did not complete demo")
					}
					meta.Close()
				}
				data, err := os.ReadFile(opts.DemoTracePath)
				if err != nil {
					t.Fatal(err)
				}
				// The writer records its own destination in metadata; normalize only that
				// field while comparing all commands, objects, sectors and RNG state.
				lines := bytes.Split(data, []byte{'\n'})
				var metadata map[string]any
				if err := json.Unmarshal(lines[0], &metadata); err != nil {
					t.Fatal(err)
				}
				delete(metadata, "trace_path")
				lines[0], err = json.Marshal(metadata)
				if err != nil {
					t.Fatal(err)
				}
				traces = append(traces, bytes.Join(lines, []byte{'\n'}))
			}
			if !bytes.Equal(traces[0], traces[1]) {
				a, b := bytes.Split(traces[0], []byte{'\n'}), bytes.Split(traces[1], []byte{'\n'})
				for i := 0; i < min(len(a), len(b)); i++ {
					if !bytes.Equal(a[i], b[i]) {
						t.Fatalf("trace diverged at line %d\nmain %s\nnative %s", i, a[i], b[i])
					}
				}
				t.Fatalf("trace line counts differ: %d / %d", len(a), len(b))
			}
		})
	}
}

func TestNativeRecordedCommandsReplayExactSimulation(t *testing.T) {
	for _, keyboard := range []bool{false, true} {
		t.Run(fmt.Sprintf("keyboard=%t", keyboard), func(t *testing.T) {
			c := nativeCampaignFixture(t, "E1M1")
			path := filepath.Join(t.TempDir(), "record.lmp")
			c.session.opts.RecordDemoPath = path
			c.Game.g.opts.RecordDemoPath = path
			c.Game.g.opts.Invulnerable = false
			type frame struct {
				checksum uint32
				x, y, z  int64
				angle    uint32
				mobjs    []demoTraceMobj
				specials []map[string]any
			}
			frames := []frame{}
			for tic := 0; tic < 90; tic++ {
				in := NativeMeshInput{Forward: 1, Run: tic > 25, Fire: tic%8 < 3, Use: tic%15 == 0, YawDelta: .005}
				if keyboard {
					in.Turn = 1
				}
				if tic == 20 {
					in.WeaponSlot = 1
				}
				if tic == 45 {
					in.WeaponSlot = 2
				}
				if tic == 60 {
					in.WeaponCycle = 1
				}
				if tic == 75 {
					in.WeaponCycle = -1
				}
				if err := c.Tick(in, false); err != nil {
					t.Fatal(err)
				}
				g := c.Game.g
				frames = append(frames, frame{g.SimChecksum(), g.p.x, g.p.y, g.p.z, g.p.angle, g.demoTraceMobjs(), g.demoTraceSpecials()})
			}
			count, err := c.FlushDemoRecording()
			if err != nil || count != len(frames) {
				t.Fatalf("flush=%d %v", count, err)
			}
			script, err := demo.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			wf, err := wad.Open(findDOOM1WAD(t))
			if err != nil {
				t.Fatal(err)
			}
			m, err := mapdata.LoadMap(wf, "E1M1")
			if err != nil {
				t.Fatal(err)
			}
			opts := c.session.opts
			opts.RecordDemoPath = ""
			opts.DemoScript = script
			replay := NewNativeCampaign(NewNativeMeshGame(m, opts), opts, c.next)
			for tic, want := range frames {
				if err := replay.Tick(NativeMeshInput{}, false); err != nil {
					t.Fatal(err)
				}
				g := replay.Game.g
				got := frame{g.SimChecksum(), g.p.x, g.p.y, g.p.z, g.p.angle, g.demoTraceMobjs(), g.demoTraceSpecials()}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("recorded simulation diverged on tic %d: pos=(%d,%d) angle=%d want (%d,%d) angle=%d checksum=%d/%d", tic, g.p.x, g.p.y, g.p.angle, want.x, want.y, want.angle, got.checksum, want.checksum)
				}
			}
		})
	}
	doomrand.Clear()
}

func TestMainRecordedWalkInputReplaysThroughNativeHost(t *testing.T) {
	fixture := nativeCampaignFixture(t, "E1M1")
	opts := fixture.session.opts
	opts.RecordDemoPath = filepath.Join(t.TempDir(), "main.lmp")
	opts.Invulnerable = false
	opts.SourcePortMode = true
	opts.InputBindings = runtimecfg.DefaultInputBindings()
	g := newGame(cloneMapForRestart(fixture.Game.g.restartTemplate), opts)
	type state struct {
		checksum uint32
		x, y, z  int64
		angle    uint32
	}
	states := make([]state, 0, 90)
	for tic := 0; tic < 90; tic++ {
		g.input = gameInputSnapshot{pressedKeys: map[ebiten.Key]struct{}{ebiten.KeyW: {}, ebiten.KeyArrowLeft: {}}, justPressedKeys: map[ebiten.Key]struct{}{}, mouseTurnRawAccum: 3420000}
		if tic > 25 {
			g.input.pressedKeys[ebiten.KeyShiftLeft] = struct{}{}
		}
		if tic%8 < 3 {
			g.input.pressedKeys[ebiten.KeyControlLeft] = struct{}{}
		}
		if tic%15 == 0 {
			g.pendingUse = true
		}
		if tic == 20 {
			g.input.justPressedKeys[ebiten.Key1] = struct{}{}
		}
		if tic == 45 {
			g.input.justPressedKeys[ebiten.Key2] = struct{}{}
		}
		if tic == 60 {
			g.input.wheelY = -1
		}
		if tic == 75 {
			g.input.wheelY = 1
		}
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		states = append(states, state{g.SimChecksum(), g.p.x, g.p.y, g.p.z, g.p.angle})
	}
	script, err := BuildRecordedDemo("E1M1", opts, g.demoRecord)
	if err != nil {
		t.Fatal(err)
	}
	data, err := FormatDemoScript(script)
	if err != nil {
		t.Fatal(err)
	}
	script, err = ParseDemoScript(data)
	if err != nil {
		t.Fatal(err)
	}
	opts.RecordDemoPath, opts.DemoScript = "", script
	replay := NewNativeCampaign(NewNativeMeshGame(cloneMapForRestart(fixture.Game.g.restartTemplate), opts), opts, fixture.next)
	for tic, want := range states {
		if err := replay.Tick(NativeMeshInput{}, false); err != nil {
			t.Fatal(err)
		}
		g := replay.Game.g
		if got := (state{g.SimChecksum(), g.p.x, g.p.y, g.p.z, g.p.angle}); got != want {
			t.Fatalf("main recording diverged at tic %d: %+v want %+v", tic, got, want)
		}
	}
	doomrand.Clear()
}

func TestNativeDemoIntermissionAndMapChangeMatchMain(t *testing.T) {
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	fixture := loadMeshExperimentMap(t, "E1M1")
	opts := fixture.opts
	opts.SourcePortMode = true
	opts.DemoScript = &DemoScript{Header: DemoHeader{Version: demo.Version110, Skill: 2, Episode: 1, Map: 1, NoMonsters: true, PlayerInGame: [4]bool{true}}, Tics: make([]DemoTic, 700)}
	for i := range opts.DemoScript.Tics {
		if i%20 == 1 {
			opts.DemoScript.Tics[i].Buttons = demo.ButtonAttack
		}
		opts.DemoScript.Tics[i].Forward = 25
	}
	next := func(current mapdata.MapName, secret bool) (*mapdata.Map, mapdata.MapName, error) {
		name, err := mapdata.NextMapName(wf, current, secret)
		if err != nil {
			return nil, "", err
		}
		m, err := mapdata.LoadMap(wf, name)
		return m, name, err
	}
	opts.DemoQuitOnComplete = true
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	rt, meta := NewRuntime(m, opts, next)
	defer meta.Close()
	main := rt.(*sessionGame)
	m, err = mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	c := NewNativeCampaign(NewNativeMeshGame(m, opts), opts, next)
	main.g.requestLevelExit(false, "exit")
	c.Game.g.requestLevelExit(false, "exit")
	ma, pa := doomrand.State()
	mb, pb := ma, pa
	reachedNext := false
	for tic := 0; tic < 700; tic++ {
		doomrand.SetState(ma, pa)
		err := rt.Update()
		if err != nil && !errors.Is(err, ebiten.Termination) && !errors.Is(err, runtimehost.ErrTerminate) {
			t.Fatal(err)
		}
		ma, pa = doomrand.State()
		expected := main.g.SimChecksum()
		doomrand.SetState(mb, pb)
		if err := c.Tick(NativeMeshInput{Fire: true}, true); err != nil {
			t.Fatal(err)
		}
		mb, pb = doomrand.State()
		if expected != c.Game.g.SimChecksum() || ma != mb || pa != pb || main.g.demoTick != c.Game.g.demoTick || main.current != c.Map().Name || !reflect.DeepEqual(main.intermission.state, c.session.intermission.state) {
			t.Fatalf("campaign demo diverged at tic %d: map=%s/%s cursor=%d/%d RNG=%d/%d vs %d/%d", tic, main.current, c.Map().Name, main.g.demoTick, c.Game.g.demoTick, ma, pa, mb, pb)
		}
		if c.Map().Name == "E1M2" {
			reachedNext = true
		}
		if c.Phase() == NativeCampaignIntermission {
			c.Patches()
		}
	}
	if !reachedNext {
		t.Fatal("test did not reach next map")
	}
}

func TestNativeRecordingFreezesOnExitAndKeepsStartingHeader(t *testing.T) {
	c := nativeCampaignFixture(t, "E1M1")
	path := filepath.Join(t.TempDir(), "record.lmp")
	c.session.opts.RecordDemoPath = path
	c.Game.g.opts.RecordDemoPath = path
	for i := 0; i < 12; i++ {
		if err := c.Tick(NativeMeshInput{Forward: 1}, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.FlushDemoRecording(); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Game.g.requestLevelExit(false, "exit")
	if err := c.Tick(NativeMeshInput{}, false); err != nil {
		t.Fatal(err)
	}
	if c.RecordingActive() {
		t.Fatal("exit did not freeze recording")
	}
	finishNativeIntermission(t, c)
	c.session.opts.SkillLevel = 5
	for i := 0; i < 4; i++ {
		if err := c.Tick(NativeMeshInput{Forward: 1}, false); err != nil {
			t.Fatal(err)
		}
	}
	script, target, err := c.RecordedDemo()
	if err != nil {
		t.Fatal(err)
	}
	if target != path || len(script.Tics) != 12 || script.Header.Map != 1 || script.Header.Skill != 2 {
		t.Fatal("next level changed frozen recording or starting header")
	}
	if _, err := c.FlushDemoRecording(); err != nil {
		t.Fatal(err)
	}
	final, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(previous, final) {
		t.Fatal("frozen checkpoint changed")
	}
	c.session.demoFlushTics = 0
	c.session.frozenDemoPath = filepath.Join(path, "missing", "record.lmp")
	if _, err := c.FlushDemoRecording(); err == nil {
		t.Fatal("failed destination accepted")
	}
	final, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(previous, final) {
		t.Fatal("failed flush damaged previous LMP")
	}
}
