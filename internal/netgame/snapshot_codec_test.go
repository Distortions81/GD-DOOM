package netgame

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/zeebo/blake3"
)

func snapshotCodecs(t *testing.T) (*SnapshotEncoder, *SnapshotDecoder) {
	t.Helper()
	encoder, err := NewSnapshotEncoder()
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := NewSnapshotDecoder()
	if err != nil {
		encoder.Close()
		t.Fatal(err)
	}
	t.Cleanup(encoder.Close)
	t.Cleanup(decoder.Close)
	return encoder, decoder
}

func noisySnapshot(id uint32) Snapshot {
	state := make([]byte, 128<<10)
	rand.New(rand.NewSource(1)).Read(state)
	return Snapshot{Epoch: 1, ID: id, Tick: id, State: state}
}

func codecWire(t *testing.T, snapshot Snapshot) Snapshot {
	t.Helper()
	value, err := UnmarshalMessage(marshalProtocol(t, snapshot))
	if err != nil {
		t.Fatal(err)
	}
	return value.(Snapshot)
}

func TestSnapshotCodecFullAndAcknowledgedDelta(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	wire, err := encoder.Encode(baseline, 0)
	if err != nil || wire.Encoding != SnapshotRaw {
		t.Fatalf("incompressible baseline: encoding=%d error=%v", wire.Encoding, err)
	}
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(codecWire(t, wire)); err != nil {
		t.Fatal(err)
	}
	next := noisySnapshot(2)
	next.State[999] ^= 1
	wire, err = encoder.Encode(next, 1)
	if err != nil || wire.Encoding != SnapshotDeltaZstd || wire.BaselineID != 1 || len(wire.State) >= len(next.State)/10 {
		t.Fatalf("delta: encoding=%d baseline=%d size=%d error=%v", wire.Encoding, wire.BaselineID, len(wire.State), err)
	}
	full, err := decoder.Decode(codecWire(t, wire))
	if err != nil || !bytes.Equal(full.State, next.State) || full.BaselineID != 0 || full.Encoding != SnapshotRaw || full.DecodedSize != 0 || full.Digest != [32]byte{} {
		t.Fatalf("delta decode failed: %v", err)
	}
	if err := encoder.Commit(next); err != nil {
		t.Fatal(err)
	}
	// The reader caches a reconstructed delta even if presentation skips it.
	last := noisySnapshot(3)
	last.State[444] ^= 1
	wire, err = encoder.Encode(last, 2)
	if err != nil {
		t.Fatal(err)
	}
	if wire.BaselineID != 2 {
		t.Fatal("did not use freshly acknowledged decoded delta")
	}
	full, err = decoder.Decode(codecWire(t, wire))
	if err != nil || !bytes.Equal(full.State, last.State) {
		t.Fatalf("reconstructed baseline: %v", err)
	}
	repeated := Snapshot{Epoch: 1, ID: 4, Tick: 4, State: bytes.Repeat([]byte("door:closed;"), 5000)}
	wire, err = encoder.Encode(repeated, 0)
	if err != nil || wire.Encoding != SnapshotZstd || wire.BaselineID != 0 {
		t.Fatalf("full compression: encoding=%d error=%v", wire.Encoding, err)
	}
	full, err = decoder.Decode(codecWire(t, wire))
	if err != nil || !bytes.Equal(full.State, repeated.State) {
		t.Fatalf("full decode: %v", err)
	}
}

func TestSnapshotCodecRequiresSentAcknowledgedBaseline(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	baseline, next := noisySnapshot(1), noisySnapshot(2)
	if _, err := encoder.Encode(baseline, 0); err != nil {
		t.Fatal(err)
	}
	wire, err := encoder.Encode(next, 1)
	if err != nil || wire.BaselineID != 0 {
		t.Fatalf("unsent baseline used: %v", err)
	}
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	for _, ack := range []uint32{0, 2, 99} {
		wire, err := encoder.Encode(next, ack)
		if err != nil || wire.BaselineID != 0 {
			t.Fatalf("unacknowledged baseline used for ack %d: %v", ack, err)
		}
	}
	wire, err = encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(wire); !errors.Is(err, ErrSnapshotBaseline) {
		t.Fatalf("missing baseline: %v", err)
	}
	if len(decoder.history.states) != 0 {
		t.Fatal("failed snapshot entered decoder history")
	}
}

func TestSnapshotCodecEpochAndBoundedHistory(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(baseline); err != nil {
		t.Fatal(err)
	}
	next := noisySnapshot(2)
	wire, err := encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	wire.Epoch = 2
	if _, err := decoder.Decode(wire); !errors.Is(err, ErrSnapshotBaseline) {
		t.Fatalf("cross-epoch baseline: %v", err)
	}
	next.Epoch = 2
	wire, err = encoder.Encode(next, 1)
	if err != nil || wire.BaselineID != 0 {
		t.Fatalf("encoder cross-epoch baseline: %v", err)
	}
	for id := uint32(1); id <= 10; id++ {
		s := noisySnapshot(id)
		s.Epoch = 3
		if err := encoder.Commit(s); err != nil {
			t.Fatal(err)
		}
		if _, err := decoder.Decode(s); err != nil {
			t.Fatal(err)
		}
	}
	if len(encoder.history.states) != 8 || len(decoder.history.states) != 8 || encoder.history.get(2) != nil || decoder.history.get(2) != nil {
		t.Fatal("history count limit failed")
	}
	// Three maximum-sized states would exceed the independent byte budget.
	large := Snapshot{Epoch: 4, ID: 1, State: make([]byte, MaxSnapshotBytes)}
	for id := uint32(1); id <= 3; id++ {
		large.ID = id
		if err := encoder.Commit(large); err != nil {
			t.Fatal(err)
		}
		if _, err := decoder.Decode(large); err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range []*snapshotHistory{&encoder.history, &decoder.history} {
		if h.bytes != maxSnapshotHistoryBytes || len(h.states) != 2 || h.get(1) != nil {
			t.Fatalf("byte bound: bytes=%d count=%d", h.bytes, len(h.states))
		}
	}
}

func TestSnapshotCodecOwnsCachedStateAndRejectsIDReuse(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	full, err := decoder.Decode(baseline)
	if err != nil {
		t.Fatal(err)
	}
	full.State[0] ^= 1
	if encoder.history.get(1)[0] == full.State[0] || decoder.history.get(1)[0] == full.State[0] {
		t.Fatal("caller mutated cached baseline")
	}
	if err := encoder.Commit(baseline); !errors.Is(err, ErrSnapshotIntegrity) {
		t.Fatalf("conflicting encoder ID reuse: %v", err)
	}
	if _, err := decoder.Decode(baseline); !errors.Is(err, ErrSnapshotIntegrity) {
		t.Fatalf("conflicting decoder ID reuse: %v", err)
	}
}

func TestSnapshotCodecRecyclesHistoryWithoutChangingPacketsOrEpochDictionaries(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	var retained []byte
	for epoch := uint64(1); epoch <= 2; epoch++ {
		base := noisySnapshot(1)
		base.Epoch = epoch
		if epoch == 2 {
			// Reuse snapshot IDs with different dictionary contents in a new map.
			for i := range base.State {
				base.State[i] ^= 0x5a
			}
		}
		if err := encoder.Commit(base); err != nil {
			t.Fatal(err)
		}
		if _, err := decoder.Decode(base); err != nil {
			t.Fatal(err)
		}
		ack := uint32(1)
		for id := uint32(2); id <= 20; id++ {
			next := base
			next.ID, next.State = id, bytes.Clone(base.State)
			next.State[int(id)*31] ^= byte(id)
			wire, err := encoder.Encode(next, ack)
			if err != nil {
				t.Fatal(err)
			}
			packet := bytes.Clone(wire.State)
			got, err := decoder.Decode(wire)
			if err != nil || !bytes.Equal(got.State, next.State) {
				t.Fatalf("epoch=%d id=%d decode: %v", epoch, id, err)
			}
			if err := encoder.Commit(next); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(wire.State, packet) {
				t.Fatal("history commit mutated encoded packet")
			}
			if id == 2 {
				retained = got.State
			}
			if retained[2*31] != base.State[2*31]^2 {
				t.Fatal("later history eviction mutated previously returned state")
			}
			ack = id
		}
		missing := base
		missing.ID = 21
		wire, err := encoder.Encode(missing, 1)
		if err != nil || wire.BaselineID != 0 {
			t.Fatalf("evicted dictionary reused: %v", err)
		}
	}
}

func TestSnapshotCodecEncodedFramesOwnOutputAcrossCalls(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	base := noisySnapshot(1)
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		t.Fatal(err)
	}
	first := base
	first.ID = 2
	first.State = bytes.Clone(base.State)
	first.State[99] ^= 1
	wire, err := encoder.Encode(first, 1)
	if err != nil {
		t.Fatal(err)
	}
	packet := bytes.Clone(wire.State)
	for id := uint32(3); id < 20; id++ {
		next := base
		next.ID, next.State = id, bytes.Clone(base.State)
		next.State[id] ^= byte(id)
		if _, err := encoder.Encode(next, 1); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(wire.State, packet) {
		t.Fatal("later compression mutated retained packet")
	}
	got, err := decoder.Decode(wire)
	if err != nil || !bytes.Equal(got.State, first.State) {
		t.Fatalf("retained frame no longer decodes: %v", err)
	}
}

func TestSnapshotCodecEvictionInvalidatesReinsertedDictionaryID(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	base := noisySnapshot(1)
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		t.Fatal(err)
	}
	next := noisySnapshot(2)
	wire, err := encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(wire); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Commit(next); err != nil {
		t.Fatal(err)
	}
	for id := uint32(3); id <= 10; id++ {
		s := noisySnapshot(id)
		if err := encoder.Commit(s); err != nil {
			t.Fatal(err)
		}
		if _, err := decoder.Decode(s); err != nil {
			t.Fatal(err)
		}
	}
	// The network reader rejects stale IDs, but the codec API itself allows a
	// no-longer-retained ID. It must not reuse the evicted dictionary's tables.
	for i := range base.State {
		base.State[i] ^= 0xa5
	}
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		t.Fatal(err)
	}
	next = base
	next.ID, next.State = 11, bytes.Clone(base.State)
	next.State[41] ^= 1
	wire, err = encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decoder.Decode(wire)
	if err != nil || !bytes.Equal(got.State, next.State) {
		t.Fatalf("stale dictionary after reinsertion: %v", err)
	}
}

func TestSnapshotCodecMalformedCompressedState(t *testing.T) {
	encoder, _ := snapshotCodecs(t)
	baseline := noisySnapshot(1)
	if err := encoder.Commit(baseline); err != nil {
		t.Fatal(err)
	}
	next := noisySnapshot(2)
	valid, err := encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	alter := func(fn func(*Snapshot)) Snapshot { s := valid; s.State = bytes.Clone(s.State); fn(&s); return s }
	wrongDictionary, err := zstd.NewWriter(nil, zstd.WithEncoderDictRaw(2, baseline.State), zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer wrongDictionary.Close()
	tests := map[string]Snapshot{
		"zero decoded size":       alter(func(s *Snapshot) { s.DecodedSize = 0 }),
		"oversized decoded size":  alter(func(s *Snapshot) { s.DecodedSize = MaxSnapshotBytes + 1 }),
		"short decoded size":      alter(func(s *Snapshot) { s.DecodedSize-- }),
		"long decoded size":       alter(func(s *Snapshot) { s.DecodedSize++ }),
		"wrong digest":            alter(func(s *Snapshot) { s.Digest[0] ^= 1 }),
		"zero digest":             alter(func(s *Snapshot) { s.Digest = [32]byte{} }),
		"truncated frame":         alter(func(s *Snapshot) { s.State = s.State[:len(s.State)-1] }),
		"corrupt magic":           alter(func(s *Snapshot) { s.State[0] ^= 1 }),
		"wrong dictionary ID":     alter(func(s *Snapshot) { s.State = wrongDictionary.EncodeAll(next.State, nil) }),
		"full with dictionary":    alter(func(s *Snapshot) { s.Encoding = SnapshotZstd; s.BaselineID = 0 }),
		"raw with delta metadata": alter(func(s *Snapshot) { s.Encoding = SnapshotRaw }),
	}
	for name, wire := range tests {
		t.Run(name, func(t *testing.T) {
			_, decoder := snapshotCodecs(t)
			if _, err := decoder.Decode(baseline); err != nil {
				t.Fatal(err)
			}
			if _, err := decoder.Decode(wire); err == nil {
				t.Fatal("accepted invalid compressed snapshot")
			}
			if decoder.history.get(2) != nil {
				t.Fatal("failed decode polluted baseline history")
			}
		})
	}
}

func TestSnapshotCodecRejectsExcessiveWindowAndOutput(t *testing.T) {
	_, decoder := snapshotCodecs(t)
	compressor, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1), zstd.WithWindowSize(2<<20), zstd.WithSingleSegment(false))
	if err != nil {
		t.Fatal(err)
	}
	defer compressor.Close()
	state := bytes.Repeat([]byte("large state"), 200000)
	frame := compressor.EncodeAll(state, nil)
	wire := Snapshot{Epoch: 1, ID: 1, Encoding: SnapshotZstd, DecodedSize: uint32(len(state)), Digest: blake3.Sum256(state), State: frame}
	if _, err := decoder.Decode(wire); !errors.Is(err, ErrSnapshotIntegrity) {
		t.Fatalf("excessive window: %v", err)
	}
	// A valid small-window frame can still claim far more output than the envelope.
	encoder, _ := snapshotCodecs(t)
	wire, err = encoder.Encode(Snapshot{Epoch: 1, ID: 1, State: state}, 0)
	if err != nil {
		t.Fatal(err)
	}
	wire.DecodedSize = 16
	if _, err := decoder.Decode(wire); !errors.Is(err, ErrSnapshotIntegrity) {
		t.Fatalf("output cap: %v", err)
	}
}

func FuzzSnapshotDecoder(f *testing.F) {
	encoder, err := NewSnapshotEncoder()
	if err != nil {
		f.Fatal(err)
	}
	seed, err := encoder.Encode(Snapshot{Epoch: 1, ID: 1, State: bytes.Repeat([]byte("snapshot"), 100)}, 0)
	encoder.Close()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(byte(seed.Encoding), seed.DecodedSize, seed.Digest[:], seed.State)
	f.Fuzz(func(t *testing.T, encoding byte, size uint32, digest, state []byte) {
		if size > 1<<20 || len(state) > 1<<20 {
			t.Skip()
		}
		decoder, err := NewSnapshotDecoder()
		if err != nil {
			t.Fatal(err)
		}
		defer decoder.Close()
		wire := Snapshot{Epoch: 1, ID: 1, Encoding: SnapshotEncoding(encoding), DecodedSize: size, State: state}
		copy(wire.Digest[:], digest)
		full, err := decoder.Decode(wire)
		if err == nil && (len(full.State) > MaxSnapshotBytes || full.Encoding != SnapshotRaw || full.BaselineID != 0) {
			t.Fatal("invalid decoded state")
		}
	})
}

func TestSnapshotCodecInvalidatesDictionaryOnEncoderEpochDetour(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	base := noisySnapshot(1)
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.Encode(noisySnapshot(2), 1); err != nil {
		t.Fatal(err)
	}
	// An encoded but uncommitted epoch clears history. Returning to the old
	// epoch must not revive its cached dictionary when the ID is reused.
	detour := noisySnapshot(1)
	detour.Epoch = 2
	if _, err := encoder.Encode(detour, 0); err != nil {
		t.Fatal(err)
	}
	newBase := noisySnapshot(1)
	newBase.State[1000] ^= 1
	if err := encoder.Commit(newBase); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(newBase); err != nil {
		t.Fatal(err)
	}
	next := noisySnapshot(2)
	wire, err := encoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decoder.Decode(wire)
	if err != nil || !bytes.Equal(got.State, next.State) {
		t.Fatalf("decode after epoch detour: %v", err)
	}
}

func TestSnapshotCodecInvalidatesDictionaryAfterFailedDecoderEpochDetour(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	base := noisySnapshot(1)
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		t.Fatal(err)
	}
	wire, err := encoder.Encode(noisySnapshot(2), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(wire); err != nil {
		t.Fatal(err)
	}
	// Even a failed epoch transition discards history, so it must also
	// discard the cached dictionary before a former ID can be reused.
	wire.Epoch = 2
	if _, err := decoder.Decode(wire); !errors.Is(err, ErrSnapshotBaseline) {
		t.Fatalf("detour: %v", err)
	}
	newBase := noisySnapshot(1)
	newBase.State[1000] ^= 1
	if _, err := decoder.Decode(newBase); err != nil {
		t.Fatal(err)
	}
	freshEncoder, _ := snapshotCodecs(t)
	if err := freshEncoder.Commit(newBase); err != nil {
		t.Fatal(err)
	}
	next := newBase
	next.ID = 2
	wire, err = freshEncoder.Encode(next, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decoder.Decode(wire)
	if err != nil || !bytes.Equal(got.State, next.State) {
		t.Fatalf("decode after failed epoch detour: %v", err)
	}
}
