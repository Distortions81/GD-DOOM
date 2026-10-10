package doomruntime

import "math"

const (
	authorityEventCapacity       = 256
	authorityEventRetentionTics  = 70
	authorityEventPlaybackTics   = 7
	authorityEventPlaybackBudget = 32
)

// IDs are monotonic within one map/network epoch. The snapshot envelope owns
// the epoch; a replacement world/predictor starts a fresh ring and cursor.
// Visual impacts already belong to the baseline and are not replayed here.
type authoritySoundEvent struct {
	ID          uint64
	Tick        uint32
	Kind        soundEvent
	X, Y        int64
	LocalPlayer byte // unpositioned player sound; other listeners hear its origin
	Audience    byte // zero is spatially audible to anyone; otherwise private feedback
}
type authorityEventLog struct {
	lastID       uint64
	start, count int
	events       [authorityEventCapacity]authoritySoundEvent
}

func (g *game) recordAuthoritySound(kind soundEvent, x, y int64, positioned bool) {
	log := g.authorityEvents
	if log == nil || log.lastID == math.MaxUint64 {
		return
	}
	event := authoritySoundEvent{Tick: uint32(g.worldTic), Kind: kind, X: x, Y: y}
	if !positioned {
		event.X, event.Y = g.p.x, g.p.y
		if g.localSlot >= 1 && g.localSlot <= 4 {
			event.LocalPlayer = byte(g.localSlot)
		}
		switch kind {
		case soundEventItemUp, soundEventWeaponUp, soundEventPowerUp, soundEventNoWay, soundEventTink:
			event.Audience = event.LocalPlayer
		}
	}
	log.lastID++
	event.ID = log.lastID
	if log.count == len(log.events) {
		log.start = (log.start + 1) % len(log.events)
		log.count--
	}
	log.events[(log.start+log.count)%len(log.events)] = event
	log.count++
}

func (a *Authority) snapshotSoundEvents(viewer byte) (uint64, []authoritySoundEvent) {
	log := a.g.authorityEvents
	if log == nil {
		return 0, nil
	}
	var events []authoritySoundEvent
	p := a.players[viewer].p
	for i := 0; i < log.count; i++ {
		event := log.events[(log.start+i)%len(log.events)]
		if a.Tic()-event.Tick > authorityEventRetentionTics || event.Audience != 0 && event.Audience != viewer {
			continue
		}
		if event.LocalPlayer != viewer && !soundMapUsesFullClip(a.g.m.Name) {
			dx, dy := abs(event.X-p.x), abs(event.Y-p.y)
			if dx+dy-min(dx, dy)/2 > doomSoundClippingDist {
				continue
			}
		}
		events = append(events, event)
	}
	return log.lastID, events
}

func (p *ClientPrediction) applyAuthoritySounds(r authorityReplica, hadBaseline bool) {
	previous := p.soundCursor
	p.soundCursor = r.SoundCursor
	if !hadBaseline {
		return
	} // late joins establish a cursor without an old burst
	var pending []authoritySoundEvent
	for _, event := range r.Sounds {
		if event.ID <= previous || r.Tic-event.Tick > authorityEventPlaybackTics {
			continue
		}
		pending = append(pending, event)
	}
	if len(pending) > authorityEventPlaybackBudget {
		pending = pending[len(pending)-authorityEventPlaybackBudget:]
	}
	for _, event := range pending {
		p.g.playAuthoritySound(event, p.epoch, p.viewer)
	}
}

func (g *game) playAuthoritySound(event authoritySoundEvent, epoch uint64, viewer byte) {
	if g.snd == nil {
		return
	}
	origin := queuedSoundOrigin{x: event.X, y: event.Y, positioned: event.LocalPlayer != viewer}
	pitch := 128
	// Cosmetic variation derives from immutable event identity, never either
	// global Doom RNG stream. Reconciliation must not consume simulation RNG.
	if g.snd.pitchShift {
		value := event.ID*0x9e3779b97f4a7c15 ^ epoch
		switch vanillaPitchModeForEvent(event.Kind) {
		case vanillaPitchSaw:
			pitch += 8 - int(value&15)
		case vanillaPitchDefault:
			pitch += 16 - int(value&31)
		}
	}
	g.snd.playEventSpatialPitch(event.Kind, origin, g.p.x, g.p.y, g.p.angle, soundMapUsesFullClip(g.m.Name), pitch)
}
