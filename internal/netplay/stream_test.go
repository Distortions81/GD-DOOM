package netplay

import (
	"bytes"
	"io"
	"net"
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

func readAudioConfig(t *testing.T, v *AudioViewer) (AudioConfig, bool, error) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok, err := v.PollAudioConfig()
		if err != nil && err != io.EOF {
			return AudioConfig{}, false, err
		}
		if ok {
			return got, true, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return AudioConfig{}, false, nil
}

func readAudioChunk(t *testing.T, v *AudioViewer) (AudioChunk, bool, error) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok, err := v.PollAudioChunk()
		if err != nil && err != io.EOF {
			return AudioChunk{}, false, err
		}
		if ok {
			return got, true, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return AudioChunk{}, false, nil
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

	want := []demo.Tic{
		{Forward: 25, Side: -5, AngleTurn: 512, Buttons: demo.ButtonAttack | demo.ButtonUse},
		{Forward: 10},
		{Side: -7},
		{Buttons: demo.ButtonUse},
	}
	for i, tc := range want {
		if err := b.BroadcastTic(tc); err != nil {
			t.Fatalf("BroadcastTic(%d) error = %v", i, err)
		}
	}

	for i, tc := range want {
		got, ok, err := readViewerTic(t, v)
		if err != nil {
			t.Fatalf("PollTic(%d) error = %v", i, err)
		}
		if !ok {
			t.Fatalf("timed out waiting for tic %d", i)
		}
		if got != tc {
			t.Fatalf("PollTic(%d) = %+v want %+v", i, got, tc)
		}
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
			if kf.MandatoryApply {
				t.Fatal("join keyframe marked mandatory, want false")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for keyframe")
}

func TestRelayViewerReceivesLegacyRawKeyframe(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	bconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial broadcaster: %v", err)
	}
	defer bconn.Close()
	if err := writeHello(bconn, helloRoleBroadcaster, 0, 0, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	_, _, sessionID, _, err := readHello(bconn)
	if err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}

	want := []byte{9, 8, 7}
	if err := writeFrame(bconn, frameHeader{
		Type:   frameTypeKeyframe,
		Flags:  0,
		Length: uint32(len(want)),
		Tic:    35,
	}, want); err != nil {
		t.Fatalf("writeFrame(keyframe) error = %v", err)
	}

	v, err := DialRelayViewer(srv.Addr(), sessionID, "")
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
			if !bytes.Equal(kf.Blob, want) {
				t.Fatalf("keyframe blob=%v want=%v", kf.Blob, want)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for legacy raw keyframe")
}

func TestRelayViewerMandatoryKeyframeDrainsQueuedTics(t *testing.T) {
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

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	stale := demo.Tic{Forward: 10}
	if err := b.BroadcastTic(stale); err != nil {
		t.Fatalf("BroadcastTic(stale) error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := b.BroadcastKeyframeWithFlags(99, []byte{1, 2, 3}, keyframeFlagMandatoryApply); err != nil {
		t.Fatalf("BroadcastKeyframeWithFlags() error = %v", err)
	}
	want := []demo.Tic{
		{Forward: 25, Buttons: demo.ButtonUse},
		{Forward: 26},
		{Forward: 27},
		{Forward: 28},
	}
	for i, tc := range want {
		if err := b.BroadcastTic(tc); err != nil {
			t.Fatalf("BroadcastTic(want[%d]) error = %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		kf, ok, err := v.PollKeyframe()
		if err != nil && err != io.EOF {
			t.Fatalf("PollKeyframe() error = %v", err)
		}
		if ok {
			if !kf.MandatoryApply {
				t.Fatal("mandatory keyframe flag missing")
			}
			if kf.Tic != 99 {
				t.Fatalf("keyframe tic=%d want=99", kf.Tic)
			}
			got, ok, err := readViewerTic(t, v)
			if err != nil {
				t.Fatalf("readViewerTic() error = %v", err)
			}
			if !ok {
				t.Fatal("timed out waiting for tic after mandatory keyframe")
			}
			if got != want[0] {
				t.Fatalf("PollTic() = %+v want %+v", got, want[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for mandatory keyframe")
}

func TestRelayBroadcasterFlushesPartialTicBatchBeforeKeyframe(t *testing.T) {
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

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	want := demo.Tic{Forward: 7, Buttons: demo.ButtonAttack}
	if err := b.BroadcastTic(want); err != nil {
		t.Fatalf("BroadcastTic() error = %v", err)
	}
	if err := b.BroadcastKeyframe(12, []byte{1, 2, 3}); err != nil {
		t.Fatalf("BroadcastKeyframe() error = %v", err)
	}

	got, ok, err := readViewerTic(t, v)
	if err != nil {
		t.Fatalf("PollTic() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for flushed tic")
	}
	if got != want {
		t.Fatalf("PollTic() = %+v want %+v", got, want)
	}
}

func TestRelayBroadcasterLowLatencyFlushesEveryTic(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{MapName: "E1M1"})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	b.SetLowLatency(true)
	defer b.Close()

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	want := demo.Tic{Forward: 13, Buttons: demo.ButtonUse}
	if err := b.BroadcastTic(want); err != nil {
		t.Fatalf("BroadcastTic() error = %v", err)
	}

	got, ok, err := readViewerTic(t, v)
	if err != nil {
		t.Fatalf("PollTic() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for immediate low-latency tic")
	}
	if got != want {
		t.Fatalf("PollTic() = %+v want %+v", got, want)
	}
}

func TestRelayViewerReceivesIntermissionAdvance(t *testing.T) {
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

	v, err := DialRelayViewer(srv.Addr(), b.SessionID(), "")
	if err != nil {
		t.Fatalf("DialRelayViewer() error = %v", err)
	}
	defer v.Close()

	if err := b.BroadcastIntermissionAdvance(); err != nil {
		t.Fatalf("BroadcastIntermissionAdvance() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := v.PollIntermissionAdvance()
		if err != nil && err != io.EOF {
			t.Fatalf("PollIntermissionAdvance() error = %v", err)
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for intermission advance")
}

func TestRelayAudioRoundTrip(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{
		WADHash: "abc123",
		MapName: "E1M1",
	})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()

	ab, err := DialRelayAudioBroadcaster(srv.Addr(), b.SessionID())
	if err != nil {
		t.Fatalf("DialRelayAudioBroadcaster() error = %v", err)
	}
	defer ab.Close()

	v, err := DialRelayAudioViewer(srv.Addr(), b.SessionID(), "abc123")
	if err != nil {
		t.Fatalf("DialRelayAudioViewer() error = %v", err)
	}
	defer v.Close()

	wantCfg := AudioConfig{
		Codec:        audioCodecIMA4To1,
		SampleRate:   48000,
		Channels:     1,
		FrameSamples: 960,
		Bitrate:      192000,
	}
	if err := ab.BroadcastAudioConfig(wantCfg); err != nil {
		t.Fatalf("BroadcastAudioConfig() error = %v", err)
	}

	wantChunk := AudioChunk{
		GameTic:     77,
		StartSample: 960 * 4,
		Payload:     []byte{1, 2, 3, 4, 5},
	}
	if err := ab.BroadcastAudioChunk(wantChunk); err != nil {
		t.Fatalf("BroadcastAudioChunk() error = %v", err)
	}

	gotCfg, ok, err := readAudioConfig(t, v)
	if err != nil {
		t.Fatalf("PollAudioConfig() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for audio config")
	}
	if gotCfg != wantCfg {
		t.Fatalf("PollAudioConfig() = %+v want %+v", gotCfg, wantCfg)
	}

	gotChunk, ok, err := readAudioChunk(t, v)
	if err != nil {
		t.Fatalf("PollAudioChunk() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for audio chunk")
	}
	if gotChunk.GameTic != wantChunk.GameTic || gotChunk.StartSample != wantChunk.StartSample || gotChunk.Silence != wantChunk.Silence || !bytes.Equal(gotChunk.Payload, wantChunk.Payload) {
		t.Fatalf("PollAudioChunk() = %+v want %+v", gotChunk, wantChunk)
	}
}

func TestRelayAudioSilenceChunkRoundTrip(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{
		WADHash: "abc123",
		MapName: "E1M1",
	})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()

	ab, err := DialRelayAudioBroadcaster(srv.Addr(), b.SessionID())
	if err != nil {
		t.Fatalf("DialRelayAudioBroadcaster() error = %v", err)
	}
	defer ab.Close()

	v, err := DialRelayAudioViewer(srv.Addr(), b.SessionID(), "abc123")
	if err != nil {
		t.Fatalf("DialRelayAudioViewer() error = %v", err)
	}
	defer v.Close()

	if err := ab.BroadcastAudioConfig(AudioConfig{
		Codec:        audioCodecPCM16Mono,
		SampleRate:   48000,
		Channels:     1,
		FrameSamples: 960,
		Bitrate:      768000,
	}); err != nil {
		t.Fatalf("BroadcastAudioConfig() error = %v", err)
	}
	wantChunk := AudioChunk{
		GameTic:     88,
		StartSample: 960 * 7,
		Silence:     true,
	}
	if err := ab.BroadcastAudioChunk(wantChunk); err != nil {
		t.Fatalf("BroadcastAudioChunk() error = %v", err)
	}

	if _, ok, err := readAudioConfig(t, v); err != nil {
		t.Fatalf("PollAudioConfig() error = %v", err)
	} else if !ok {
		t.Fatal("timed out waiting for audio config")
	}
	gotChunk, ok, err := readAudioChunk(t, v)
	if err != nil {
		t.Fatalf("PollAudioChunk() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for audio chunk")
	}
	if gotChunk.GameTic != wantChunk.GameTic || gotChunk.StartSample != wantChunk.StartSample || !gotChunk.Silence || len(gotChunk.Payload) != 0 {
		t.Fatalf("PollAudioChunk() = %+v want silence chunk %+v", gotChunk, wantChunk)
	}
}

func TestRelayAudioLateJoinDoesNotReceiveStaleChunks(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	b, err := DialRelayBroadcaster(srv.Addr(), 0, SessionConfig{
		WADHash: "abc123",
		MapName: "E1M1",
	})
	if err != nil {
		t.Fatalf("DialRelayBroadcaster() error = %v", err)
	}
	defer b.Close()

	ab, err := DialRelayAudioBroadcaster(srv.Addr(), b.SessionID())
	if err != nil {
		t.Fatalf("DialRelayAudioBroadcaster() error = %v", err)
	}
	defer ab.Close()

	wantCfg := AudioConfig{
		Codec:        audioCodecPCM16Mono,
		SampleRate:   48000,
		Channels:     1,
		FrameSamples: 480,
		Bitrate:      768000,
	}
	if err := ab.BroadcastAudioConfig(wantCfg); err != nil {
		t.Fatalf("BroadcastAudioConfig() error = %v", err)
	}
	if err := ab.BroadcastAudioChunk(AudioChunk{
		GameTic:     77,
		StartSample: 0,
		Payload:     []byte{1, 2, 3, 4},
	}); err != nil {
		t.Fatalf("BroadcastAudioChunk() error = %v", err)
	}

	v, err := DialRelayAudioViewer(srv.Addr(), b.SessionID(), "abc123")
	if err != nil {
		t.Fatalf("DialRelayAudioViewer() error = %v", err)
	}
	defer v.Close()

	gotCfg, ok, err := readAudioConfig(t, v)
	if err != nil {
		t.Fatalf("PollAudioConfig() error = %v", err)
	}
	if !ok {
		t.Fatal("timed out waiting for audio config")
	}
	if gotCfg != wantCfg {
		t.Fatalf("PollAudioConfig() = %+v want %+v", gotCfg, wantCfg)
	}

	time.Sleep(100 * time.Millisecond)
	if got, ok, err := v.PollAudioChunk(); err != nil && err != io.EOF {
		t.Fatalf("PollAudioChunk() error = %v", err)
	} else if ok {
		t.Fatalf("PollAudioChunk() = %+v want no stale chunk", got)
	}
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
