package netgame

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"github.com/zeebo/blake3"
)

type SnapshotEncoding byte

const (
	SnapshotRaw SnapshotEncoding = iota
	SnapshotZstd
	SnapshotDeltaZstd
	maxSnapshotHistory      = 8
	maxSnapshotHistoryBytes = 16 << 20
	snapshotWindowBytes     = 1 << 20
	// The outer epoch/BaselineID identify the raw dictionary. This fixed ID is
	// in zstd's application dictionary range and is never confused with a
	// snapshot counter (whose entire uint32 range is valid).
	snapshotDictionaryID = 1
)

var (
	ErrSnapshotBaseline  = errors.New("snapshot baseline is unavailable")
	ErrSnapshotIntegrity = errors.New("snapshot integrity check failed")
)

type snapshotBaseline struct {
	id    uint32
	state []byte
}

type snapshotHistory struct {
	epoch  uint64
	states []snapshotBaseline
	bytes  int
}

func (h *snapshotHistory) reset(epoch uint64) {
	if h.epoch != epoch {
		h.epoch, h.states, h.bytes = epoch, nil, 0
	}
}

func (h *snapshotHistory) get(id uint32) []byte {
	for _, baseline := range h.states {
		if baseline.id == id {
			return baseline.state
		}
	}
	return nil
}

func (h *snapshotHistory) put(snapshot Snapshot) error {
	h.reset(snapshot.Epoch)
	if existing := h.get(snapshot.ID); existing != nil {
		if !bytes.Equal(existing, snapshot.State) {
			return fmt.Errorf("%w: snapshot ID reused with different state", ErrSnapshotIntegrity)
		}
		return nil
	}
	for len(h.states) >= maxSnapshotHistory || h.bytes+len(snapshot.State) > maxSnapshotHistoryBytes {
		h.bytes -= len(h.states[0].state)
		h.states[0] = snapshotBaseline{}
		h.states = h.states[1:]
	}
	state := bytes.Clone(snapshot.State)
	h.states = append(h.states, snapshotBaseline{id: snapshot.ID, state: state})
	h.bytes += len(state)
	return nil
}

// SnapshotEncoder belongs to one connection writer. Encode never makes a
// baseline acknowledgeable: Commit must follow a successful complete wire write.
// Deltas always refer to one acknowledged reconstructed state, never blindly to
// the preceding sent frame. Missing/evicted acknowledgments produce full frames.
type SnapshotEncoder struct {
	full    *zstd.Encoder
	delta   *zstd.Encoder
	history snapshotHistory
}

func NewSnapshotEncoder() (*SnapshotEncoder, error) {
	options := []zstd.EOption{zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithWindowSize(snapshotWindowBytes), zstd.WithEncoderCRC(true)}
	full, err := zstd.NewWriter(nil, options...)
	if err != nil {
		return nil, err
	}
	delta, err := zstd.NewWriter(nil, options...)
	if err != nil {
		full.Close()
		return nil, err
	}
	return &SnapshotEncoder{full: full, delta: delta}, nil
}

func (encoder *SnapshotEncoder) Encode(full Snapshot, acknowledgedID uint32) (Snapshot, error) {
	if _, err := validateMessage(full); err != nil {
		return Snapshot{}, err
	}
	if full.Encoding != SnapshotRaw || full.BaselineID != 0 {
		return Snapshot{}, ErrProtocol
	}
	encoder.history.reset(full.Epoch)
	result := full
	compressed := encoder.full.EncodeAll(full.State, nil)
	if len(compressed) < len(result.State) {
		result.Encoding, result.State = SnapshotZstd, compressed
	}
	if acknowledgedID > 0 && acknowledgedID < full.ID {
		if baseline := encoder.history.get(acknowledgedID); baseline != nil {
			if err := encoder.delta.ResetWithOptions(nil, zstd.WithEncoderDictRaw(snapshotDictionaryID, baseline)); err != nil {
				return Snapshot{}, err
			}
			delta := encoder.delta.EncodeAll(full.State, nil)
			if len(delta) < len(result.State) {
				result.Encoding, result.BaselineID, result.State = SnapshotDeltaZstd, acknowledgedID, delta
			}
		}
	}
	if result.Encoding != SnapshotRaw {
		result.DecodedSize = uint32(len(full.State))
		result.Digest = blake3.Sum256(full.State)
	}
	return result, nil
}

func (encoder *SnapshotEncoder) Commit(full Snapshot) error {
	if _, err := validateMessage(full); err != nil {
		return err
	}
	if full.Encoding != SnapshotRaw || full.BaselineID != 0 {
		return ErrProtocol
	}
	return encoder.history.put(full)
}

func (encoder *SnapshotEncoder) Close() {
	if encoder == nil {
		return
	}
	encoder.full.Close()
	encoder.delta.Close()
	encoder.history = snapshotHistory{}
}

// SnapshotDecoder belongs to one connection reader. It caches every verified
// network snapshot before callers coalesce presentation updates. Memory, window,
// and decoded output are bounded independently of claims in the compressed frame.
type SnapshotDecoder struct {
	decoder *zstd.Decoder
	history snapshotHistory
}

func NewSnapshotDecoder() (*SnapshotDecoder, error) {
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(MaxSnapshotBytes), zstd.WithDecoderMaxWindow(snapshotWindowBytes), zstd.WithDecodeAllCapLimit(true))
	if err != nil {
		return nil, err
	}
	return &SnapshotDecoder{decoder: decoder}, nil
}

func (decoder *SnapshotDecoder) Decode(wire Snapshot) (Snapshot, error) {
	if _, err := validateMessage(wire); err != nil {
		return Snapshot{}, err
	}
	decoder.history.reset(wire.Epoch)
	result := wire
	if wire.Encoding != SnapshotRaw {
		options := []zstd.DOption{zstd.WithDecoderDictDelete()}
		if wire.Encoding == SnapshotDeltaZstd {
			baseline := decoder.history.get(wire.BaselineID)
			if baseline == nil {
				return Snapshot{}, fmt.Errorf("%w: epoch=%d id=%d", ErrSnapshotBaseline, wire.Epoch, wire.BaselineID)
			}
			options = append(options, zstd.WithDecoderDictRaw(snapshotDictionaryID, baseline))
		}
		if err := decoder.decoder.ResetWithOptions(nil, options...); err != nil {
			return Snapshot{}, err
		}
		// The protocol validated DecodedSize before this allocation. CapLimit
		// stops zstd from growing output beyond exactly this advertised bound.
		state, err := decoder.decoder.DecodeAll(wire.State, make([]byte, 0, int(wire.DecodedSize)))
		if err != nil {
			return Snapshot{}, fmt.Errorf("%w: %v", ErrSnapshotIntegrity, err)
		}
		if len(state) != int(wire.DecodedSize) || blake3.Sum256(state) != wire.Digest {
			return Snapshot{}, ErrSnapshotIntegrity
		}
		result.State = state
		result.Encoding, result.BaselineID, result.DecodedSize, result.Digest = SnapshotRaw, 0, 0, [32]byte{}
	}
	if err := decoder.history.put(result); err != nil {
		return Snapshot{}, err
	}
	return result, nil
}

func (decoder *SnapshotDecoder) Close() {
	if decoder == nil {
		return
	}
	decoder.decoder.Close()
	decoder.history = snapshotHistory{}
}
