package netgame

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"

	"gddoom/internal/demo"
)

func protocolExamples() []any {
	return []any{
		Ping{Nonce: 1}, Pong{Nonce: math.MaxUint64},
		Hello{Compatibility: "engine:wad:rules", Name: "Doom marine 🌍"},
		Welcome{Epoch: 12, PlayerID: 4, ServerTick: 12345, InputLead: 3},
		InputBatch{Epoch: 12, SnapshotAck: 42, Inputs: []Input{}},
		InputBatch{Epoch: 12, SnapshotAck: 42, Inputs: []Input{
			{Sequence: 100, Tick: 12346, Command: demo.Tic{Forward: -50, Side: 80, AngleTurn: math.MinInt16, Buttons: 3}},
			{Sequence: 101, Tick: 12347, Command: demo.Tic{Forward: 50, Side: -80, AngleTurn: math.MaxInt16, Buttons: 60}},
		}},
		Snapshot{Epoch: 12, ID: 1, Tick: 0, State: []byte{1, 2, 3}},
		Snapshot{Epoch: 12, ID: 43, BaselineID: 42, Tick: 12348, Encoding: SnapshotDeltaZstd, DecodedSize: 3, Digest: [32]byte{1},
			Finalized: InputAck{Tick: 12348, Sequence: 101, HasTick: true, HasSequence: true}, State: []byte{0xff, 0, 1}},
		Snapshot{Epoch: 12, ID: 44, Tick: 12349,
			Finalized: InputAck{Tick: 12349, HasTick: true}, State: []byte{9}},
		Disconnect{Reason: "Map changed"},
		MapChange{PreviousEpoch: 12, Welcome: Welcome{Epoch: 13, PlayerID: 4, InputLead: 3}, Map: "E1M2", Compatibility: "engine:wad:next-map:rules"},
	}
}

func marshalProtocol(t *testing.T, message any) []byte {
	t.Helper()
	frame, err := MarshalMessage(message)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func protocolFrame(kind MessageKind, body []byte) []byte {
	frame := make([]byte, messageHeaderBytes+len(body))
	copy(frame, "GDMP")
	frame[4], frame[5] = ProtocolVersion, byte(kind)
	binary.LittleEndian.PutUint32(frame[6:10], uint32(len(body)))
	copy(frame[messageHeaderBytes:], body)
	return frame
}

func TestProtocolRoundTrip(t *testing.T) {
	for _, message := range protocolExamples() {
		t.Run(reflect.TypeOf(message).Name(), func(t *testing.T) {
			frame := marshalProtocol(t, message)
			decoded, err := UnmarshalMessage(frame)
			if err != nil || !reflect.DeepEqual(decoded, message) {
				t.Fatalf("decoded = %#v, error = %v, want %#v", decoded, err, message)
			}
			encoded := marshalProtocol(t, decoded)
			if !bytes.Equal(frame, encoded) {
				t.Fatal("roundtrip changed canonical wire bytes")
			}
		})
	}
}

func TestProtocolWelcomeGoldenFrame(t *testing.T) {
	// Header, body length 51, epoch 12, player 4, tic 12345, lead 3,
	// followed by a disabled (all zero) resume token and grace period.
	want, err := hex.DecodeString("47444d500202330000000c0000000000000004393000000300" + strings.Repeat("00", 36))
	if err != nil {
		t.Fatal(err)
	}
	got := marshalProtocol(t, Welcome{Epoch: 12, PlayerID: 4, ServerTick: 12345, InputLead: 3})
	if !bytes.Equal(got, want) {
		t.Fatalf("wire = %x, want %x", got, want)
	}
}

func TestProtocolTruncationAndTrailingData(t *testing.T) {
	for _, message := range protocolExamples() {
		t.Run(reflect.TypeOf(message).Name(), func(t *testing.T) {
			frame := marshalProtocol(t, message)
			for end := 0; end < len(frame); end++ {
				if _, err := UnmarshalMessage(frame[:end]); err == nil {
					t.Fatalf("accepted truncated frame at byte %d", end)
				}
				if end >= messageHeaderBytes {
					shortBody := protocolFrame(MessageKind(frame[5]), frame[messageHeaderBytes:end])
					if _, err := UnmarshalMessage(shortBody); err == nil {
						t.Fatalf("accepted truncated body at byte %d with corrected frame length", end)
					}
				}
			}
			if _, err := UnmarshalMessage(append(bytes.Clone(frame), 0)); !errors.Is(err, ErrProtocol) {
				t.Fatalf("trailing datagram data error = %v", err)
			}
			body := append(bytes.Clone(frame[messageHeaderBytes:]), 0)
			if _, err := UnmarshalMessage(protocolFrame(MessageKind(frame[5]), body)); !errors.Is(err, ErrProtocol) {
				t.Fatalf("trailing inner payload data error = %v", err)
			}
		})
	}
}

type bytewiseReader struct{ io.Reader }

func (r bytewiseReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

func TestProtocolStreamBoundaries(t *testing.T) {
	var stream bytes.Buffer
	for _, message := range protocolExamples() {
		stream.Write(marshalProtocol(t, message))
	}
	for _, want := range protocolExamples() {
		got, err := ReadMessage(bytewiseReader{&stream})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("stream decoded = %#v, error = %v, want %#v", got, err, want)
		}
	}
	if _, err := ReadMessage(&stream); !errors.Is(err, io.EOF) {
		t.Fatalf("empty stream error = %v", err)
	}
	frame := marshalProtocol(t, Hello{Compatibility: "build", Name: "player"})
	if _, err := UnmarshalMessage(append(bytes.Clone(frame), frame...)); !errors.Is(err, ErrProtocol) {
		t.Fatalf("multiple datagram frames error = %v", err)
	}
}

func TestProtocolRejectsInvalidValuesOnEncode(t *testing.T) {
	validSnapshot := Snapshot{Epoch: 1, ID: 10, Tick: 20, State: []byte{1}}
	withSnapshot := func(change func(*Snapshot)) Snapshot {
		m := validSnapshot
		change(&m)
		return m
	}
	invalid := []any{
		nil, &Hello{}, struct{}{}, Ping{}, Pong{},
		Hello{Name: "player"}, Hello{Compatibility: "build"},
		Hello{Compatibility: strings.Repeat("x", 257), Name: "player"},
		Hello{Compatibility: "build", Name: strings.Repeat("x", 65)},
		Hello{Compatibility: "\xff", Name: "player"}, Hello{Compatibility: "build", Name: "\xff"},
		Welcome{PlayerID: 1}, Welcome{Epoch: 1, PlayerID: MaxPlayers + 1},
		Welcome{Epoch: 1, PlayerID: 1, InputLead: TickRate + 1},
		InputBatch{}, InputBatch{Epoch: 1, Inputs: make([]Input, MaxInputBatch+1)},
		InputBatch{Epoch: 1, Inputs: []Input{{Command: demo.Tic{Forward: 51}}}},
		InputBatch{Epoch: 1, Inputs: []Input{{Command: demo.Tic{Buttons: demo.ButtonSpecial}}}},
		withSnapshot(func(m *Snapshot) { m.Epoch = 0 }),
		withSnapshot(func(m *Snapshot) { m.ID = 0 }),
		withSnapshot(func(m *Snapshot) { m.BaselineID = m.ID }),
		withSnapshot(func(m *Snapshot) { m.BaselineID = m.ID + 1 }),
		withSnapshot(func(m *Snapshot) { m.State = nil }),
		withSnapshot(func(m *Snapshot) { m.State = make([]byte, MaxSnapshotBytes+1) }),
		withSnapshot(func(m *Snapshot) { m.Finalized = InputAck{Tick: 21, HasTick: true} }),
		withSnapshot(func(m *Snapshot) { m.Finalized = InputAck{HasSequence: true} }),
		withSnapshot(func(m *Snapshot) { m.Finalized.Tick = 1 }),
		withSnapshot(func(m *Snapshot) { m.Finalized.Sequence = 1 }),
		Disconnect{}, Disconnect{Reason: strings.Repeat("x", 257)}, Disconnect{Reason: "\xff"},
	}
	for _, message := range invalid {
		if _, err := MarshalMessage(message); !errors.Is(err, ErrProtocol) {
			t.Fatalf("accepted invalid %T: error %v", message, err)
		}
	}
}

func TestProtocolRejectsMalformedWireValues(t *testing.T) {
	hello := marshalProtocol(t, Hello{Compatibility: "build", Name: "player"})
	welcome := marshalProtocol(t, Welcome{Epoch: 1, PlayerID: 1})
	input := marshalProtocol(t, InputBatch{Epoch: 1, Inputs: []Input{{}}})
	snapshot := marshalProtocol(t, Snapshot{Epoch: 1, ID: 2, Tick: 10, State: []byte{1}})
	corrupt := func(frame []byte, modify func([]byte)) []byte {
		frame = bytes.Clone(frame)
		modify(frame)
		return frame
	}
	corruptBody := func(frame []byte, modify func([]byte)) []byte {
		return corrupt(frame, func(frame []byte) { modify(frame[messageHeaderBytes:]) })
	}
	tests := map[string][]byte{
		"magic":                          corrupt(hello, func(b []byte) { b[0] = 'X' }),
		"version":                        corrupt(hello, func(b []byte) { b[4] = ProtocolVersion + 1 }),
		"kind":                           corrupt(hello, func(b []byte) { b[5] = 0xff }),
		"zero body":                      corrupt(hello, func(b []byte) { binary.LittleEndian.PutUint32(b[6:10], 0) }),
		"huge body":                      corrupt(hello, func(b []byte) { binary.LittleEndian.PutUint32(b[6:10], math.MaxUint32) }),
		"zero string":                    corruptBody(hello, func(b []byte) { binary.LittleEndian.PutUint16(b[:2], 0) }),
		"large string":                   corruptBody(hello, func(b []byte) { binary.LittleEndian.PutUint16(b[:2], 257) }),
		"utf8":                           corruptBody(hello, func(b []byte) { b[2] = 0xff }),
		"epoch":                          corruptBody(welcome, func(b []byte) { b[0] = 0 }),
		"slot":                           corruptBody(welcome, func(b []byte) { b[8] = MaxPlayers + 1 }),
		"lead":                           corruptBody(welcome, func(b []byte) { binary.LittleEndian.PutUint16(b[13:15], TickRate+1) }),
		"input count":                    corruptBody(input, func(b []byte) { b[12] = MaxInputBatch + 1 }),
		"input count mismatch":           corruptBody(input, func(b []byte) { b[12] = 0 }),
		"input movement":                 corruptBody(input, func(b []byte) { b[21] = 51 }),
		"input buttons":                  corruptBody(input, func(b []byte) { b[25] = demo.ButtonSpecial }),
		"snapshot flags":                 corruptBody(snapshot, func(b []byte) { b[20] = 0x80 }),
		"snapshot sequence without tick": corruptBody(snapshot, func(b []byte) { b[20] = 2 }),
		"snapshot unflagged tick":        corruptBody(snapshot, func(b []byte) { b[21] = 1 }),
		"snapshot unflagged sequence":    corruptBody(snapshot, func(b []byte) { b[25] = 1 }),
		"snapshot ack future":            corruptBody(snapshot, func(b []byte) { b[20], b[21] = 1, 11 }),
		"snapshot no state":              corruptBody(snapshot, func(b []byte) { binary.LittleEndian.PutUint32(b[66:70], 0) }),
		"snapshot state length":          corruptBody(snapshot, func(b []byte) { binary.LittleEndian.PutUint32(b[66:70], 2) }),
		"snapshot huge state":            corruptBody(snapshot, func(b []byte) { binary.LittleEndian.PutUint32(b[66:70], math.MaxUint32) }),
		"snapshot encoding":              corruptBody(snapshot, func(b []byte) { b[29] = 3 }),
		"snapshot raw size":              corruptBody(snapshot, func(b []byte) { binary.LittleEndian.PutUint32(b[30:34], 2) }),
		"snapshot digest":                corruptBody(snapshot, func(b []byte) { b[34] ^= 1 }),
		"snapshot baseline":              corruptBody(snapshot, func(b []byte) { binary.LittleEndian.PutUint32(b[12:16], 2) }),
	}
	for name, frame := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := UnmarshalMessage(frame); !errors.Is(err, ErrProtocol) {
				t.Fatalf("malformed frame error = %v", err)
			}
		})
	}
}

func TestProtocolMaximumLegalSizes(t *testing.T) {
	for _, message := range []any{
		Hello{Compatibility: strings.Repeat("x", 256), Name: strings.Repeat("x", 64)},
		InputBatch{Epoch: 1, Inputs: make([]Input, MaxInputBatch)},
		Snapshot{Epoch: 1, ID: 1, State: bytes.Repeat([]byte{0x75}, MaxSnapshotBytes)},
		Disconnect{Reason: strings.Repeat("x", 256)},
	} {
		frame := marshalProtocol(t, message)
		decoded, err := UnmarshalMessage(frame)
		if err != nil || !reflect.DeepEqual(decoded, message) {
			t.Fatalf("maximum-size %T failed: %v", message, err)
		}
	}
}

func TestProtocolHeaderBoundsBeforePayloadRead(t *testing.T) {
	for kind, maximum := range map[MessageKind]uint32{
		KindHello: 357, KindWelcome: welcomeBodyBytes, KindPing: 8, KindPong: 8, KindInput: inputBodyHeaderBytes + MaxInputBatch*inputCommandBytes,
		KindSnapshot: snapshotBodyHeaderBytes + MaxSnapshotBytes, KindDisconnect: 258, KindMapChange: 8 + welcomeBodyBytes + 2 + 32 + 2 + 256,
	} {
		header := protocolFrame(kind, nil)
		binary.LittleEndian.PutUint32(header[6:10], maximum+1)
		// There is no body: an attempted payload read would return EOF. The
		// required ErrProtocol proves the size was rejected from the header.
		if _, err := ReadMessage(bytes.NewReader(header)); !errors.Is(err, ErrProtocol) {
			t.Fatalf("kind %d oversize header error = %v", kind, err)
		}
	}
}

func TestProtocolClientReaderRejectsServerFramesBeforePayload(t *testing.T) {
	for _, message := range protocolExamples() {
		frame := marshalProtocol(t, message)
		switch message.(type) {
		case Snapshot, Welcome, MapChange, Pong:
			if _, err := ReadClientMessage(bytes.NewReader(frame[:messageHeaderBytes])); !errors.Is(err, ErrProtocol) {
				t.Fatalf("server-only %T payload read attempted: %v", message, err)
			}
		default:
			got, err := ReadClientMessage(bytes.NewReader(frame))
			if err != nil || !reflect.DeepEqual(got, message) {
				t.Fatalf("client %T rejected: %v", message, err)
			}
		}
	}
}

func TestProtocolSnapshotTruncatedReadCannotBeCleared(t *testing.T) {
	frame := marshalProtocol(t, Snapshot{Epoch: 1, ID: 1, State: []byte{1}})
	for end := 0; end < snapshotBodyHeaderBytes; end++ {
		if _, err := decodeBody(KindSnapshot, frame[messageHeaderBytes:messageHeaderBytes+end]); err == nil {
			t.Fatalf("short snapshot fixed fields accepted at %d bytes", end)
		}
	}
}

func FuzzUnmarshalMessage(f *testing.F) {
	for _, message := range protocolExamples() {
		frame, err := MarshalMessage(message)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(frame)
	}
	f.Add([]byte{})
	f.Add([]byte("GDMP"))
	f.Fuzz(func(t *testing.T, frame []byte) {
		message, err := UnmarshalMessage(frame)
		if err != nil {
			return
		}
		encoded, err := MarshalMessage(message)
		if err != nil || !bytes.Equal(frame, encoded) {
			t.Fatalf("accepted noncanonical frame: encode error %v", err)
		}
	})
}
