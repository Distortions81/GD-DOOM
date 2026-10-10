package netgame

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

func TestSnapshotDictionaryReuseWithLossAndSkippedBroadcasts(t *testing.T) {
	encoder, decoder := snapshotCodecs(t)
	base := noisySnapshot(1)
	if err := encoder.Commit(base); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		t.Fatal(err)
	}
	ack := uint32(1)
	var previousBaseline uint32
	var reused bool
	for _, id := range []uint32{2, 3, 4, 5, 6, 7, 15, 16, 17, 18, 19, 20} {
		next := base
		next.ID, next.State = id, bytes.Clone(base.State)
		next.State[999] ^= byte(id)
		wire, err := encoder.Encode(next, ack)
		if err != nil {
			t.Fatal(err)
		}
		if wire.BaselineID == 0 || wire.BaselineID > ack {
			t.Fatalf("unconfirmed baseline at id=%d: %+v", id, wire)
		}
		if id == 3 && wire.BaselineID == previousBaseline {
			reused = true
		}
		if id == 15 && wire.BaselineID != ack {
			t.Fatal("broadcast gap retained an old dictionary")
		}
		previousBaseline = wire.BaselineID
		if err := encoder.Commit(next); err != nil {
			t.Fatal(err)
		}
		if id == 4 || id == 16 {
			continue // Lost datagrams never become acknowledged baselines.
		}
		got, err := decoder.Decode(codecWire(t, wire))
		if err != nil || !bytes.Equal(got.State, next.State) {
			t.Fatalf("id=%d after loss/gap: %v", id, err)
		}
		ack = id
	}
	if !reused {
		t.Fatal("dictionary was rebuilt for every advancing acknowledgment")
	}
	// An evicted acknowledgment must still yield an independent baseline.
	next := base
	next.ID = 21
	wire, err := encoder.Encode(next, 1)
	if err != nil || wire.BaselineID != 0 {
		t.Fatalf("evicted acknowledgment: %v", err)
	}
	// Recovery cancels reuse even when the old dictionary is still cached.
	if _, err := encoder.Encode(next, 0); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Commit(next); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Decode(next); err != nil {
		t.Fatal(err)
	}
	next.ID = 22
	wire, err = encoder.Encode(next, 21)
	if err != nil || wire.BaselineID != 21 {
		t.Fatalf("recovery depended on an old dictionary: %v", err)
	}
}

func TestGameDatagramDirectDecodeBoundsAndOwnership(t *testing.T) {
	snapshot := Snapshot{Epoch: 1, ID: 2, BaselineID: 1, Encoding: SnapshotDeltaZstd, DecodedSize: 4096, Digest: [32]byte{1}, State: []byte("owned payload")}
	frame := marshalProtocol(t, snapshot)
	for end := range len(frame) {
		if _, err := decodeGameDatagram(frame[:end], false); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted truncation at %d: %v", end, err)
		}
		if end >= messageHeaderBytes {
			short := bytes.Clone(frame[:end])
			binary.LittleEndian.PutUint32(short[6:10], uint32(end-messageHeaderBytes))
			if _, err := decodeGameDatagram(short, false); !errors.Is(err, ErrProtocol) {
				t.Fatalf("accepted truncated body at %d: %v", end, err)
			}
		}
	}
	for _, offset := range []int{0, 4, 6} {
		bad := bytes.Clone(frame)
		bad[offset] ^= 0xff
		if _, err := decodeGameDatagram(bad, false); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted invalid header at %d", offset)
		}
	}
	bad := bytes.Clone(frame)
	binary.LittleEndian.PutUint32(bad[6:10], math.MaxUint32)
	if _, err := decodeGameDatagram(bad, false); !errors.Is(err, ErrProtocol) {
		t.Fatal("accepted oversized claimed body")
	}
	value, err := decodeGameDatagram(frame, false)
	if err != nil {
		t.Fatal(err)
	}
	got := value.(Snapshot)
	if &got.State[0] != &frame[messageHeaderBytes+snapshotBodyHeaderBytes] {
		t.Fatal("datagram payload was copied instead of adopting the owned buffer")
	}
	second := marshalProtocol(t, snapshot)
	second[len(second)-1] ^= 1
	if _, err := decodeGameDatagram(second, false); err != nil || !bytes.Equal(got.State, snapshot.State) {
		t.Fatalf("later decode mutated retained payload: %v", err)
	}
}

func TestWebTransportReusedReadTimer(t *testing.T) {
	transport, _, _ := testWTTransport(t, true)
	for range 3 {
		if err := transport.SetReadDeadline(time.Now().Add(time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		if _, err := transport.ReadMessage(); !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("reused timer deadline: %v", err)
		}
	}
	if err := transport.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	transport.datagrams <- InputBatch{Epoch: 1}
	if _, err := transport.ReadMessage(); err != nil {
		t.Fatalf("stale timer interrupted fresh read: %v", err)
	}
}
