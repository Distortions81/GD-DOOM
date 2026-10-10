package doomruntime

import (
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/netgame"
	"gddoom/internal/platformcfg"
)

type authoritySoundPlayback struct {
	kind   soundEvent
	origin queuedSoundOrigin
	pitch  int
}

func captureAuthoritySoundBatch(g *game) *[]authoritySoundPlayback {
	played := new([]authoritySoundPlayback)
	g.snd = &soundSystem{vanillaVolume: 15, pitchShift: true, nativePlay: func(ev soundEvent, origin queuedSoundOrigin, _ int64, _ int64, _ uint32, _ bool, pitch int) {
		*played = append(*played, authoritySoundPlayback{ev, origin, pitch})
	}}
	return played
}

func TestAuthorityWASMSoundBurstKeepsLoudestIdentityAndExpiresDroppedHistory(t *testing.T) {
	previous := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(previous)
	a, p := predictionTestWorld(t)
	played := captureAuthoritySoundBatch(p.g)
	// These older distinct sounds are outside the latest-32 recovery window.
	a.g.emitSoundEvent(soundEventSwitchOn)
	for range 40 {
		a.g.emitSoundEventAt(soundEventShootPistol, a.g.p.x+600*fracUnit, a.g.p.y)
	}
	// A closer local shot must replace the quieter duplicate while retaining
	// this event's immutable pitch, not the first occurrence's identity.
	a.g.emitSoundEvent(soundEventShootPistol)
	lastID := a.g.authorityEvents.lastID
	rng, playRNG := doomrand.State()
	if _, err := p.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	expectedPitch := 128 + 16 - int((lastID*0x9e3779b97f4a7c15^p.epoch)&31)
	if len(*played) != 1 || (*played)[0].kind != soundEventShootPistol || (*played)[0].origin.positioned || (*played)[0].pitch != expectedPitch {
		t.Fatalf("burst playback=%+v; want one local shot with event %d pitch %d", *played, lastID, expectedPitch)
	}
	if p.soundCursor != lastID {
		t.Fatalf("cursor=%d want %d", p.soundCursor, lastID)
	}
	if _, err := p.Reconcile(predictionSnapshot(t, a, 3, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 1 {
		t.Fatal("coalesced or discarded sound replayed from retained history")
	}
	a.g.emitSoundEvent(soundEventShootShotgun)
	if _, err := p.Reconcile(predictionSnapshot(t, a, 4, netgame.InputAck{})); err != nil {
		t.Fatal(err)
	}
	if len(*played) != 2 || (*played)[1].kind != soundEventShootShotgun {
		t.Fatalf("new event was lost after coalescing: %+v", *played)
	}
	if after, afterPlay := doomrand.State(); after != rng || afterPlay != playRNG {
		t.Fatal("browser sound selection consumed gameplay RNG")
	}
}

func TestAuthorityWASMSoundBurstBudgetsAndWeaponPriority(t *testing.T) {
	previous := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(previous)
	for _, tc := range []struct {
		name              string
		nonvocal          []soundEvent
		wantTotal, vocals int
	}{
		{"vocal budget", []soundEvent{soundEventShootPistol}, 7, 6},
		{"total budget", []soundEvent{soundEventShootPistol, soundEventShootShotgun, soundEventSwitchOn, soundEventItemUp}, 8, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, p := predictionTestWorld(t)
			played := captureAuthoritySoundBatch(p.g)
			for _, kind := range []soundEvent{soundEventMonsterSeePosit1, soundEventMonsterSeePosit2, soundEventMonsterSeePosit3, soundEventMonsterSeeImp1, soundEventMonsterSeeImp2, soundEventMonsterSeeDemon, soundEventMonsterSeeCaco, soundEventMonsterSeeBaron, soundEventMonsterSeeKnight} {
				a.g.emitSoundEvent(kind)
			}
			for _, kind := range tc.nonvocal {
				a.g.emitSoundEvent(kind)
			}
			if _, err := p.Reconcile(predictionSnapshot(t, a, 2, netgame.InputAck{})); err != nil {
				t.Fatal(err)
			}
			if len(*played) != tc.wantTotal {
				t.Fatalf("played %d sounds want %d: %+v", len(*played), tc.wantTotal, *played)
			}
			vocals := 0
			for i, event := range *played {
				if i < len(tc.nonvocal) && event.kind != tc.nonvocal[i] {
					t.Fatalf("weapon/pickup/mover priority lost at %d: %+v", i, *played)
				}
				if isMonsterVocalSound(event.kind) {
					vocals++
				}
			}
			if vocals != tc.vocals {
				t.Fatalf("played %d vocals want %d", vocals, tc.vocals)
			}
			if _, err := p.Reconcile(predictionSnapshot(t, a, 3, netgame.InputAck{})); err != nil {
				t.Fatal(err)
			}
			if len(*played) != tc.wantTotal {
				t.Fatal("budgeted-out history was replayed on a later snapshot")
			}
		})
	}
}
