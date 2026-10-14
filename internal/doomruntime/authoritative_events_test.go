package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/netgame"
)

func captureAuthoritySoundPlayback(g *game) *[]soundEvent {
	played := new([]soundEvent)
	g.snd = &soundSystem{vanillaVolume: 15, pitchShift: true, nativePlay: func(ev soundEvent, _ queuedSoundOrigin, _ int64, _ int64, _ uint32, _ bool, _ int) {
		*played = append(*played, ev)
	}}
	return played
}

func TestAuthorityEventsRecoverLossDeduplicateAndDoNotConsumeRNG(t *testing.T) {
	a, p := predictionTestWorld(t)
	played := captureAuthoritySoundPlayback(p.g)
	a.g.emitSoundEvent(soundEventShootPistol)
	first := predictionSnapshot(t, a, 2, netgame.InputAck{})
	rng, playRNG := doomrand.State()
	if _, err := p.Reconcile(first); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 1 || (*played)[0] != soundEventShootPistol {
		t.Fatal("new shot not played")
	}
	first.ID = 3
	if _, err := p.Reconcile(first); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 1 {
		t.Fatal("same event replayed from later full baseline")
	}
	a.g.emitSoundEvent(soundEventShootShotgun) // its first network snapshot is lost
	a.g.emitSoundEvent(soundEventShootRocket)
	if _, err := p.Reconcile(predictionSnapshot(t, a, 4, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 3 || (*played)[1] != soundEventShootShotgun || (*played)[2] != soundEventShootRocket {
		t.Fatal("retained event history did not recover missing sounds")
	}
	if after, afterPlay := doomrand.State(); after != rng || afterPlay != playRNG {
		t.Fatal("client sound replay consumed global RNG")
	}
	if len(p.g.soundQueue) != 0 || len(p.g.projectileImpacts) != 0 {
		t.Fatal("replicated sound produced duplicate queued/visual effects")
	}
}

func TestAuthorityEventsLateJoinBoundedBurstAndAgeExpiry(t *testing.T) {
	a, p := predictionTestWorld(t)
	a.g.emitSoundEvent(soundEventShootPistol)
	joined, err := newClientPrediction(p.g, netgame.Welcome{Epoch: 7, PlayerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	played := captureAuthoritySoundPlayback(joined.g)
	if _, err := joined.Reconcile(predictionSnapshot(t, a, 1, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 0 || joined.soundCursor == 0 {
		t.Fatal("late join replayed old sounds or failed to establish cursor")
	}
	for range authorityEventCapacity + 100 {
		a.g.emitSoundEvent(soundEventShootPistol)
	}
	if a.g.authorityEvents.count != authorityEventCapacity {
		t.Fatal("event ring grew beyond fixed capacity")
	}
	if _, err := joined.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != authorityEventPlaybackBudget {
		t.Fatalf("lag burst not capped: %d", len(*played))
	}
	a.g.emitSoundEvent(soundEventShootRocket)
	a.g.worldTic += authorityEventPlaybackTics + 1
	if _, err := joined.Reconcile(predictionSnapshot(t, a, 3, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != authorityEventPlaybackBudget || joined.soundCursor != a.g.authorityEvents.lastID {
		t.Fatal("expired sound played or cursor did not advance")
	}
	a.g.worldTic += authorityEventRetentionTics
	_, events := a.snapshotSoundEvents(1)
	if len(events) != 0 {
		t.Fatal("stale server event history retained in baselines")
	}
}

func TestAuthorityEventsAudienceOriginAndValidation(t *testing.T) {
	a, _, _ := snapshotFixture(t)
	a.g.authorityEvents = &authorityEventLog{}
	a.g.withAuthoritativePlayer(a.players[2], func() { a.g.emitSoundEvent(soundEventItemUp); a.g.emitSoundEvent(soundEventShootPistol) })
	_, mine := a.snapshotSoundEvents(2)
	_, other := a.snapshotSoundEvents(1)
	if len(mine) != 2 || mine[0].Audience != 2 || mine[1].LocalPlayer != 2 || mine[1].X != a.players[2].p.x {
		t.Fatal("player sound lost its active origin/recipient")
	}
	for _, event := range other {
		if event.Kind == soundEventItemUp {
			t.Fatal("private pickup sound leaked to another viewer")
		}
	}
	before := a.g.authorityEvents.lastID
	a.g.emitSoundEventAt(soundEventDoorOpen, 30000*fracUnit, 30000*fracUnit)
	cursor, events := a.snapshotSoundEvents(2)
	if cursor != before+1 || len(events) != len(mine) {
		t.Fatal("inaudible event was not filtered independently of global cursor")
	}
	valid, err := a.Snapshot(2)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*authorityReplica){
		func(r *authorityReplica) { r.Sounds[0].ID = 0 },
		func(r *authorityReplica) { r.Sounds[0].Tick = r.Tic + 1 },
		func(r *authorityReplica) { r.Sounds[0].Kind = soundEventMonsterRaise + 1 },
		func(r *authorityReplica) { r.Sounds[0].Audience = 1 },
		func(r *authorityReplica) { r.Sounds[0].X = 1 << 40 },
	} {
		r, err := decodeAuthorityReplica(valid)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&r)
		if err := validateAuthorityReplica(a.g, r); err == nil {
			t.Fatal("accepted malformed sound event")
		}
	}
}
