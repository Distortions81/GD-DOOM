package doomruntime

import (
	"bytes"
	"reflect"
	"testing"

	"gddoom/internal/doomrand"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestAuthoritySnapshotViewerOverlayMatchesActivatedSave(t *testing.T) {
	a := testAuthority(t)
	if err := a.AddPlayer(2); err != nil {
		t.Fatal(err)
	}
	p := *a.players[2]
	p.useFlash, p.useText = 17, "viewer two"
	p.prevPX, p.prevPY, p.prevAngle, p.playerViewZ = 11, 22, 33, 44
	p.cheatLevel, p.invulnerable, p.noClip = 2, true, true
	p.inventory.BlueKey, p.inventory.LightAmpTics = true, 91
	p.lastAttackRange = 55
	p.alwaysRun, p.autoWeaponSwitch = true, false
	p.weaponRefire, p.weaponAttackDown = true, false
	p.weaponState, p.weaponStateTics = weaponPspriteState(12), 13
	p.weaponFlashState, p.weaponFlashTics, p.weaponPSpriteY = weaponPspriteState(14), 15, 16
	p.stats = playerStats{Health: 71, Armor: 22, ArmorType: 1, Bullets: 8, Shells: 9, Rockets: 10, Cells: 11}
	p.playerKillCount, p.playerItemCount, p.playerBlockOrder = 3, 4, 99
	p.secretsFound, p.isDead, p.playerMobjHealth = 6, true, -5
	p.damageFlashTic, p.bonusFlashTic = 7, 8
	got := captureGameSaveState(a.g)
	applyReplicaViewerToSave(&got, captureReplicaPlayer(p))
	// Independent reference: the original snapshot activated a player before
	// capturing the entire save. This catches missing or incorrectly mapped
	// fields without using the optimized overlay to construct expectations.
	a.g.applyAuthoritativePlayer(p)
	want := captureGameSaveState(a.g)
	want.Session.PlayerSlot = 2
	if !reflect.DeepEqual(got, want) {
		gv, wv := reflect.ValueOf(got), reflect.ValueOf(want)
		for i := 0; i < gv.NumField(); i++ {
			if !reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
				t.Errorf("viewer field %s differs from activated save", gv.Type().Field(i).Name)
			}
		}
	}
}

func TestAuthoritySnapshotBatchPreservesViewersSoundsAndOwnership(t *testing.T) {
	a, _, _ := snapshotFixture(t)
	a.g.authorityEvents = &authorityEventLog{lastID: 3, count: 3}
	a.g.authorityEvents.events[0] = authoritySoundEvent{ID: 1, Tick: a.Tic(), Kind: soundEventItemUp, Audience: 1, LocalPlayer: 1}
	a.g.authorityEvents.events[1] = authoritySoundEvent{ID: 2, Tick: a.Tic(), Kind: soundEventItemUp, Audience: 2, LocalPlayer: 2}
	a.g.authorityEvents.events[2] = authoritySoundEvent{ID: 3, Tick: a.Tic(), Kind: soundEventDoorOpen, X: a.players[1].p.x, Y: a.players[1].p.y}
	before := captureGameSaveState(a.g)
	active := a.g.captureAuthoritativePlayer()
	rng, playRNG := doomrand.State()
	states, err := a.SnapshotBatch([]byte{2, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("duplicate viewer generated extra payloads: %d", len(states))
	}
	for _, viewer := range []byte{1, 2} {
		r, err := decodeAuthorityReplica(states[viewer])
		if err != nil {
			t.Fatal(err)
		}
		if r.Viewer != viewer || r.Game.Session.PlayerSlot != int(viewer) || r.Game.Player != capturePlayerSaveState(a.players[viewer].p) {
			t.Fatalf("viewer %d got another player's state", viewer)
		}
		if len(r.Sounds) != 2 || r.Sounds[0].ID != uint64(viewer) || r.Sounds[1].ID != 3 || r.SoundCursor != 3 {
			t.Fatalf("viewer %d sound filtering changed: %+v", viewer, r.Sounds)
		}
		single, err := a.Snapshot(viewer)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(states[viewer], single) {
			t.Fatalf("viewer %d batched and individual payloads differ", viewer)
		}
	}
	if !reflect.DeepEqual(before, captureGameSaveState(a.g)) || !reflect.DeepEqual(active, a.g.captureAuthoritativePlayer()) {
		t.Fatal("batched capture mutated simulation or active player")
	}
	if after, playAfter := doomrand.State(); after != rng || playAfter != playRNG {
		t.Fatal("batched capture consumed simulation RNG")
	}
	other := bytes.Clone(states[2])
	states[1][0] ^= 1
	if !bytes.Equal(states[2], other) {
		t.Fatal("viewer payloads share mutable backing storage")
	}
}

func TestAuthoritySnapshotBatchRejectsInvalidViewerAtomically(t *testing.T) {
	a := testAuthority(t)
	if err := a.AddPlayer(1); err != nil {
		t.Fatal(err)
	}
	for _, viewers := range [][]byte{{1, 0}, {1, 2}, {1, 5}} {
		if states, err := a.SnapshotBatch(viewers); err == nil || states != nil {
			t.Fatalf("invalid viewer batch returned partial snapshots: %v, %v", states, err)
		}
	}
	if states, err := a.SnapshotBatch(nil); err != nil || len(states) != 0 {
		t.Fatalf("empty viewer batch = %v, %v", states, err)
	}
}

func BenchmarkAuthoritySnapshotBroadcast(b *testing.B) {
	for _, name := range []mapdata.MapName{"E1M1", "E1M3"} {
		b.Run(string(name), func(b *testing.B) {
			wf, err := wad.Open(findDOOM1WAD(b))
			if err != nil {
				b.Fatal(err)
			}
			m, err := mapdata.LoadMap(wf, name)
			if err != nil {
				b.Fatal(err)
			}
			a, err := NewAuthority(m, Options{SkillLevel: 3})
			if err != nil {
				b.Fatal(err)
			}
			viewers := []byte{1, 2, 3, 4}
			for _, viewer := range viewers {
				if err := a.AddPlayer(viewer); err != nil {
					b.Fatal(err)
				}
			}
			b.Run("FourSeparate", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					for _, viewer := range viewers {
						if _, err := a.Snapshot(viewer); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
			b.Run("SharedCapture", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := a.SnapshotBatch(viewers); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
