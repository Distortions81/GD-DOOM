package doomruntime

import (
	"bytes"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/netgame"
)

func TestAuthorityE1M1SnapshotBandwidth(t *testing.T) {
	authority := testAuthority(t)
	for _, id := range []byte{1, 2} {
		if err := authority.AddPlayer(id); err != nil {
			t.Fatal(err)
		}
	}
	encoder, err := netgame.NewSnapshotEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	decoder, err := netgame.NewSnapshotDecoder()
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	var rawTotal, wireTotal, fullBytes, deltaBytes int
	var acknowledged uint32
	for id := uint32(1); id <= 36; id++ {
		if id > 1 {
			for range 2 {
				if err := authority.Step(map[byte]demo.Tic{1: {Forward: 25, Buttons: demo.ButtonAttack}, 2: {Side: 10, AngleTurn: 16}}); err != nil {
					t.Fatal(err)
				}
			}
		}
		state, err := authority.Snapshot(1)
		if err != nil {
			t.Fatal(err)
		}
		full := netgame.Snapshot{Epoch: 1, ID: id, Tick: authority.Tic(), State: state}
		wire, err := encoder.Encode(full, acknowledged)
		if err != nil {
			t.Fatal(err)
		}
		framed, err := netgame.MarshalMessage(wire)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := netgame.UnmarshalMessage(framed)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decoder.Decode(parsed.(netgame.Snapshot))
		if err != nil || !bytes.Equal(decoded.State, state) {
			t.Fatalf("snapshot %d reconstruction: %v", id, err)
		}
		if err := encoder.Commit(full); err != nil {
			t.Fatal(err)
		}
		acknowledged = decoded.ID
		rawTotal += len(state)
		wireTotal += len(framed)
		if id == 1 {
			fullBytes = len(framed)
		} else {
			deltaBytes += len(framed)
		}
	}
	t.Logf("E1M1 2-player moving/firing: raw average=%d B; first full=%d B; subsequent average=%d B; wire/raw=%.2f%%; at17.5Hz=%.1f KiB/s per viewer", rawTotal/36, fullBytes, deltaBytes/35, 100*float64(wireTotal)/float64(rawTotal), float64(deltaBytes)*17.5/35/1024)
	if wireTotal >= rawTotal/4 {
		t.Fatalf("insufficient snapshot compression: %d / %d", wireTotal, rawTotal)
	}
}
