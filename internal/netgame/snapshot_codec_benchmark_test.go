package netgame

import (
	"bytes"
	"math/rand"
	"testing"
)

func benchmarkSnapshotState(size int) []byte {
	state := make([]byte, size)
	rng := rand.New(rand.NewSource(7))
	// Repeated zero/status fields interspersed with changing actor coordinates.
	for i := 0; i < len(state); i += 32 {
		rng.Read(state[i : i+8])
	}
	return state
}

func BenchmarkSnapshotCodec(b *testing.B) {
	for _, size := range []int{128 << 10, 512 << 10} {
		name := "128KiB"
		if size > 128<<10 {
			name = "512KiB"
		}
		b.Run(name, func(b *testing.B) {
			base := Snapshot{Epoch: 1, ID: 1, Tick: 1, State: benchmarkSnapshotState(size)}
			next := base
			next.ID, next.Tick, next.State = 2, 2, bytes.Clone(base.State)
			for i := 0; i < len(next.State); i += 4096 {
				next.State[i] ^= 1
			}
			for _, delta := range []bool{false, true} {
				mode, ack := "Full", uint32(0)
				if delta {
					mode, ack = "Delta", 1
				}
				b.Run(mode, func(b *testing.B) {
					encoder, err := NewSnapshotEncoder()
					if err != nil {
						b.Fatal(err)
					}
					defer encoder.Close()
					if err := encoder.Commit(base); err != nil {
						b.Fatal(err)
					}
					wire, err := encoder.Encode(next, ack)
					if err != nil {
						b.Fatal(err)
					}
					b.Run("Encode", func(b *testing.B) {
						b.ReportAllocs()
						for b.Loop() {
							if _, err := encoder.Encode(next, ack); err != nil {
								b.Fatal(err)
							}
						}
						b.ReportMetric(float64(len(wire.State)), "wire-B")
					})
					b.Run("Decode", func(b *testing.B) {
						decoder, err := NewSnapshotDecoder()
						if err != nil {
							b.Fatal(err)
						}
						defer decoder.Close()
						if _, err := decoder.Decode(base); err != nil {
							b.Fatal(err)
						}
						b.ReportAllocs()
						for b.Loop() {
							if _, err := decoder.Decode(wire); err != nil {
								b.Fatal(err)
							}
						}
					})
				})
			}
			b.Run("Commit", func(b *testing.B) {
				encoder, err := NewSnapshotEncoder()
				if err != nil {
					b.Fatal(err)
				}
				defer encoder.Close()
				b.ReportAllocs()
				for b.Loop() {
					next.ID++
					if err := encoder.Commit(next); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("Marshal", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := MarshalMessage(next); err != nil {
						b.Fatal(err)
					}
				}
			})
			data, err := MarshalMessage(next)
			if err != nil {
				b.Fatal(err)
			}
			b.Run("Unmarshal", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := UnmarshalMessage(data); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// Each frame acknowledges its predecessor, so dictionary setup, wire framing,
// decode and bounded-history eviction are included rather than amortized away.
func BenchmarkSnapshotCodecDeltaStream(b *testing.B) {
	for _, size := range []int{128 << 10, 512 << 10} {
		name := "128KiB"
		if size > 128<<10 {
			name = "512KiB"
		}
		b.Run(name, func(b *testing.B) {
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
			base := Snapshot{Epoch: 1, ID: 1, State: benchmarkSnapshotState(size)}
			if err := encoder.Commit(base); err != nil {
				b.Fatal(err)
			}
			if _, err := decoder.Decode(base); err != nil {
				b.Fatal(err)
			}
			states := make([][]byte, 16)
			for i := range states {
				states[i] = bytes.Clone(base.State)
				for j := 0; j < size; j += 4096 {
					states[i][j] ^= byte(i + 1)
				}
			}
			id, wireBytes, frames := uint32(1), 0, 0
			b.ReportAllocs()
			for b.Loop() {
				id++
				full := Snapshot{Epoch: 1, ID: id, Tick: id, State: states[id%uint32(len(states))]}
				wire, err := encoder.Encode(full, id-1)
				if err != nil {
					b.Fatal(err)
				}
				packet, err := MarshalMessage(wire)
				if err != nil {
					b.Fatal(err)
				}
				parsed, err := UnmarshalMessage(packet)
				if err != nil {
					b.Fatal(err)
				}
				got, err := decoder.Decode(parsed.(Snapshot))
				if err != nil || !bytes.Equal(got.State, full.State) {
					b.Fatalf("decode: %v", err)
				}
				if err := encoder.Commit(full); err != nil {
					b.Fatal(err)
				}
				wireBytes += len(packet)
				frames++
			}
			b.ReportMetric(float64(wireBytes)/float64(frames), "wire-B")
		})
	}
}
