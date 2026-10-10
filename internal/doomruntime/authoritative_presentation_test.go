package doomruntime

import (
	"math"
	"testing"
	"time"

	"gddoom/internal/doomrand"
	"gddoom/internal/netgame"
)

func TestAuthorityPresentationInterpolatesWithoutChangingCollision(t *testing.T) {
	a, g, data := snapshotFixture(t)
	p, err := newClientPrediction(g, netgame.Welcome{Epoch: 7, PlayerID: 2})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	if _, err := p.reconcileAt(netgame.Snapshot{Epoch: 7, ID: 1, Tick: 10, State: data}, now); err != nil {
		t.Fatal(err)
	}
	oldPlayer := a.players[1].p
	thingIndex := -1
	for i, thing := range a.g.m.Things {
		if thing.Type == barrelThingType {
			thingIndex = i
			break
		}
	}
	if thingIndex < 0 {
		t.Fatal("map fixture has no barrel")
	}
	oldThingX, oldThingY := a.g.thingPosFixed(thingIndex, a.g.m.Things[thingIndex])
	a.players[1].p.x += 20 * fracUnit
	a.g.setThingPosFixed(thingIndex, oldThingX+20*fracUnit, oldThingY)
	a.g.projectiles[0].x += 60 * fracUnit
	a.g.worldTic = 13
	next, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second * 3 / 35)
	if _, err := p.reconcileAt(netgame.Snapshot{Epoch: 7, ID: 2, Tick: 13, State: next}, now); err != nil {
		t.Fatal(err)
	}
	rng, play := doomrand.State()
	g.prepareRenderStateAt(now.Add(time.Second * 3 / 35 / 2))
	pose := g.authorityRemoteRenderPose(1, g.remotePlayers[1].p)
	if abs(pose.x-(oldPlayer.x+10*fracUnit)) > 1 {
		t.Fatalf("remote interpolation: %d", pose.x-oldPlayer.x)
	}
	x, _, _ := g.thingRenderPosFixed(thingIndex, g.m.Things[thingIndex], 1)
	if abs(x-(oldThingX+10*fracUnit)) > 1 {
		t.Fatal("world thing did not interpolate independently of local tic alpha")
	}
	x, _, _ = g.projectileRenderPosFixed(g.projectiles[0], 1)
	if abs(x-30*fracUnit) > 1 {
		t.Fatal("projectile did not interpolate by stable thinker identity")
	}
	if g.remotePlayers[1].p != a.players[1].p || g.thingX[thingIndex] != oldThingX+20*fracUnit || g.projectiles[0].x != 60*fracUnit || g.worldTic != 13 {
		t.Fatal("presentation interpolation changed canonical collision/world state")
	}
	if raw := g.captureAuthorityRenderFrame(now.Add(time.Second * 3 / 35 / 2)); raw.players[1].pose.x != a.players[1].p.x {
		t.Fatal("next snapshot would rebase from an interpolated position")
	}
	if after, afterPlay := doomrand.State(); after != rng || afterPlay != play {
		t.Fatal("render interpolation consumed gameplay/effects RNG")
	}
	g.prepareRenderStateAt(now.Add(10 * time.Second))
	if pose := g.authorityRemoteRenderPose(1, g.remotePlayers[1].p); pose.x != a.players[1].p.x {
		t.Fatal("stalled stream extrapolated remote player")
	}
	// A sparse baseline can report a teleport after its one-tic flag cleared.
	a.g.authorityRules.Scores[1].MovementEpoch++
	a.players[1].p.x += 20 * fracUnit
	a.g.worldTic++
	next, err = a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.reconcileAt(netgame.Snapshot{Epoch: 7, ID: 3, Tick: 14, State: next}, now.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if pose := g.authorityRemoteRenderPose(1, g.remotePlayers[1].p); pose.x != a.players[1].p.x {
		t.Fatal("teleport blended across discontinuity")
	}
	// The same rule holds when a slot is reused by another incarnation.
	g.authorityRules.Scores[1].Generation++
	if pose := g.authorityRemoteRenderPose(1, g.remotePlayers[1].p); pose.x != g.remotePlayers[1].p.x {
		t.Fatal("new incarnation blended with prior body")
	}
}

func authorityTimelineTestFrame(tic int) authorityRenderFrame {
	pose := authorityRenderPose{x: int64(tic) * 10 * fracUnit, y: int64(tic) * 5 * fracUnit, angle: uint32(tic) * 0x01000000}
	return authorityRenderFrame{
		tic:         tic,
		players:     map[int]authorityRenderPlayer{1: {pose: pose, generation: 1}},
		things:      []authorityRenderThing{{pose: pose, kind: barrelThingType}},
		projectiles: map[int64]authorityRenderPose{1: pose},
	}
}

func TestAuthorityPresentationUnevenArrivalKeepsConstantVelocity(t *testing.T) {
	start := time.Unix(100, 0)
	stamp := func(tic float64) time.Time { return start.Add(time.Duration(tic * float64(time.Second) / 35)) }
	s := newAuthorityRenderTimeline(authorityTimelineTestFrame(0), authorityTimelineTestFrame(2), stamp(2))
	g := &game{authorityRender: s, authorityRules: &authorityRulesState{}}
	g.authorityRules.Scores[1].Generation = 1
	arrivals := []struct {
		tic int
		at  float64
	}{{4, 3.8}, {6, 6.2}, {8, 7.7}, {10, 10.3}, {12, 11.8}, {14, 13.9}}
	latest, next := 2, 0
	for quarter := 8; quarter <= 56; quarter++ {
		at := float64(quarter) / 4
		for next < len(arrivals) && arrivals[next].at <= at {
			arrival := arrivals[next]
			s.appendFrame(authorityTimelineTestFrame(arrival.tic), stamp(arrival.at))
			latest = arrival.tic
			next++
		}
		s.prepare(stamp(at))
		current := authorityTimelineTestFrame(latest).players[1].pose
		body := player{x: current.x, y: current.y, angle: current.angle}
		poses := []authorityRenderPose{
			g.authorityRemoteRenderPose(1, body),
			g.authorityThingRenderPose(0, barrelThingType, current),
			g.authorityProjectileRenderPose(projectile{order: 1, x: current.x, y: current.y}),
		}
		wantTic := math.Max(0, stamp(at).Sub(stamp(3)).Seconds()*35)
		for kind, pose := range poses {
			if math.Abs(float64(pose.x)-wantTic*10*fracUnit) > 1 || math.Abs(float64(pose.y)-wantTic*5*fracUnit) > 1 {
				t.Fatalf("at %.2f ticks actor %d changed velocity: pose=%+v want tick %.9f", at, kind, pose, wantTic)
			}
		}
		if delta := int32(poses[0].angle - uint32(wantTic*0x01000000)); delta < -1 || delta > 1 {
			t.Fatalf("at %.2f ticks yaw changed velocity: %x", at, poses[0].angle)
		}
	}
}

func TestAuthorityPresentationBoundsRecoveryAndNeverExtrapolates(t *testing.T) {
	now := time.Unix(100, 0)
	s := &authorityRenderState{frames: []authorityRenderFrame{authorityTimelineTestFrame(0)}, stamp: now}
	for tic := 1; tic <= 100; tic++ {
		s.appendFrame(authorityTimelineTestFrame(tic), now)
		if len(s.frames) > authorityRenderMaxFrames || float64(tic)-s.tic > authorityRenderMaxDelayTics {
			t.Fatal("snapshot burst grew an unbounded interpolation backlog")
		}
	}
	s.prepare(now.Add(time.Second))
	if s.tic != 100 || s.alpha != 1 {
		t.Fatalf("stalled stream extrapolated: tick=%v alpha=%v", s.tic, s.alpha)
	}
	s.prepare(now)
	if s.tic != 100 {
		t.Fatal("render cursor moved backwards with clock")
	}
}

func TestAuthorityPresentationPlayerCutCannotRewindWhenMotionResumes(t *testing.T) {
	for _, identity := range []string{"teleport", "generation"} {
		t.Run(identity, func(t *testing.T) {
			start := time.Unix(100, 0)
			stamp := func(tic float64) time.Time { return start.Add(time.Duration(tic * float64(time.Second) / 35)) }
			s := newAuthorityRenderTimeline(authorityTimelineTestFrame(0), authorityTimelineTestFrame(2), stamp(2))
			g := &game{authorityRender: s, authorityRules: &authorityRulesState{}}
			g.authorityRules.Scores[1].Generation = 1
			if identity == "teleport" {
				g.authorityRules.Scores[1].MovementEpoch = 1
			} else {
				g.authorityRules.Scores[1].Generation = 2
			}
			var confirmed player
			lastX := int64(100 * fracUnit)
			for quarter := 16; quarter <= 44; quarter++ {
				at := float64(quarter) / 4
				if quarter%8 == 0 {
					frame := authorityTimelineTestFrame(int(at))
					entry := frame.players[1]
					entry.pose.x = int64(100+10*(at-4)) * fracUnit
					entry.generation = g.authorityRules.Scores[1].Generation
					entry.movement = g.authorityRules.Scores[1].MovementEpoch
					frame.players[1] = entry
					confirmed = player{x: entry.pose.x, y: entry.pose.y, angle: entry.pose.angle}
					s.appendFrame(frame, stamp(at))
				}
				s.prepare(stamp(at))
				pose := g.authorityRemoteRenderPose(1, confirmed)
				if pose.x < lastX || pose.x > confirmed.x {
					t.Fatalf("at %.2f ticks cut rewound/extrapolated: previous=%d rendered=%d confirmed=%d", at, lastX, pose.x, confirmed.x)
				}
				want := float64(100*fracUnit) + math.Max(0, at-7)*10*fracUnit
				if math.Abs(float64(pose.x)-want) > 1 {
					t.Fatalf("at %.2f ticks cut did not join delayed motion smoothly: got %d want %.1f", at, pose.x, want)
				}
				lastX = pose.x
			}
		})
	}
}

func TestAuthorityPresentationNewActorsCannotRewindIntoBirth(t *testing.T) {
	start := time.Unix(100, 0)
	stamp := func(tic float64) time.Time { return start.Add(time.Duration(tic * float64(time.Second) / 35)) }
	frame := func(tic int) authorityRenderFrame {
		pose := authorityRenderPose{x: int64(100+(tic-2)*10) * fracUnit}
		return authorityRenderFrame{tic: tic, projectiles: map[int64]authorityRenderPose{9: pose}, things: []authorityRenderThing{{pose: pose, kind: 3004}, {pose: pose, kind: 3004}}}
	}
	// Slot zero changes type; slot one is appended. Neither may borrow the
	// previous actor's pose or display a future position before rewinding.
	before := authorityRenderFrame{tic: 0, things: []authorityRenderThing{{kind: barrelThingType}}}
	s := newAuthorityRenderTimeline(before, frame(2), stamp(2))
	g := &game{authorityRender: s}
	confirmed := frame(2).projectiles[9]
	lastX := int64(100 * fracUnit)
	for quarter := 8; quarter <= 32; quarter++ {
		at := float64(quarter) / 4
		if quarter > 8 && quarter%8 == 0 {
			next := frame(int(at))
			confirmed = next.projectiles[9]
			s.appendFrame(next, stamp(at))
		}
		s.prepare(stamp(at))
		poses := []authorityRenderPose{
			g.authorityProjectileRenderPose(projectile{order: 9, x: confirmed.x}),
			g.authorityThingRenderPose(0, 3004, confirmed),
			g.authorityThingRenderPose(1, 3004, confirmed),
		}
		want := float64(100*fracUnit) + math.Max(0, at-5)*10*fracUnit
		for kind, pose := range poses {
			if pose.x < lastX || pose.x > confirmed.x || math.Abs(float64(pose.x)-want) > 1 {
				t.Fatalf("at %.2f ticks actor %d rewound after birth: got %d want %.1f", at, kind, pose.x, want)
			}
		}
		lastX = poses[0].x
	}
}

func TestAuthorityRemotePlayersRenderMirroredAndCorpseSprites(t *testing.T) {
	pixels := make([]byte, 32*56*4)
	for i := range pixels {
		pixels[i] = 255
	}
	texture := WallTexture{Width: 32, Height: 56, OffsetX: 16, OffsetY: 56, RGBA: pixels}
	g := newGame(predictionTestMap(), Options{Headless: true, SkillLevel: 3, SpritePatchBank: map[string]WallTexture{"PLAYA2A8": texture, "PLAYN0": texture}})
	g.viewW, g.viewH = 64, 64
	g.wallPix32 = make([]uint32, 64*64)
	state := g.captureAuthoritativePlayer()
	state.localSlot = 2
	state.p.x, state.p.y, state.p.angle = 64*fracUnit, 0, 0xa0000000 // viewer sees rotation 8
	g.authorityPlayers = []*authoritativePlayerState{&state}
	g.remotePlayers = map[int]*remotePlayer{2: {slot: 2, p: state.p}}
	g.appendRemotePlayerCutoutItems(0, 0, 0, 32, 32, 1)
	if len(g.billboardQueueScratch) != 1 || !g.billboardQueueScratch[0].flip {
		t.Fatal("paired mirrored remote sprite was invisible or unflipped")
	}
	state.isDead = true
	g.billboardQueueScratch = nil
	g.appendRemotePlayerCutoutItems(0, 0, 0, 32, 32, 1)
	if len(g.billboardQueueScratch) != 1 || g.billboardQueueScratch[0].flip {
		t.Fatal("remote corpse did not use nonrotating death sprite")
	}
	state.isDead = false
	for _, test := range []struct {
		state int
		frame byte
	}{{doomStatePlayerAttack1, 'E'}, {doomStatePlayerAttack2, 'F'}, {doomStatePlayerPain1, 'G'}, {doomStatePlayerPain2, 'G'}} {
		state.playerMobjState = test.state
		if frame, ok := g.authoritativePlayerFrame(2); !ok || frame != test.frame {
			t.Fatalf("player pose %d = %c", test.state, frame)
		}
	}
}
