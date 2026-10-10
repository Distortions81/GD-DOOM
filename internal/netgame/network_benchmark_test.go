package netgame

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func BenchmarkGameDatagramDecode(b *testing.B) {
	for _, message := range []any{
		InputBatch{Epoch: 1, SnapshotAck: 1, Inputs: make([]Input, MaxInputBatch)},
		Snapshot{Epoch: 1, ID: 2, BaselineID: 1, Encoding: SnapshotDeltaZstd, DecodedSize: 128 << 10, Digest: [32]byte{1}, State: make([]byte, 900)},
	} {
		data, err := MarshalMessage(message)
		if err != nil {
			b.Fatal(err)
		}
		_, serverSide := message.(InputBatch)
		name := "Snapshot"
		if serverSide {
			name = "Input"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := decodeGameDatagram(data, serverSide); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWebTransportQueuedRead(b *testing.B) {
	t := &webTransportTransport{reliable: make(chan transportRead, 1), datagrams: make(chan any, 1), readChanged: make(chan struct{})}
	if err := t.SetReadDeadline(time.Now().Add(time.Hour)); err != nil {
		b.Fatal(err)
	}
	message := any(InputBatch{Epoch: 1})
	b.ReportAllocs()
	for b.Loop() {
		t.datagrams <- message
		if _, err := t.ReadMessage(); err != nil {
			b.Fatal(err)
		}
	}
}

// Continuously moving actors exercise compression against increasingly older
// baselines, instead of changing the same few bytes in otherwise static worlds.
func BenchmarkSnapshotMovingActors(b *testing.B) {
	encoder, err := NewSnapshotEncoder()
	if err != nil {
		b.Fatal(err)
	}
	defer encoder.Close()
	decoder, err := NewSnapshotDecoder()
	if err != nil {
		b.Fatal(err)
	}
	defer decoder.Close()
	base := Snapshot{Epoch: 1, ID: 1, State: benchmarkSnapshotState(128 << 10)}
	if err := encoder.Commit(base); err != nil {
		b.Fatal(err)
	}
	if _, err := decoder.Decode(base); err != nil {
		b.Fatal(err)
	}
	state := bytes.Clone(base.State)
	ack, id := uint32(1), uint32(1)
	wireBytes, frames := 0, 0
	b.ReportAllocs()
	for b.Loop() {
		id++
		for actor := range 64 {
			binary.LittleEndian.PutUint32(state[actor*64:], id*uint32(actor+1))
		}
		full := Snapshot{Epoch: 1, ID: id, Tick: id, State: state}
		wire, err := encoder.Encode(full, ack)
		if err != nil {
			b.Fatal(err)
		}
		got, err := decoder.Decode(wire)
		if err != nil || !bytes.Equal(got.State, state) {
			b.Fatalf("decode: %v", err)
		}
		if err := encoder.Commit(full); err != nil {
			b.Fatal(err)
		}
		ack = id
		wireBytes += messageHeaderBytes + snapshotBodyHeaderBytes + len(wire.State)
		frames++
	}
	b.ReportMetric(float64(wireBytes)/float64(frames), "wire-B")
}
