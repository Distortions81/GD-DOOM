package doomruntime

import (
	"fmt"
	"testing"
	"time"

	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/session"
	"github.com/hajimehoshi/ebiten/v2"
)

// Construction only inspects the roster; the speed guard never polls peers.
type speedTestCoopPeers struct{ runtimecfg.CoopPeerSource }

func (*speedTestCoopPeers) ActivePeerIDs() []byte { return nil }

func TestSimulationSpeedLockedOutsideLocalSourcePort(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"faithful", Options{}},
		{"authority", Options{SourcePortMode: true, AuthorityClient: &fakeAuthorityClient{}}},
		{"relay-host", Options{SourcePortMode: true, LiveTicSink: &testLiveTicSink{}}},
		{"watcher", Options{SourcePortMode: true, LiveTicSource: &testLiveTicSource{}}},
		{"coop", Options{SourcePortMode: true, CoopPeers: &speedTestCoopPeers{}}},
		{"demo", Options{SourcePortMode: true, DemoScript: &DemoScript{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &game{opts: tc.opts, simTickScale: 8, simTickAccum: 0.9}
			// Even a stale local scale cannot affect a network/faithful clock.
			if g.expectedSimStepSeconds() != 1.0/doomTicsPerSecond {
				t.Fatal("nonlocal clock inherited local speed")
			}
			for _, speed := range []float64{0.1, 0.5, 2, 8} {
				g.setSimTickScale(speed)
				if g.simTickScale != 1 || g.simTickAccum != 0 || g.useText != "" {
					t.Fatal("blocked speed control changed rate or displayed speed feedback")
				}
			}
			g.simTickScale, g.simTickAccum = 8, 0.9
			ticks := 0
			for range 35 {
				ticks += g.consumeSimTicks()
			}
			if ticks != 35 || g.simTickScale != 1 || g.simTickAccum != 0 {
				t.Fatalf("35 updates advanced %d ticks with scale %v", ticks, g.simTickScale)
			}
		})
	}
}

func TestFaithfulSpeedHotkeysDoNotChangeCadence(t *testing.T) {
	for _, key := range []ebiten.Key{ebiten.KeyComma, ebiten.KeyPeriod, ebiten.KeySlash} {
		t.Run(fmt.Sprint(key), func(t *testing.T) {
			g := newGame(predictionTestMap(), Options{Headless: true, NoMonsters: true})
			g.input.justPressedKeys = map[ebiten.Key]struct{}{key: {}}
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
			if g.worldTic != 1 || g.simTickScale != 1 || g.useText != "" {
				t.Fatal("faithful speed hotkey altered simulation or HUD")
			}
		})
	}
}

func TestNativeMultiplayerSpeedControlsAreDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"authority", Options{AuthorityClient: &fakeAuthorityClient{}}},
		{"relay-host", Options{LiveTicSink: &testLiveTicSink{}}},
		{"watcher", Options{LiveTicSource: &testLiveTicSource{}}},
		{"coop", Options{CoopPeers: &speedTestCoopPeers{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Headless = true
			c := NewNativeCampaign(NewNativeMeshGame(predictionTestMap(), tc.opts), tc.opts, nil)
			c.SetSimulationSpeed(8)
			if c.SimulationSpeed() != 1 || c.ConsumeSimulationTicks() != 1 || c.Game.g.useText != "" {
				t.Fatal("native multiplayer accepted a speed control")
			}
		})
	}
}

func TestFaithfulRenderUsesTickStateIncludingMovingDoors(t *testing.T) {
	g := newGame(predictionTestMap(), Options{Headless: true, SmoothCameraYaw: true})
	now := time.Unix(100, 0)
	g.capturePrevState()
	g.p.x += 20 * fracUnit
	g.p.y += 10 * fracUnit
	g.p.angle += doomAng90
	g.State.SetCamera(float64(g.p.x)/fracUnit, float64(g.p.y)/fracUnit)
	g.doors[0] = &doorThinker{direction: -1, speed: 2 * fracUnit}
	g.markSimUpdate(now)
	for _, elapsed := range []time.Duration{0, time.Second / 70, time.Second} {
		g.prepareRenderStateAt(now.Add(elapsed))
		if g.renderAlpha != 1 || g.renderPX != float64(g.p.x)/fracUnit || g.renderPY != float64(g.p.y)/fracUnit || g.renderAngle != g.p.angle {
			t.Fatal("faithful camera used an intermediate pose")
		}
		if g.State.RenderCamX != g.renderPX || g.State.RenderCamY != g.renderPY {
			t.Fatal("automap camera diverged from tick state")
		}
		if floor, ceil, _ := g.sectorHeightRenderSnapshot(0); floor != g.sectorFloor[0] || ceil != g.sectorCeil[0] {
			t.Fatal("faithful door rendered ahead of canonical heights")
		}
	}
}

func TestFaithfulMultiplayerSnapshotsAndCameraSnap(t *testing.T) {
	a, g, data := snapshotFixture(t)
	g.opts.SourcePortMode = false
	p, err := newClientPrediction(g, netgame.Welcome{Epoch: 7, PlayerID: 2})
	if err != nil {
		t.Fatal(err)
	}
	g.clientPrediction = p
	now := time.Unix(100, 0)
	if _, err := p.reconcileAt(netgame.Snapshot{Epoch: 7, ID: 1, Tick: 10, State: data}, now); err != nil {
		t.Fatal(err)
	}
	a.players[1].p.x += 20 * fracUnit
	a.players[2].p.x += 10 * fracUnit
	a.players[2].playerViewZ += 4 * fracUnit
	a.g.projectiles[0].x += 30 * fracUnit
	a.g.sectorFloor[0] += 6 * fracUnit
	a.g.sectorCeil[0] += 12 * fracUnit
	for i, thing := range a.g.m.Things {
		if thing.Type == barrelThingType {
			x, y := a.g.thingPosFixed(i, thing)
			a.g.setThingPosFixed(i, x+20*fracUnit, y)
		}
	}
	// A same-tic correction exercises reconciliation smoothing as well as
	// the independently timed remote snapshot presentation path.
	next, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.reconcileAt(netgame.Snapshot{Epoch: 7, ID: 2, Tick: 10, State: next}, now); err != nil {
		t.Fatal(err)
	}
	if g.authorityRender != nil || p.renderCorrection != (predictionRenderCorrection{}) || p.supportCorrection != (predictionSupportCorrection{}) {
		t.Fatal("faithful reconciliation retained smoothing")
	}
	// Also reject residual presentation state after switching out of Source Port.
	g.authorityRender = newAuthorityRenderTimeline(authorityTimelineTestFrame(10), authorityTimelineTestFrame(13), now)
	p.renderCorrection = predictionRenderCorrection{x: 3, y: 2, z: 4, angle: float64(doomAng5), started: now}
	p.renderEyeOffset = 4
	for _, elapsed := range []time.Duration{0, time.Second / 70, time.Second} {
		g.prepareRenderStateAt(now.Add(elapsed))
		if g.renderAlpha != 1 || g.renderPX != float64(g.p.x)/fracUnit || g.renderPY != float64(g.p.y)/fracUnit || g.renderAngle != g.p.angle || g.playerEyeZ() != g.playerBaseEyeZ() {
			t.Fatal("faithful local camera retained multiplayer smoothing")
		}
		if got := g.authorityRemoteRenderPose(1, g.remotePlayers[1].p); got != playerRenderPose(g.remotePlayers[1].p) {
			t.Fatal("faithful remote player used snapshot interpolation")
		}
		for i, thing := range g.m.Things {
			x, y := g.thingPosFixed(i, thing)
			z, _, _ := g.thingSupportState(i, thing)
			if rx, ry, rz := g.thingRenderPosFixed(i, thing, 1); rx != x || ry != y || rz != z {
				t.Fatal("faithful actor used snapshot interpolation")
			}
		}
		for _, projectile := range g.projectiles {
			if x, y, z := g.projectileRenderPosFixed(projectile, 1); x != projectile.x || y != projectile.y || z != projectile.z {
				t.Fatal("faithful projectile used snapshot interpolation")
			}
		}
		if floor, ceil, _ := g.sectorHeightRenderSnapshot(0); floor != g.sectorFloor[0] || ceil != g.sectorCeil[0] {
			t.Fatal("faithful world mover used snapshot interpolation")
		}
	}
	g.opts.AuthorityClient = &fakeAuthorityClient{welcome: netgame.Welcome{PlayerID: 0}}
	g.prepareRenderStateAt(now.Add(time.Second / 70))
	if g.renderPX != float64(g.p.x)/fracUnit || g.renderPY != float64(g.p.y)/fracUnit || g.renderAngle != g.p.angle {
		t.Fatal("faithful spectator camera used snapshot interpolation")
	}
}

func TestFaithfulPresentationHoldsFramesAtSessionCadence(t *testing.T) {
	for _, sourcePort := range []bool{false, true} {
		t.Run(fmt.Sprintf("sourceport=%v", sourcePort), func(t *testing.T) {
			opts := Options{Headless: true, Width: 320, Height: 200, SourcePortMode: sourcePort, NoFPS: true}
			g := newGame(predictionTestMap(), opts)
			g.opts.DemoScript = &DemoScript{Tics: make([]DemoTic, 100)}
			sg := &sessionGame{g: g, rt: g, opts: opts}
			host := session.New(sg)
			screen := ebiten.NewImage(320, 240)
			for range 140 {
				if err := host.Update(); err != nil {
					t.Fatal(err)
				}
				host.Draw(screen)
				host.Draw(screen)
			}
			want := 35
			if sourcePort {
				want = 280
			}
			if g.worldTic != 35 || g.demoBenchDraws != want {
				t.Fatalf("140 host updates, 280 presents: simulation=%d draws=%d; want 35 ticks, %d draws", g.worldTic, g.demoBenchDraws, want)
			}
			// A resize must redraw immediately instead of scaling stale UI.
			host.Draw(ebiten.NewImage(640, 480))
			if g.demoBenchDraws != want+1 {
				t.Fatal("resizing did not redraw faithful presentation")
			}
		})
	}
}
