package doomruntime

import (
	"math"
	"reflect"
	"testing"
	"time"

	"gddoom/internal/doomrand"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/netplay"
	"gddoom/internal/runtimecfg"
)

func TestNativeWatchReplaysExitAndHostIntermissionAdvance(t *testing.T) {
	host := nativeSaveFixture(t)
	sink := &testLiveTicSink{}
	host.session.opts.LiveTicSink, host.Game.g.opts.LiveTicSink = sink, sink
	var exitPose bool
	for _, line := range host.Map().Linedefs {
		if line.Special != 11 || line.SideNum[0] < 0 {
			continue
		}
		a, b := host.Map().Vertexes[line.V1], host.Map().Vertexes[line.V2]
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		length := math.Hypot(dx, dy)
		if length == 0 {
			continue
		}
		front := host.Map().Sectors[host.Map().Sidedefs[line.SideNum[0]].Sector]
		if err := host.Game.SetPose((float64(a.X)+float64(b.X))/2+dy/length*24, (float64(a.Y)+float64(b.Y))/2-dx/length*24, float64(front.FloorHeight)+41, math.Atan2(dx, -dy)); err != nil {
			t.Fatal(err)
		}
		exitPose = true
		break
	}
	if !exitPose {
		t.Fatal("missing E1M1 exit switch")
	}
	keyframe, err := host.CaptureKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	watch := nativeSaveFixture(t)
	source := &testLiveTicSource{}
	watch.session.opts.LiveTicSource, watch.Game.g.opts.LiveTicSource = source, source
	if err := watch.LoadKeyframe(keyframe); err != nil {
		t.Fatal(err)
	}
	for i := range 1000 {
		beforeMenu, beforePlay := doomrand.State()
		priorAdvances := sink.intermissionAdvance
		if err := host.Tick(NativeMeshInput{Use: i == 1}, i == 25 || i == 45); err != nil {
			t.Fatal(err)
		}
		afterMenu, afterPlay := doomrand.State()
		source.tics = append(source.tics, sink.tics...)
		sink.tics = nil
		source.intermissionAdvance += sink.intermissionAdvance - priorAdvances
		doomrand.SetState(beforeMenu, beforePlay)
		// A watcher repeatedly pressing skip cannot substitute for the host.
		if err := watch.Tick(NativeMeshInput{Use: true, Forward: 1}, true); err != nil {
			t.Fatal(err)
		}
		if watch.Phase() != host.Phase() || watch.Map().Name != host.Map().Name || !reflect.DeepEqual(watch.session.intermission.state, host.session.intermission.state) {
			t.Fatalf("watch campaign diverged at tic %d", i)
		}
		if watch.Phase() == NativeCampaignPlaying && watch.Game.g.SimChecksum() != host.Game.g.SimChecksum() {
			t.Fatalf("watch simulation diverged at tic %d", i)
		}
		doomrand.SetState(afterMenu, afterPlay)
		if host.Map().Name == "E1M2" {
			return
		}
	}
	t.Fatal("use-triggered exit did not reach E1M2")
}

func TestNativeRelayLateJoinReplaysExactMainCompatibleState(t *testing.T) {
	c := nativeSaveFixture(t)
	opts := c.session.opts
	opts.NoMonsters = false
	c = NewNativeCampaign(NewNativeMeshGame(c.Map(), opts), opts, c.next)
	srv, err := netplay.ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	b, err := netplay.DialRelayBroadcaster(srv.Addr(), 0, launchcatalog.BroadcastSessionConfig(c.Map().Name, opts))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	b.SetLowLatency(true)
	c.session.opts.LiveTicSink, c.Game.g.opts.LiveTicSink = b, b
	initial, err := c.CaptureKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	main := &sessionGame{opts: opts}
	if err := main.unmarshalNetplayKeyframe(initial); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(captureLiveRuntimeRoundTripState(c.Game.g), captureLiveRuntimeRoundTripState(main.g)) {
		t.Fatal("native network keyframe differs when loaded by main")
	}
	if err := c.BroadcastInitialKeyframe(); err != nil {
		t.Fatal(err)
	}
	for i := range 205 {
		in := NativeMeshInput{Turn: 1, Run: true, Fire: i%6 == 0, Use: i == 25, Forward: 1, YawDelta: .015, WeaponSlot: 1}
		if err := c.Tick(in, false); err != nil {
			t.Fatal(err)
		}
	}
	checksum := c.Game.g.SimChecksum()
	state := captureLiveRuntimeRoundTripState(c.Game.g)
	v, err := netplay.DialRelayViewer(srv.Addr(), b.SessionID(), opts.WADHash)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	deadline := time.Now().Add(3 * time.Second)
	var blob []byte
	for time.Now().Before(deadline) {
		kf, ready, err := v.PollKeyframe()
		if err != nil {
			t.Fatal(err)
		}
		if ready {
			if kf.Tic != 175 {
				t.Fatalf("periodic late-join keyframe tic=%d want175", kf.Tic)
			}
			blob = kf.Blob
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(blob) == 0 {
		t.Fatal("late join has no periodic keyframe")
	}
	opts.LiveTicSource = v
	watch := NewNativeCampaign(NewNativeMeshGame(c.Map(), opts), opts, c.next)
	if err := watch.LoadKeyframe(blob); err != nil {
		t.Fatal(err)
	}
	for time.Now().Before(deadline) && watch.Game.g.worldTic < 205 {
		if err := watch.Tick(NativeMeshInput{Forward: -1, Fire: true}, false); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if watch.Game.g.worldTic != 205 || watch.Game.g.SimChecksum() != checksum {
		t.Fatalf("late join diverged: tic=%d checksum=%x want=%x", watch.Game.g.worldTic, watch.Game.g.SimChecksum(), checksum)
	}
	// Recording/network quantization must replay player, actors and moving sectors.
	got := captureLiveRuntimeRoundTripState(watch.Game.g)
	if !reflect.DeepEqual(got, state) {
		t.Fatal("network replay has different full runtime state")
	}
	before := watch.Game.g.SimChecksum()
	if err := watch.Tick(NativeMeshInput{Forward: 1, Use: true, Fire: true}, false); err != nil {
		t.Fatal(err)
	}
	if watch.Game.g.worldTic != 205 || watch.Game.g.SimChecksum() != before {
		t.Fatal("local watcher input advanced the simulation")
	}
	// Use both real endpoints: history is delivered by relay echo, including
	// spectator messages; neither adapter invents its own local echo.
	for _, send := range []func(runtimecfg.ChatMessage) error{b.SendRuntimeChat, v.SendRuntimeChat} {
		if err := send(runtimecfg.ChatMessage{Name: "P1", Text: "hello relay"}); err != nil {
			t.Fatal(err)
		}
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && (len(c.Game.g.chatHistory) < 2 || len(watch.Game.g.chatHistory) < 2) {
		if err := c.PollNetwork(); err != nil {
			t.Fatal(err)
		}
		if err := watch.PollNetwork(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if !reflect.DeepEqual(c.Game.g.chatHistory, watch.Game.g.chatHistory) || len(watch.Game.g.chatHistory) != 2 {
		t.Fatal("relay chat did not reach both hosts")
	}
}

func TestNativeMandatoryKeyframeRestoresMapAndRejectsCorruption(t *testing.T) {
	c := nativeSaveFixture(t)
	sink := &testLiveTicSink{}
	c.session.opts.LiveTicSink, c.Game.g.opts.LiveTicSink = sink, sink
	data, err := c.SaveData("network load")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.LoadData(data); err != nil {
		t.Fatal(err)
	}
	if len(sink.keyframes) != 1 || sink.keyframeFlags[0] != saveMandatoryApplyKeyframeFlag {
		t.Fatal("save load omitted mandatory network snapshot")
	}
	watch := nativeSaveFixture(t)
	source := &testLiveTicSource{}
	watch.session.opts.LiveTicSource, watch.Game.g.opts.LiveTicSource = source, source
	keyframe := sink.keyframes[0]
	old, checksum := watch.Game, watch.Game.g.SimChecksum()
	broken := append([]byte(nil), keyframe...)
	broken[len(broken)-1] ^= 1
	if err := watch.LoadKeyframe(broken); err == nil || watch.Game != old || watch.Game.g.SimChecksum() != checksum {
		t.Fatal("corrupt keyframe changed live watcher")
	}
	if err := watch.LoadKeyframe(keyframe); err != nil {
		t.Fatal(err)
	}
	if watch.Game.g.worldTic != c.Game.g.worldTic || watch.Game.g.SimChecksum() != c.Game.g.SimChecksum() {
		t.Fatal("mandatory snapshot did not restore simulation")
	}
	// The same envelope can carry a fresh map, as New Game/IDCLEV do.
	fresh, err := c.session.opts.NewGameLoader("E1M2")
	if err != nil {
		t.Fatal(err)
	}
	c.Game = NewNativeMeshGame(fresh, c.session.opts)
	c.session.g, c.session.current = c.Game.g, fresh.Name
	blob, err := c.CaptureKeyframe()
	if err != nil {
		t.Fatal(err)
	}
	if err := watch.LoadKeyframe(blob); err != nil {
		t.Fatal(err)
	}
	if watch.Map().Name != "E1M2" {
		t.Fatal("network keyframe did not change map")
	}
	fresh, err = c.session.opts.NewGameLoader("E1M2")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Restart(fresh); err != nil {
		t.Fatal(err)
	}
	if len(sink.keyframes) != 2 || sink.keyframeFlags[1] != saveMandatoryApplyKeyframeFlag {
		t.Fatal("restart omitted mandatory watcher snapshot")
	}
}

func TestNativeBroadcastCheatsSynchronizeOutsideCommandStream(t *testing.T) {
	host, watch := nativeSaveFixture(t), nativeSaveFixture(t)
	sink, source := &testLiveTicSink{}, &testLiveTicSource{}
	host.session.opts.LiveTicSink, host.Game.g.opts.LiveTicSink = sink, sink
	watch.session.opts.LiveTicSource, watch.Game.g.opts.LiveTicSource = source, source
	for _, code := range []string{"idkfa", "iddqd", "idclip", "idbeholdv"} {
		for _, ch := range code {
			if _, err := host.TypeCheats([]rune{ch}); err != nil {
				t.Fatal(err)
			}
		}
		if len(sink.keyframes) != 1 || sink.keyframeFlags[0] != saveMandatoryApplyKeyframeFlag {
			t.Fatalf("%s did not publish one authoritative snapshot", code)
		}
		source.keyframes = append(source.keyframes, runtimecfg.RuntimeKeyframe{Blob: sink.keyframes[0], MandatoryApply: true})
		sink.keyframes, sink.keyframeFlags = nil, nil
		if err := watch.PollNetwork(); err != nil {
			t.Fatal(err)
		}
		// Sparse snapshot encoding omits false ownership entries (e.g. Doom
		// 1's unavailable super shotgun). Their absence means the same thing.
		canonical := func(inv playerInventory) playerInventorySaveState {
			s := capturePlayerInventorySaveState(inv)
			for id, owned := range s.Weapons {
				if !owned {
					delete(s.Weapons, id)
				}
			}
			return s
		}
		if !reflect.DeepEqual(canonical(watch.Game.g.inventory), canonical(host.Game.g.inventory)) || watch.Game.g.invulnerable != host.Game.g.invulnerable || watch.Game.g.noClip != host.Game.g.noClip || watch.Game.g.stats != host.Game.g.stats {
			t.Fatalf("watcher lost %s state: inventory native=%+v watch=%+v invulnerable=%v/%v noclip=%v/%v", code, host.Game.g.inventory, watch.Game.g.inventory, host.Game.g.invulnerable, watch.Game.g.invulnerable, host.Game.g.noClip, watch.Game.g.noClip)
		}
	}
	before := watch.Game.g.noClip
	if active, err := watch.TypeCheats([]rune("idclip")); active || err != nil || watch.Game.g.noClip != before {
		t.Fatal("watcher accepted a local cheat")
	}
}
