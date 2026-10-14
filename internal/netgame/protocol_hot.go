package netgame

import (
	"encoding/binary"
	"fmt"

	"gddoom/internal/demo"
	"github.com/zeebo/blake3"
)

// Gameplay frames have fixed layouts. Write them directly into the owned wire
// buffer, avoiding reflection and a second complete snapshot-sized buffer.
func newMessageFrame(kind MessageKind, bodyBytes int) ([]byte, []byte) {
	frame := make([]byte, messageHeaderBytes+bodyBytes)
	copy(frame, "GDMP")
	frame[4], frame[5] = ProtocolVersion, byte(kind)
	binary.LittleEndian.PutUint32(frame[6:10], uint32(bodyBytes))
	return frame, frame[messageHeaderBytes:]
}

func marshalSnapshot(m Snapshot) []byte {
	frame, b := newMessageFrame(KindSnapshot, snapshotBodyHeaderBytes+len(m.State))
	binary.LittleEndian.PutUint64(b[0:8], m.Epoch)
	binary.LittleEndian.PutUint32(b[8:12], m.ID)
	binary.LittleEndian.PutUint32(b[12:16], m.BaselineID)
	binary.LittleEndian.PutUint32(b[16:20], m.Tick)
	if m.Finalized.HasTick {
		b[20] |= 1
	}
	if m.Finalized.HasSequence {
		b[20] |= 2
	}
	binary.LittleEndian.PutUint32(b[21:25], m.Finalized.Tick)
	binary.LittleEndian.PutUint32(b[25:29], m.Finalized.Sequence)
	b[29] = byte(m.Encoding)
	size, digest := m.DecodedSize, m.Digest
	if m.Encoding == SnapshotRaw {
		size, digest = uint32(len(m.State)), blake3.Sum256(m.State)
	}
	binary.LittleEndian.PutUint32(b[30:34], size)
	copy(b[34:66], digest[:])
	binary.LittleEndian.PutUint32(b[66:70], uint32(len(m.State)))
	copy(b[70:], m.State)
	return frame
}

func decodeSnapshot(b []byte) (any, error) {
	if len(b) <= snapshotBodyHeaderBytes {
		return nil, ErrProtocol
	}
	n := binary.LittleEndian.Uint32(b[66:70])
	if b[20] & ^byte(3) != 0 || n == 0 || n > MaxSnapshotBytes || uint64(n) != uint64(len(b)-snapshotBodyHeaderBytes) {
		return nil, ErrProtocol
	}
	m := Snapshot{
		Epoch: binary.LittleEndian.Uint64(b[0:8]), ID: binary.LittleEndian.Uint32(b[8:12]),
		BaselineID: binary.LittleEndian.Uint32(b[12:16]), Tick: binary.LittleEndian.Uint32(b[16:20]),
		Finalized: InputAck{HasTick: b[20]&1 != 0, HasSequence: b[20]&2 != 0, Tick: binary.LittleEndian.Uint32(b[21:25]), Sequence: binary.LittleEndian.Uint32(b[25:29])},
		Encoding:  SnapshotEncoding(b[29]), DecodedSize: binary.LittleEndian.Uint32(b[30:34]), State: b[70:],
	}
	copy(m.Digest[:], b[34:66])
	if m.Encoding == SnapshotRaw {
		if m.DecodedSize != n || m.Digest != blake3.Sum256(m.State) {
			return nil, fmt.Errorf("%w: %w", ErrProtocol, ErrSnapshotIntegrity)
		}
		m.DecodedSize, m.Digest = 0, [32]byte{}
	}
	if _, err := validateMessage(m); err != nil {
		return nil, err
	}
	return m, nil
}

func marshalInput(m InputBatch) []byte {
	frame, b := newMessageFrame(KindInput, inputBodyHeaderBytes+len(m.Inputs)*inputCommandBytes)
	binary.LittleEndian.PutUint64(b[0:8], m.Epoch)
	binary.LittleEndian.PutUint32(b[8:12], m.SnapshotAck)
	b[12] = byte(len(m.Inputs))
	for i, input := range m.Inputs {
		command := b[inputBodyHeaderBytes+i*inputCommandBytes:]
		binary.LittleEndian.PutUint32(command[0:4], input.Sequence)
		binary.LittleEndian.PutUint32(command[4:8], input.Tick)
		command[8], command[9] = byte(input.Command.Forward), byte(input.Command.Side)
		binary.LittleEndian.PutUint16(command[10:12], uint16(input.Command.AngleTurn))
		command[12] = input.Command.Buttons
	}
	return frame
}

func decodeInput(b []byte) (any, error) {
	if len(b) < inputBodyHeaderBytes || b[12] > MaxInputBatch || len(b) != inputBodyHeaderBytes+int(b[12])*inputCommandBytes {
		return nil, ErrProtocol
	}
	m := InputBatch{Epoch: binary.LittleEndian.Uint64(b[0:8]), SnapshotAck: binary.LittleEndian.Uint32(b[8:12]), Inputs: make([]Input, int(b[12]))}
	for i := range m.Inputs {
		command := b[inputBodyHeaderBytes+i*inputCommandBytes:]
		m.Inputs[i] = Input{Sequence: binary.LittleEndian.Uint32(command[0:4]), Tick: binary.LittleEndian.Uint32(command[4:8]), Command: demo.Tic{
			Forward: int8(command[8]), Side: int8(command[9]), AngleTurn: int16(binary.LittleEndian.Uint16(command[10:12])), Buttons: command[12],
		}}
	}
	if _, err := validateMessage(m); err != nil {
		return nil, err
	}
	return m, nil
}
