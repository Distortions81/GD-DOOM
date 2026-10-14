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
	// Once a delta is tiny, recompressing the entire world can save at most
	// this many wire bytes. Prefer bounded CPU work over that small difference.
	tinySnapshotDeltaBytes = 2 << 10
	// Reusing a confirmed dictionary avoids rebuilding its match table for
	// every acknowledgment. Rotate well before the receiver's eight-state
	// history limit; ID distance also bounds reuse when broadcasts are skipped.
	snapshotDictionaryReuseIDs = 4
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
	var state []byte
	evicted := 0
	for len(h.states)-evicted >= maxSnapshotHistory || h.bytes+len(snapshot.State) > maxSnapshotHistoryBytes {
		old := h.states[evicted].state
		h.bytes -= len(old)
		// Recycle only equal-sized evicted storage. This keeps the same memory
		// bound even when a large world is followed by much smaller snapshots.
		if len(old) == len(snapshot.State) {
			state = old
		}
		evicted++
	}
	if evicted != 0 {
		n := copy(h.states, h.states[evicted:])
		clear(h.states[n:])
		h.states = h.states[:n]
	}
	if state == nil {
		state = make([]byte, len(snapshot.State))
	}
	copy(state, snapshot.State)
	h.states = append(h.states, snapshotBaseline{id: snapshot.ID, state: state})
	h.bytes += len(state)
	return nil
}

// SnapshotEncoder belongs to one connection writer. Encode never makes a
// baseline acknowledgeable: Commit must follow a successful complete wire write.
// Deltas always refer to one acknowledged reconstructed state, never blindly to
// the preceding sent frame. Missing/evicted acknowledgments produce full frames.
type SnapshotEncoder struct {
	full                        *zstd.Encoder
	delta                       *zstd.Encoder
	history                     snapshotHistory
	fullSizeHint, deltaSizeHint int
	dictionaryEpoch             uint64
	dictionaryID                uint32
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
	encoder.resetEpoch(full.Epoch)
	if acknowledgedID == 0 {
		// A resync baseline must end dictionary reuse, so the next update can
		// depend solely on the newly confirmed state.
		encoder.dictionaryEpoch, encoder.dictionaryID = 0, 0
	}
	result := full
	if acknowledgedID > 0 && acknowledgedID < full.ID {
		if baseline := encoder.history.get(acknowledgedID); baseline != nil {
			if encoder.dictionaryEpoch == full.Epoch && encoder.dictionaryID > 0 &&
				encoder.dictionaryID <= acknowledgedID && full.ID-encoder.dictionaryID <= snapshotDictionaryReuseIDs {
				if cached := encoder.history.get(encoder.dictionaryID); cached != nil {
					acknowledgedID, baseline = encoder.dictionaryID, cached
				}
			}
			if encoder.dictionaryEpoch != full.Epoch || encoder.dictionaryID != acknowledgedID {
				if err := encoder.delta.ResetWithOptions(nil, zstd.WithEncoderDictRaw(snapshotDictionaryID, baseline)); err != nil {
					return Snapshot{}, err
				}
				encoder.dictionaryEpoch, encoder.dictionaryID = full.Epoch, acknowledgedID
			}
			delta := encoder.delta.EncodeAll(full.State, snapshotEncodeBuffer(len(full.State), encoder.deltaSizeHint, 1024))
			encoder.deltaSizeHint = len(delta)
			if len(delta) < len(result.State) {
				result.Encoding, result.BaselineID, result.State = SnapshotDeltaZstd, acknowledgedID, delta
			}
		}
	}
	if result.Encoding != SnapshotDeltaZstd || len(result.State) > tinySnapshotDeltaBytes || len(result.State) > len(full.State)/32 {
		compressed := encoder.full.EncodeAll(full.State, snapshotEncodeBuffer(len(full.State), encoder.fullSizeHint, len(full.State)))
		encoder.fullSizeHint = len(compressed)
		if len(compressed) < len(result.State) {
			result.Encoding, result.BaselineID, result.State = SnapshotZstd, 0, compressed
		}
	}
	if result.Encoding != SnapshotRaw {
		result.DecodedSize = uint32(len(full.State))
		result.Digest = blake3.Sum256(full.State)
	}
	return result, nil
}

// EncodeAll otherwise reserves raw-size output even for a tiny delta. Reuse a
// size estimate, not the output itself: returned packets must remain immutable
// across subsequent calls and writes. A sudden larger frame grows normally.
func snapshotEncodeBuffer(rawSize, previousSize, initialSize int) []byte {
	size := initialSize
	if previousSize > 0 {
		size = previousSize + previousSize/8 + 64
	}
	return make([]byte, 0, min(rawSize, size))
}

func (encoder *SnapshotEncoder) resetEpoch(epoch uint64) {
	if encoder.history.epoch != epoch {
		encoder.dictionaryEpoch, encoder.dictionaryID = 0, 0
	}
	encoder.history.reset(epoch)
}

func (encoder *SnapshotEncoder) Commit(full Snapshot) error {
	if _, err := validateMessage(full); err != nil {
		return err
	}
	if full.Encoding != SnapshotRaw || full.BaselineID != 0 {
		return ErrProtocol
	}
	encoder.resetEpoch(full.Epoch)
	if err := encoder.history.put(full); err != nil {
		return err
	}
	if encoder.dictionaryEpoch != full.Epoch || encoder.history.get(encoder.dictionaryID) == nil {
		encoder.dictionaryEpoch, encoder.dictionaryID = 0, 0
	}
	return nil
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
	decoder         *zstd.Decoder
	history         snapshotHistory
	dictionaryEpoch uint64
	dictionaryID    uint32
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
	if decoder.history.epoch != wire.Epoch {
		decoder.dictionaryEpoch, decoder.dictionaryID = 0, 0
	}
	decoder.history.reset(wire.Epoch)
	result := wire
	if wire.Encoding != SnapshotRaw {
		var baseline []byte
		if wire.Encoding == SnapshotDeltaZstd {
			baseline = decoder.history.get(wire.BaselineID)
			if baseline == nil {
				return Snapshot{}, fmt.Errorf("%w: epoch=%d id=%d", ErrSnapshotBaseline, wire.Epoch, wire.BaselineID)
			}
		}
		if decoder.dictionaryEpoch != wire.Epoch || decoder.dictionaryID != wire.BaselineID {
			options := []zstd.DOption{zstd.WithDecoderDictDelete()}
			if baseline != nil {
				options = append(options, zstd.WithDecoderDictRaw(snapshotDictionaryID, baseline))
			}
			if err := decoder.decoder.ResetWithOptions(nil, options...); err != nil {
				return Snapshot{}, err
			}
			decoder.dictionaryEpoch, decoder.dictionaryID = wire.Epoch, wire.BaselineID
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
	if decoder.dictionaryEpoch != result.Epoch || decoder.dictionaryID != 0 && decoder.history.get(decoder.dictionaryID) == nil {
		decoder.dictionaryEpoch, decoder.dictionaryID = 0, 0
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
