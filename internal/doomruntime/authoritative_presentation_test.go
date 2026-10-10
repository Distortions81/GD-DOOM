package doomruntime

import (
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
	g.prepareRenderStateAt(now.Add(g.authorityRender.duration / 2))
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
