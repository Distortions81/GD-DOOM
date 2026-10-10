package doomruntime

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

// This deterministic packet path exercises the production Client, framing,
// compression, Match input scheduler and E1M1 simulation/predictor. Actual QUIC
// encryption, origin policy and sockets are covered in netgame transport tests.
type authorityFaultTransport struct {
	incoming chan any
	outgoing chan any
	done     chan struct{}
	once     sync.Once
}

func (f *authorityFaultTransport) ReadMessage() (any, error) {
	select {
	case m := <-f.incoming:
		return m, nil
	case <-f.done:
		return nil, net.ErrClosed
	}
}
func (f *authorityFaultTransport) WriteMessage(m any) error {
	bytes, err := netgame.MarshalMessage(m)
	if err != nil {
		return err
	}
	m, err = netgame.UnmarshalMessage(bytes)
	if err != nil {
		return err
	}
	select {
	case f.outgoing <- m:
		return nil
	case <-f.done:
		return net.ErrClosed
	}
}
func (f *authorityFaultTransport) Close() error            { f.once.Do(func() { close(f.done) }); return nil }
func (*authorityFaultTransport) UnreliableSnapshots() bool { return true }

type authorityFaultWorld struct {
	*Authority
	steps, attackTics, ammoSpent int
}

func (w *authorityFaultWorld) Step(commands map[byte]demo.Tic) error {
	before := w.players[1].stats.Bullets
	if commands[1].Buttons&demo.ButtonAttack != 0 {
		w.attackTics++
	}
	if err := w.Authority.Step(commands); err != nil {
		return err
	}
	w.steps++
	after := w.players[1].stats.Bullets
	if before > after {
		w.ammoSpent += before - after
	}
	return nil
}

type authorityFaultPacket struct {
	due     uint32
	message any
}

func TestAuthoritativeE1M1DatagramFaultsReconcileWithoutExtraFire(t *testing.T) {
	authority := testAuthority(t)
	renderer := newGame(cloneMapForRestart(authority.g.restartTemplate), Options{Headless: true, SkillLevel: 3, NoMonsters: true})
	world := &authorityFaultWorld{Authority: authority}
	match, err := netgame.NewMatch(world, authority, netgame.MatchConfig{Epoch: 21, Compatibility: "E1M1-faults", PlayerLimit: 1, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2})
	if err != nil {
		t.Fatal(err)
	}
	hello := netgame.Hello{Compatibility: "E1M1-faults", Name: "fault test"}
	handle, welcome, err := match.Join(hello)
	if err != nil {
		t.Fatal(err)
	}
	initial := authority.players[1].p
	transport := &authorityFaultTransport{incoming: make(chan any, 128), outgoing: make(chan any, 128), done: make(chan struct{})}
	transport.incoming <- welcome
	client, err := netgame.Connect(context.Background(), transport, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	<-transport.outgoing // The real Hello was validated by Match.Join above.
	renderer.opts.AuthorityClient = client
	encoder, err := netgame.NewSnapshotEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	var upstream, downstream []authorityFaultPacket
	var acked, writtenAck, writtenSequence uint32
	var upstreamCount, downstreamCount, dropped, duplicates, reordered, maxHistory, samples int
	var latestFull netgame.Snapshot
	var lastDelivered uint32
	now := time.Unix(100, 0)
	sample := func() demo.Tic {
		samples++
		command := demo.Tic{Forward: 10}
		if samples > 40 {
			command = demo.Tic{Side: 10}
		}
		if samples == 60 {
			command.Buttons = demo.ButtonAttack
		}
		return command
	}
	enqueueInput := func(tick uint32, batch netgame.InputBatch) {
		upstreamCount++
		if batch.SnapshotAck > writtenAck {
			writtenAck = batch.SnapshotAck
		}
		for _, in := range batch.Inputs {
			if in.Sequence > writtenSequence {
				writtenSequence = in.Sequence
			}
		}
		// A sustained upstream outage outlasts the two-tic movement hold. Normal
		// packets also have varying delay, duplication, and isolated loss.
		if (tick >= 24 && tick <= 40) || upstreamCount%9 == 0 {
			dropped++
			return
		}
		delay := []uint32{0, 2, 5, 1}[upstreamCount%4]
		upstream = append(upstream, authorityFaultPacket{due: tick + delay, message: batch})
		if upstreamCount%4 == 0 {
			duplicates++
			upstream = append(upstream, authorityFaultPacket{due: tick + delay + 2, message: batch})
		}
	}
	consumeWrite := func(tick uint32, message any) {
		switch m := message.(type) {
		case netgame.InputBatch:
			enqueueInput(tick, m)
		case netgame.Ping:
			transport.incoming <- netgame.Pong{Nonce: m.Nonce}
		default:
			t.Fatalf("unexpected client message %T", message)
		}
	}
	pumpRenderer := func(tick uint32, target uint32, generate bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			stamp := now.Add(time.Duration(tick) * (time.Second / netgame.TickRate))
			if generate {
				if err := renderer.updateAuthoritativeClientAt(stamp, sample); err != nil {
					t.Fatal(err)
				}
			} else {
				for {
					snapshot, ok, err := client.PollSnapshot()
					if err != nil {
						t.Fatal(err)
					}
					if !ok {
						break
					}
					if _, err := renderer.clientPrediction.reconcileAt(snapshot, stamp); err != nil {
						t.Fatal(err)
					}
					if err := client.SendInputs(netgame.InputBatch{Epoch: 21, SnapshotAck: snapshot.ID}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if renderer.authorityFailure != nil {
				t.Fatalf("client fault status: %+v", renderer.authorityFailure)
			}
			if renderer.clientPrediction != nil && len(renderer.clientPrediction.history) > maxHistory {
				maxHistory = len(renderer.clientPrediction.history)
			}
			if target == 0 || (renderer.clientPrediction != nil && renderer.clientPrediction.SnapshotID() >= target) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("decoder/predictor stalled waiting for snapshot %d", target)
			}
			time.Sleep(time.Millisecond)
		}
		// Bound goroutine scheduling independently of simulated time: every sampled
		// command has entered the packet emulator before advancing the next tic.
		var expectedAck uint32
		if renderer.clientPrediction != nil {
			expectedAck = renderer.clientPrediction.SnapshotID()
		}
		for writtenSequence < renderer.clientUpdate.sequence || writtenAck < expectedAck {
			select {
			case message := <-transport.outgoing:
				consumeWrite(tick, message)
			case <-time.After(time.Second):
				t.Fatal("client writer stalled")
			}
		}
		for {
			select {
			case message := <-transport.outgoing:
				consumeWrite(tick, message)
			default:
				return
			}
		}
	}
	for tick := uint32(1); tick <= 160; tick++ {
		// Reverse iteration intentionally delivers newer same-time packets first.
		for i := len(upstream) - 1; i >= 0; i-- {
			if upstream[i].due <= tick {
				batch := upstream[i].message.(netgame.InputBatch)
				if err := match.Submit(handle, batch); err != nil {
					t.Fatalf("tic%d input: %v", tick, err)
				}
				if batch.SnapshotAck > acked {
					acked = batch.SnapshotAck
				}
				upstream = append(upstream[:i], upstream[i+1:]...)
			}
		}
		if authority.Tic() != tick-1 {
			t.Fatal("packet burst advanced simulation")
		}
		result, err := match.Step()
		if err != nil {
			t.Fatal(err)
		}
		if authority.Tic() != tick || world.steps != int(tick) {
			t.Fatal("server skipped or repeated a world tic")
		}
		if full, ok := result.Snapshots[handle]; ok {
			latestFull = full
			wire, err := encoder.Encode(full, acked)
			if err != nil {
				t.Fatal(err)
			}
			bytes, err := netgame.MarshalMessage(wire)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := netgame.UnmarshalMessage(bytes)
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Commit(full); err != nil {
				t.Fatal(err)
			}
			downstreamCount++
			datagram := wire.Encoding == netgame.SnapshotDeltaZstd && len(bytes) <= netgame.MaxGameDatagramBytes
			if datagram && ((tick >= 60 && tick <= 104) || downstreamCount%7 == 0) {
				dropped++
			} else {
				delay := uint32(0)
				if datagram {
					delay = []uint32{4, 0, 2}[downstreamCount%3]
				}
				downstream = append(downstream, authorityFaultPacket{due: tick + delay, message: parsed})
				if datagram && downstreamCount%5 == 0 {
					duplicates++
					downstream = append(downstream, authorityFaultPacket{due: tick + delay + 3, message: parsed})
				}
			}
		}
		var target uint32
		for i := len(downstream) - 1; i >= 0; i-- {
			if downstream[i].due <= tick {
				snapshot := downstream[i].message.(netgame.Snapshot)
				if snapshot.ID < lastDelivered {
					reordered++
				}
				if snapshot.ID > lastDelivered {
					lastDelivered = snapshot.ID
				}
				if snapshot.ID > target {
					target = snapshot.ID
				}
				transport.incoming <- snapshot
				downstream = append(downstream[:i], downstream[i+1:]...)
			}
		}
		pumpRenderer(tick, target, tick <= 120)
	}
	// Finish with an independent reliable baseline, just as recovery/terminal
	// delivery does. No future commands remain, so correction must be exact.
	full, err := encoder.Encode(latestFull, 0)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := netgame.MarshalMessage(full)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := netgame.UnmarshalMessage(bytes)
	if err != nil {
		t.Fatal(err)
	}
	transport.incoming <- parsed
	pumpRenderer(160, latestFull.ID, false)
	if len(renderer.clientPrediction.PendingInputs()) != 0 || maxHistory > 35 {
		t.Fatalf("unbounded/unexpired history: max=%d pending=%d", maxHistory, len(renderer.clientPrediction.history))
	}
	if renderer.p != authority.players[1].p || renderer.stats.Bullets != authority.players[1].stats.Bullets {
		t.Fatal("final corrected player pose/ammo differs from authority")
	}
	if initial.x == renderer.p.x && initial.y == renderer.p.y {
		t.Fatal("faulted player never moved")
	}
	if world.attackTics != 1 || world.ammoSpent != 1 {
		t.Fatalf("duplicate/lost fire: attacktics=%d bullets=%d samples=%d", world.attackTics, world.ammoSpent, samples)
	}
	if dropped == 0 || duplicates == 0 || reordered == 0 {
		t.Fatalf("fault fixture did not exercise faults: drop=%d dup=%d reorder=%d", dropped, duplicates, reordered)
	}
	if client.Err() != nil && !errors.Is(client.Err(), net.ErrClosed) {
		t.Fatal(client.Err())
	}
	t.Logf("E1M1:160 exact server tics, %d dropped/%d duplicate/%d reordered packets, max pending history%d, one attack/one bullet, final pose=(%d,%d,%d) exactly reconciled", dropped, duplicates, reordered, maxHistory, renderer.p.x, renderer.p.y, renderer.p.z)
}
