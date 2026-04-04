package netplay

import (
	"bytes"
	"io"
	"testing"
	"time"

	"gddoom/internal/demo"
)

func readViewerTic(t *testing.T, v *Viewer) (demo.Tic, bool, error) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok, err := v.PollTic()
		if err != nil && err != io.EOF {
			return demo.Tic{}, false, err
		}
		if ok {
			return got, true, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return demo.Tic{}, false, nil
}

func TestRelayBroadcastWatchRoundTrip(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{
		WADHash:          "abc123",
		MapName:          "E1M1",
		PlayerSlot:       1,
		SkillLevel:       3,
		GameMode:         "single",
		AutoWeaponSwitch: true,
	})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "abc123")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	if got := v.Session().MapName; got != "E1M1" {
		t.Fatalf("Session().MapName = %q want %q", got, "E1M1")
	}
	if got := v.Session().GameMode; got != "single" {
		t.Fatalf("Session().GameMode = %q want %q", got, "single")
	}

	want := demo.Tic{Forward: 25, Side: -5, AngleTurn: 512, Buttons: demo.ButtonAttack | demo.ButtonUse}
	if err := b.BroadcastTic(want); err != nil {
		t.Fatalf("BroadcastTic() error = %v", err)
	}

	got, ok, err := readViewerTic(t, v)
	if err != nil {
		t.Fatalf("PollTic() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for tic")
	}
	if got != want {
		t.Fatalf("PollTic() = %+v want %+v", got, want)
	}
}

func TestRelayWatchRejectsWADHashMismatch(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{WADHash: "host"})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()

	if _, err := DialRelayViewer(srv.Addr(), b.SessionID(), "local"); err == nil {
		t.Fatal("DialRelayViewer() error = nil want mismatch")
	}
}

func TestRelayViewerReceivesKeyframe(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{MapName: "E1M1"})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()
	if err := b.BroadcastKeyframe(35, []byte{9, 8, 7}); err != nil {
		t.Fatalf("BroadcastKeyframe() error = %v", err)
	}

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		kf, ok, err := v.PollKeyframe()
		if err != nil && err != io.EOF {
			t.Fatalf("PollKeyframe() error = %v", err)
		}
		if ok {
			if kf.Tic != 35 {
				t.Fatalf("keyframe tic=%d want=35", kf.Tic)
			}
			if !bytes.Equal(kf.Blob, []byte{9, 8, 7}) {
				t.Fatalf("keyframe blob=%v want=%v", kf.Blob, []byte{9, 8, 7})
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for keyframe")
}

func TestHelloRoundTripBinary(t *testing.T) {
	var buf bytes.Buffer
	want := SessionConfig{
		WADHash:          "abc123",
		MapName:          "E1M1",
		PlayerSlot:       1,
		SkillLevel:       3,
		GameMode:         "single",
		ShowNoSkillItems: true,
		FastMonsters:     true,
		AutoWeaponSwitch: true,
		CheatLevel:       2,
		Invulnerable:     true,
		SourcePortMode:   true,
	}
	if err := writeHello(&buf, helloRoleBroadcaster, 7, 42, want); err != nil {
		t.Fatalf("writeHello() error = %v", err)
	}
	role, flags, sessionID, got, err := readHello(&buf)
	if err != nil {
		t.Fatalf("readHello() error = %v", err)
	}
	if role != helloRoleBroadcaster {
		t.Fatalf("role=%d want=%d", role, helloRoleBroadcaster)
	}
	if flags != 7 {
		t.Fatalf("flags=%d want=7", flags)
	}
	if sessionID != 42 {
		t.Fatalf("sessionID=%d want=42", sessionID)
	}
	if got != want {
		t.Fatalf("session=%+v want %+v", got, want)
	}
}

func TestReadHelloRejectsBadMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("NOPE")
	buf.Write([]byte{protocolVersion, helloRoleBroadcaster, 0, 0})
	buf.Write(make([]byte, 12))
	if _, _, _, _, err := readHello(&buf); err == nil {
		t.Fatal("readHello() error = nil want bad magic")
	}
}

func TestFrameRoundTripBinary(t *testing.T) {
	var buf bytes.Buffer
	header := frameHeader{Type: frameTypeTicBatch, Flags: 3, Tic: 99}
	payload := []byte{1, 0, 0, 0, 25, 0, 2, demo.ButtonUse}
	if err := writeFrame(&buf, header, payload); err != nil {
		t.Fatalf("writeFrame() error = %v", err)
	}
	gotHeader, gotPayload, err := readFrame(&buf)
	if err != nil {
		t.Fatalf("readFrame() error = %v", err)
	}
	if gotHeader.Type != header.Type || gotHeader.Flags != header.Flags || gotHeader.Tic != header.Tic {
		t.Fatalf("header=%+v want %+v", gotHeader, header)
	}
	if gotHeader.Length != uint32(len(payload)) {
		t.Fatalf("length=%d want=%d", gotHeader.Length, len(payload))
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("payload=%v want=%v", gotPayload, payload)
	}
}

func TestPackDemoTicMatchesDemoFormatRounding(t *testing.T) {
	tc := demo.Tic{Forward: 1, Side: -2, AngleTurn: 129, Buttons: demo.ButtonUse}
	got := packDemoTic(tc)
	want := []byte{1, 0xfe, 1, demo.ButtonUse}
	if !bytes.Equal(got, want) {
		t.Fatalf("packDemoTic()=%v want=%v", got, want)
	}
}
