package doomruntime

import (
	"context"
	"math"
	"testing"
	"time"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/wad"
)

// These acceptance fixtures skip walking an entire level: players may be placed
// beside real map objects/switches, or in a clear firing lane with low health.
// Pickups, exit activation, firing, death, respawn and match completion still
// happen only through independently framed Client inputs consumed by Match.
// The deterministic transport deliberately excludes sockets; native TCP/WS/QUIC
// delivery and browser rendering have separate integration coverage.
type authorityAcceptancePeer struct {
	client    *netgame.Client
	transport *authorityFaultTransport
	handle    netgame.ConnectionID
	sequence  uint32
	ack       uint32
	replica   authorityReplica
}

type authorityAcceptanceSession struct {
	t     *testing.T
	a     *Authority
	match *netgame.Match
	peers [2]*authorityAcceptancePeer
}

func acceptanceRealMap(t *testing.T, name string) *mapdata.Map {
	t.Helper()
	w, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(w, mapdata.MapName(name))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func newAuthorityAcceptanceSession(t *testing.T, mapName, mode string, rules AuthorityRules) *authorityAcceptanceSession {
	t.Helper()
	a, err := NewAuthority(acceptanceRealMap(t, mapName), Options{SkillLevel: 3, NoMonsters: true, GameMode: mode})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetRules(rules); err != nil {
		t.Fatal(err)
	}
	key := mapName + "-acceptance"
	m, err := netgame.NewMatch(a, a, netgame.MatchConfig{Epoch: 31, Compatibility: key, PlayerLimit: 2, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 1})
	if err != nil {
		t.Fatal(err)
	}
	s := &authorityAcceptanceSession{t: t, a: a, match: m}
	for i := range s.peers {
		hello := netgame.Hello{Compatibility: key, Name: []string{"Alice", "Bob"}[i]}
		handle, welcome, err := m.Join(hello)
		if err != nil {
			t.Fatal(err)
		}
		transport := &authorityFaultTransport{incoming: make(chan any, 128), outgoing: make(chan any, 128), done: make(chan struct{})}
		transport.incoming <- welcome
		client, err := netgame.Connect(context.Background(), transport, hello)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		<-transport.outgoing // Match.Join authenticated the same Hello above.
		s.peers[i] = &authorityAcceptancePeer{client: client, transport: transport, handle: handle}
	}
	return s
}

func (s *authorityAcceptanceSession) deliver(p *authorityAcceptancePeer, message any) {
	s.t.Helper()
	frame, err := netgame.MarshalMessage(message)
	if err != nil {
		s.t.Fatal(err)
	}
	message, err = netgame.UnmarshalMessage(frame)
	if err != nil {
		s.t.Fatal(err)
	}
	p.transport.incoming <- message
}

func (s *authorityAcceptanceSession) step(commands ...demo.Tic) netgame.TickResult {
	s.t.Helper()
	for i, p := range s.peers {
		command := demo.Tic{}
		if i < len(commands) {
			command = commands[i]
		}
		p.sequence++
		batch := netgame.InputBatch{Epoch: s.match.Epoch(), SnapshotAck: p.ack, Inputs: []netgame.Input{{Sequence: p.sequence, Tick: s.a.Tic() + 1, Command: command}}}
		if err := p.client.SendInputs(batch); err != nil {
			s.t.Fatal(err)
		}
		written := false
		deadline := time.After(time.Second)
		for !written {
			select {
			case message := <-p.transport.outgoing:
				switch m := message.(type) {
				case netgame.InputBatch:
					if err := s.match.Submit(p.handle, m); err != nil {
						s.t.Fatal(err)
					}
					for _, in := range m.Inputs {
						written = written || in.Sequence == p.sequence
					}
				case netgame.Ping:
					s.deliver(p, netgame.Pong{Nonce: m.Nonce})
				default:
					s.t.Fatalf("unexpected client record %T", message)
				}
			case <-deadline:
				s.t.Fatal("client did not transmit scheduled command")
			}
		}
	}
	result, err := s.match.Step()
	if err != nil {
		s.t.Fatal(err)
	}
	for i, p := range s.peers {
		want, ok := result.Snapshots[p.handle]
		if !ok {
			s.t.Fatalf("player %d missing canonical snapshot", i+1)
		}
		s.deliver(p, want)
		deadline := time.Now().Add(time.Second)
		for {
			snapshot, ok, err := p.client.PollSnapshot()
			if err != nil {
				s.t.Fatal(err)
			}
			if ok {
				if snapshot.ID != want.ID || snapshot.Tick != result.Tick || snapshot.Finalized.Sequence != p.sequence {
					s.t.Fatalf("player %d snapshot not finalized for its own command: %+v", i+1, snapshot.Finalized)
				}
				p.replica, err = decodeAuthorityReplica(snapshot.State)
				if err != nil {
					s.t.Fatal(err)
				}
				if p.replica.Viewer != byte(i+1) || p.replica.Tic != result.Tick || len(p.replica.Players) != 2 {
					s.t.Fatalf("player %d received incorrect canonical viewer/world", i+1)
				}
				p.ack = snapshot.ID
				break
			}
			if time.Now().After(deadline) {
				s.t.Fatal("client did not receive canonical snapshot")
			}
			time.Sleep(time.Millisecond)
		}
	}
	return result
}

func (s *authorityAcceptanceSession) advanceMap(name string) {
	s.t.Helper()
	oldEpoch := s.match.Epoch()
	if err := s.a.AdvanceMap(acceptanceRealMap(s.t, name)); err != nil {
		s.t.Fatal(err)
	}
	key := name + "-acceptance"
	welcomes, err := s.match.ResetEpoch(oldEpoch+1, key, name)
	if err != nil {
		s.t.Fatal(err)
	}
	for i, p := range s.peers {
		change := netgame.MapChange{PreviousEpoch: oldEpoch, Welcome: welcomes[p.handle], Map: name, Compatibility: key}
		s.deliver(p, change)
		deadline := time.Now().Add(time.Second)
		for {
			got, ok, err := p.client.PollTransition()
			if err != nil {
				s.t.Fatal(err)
			}
			if ok {
				if got != change || p.client.Welcome().PlayerID != byte(i+1) {
					s.t.Fatalf("player %d did not retain its slot in the new epoch", i+1)
				}
				break
			}
			if time.Now().After(deadline) {
				s.t.Fatal("client did not receive map transition")
			}
			time.Sleep(time.Millisecond)
		}
		p.sequence, p.ack = 0, 0
	}
	s.step()
	for _, p := range s.peers {
		if string(p.replica.Map) != name || p.replica.Rules.Ended {
			s.t.Fatal("new map canonical baseline retained old completion")
		}
	}
}

func (s *authorityAcceptanceSession) placeFixturePlayer(id byte, x, y int64, angle uint32) {
	s.t.Helper()
	g := s.a.g
	g.withAuthoritativePlayer(s.a.players[id], func() {
		g.setPlayerPosFixed(x, y)
		floor, ceiling, ok := g.subsectorFloorCeilAt(x, y)
		if !ok {
			s.t.Fatal("fixture position has no map sector")
		}
		g.p.z, g.p.floorz, g.p.ceilz = floor, floor, ceiling
		g.p.momx, g.p.momy, g.p.momz = 0, 0, 0
		g.p.angle = angle
	})
}

func (s *authorityAcceptanceSession) placeAtRealExit(id byte) {
	s.t.Helper()
	g := s.a.g
	for _, line := range g.lines {
		if g.lineSpecial[line.idx] != 11 {
			continue
		}
		length := math.Hypot(float64(line.dx), float64(line.dy))
		x := (line.x1+line.x2)/2 + int64(float64(line.dy)*24*fracUnit/length)
		y := (line.y1+line.y2)/2 - int64(float64(line.dx)*24*fracUnit/length)
		s.placeFixturePlayer(id, x, y, vectorToAngle(-line.dy, line.dx))
		valid := false
		g.withAuthoritativePlayer(s.a.players[id], func() {
			index, trace := g.peekUseTargetLine()
			valid = index == line.idx && trace == useTraceSpecial && g.pointOnLineSide(x, y, line) == 0
		})
		if valid {
			return
		}
	}
	s.t.Fatal("real map has no reachable normal exit switch fixture")
}

func (s *authorityAcceptanceSession) placeAtRealPickup(id byte, typ int16) int {
	s.t.Helper()
	for i, thing := range s.a.g.m.Things {
		if thing.Type == typ && !s.a.g.thingCollected[i] {
			s.placeFixturePlayer(id, int64(thing.X)*fracUnit, int64(thing.Y)*fracUnit, 0)
			return i
		}
	}
	s.t.Fatalf("real map %s has no uncollected pickup type %d", s.a.MapName(), typ)
	return -1
}

func TestAuthoritativeTwoClientsCoopRealExitAndMapCarryover(t *testing.T) {
	s := newAuthorityAcceptanceSession(t, "E1M2", gameModeCoop, AuthorityRules{RespawnDelayTics: 2})
	// Fixture: earlier level progress leaves the two players different health and
	// ammo totals. Each collects a real resource, then player 1 collects a key.
	for id := byte(1); id <= 2; id++ {
		s.a.g.withAuthoritativePlayer(s.a.players[id], func() {
			s.a.g.stats.Health, s.a.g.playerMobjHealth = 60+int(id), 60+int(id)
			s.a.g.stats.Bullets = 20 + int(id)
		})
	}
	ammoIndex := s.placeAtRealPickup(1, 2007)
	healthIndex := s.placeAtRealPickup(2, 2011)
	s.step(demo.Tic{Forward: 1}, demo.Tic{Side: 1})
	if !s.a.g.thingCollected[ammoIndex] || !s.a.g.thingCollected[healthIndex] || s.a.players[1].stats.Bullets <= 21 || s.a.players[2].stats.Health <= 62 {
		t.Fatal("independent movement did not collect real ammo and health")
	}
	keyIndex := s.placeAtRealPickup(1, 13)
	s.step(demo.Tic{Forward: 1}, demo.Tic{AngleTurn: 256})
	for _, p := range s.peers {
		for _, player := range p.replica.Players {
			if !player.Inventory.RedKey {
				t.Fatal("real key pickup was not shared in both clients' baselines")
			}
		}
	}
	if !s.a.g.thingCollected[keyIndex] {
		t.Fatal("scheduled movement did not collect the real map key")
	}
	// Player 2 independently uses the map's real exit switch. Player 1 remains
	// connected and idle; neither client can submit an end-of-level state.
	s.placeAtRealExit(2)
	result := s.step(demo.Tic{}, demo.Tic{Buttons: demo.ButtonUse})
	if !result.Completed || s.a.MatchState().Reason != "map_exit" || !s.a.g.levelExitRequested {
		t.Fatalf("real switch use did not complete the co-op map: %+v", s.a.MatchState())
	}
	for _, p := range s.peers {
		if !p.replica.Rules.Ended || p.replica.Rules.EndReason != "map_exit" || !p.replica.Rules.TeamKeys.Red {
			t.Fatal("clients did not receive completed co-op state with shared key")
		}
	}
	wantStats := [3]playerStats{}
	for id := 1; id <= 2; id++ {
		wantStats[id] = s.a.players[id].stats
	}
	s.advanceMap("E1M3")
	for _, p := range s.peers {
		for _, player := range p.replica.Players {
			id := player.LocalSlot
			if player.Stats.Health != wantStats[id].Health || player.Stats.Bullets != wantStats[id].Bullets || player.Inventory.RedKey || p.replica.Rules.Scores[id].Generation != 2 {
				t.Fatalf("co-op map carryover/reset incorrect for player %d: %+v", id, player.Stats)
			}
		}
	}
}

func (s *authorityAcceptanceSession) placeFiringLane() {
	s.t.Helper()
	// Fixture uses the real co-op starts' room as a short, unobstructed lane.
	// Only positioning and target health are arranged; firing/damage/score are real.
	var x, y int64
	for _, thing := range s.a.g.m.Things {
		if thing.Type == 1 {
			x, y = int64(thing.X)*fracUnit, int64(thing.Y)*fracUnit
			break
		}
	}
	for _, delta := range [][2]int64{{64 * fracUnit, 0}, {-64 * fracUnit, 0}, {0, 64 * fracUnit}, {0, -64 * fracUnit}} {
		s.placeFixturePlayer(1, x, y, vectorToAngle(delta[0], delta[1]))
		s.placeFixturePlayer(2, x+delta[0], y+delta[1], vectorToAngle(-delta[0], -delta[1]))
		valid := false
		s.a.g.withAuthoritativePlayer(s.a.players[1], func() {
			_, _, _, fits := s.a.g.checkPositionForWithPickupTouch(s.a.g.p.x, s.a.g.p.y, false, false)
			_, target, ok := s.a.g.aimLineAttackTarget(s.a.g.playerLineAttackActor(), s.a.g.p.angle, 1024*fracUnit)
			valid = fits && ok && target.kind == lineAttackTargetPlayer && target.playerSlot == 2
		})
		s.a.g.withAuthoritativePlayer(s.a.players[2], func() {
			_, _, _, fits := s.a.g.checkPositionForWithPickupTouch(s.a.g.p.x, s.a.g.p.y, false, false)
			valid = valid && fits
		})
		if valid {
			s.a.g.withAuthoritativePlayer(s.a.players[2], func() {
				s.a.g.stats.Health, s.a.g.playerMobjHealth, s.a.g.stats.Armor = 1, 1, 0
			})
			return
		}
	}
	s.t.Fatal("real E1M1 start room has no clear two-player firing lane")
}

func (s *authorityAcceptanceSession) fireOnce() netgame.TickResult {
	s.t.Helper()
	before := s.a.players[1].stats.Bullets
	result := s.step(demo.Tic{Buttons: demo.ButtonAttack}, demo.Tic{})
	for i := 0; i < 35 && !s.a.players[2].isDead; i++ {
		result = s.step()
	}
	if !s.a.players[2].isDead || s.a.players[1].stats.Bullets != before-1 {
		s.t.Fatalf("scheduled pistol shot did not cause exactly one shot/death: dead=%v bullets=%d -> %d", s.a.players[2].isDead, before, s.a.players[1].stats.Bullets)
	}
	return result
}

func TestAuthoritativeTwoClientsDeathmatchCombatRespawnFragRotation(t *testing.T) {
	s := newAuthorityAcceptanceSession(t, "E1M1", gameModeDeathmatch, AuthorityRules{FragLimit: 2, RespawnDelayTics: 2})
	// Let both actual weapon state machines raise their starting pistols.
	for i := 0; i < 20; i++ {
		s.step()
	}
	s.placeFiringLane()
	if result := s.fireOnce(); result.Completed {
		t.Fatal("first frag prematurely completed the two-frag round")
	}
	for _, p := range s.peers {
		if p.replica.Rules.Scores[1].Frags != 1 || p.replica.Rules.Scores[2].Deaths != 1 || !p.replica.Players[1].IsDead {
			t.Fatal("both clients did not observe the actual first kill and attribution")
		}
	}
	deathTic, world := s.a.Tic(), s.a.g
	for i := 0; i < 35 && s.a.players[2].isDead; i++ {
		s.step(demo.Tic{}, demo.Tic{Buttons: demo.ButtonUse})
	}
	if s.a.players[2].isDead || s.a.players[2].stats.Health != 100 || s.a.players[2].stats.Bullets != 50 || s.a.g != world || s.a.Tic() <= deathTic || s.a.g.authorityRules.Scores[2].Generation != 2 {
		t.Fatal("player 2's scheduled use did not respawn independently in the running world")
	}
	for i := 0; i < 20; i++ {
		s.step()
	}
	s.placeFiringLane()
	result := s.fireOnce()
	if !result.Completed || s.a.MatchState().Reason != "frag_limit" || s.a.MatchState().WinnerID != 1 {
		t.Fatalf("second actual pistol kill did not reach frag limit: %+v", s.a.MatchState())
	}
	for _, p := range s.peers {
		if p.replica.Rules.Scores[1].Frags != 2 || p.replica.Rules.Scores[2].Deaths != 2 || p.replica.Rules.EndReason != "frag_limit" {
			t.Fatal("clients did not agree on the completed deathmatch score")
		}
	}
	s.advanceMap("E1M2")
	for _, p := range s.peers {
		for _, player := range p.replica.Players {
			score := p.replica.Rules.Scores[player.LocalSlot]
			if score.Frags != 0 || score.Deaths != 0 || player.IsDead || player.Stats.Health != 100 || player.Stats.Bullets != 50 {
				t.Fatal("new deathmatch round did not reset both clients' score/loadout")
			}
		}
	}
}
