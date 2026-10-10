package doomruntime

import (
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

func authorityMouseFrame(t *testing.T, g *game, stamp time.Time, x int, sample func() demo.Tic) {
	t.Helper()
	g.sampleMouseLookPosition(x)
	if err := g.updateAuthoritativeClientAt(stamp, sample); err != nil {
		t.Fatal(err)
	}
	// Match the host's Update lifecycle: all per-frame input is discarded.
	g.clearSampledInput()
}

func TestAuthorityMouseStartupSuppressionExpiresAndFramesReachPrediction(t *testing.T) {
	a, g, client := authorityClientTestWorld(t, 0)
	g.opts.MouseLook, g.opts.MouseLookSpeed, g.opts.SourcePortMode = true, 1, true
	client.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	now := time.Unix(100, 0)
	initialAngle := g.p.angle
	// newGame initially suppresses capture/layout motion. It must expire even
	// though multiplayer never calls the single-player updateWalkMode path.
	for i, x := range []int{0, 10, 20} {
		authorityMouseFrame(t, g, now.Add(time.Duration(i)*10*time.Millisecond), x, g.buildAuthoritativeClientTic)
	}
	if g.mouseLookSuppressTicks != 0 || g.p.angle != initialAngle {
		t.Fatal("startup suppression never expired or allowed a capture jump")
	}
	for i, x := range []int{26, 31} {
		authorityMouseFrame(t, g, now.Add(time.Duration(25+i*2)*time.Millisecond), x, g.buildAuthoritativeClientTic)
	}
	if len(client.sent) != 1 || g.clientUpdate.mouseTurnPending == 0 || g.p.angle != initialAngle {
		t.Fatal("between-tic mouse input was discarded or predicted before its command")
	}
	authorityMouseFrame(t, g, now.Add(30*time.Millisecond), 35, g.buildAuthoritativeClientTic)
	batch := client.sent[len(client.sent)-1]
	last := batch.Inputs[len(batch.Inputs)-1]
	if last.Tick != 2 || last.Command.AngleTurn != -600 {
		t.Fatalf("relative mouse command=%+v; want all15pixels once", last)
	}
	if g.p.angle == initialAngle || g.clientUpdate.mouseTurnPending != 0 || g.worldTic != 0 {
		t.Fatal("mouse did not predict local yaw independently of world clock")
	}
	if err := a.Step(nil); err != nil {
		t.Fatal(err)
	}
	if err := a.Step(map[byte]demo.Tic{1: last.Command}); err != nil {
		t.Fatal(err)
	}
	if g.p.angle != a.players[1].p.angle {
		t.Fatal("mouse prediction differs from authoritative command replay")
	}
	angle := g.p.angle
	client.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 2, netgame.InputAck{
		HasTick: true, Tick: last.Tick, HasSequence: true, Sequence: last.Sequence,
	})}
	authorityMouseFrame(t, g, now.Add(60*time.Millisecond), 35, g.buildAuthoritativeClientTic)
	batch = client.sent[len(client.sent)-1]
	if batch.Inputs[len(batch.Inputs)-1].Command.AngleTurn != 0 || g.p.angle != angle {
		t.Fatal("mouse delta was applied more than once")
	}
}

func TestAuthorityMouseRepeatedHostSamplesAndCatchupConsumeMotionOnce(t *testing.T) {
	a, g, client := authorityClientTestWorld(t, 0)
	g.opts.MouseLook, g.opts.MouseLookSpeed = true, 1
	g.mouseLookSuppressTicks = 0
	client.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
	now := time.Unix(100, 0)
	authorityMouseFrame(t, g, now, 100, g.buildAuthoritativeClientTic)
	// The outer host samples four times per runtime update. Repeated samples
	// at the same position contribute nothing, and catch-up commands consume
	// the accumulated delta only in their first scheduled tic.
	for _, x := range []int{102, 102, 106, 110} {
		g.sampleMouseLookPosition(x)
	}
	if err := g.updateAuthoritativeClientAt(now.Add(100*time.Millisecond), g.buildAuthoritativeClientTic); err != nil {
		t.Fatal(err)
	}
	g.clearSampledInput()
	batch := client.sent[len(client.sent)-1]
	if len(batch.Inputs) != 4 {
		t.Fatalf("commands=%d want initial command plus three catch-up commands", len(batch.Inputs))
	}
	for index, input := range batch.Inputs {
		want := int16(0)
		if index == 1 {
			want = -400
		}
		if input.Command.AngleTurn != want {
			t.Fatalf("command %d angle=%d want=%d", index, input.Command.AngleTurn, want)
		}
	}
	if g.input.mouseTurnRawAccum != 0 || g.clientUpdate.mouseTurnPending != 0 {
		t.Fatal("consumed mouse input remains queued")
	}
}

func TestAuthorityMouseSensitivityAndInvertReachCommands(t *testing.T) {
	for _, inverted := range []bool{false, true} {
		a, g, client := authorityClientTestWorld(t, 0)
		g.opts.MouseLook, g.opts.MouseLookSpeed, g.opts.MouseInvert = true, .5, inverted
		g.mouseInputScaleX = 2
		g.mouseLookSuppressTicks = 0
		client.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
		now := time.Unix(100, 0)
		authorityMouseFrame(t, g, now, 100, g.buildAuthoritativeClientTic)
		authorityMouseFrame(t, g, now.Add(time.Second/35), 110, g.buildAuthoritativeClientTic)
		batch := client.sent[len(client.sent)-1]
		got := batch.Inputs[len(batch.Inputs)-1].Command.AngleTurn
		want := int16(-400)
		if inverted {
			want = 400
		}
		if got != want {
			t.Fatalf("inverted=%t mouse turn=%d want=%d", inverted, got, want)
		}
	}
}

func TestAuthorityMouseMenusChatAndOfflineDiscardPendingIntent(t *testing.T) {
	for _, state := range []string{"menu", "chat", "offline"} {
		t.Run(state, func(t *testing.T) {
			a, g, client := authorityClientTestWorld(t, 0)
			resuming := &fakeResumingAuthorityClient{fakeAuthorityClient: client, status: netgame.ConnectionStatus{State: netgame.ConnectionConnected}}
			g.opts.AuthorityClient = resuming
			g.opts.MouseLook, g.opts.MouseLookSpeed = true, 1
			g.mouseLookSuppressTicks = 0
			client.snapshots = []netgame.Snapshot{predictionSnapshot(t, a, 1, netgame.InputAck{})}
			now := time.Unix(100, 0)
			authorityMouseFrame(t, g, now, 0, g.buildAuthoritativeClientTic)
			authorityMouseFrame(t, g, now.Add(10*time.Millisecond), 10, g.buildAuthoritativeClientTic)
			if g.clientUpdate.mouseTurnPending == 0 {
				t.Fatal("test did not queue between-tic motion")
			}
			g.pendingUse, g.demoWeaponSlot = true, 3
			sample := g.buildAuthoritativeClientTic
			switch state {
			case "menu":
				g.frontendActive = true
				sample = nil
			case "chat":
				g.chatComposeOpen = true
			case "offline":
				resuming.status.State = netgame.ConnectionReconnecting
			}
			authorityMouseFrame(t, g, now.Add(20*time.Millisecond), 400, sample)
			if g.clientUpdate.mouseTurnPending != 0 || g.pendingUse || g.demoWeaponSlot != 0 || g.mouseLookSet {
				t.Fatal("blocked input retained gameplay intent")
			}
			authorityMouseFrame(t, g, now.Add(40*time.Millisecond), 800, sample)
			if state != "offline" {
				batch := client.sent[len(client.sent)-1]
				if batch.Inputs[len(batch.Inputs)-1].Command != (demo.Tic{}) {
					t.Fatal("menu/chat command was not neutral")
				}
			} else if len(client.sent) != 1 {
				t.Fatal("offline client submitted input")
			}
			g.frontendActive, g.chatComposeOpen = false, false
			resuming.status.State = netgame.ConnectionConnected
			authorityMouseFrame(t, g, now.Add(70*time.Millisecond), 1200, g.buildAuthoritativeClientTic)
			batch := client.sent[len(client.sent)-1]
			if batch.Inputs[len(batch.Inputs)-1].Command.AngleTurn != 0 {
				t.Fatal("cursor jump leaked after input ownership returned")
			}
			authorityMouseFrame(t, g, now.Add(100*time.Millisecond), 1205, g.buildAuthoritativeClientTic)
			batch = client.sent[len(client.sent)-1]
			if batch.Inputs[len(batch.Inputs)-1].Command.AngleTurn != -200 {
				t.Fatal("fresh relative motion did not resume after block")
			}
		})
	}
}

func TestAuthorityMouseLookToggleClearsIntentAndRestartsCapture(t *testing.T) {
	_, g, _ := authorityClientTestWorld(t, 0)
	g.opts.MouseLook = true
	g.mouseLookSet = true
	g.clientUpdate.mouseTurnPending = 100 << 16
	g.toggleAuthoritativeMouseLook()
	if g.opts.MouseLook || g.mouseLookSet || g.clientUpdate.mouseTurnPending != 0 {
		t.Fatal("disabling mouse look retained camera intent")
	}
	g.toggleAuthoritativeMouseLook()
	if !g.opts.MouseLook || g.mouseLookSet || g.mouseLookSuppressTicks != detailMouseSuppressTicks {
		t.Fatal("enabling mouse look did not establish safe capture baseline")
	}
}
