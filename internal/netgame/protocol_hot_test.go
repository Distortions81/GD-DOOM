package netgame

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/zeebo/blake3"
)

func TestGameplayFramesRetainProtocolV2Layout(t *testing.T) {
	// Keep an independent field-by-field reference for wire compatibility;
	// a round trip alone would miss matching offset bugs in reader and writer.
	for _, message := range protocolExamples() {
		var body bytes.Buffer
		put := func(value any) {
			if err := binary.Write(&body, binary.LittleEndian, value); err != nil {
				t.Fatal(err)
			}
		}
		var kind MessageKind
		switch m := message.(type) {
		case InputBatch:
			kind = KindInput
			put(m.Epoch)
			put(m.SnapshotAck)
			put(byte(len(m.Inputs)))
			for _, input := range m.Inputs {
				put(input.Sequence)
				put(input.Tick)
				put(input.Command.Forward)
				put(input.Command.Side)
				put(input.Command.AngleTurn)
				put(input.Command.Buttons)
			}
		case Snapshot:
			kind = KindSnapshot
			put(m.Epoch)
			put(m.ID)
			put(m.BaselineID)
			put(m.Tick)
			var flags byte
			if m.Finalized.HasTick {
				flags |= 1
			}
			if m.Finalized.HasSequence {
				flags |= 2
			}
			put(flags)
			put(m.Finalized.Tick)
			put(m.Finalized.Sequence)
			put(m.Encoding)
			if m.Encoding == SnapshotRaw {
				put(uint32(len(m.State)))
				put(blake3.Sum256(m.State))
			} else {
				put(m.DecodedSize)
				put(m.Digest)
			}
			put(uint32(len(m.State)))
			body.Write(m.State)
		default:
			continue
		}
		if got := marshalProtocol(t, message); !bytes.Equal(got, protocolFrame(kind, body.Bytes())) {
			t.Fatalf("wire format changed for %T", message)
		}
	}
}
