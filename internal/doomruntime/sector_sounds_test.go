package doomruntime

import (
	"gddoom/internal/netgame"
	"math"
	"testing"
)

func TestSectorSoundOriginIdentityAndAuthority(t *testing.T) {
	a, p := predictionTestWorld(t)
	g := a.g
	var received []queuedSoundOrigin
	p.g.snd = &soundSystem{vanillaVolume: 15, nativePlay: func(_ soundEvent, o queuedSoundOrigin, _ int64, _ int64, _ uint32, _ bool, _ int) {
		received = append(received, o)
	}}
	g.emitSectorSound(0, soundEventSectorMove)
	g.emitSectorSound(0, soundEventPlatStart)
	if _, err := p.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].sectorPlusOne != 1 || received[1].sectorPlusOne != 1 {
		t.Fatalf("replication lost sector identities: %+v", received)
	}
	// Two sector identities with coincident origins must have separate channels.
	received[1].sectorPlusOne = 2
	if soundOriginGroup(soundEventSectorMove, received[0]) == soundOriginGroup(soundEventPlatStart, received[1]) {
		t.Fatal("coincident sectors share a channel")
	}
	if soundOriginGroup(soundEventSectorMove, received[0]) != soundOriginGroup(soundEventPlatStop, received[0]) {
		t.Fatal("stop sound does not replace movement channel")
	}
	g.authorityEvents.lastID = math.MaxUint64
	previous := g.authorityEvents.events[(g.authorityEvents.start+g.authorityEvents.count-1)%authorityEventCapacity]
	g.emitSectorSound(0, soundEventPlatStop)
	if g.authorityEvents.events[(g.authorityEvents.start+g.authorityEvents.count-1)%authorityEventCapacity] != previous {
		t.Fatal("exhausted event IDs mutated an earlier origin")
	}
}

func TestSectorSoundBankAndLoopKinds(t *testing.T) {
	s := soundSystem{bank: SoundBank{SectorMove: PCMSample{Data: []byte{1}}, PlatStart: PCMSample{Data: []byte{2}}, PlatStop: PCMSample{Data: []byte{3}}}}
	for _, c := range []struct {
		ev      soundEvent
		lump    string
		value   byte
		timeout int
	}{{soundEventSectorMove, "DSSTNMOV", 1, 8}, {soundEventSectorMoveStart, "DSSTNMOV", 1, 0}, {soundEventPlatStart, "DSPSTART", 2, 0}, {soundEventPlatStop, "DSPSTOP", 3, 0}} {
		name, ok := soundEventDSName(c.ev)
		sample, found := s.sampleForEvent(c.ev)
		if !ok || !found || name != c.lump || len(sample.Data) != 1 || sample.Data[0] != c.value || soundEventLoopTics(c.ev) != c.timeout {
			t.Fatalf("bad sector sound mapping: %+v", c)
		}
	}
}
