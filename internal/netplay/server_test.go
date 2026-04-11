package netplay

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"gddoom/internal/demo"
)

func TestRelayServerForwardsBroadcastFramesToViewer(t *testing.T) {
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
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 99, SessionConfig{
		WADHash:  "abc123",
		MapName:  "E1M1",
		GameMode: "single",
	}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	role, _, sessionID, _, err := readHello(bconn)
	if err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}
	if role != helloRoleServer {
		t.Fatalf("ack role=%d want=%d", role, helloRoleServer)
	}
	if sessionID != 99 {
		t.Fatalf("ack sessionID=%d want=99", sessionID)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 99, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}

	role, _, sessionID, session, err := readHello(vconn)
	if err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}
	if role != helloRoleBroadcaster {
		t.Fatalf("role=%d want=%d", role, helloRoleBroadcaster)
	}
	if sessionID != 99 {
		t.Fatalf("sessionID=%d want=99", sessionID)
	}
	if session.MapName != "E1M1" {
		t.Fatalf("MapName=%q want=E1M1", session.MapName)
	}

	tc := demo.Tic{Forward: 25, Side: -5, AngleTurn: 512, Buttons: demo.ButtonUse}
	payload := make([]byte, ticBatchOverhead+4)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	copy(payload[ticBatchOverhead:], packDemoTic(tc))
	if err := writeFrame(bconn, frameHeader{Type: frameTypeTicBatch, Tic: 7}, payload); err != nil {
		t.Fatalf("writeFrame broadcaster: %v", err)
	}

	header, gotPayload, err := readFrame(vconn)
	if err != nil {
		t.Fatalf("readFrame viewer: %v", err)
	}
	if header.Type != frameTypeTicBatch {
		t.Fatalf("header=%+v want type=%d", header, frameTypeTicBatch)
	}
	if len(gotPayload) != len(payload) {
		t.Fatalf("payload len=%d want=%d", len(gotPayload), len(payload))
	}
	if got := unpackDemoTic(gotPayload[ticBatchOverhead : ticBatchOverhead+4]); got != tc {
		t.Fatalf("tic=%+v want %+v", got, tc)
	}
}

func TestRelayServerReplaysLatestKeyframeToNewViewer(t *testing.T) {
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
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 88, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	if _, _, _, _, err := readHello(bconn); err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}

	keyframe := []byte{1, 2, 3, 4}
	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Tic: 35}, keyframe); err != nil {
		t.Fatalf("writeFrame keyframe: %v", err)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 88, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}
	if _, _, _, _, err := readHello(vconn); err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}
	header, payload, err := readFrame(vconn)
	if err != nil {
		t.Fatalf("readFrame keyframe: %v", err)
	}
	if header.Type != frameTypeKeyframe || header.Tic != 35 {
		t.Fatalf("header=%+v want type=%d tic=35", header, frameTypeKeyframe)
	}
	if string(payload) != string(keyframe) {
		t.Fatalf("payload=%v want=%v", payload, keyframe)
	}
}

func TestRelayServerReplaysBufferedTicsAfterKeyframe(t *testing.T) {
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
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 91, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	if _, _, _, _, err := readHello(bconn); err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}
	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Tic: 0}, []byte{1}); err != nil {
		t.Fatalf("write keyframe: %v", err)
	}
	tc := demo.Tic{Forward: 25, AngleTurn: 512}
	payload := make([]byte, ticBatchOverhead+4)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	copy(payload[ticBatchOverhead:], packDemoTic(tc))
	if err := writeFrame(bconn, frameHeader{Type: frameTypeTicBatch, Tic: 1}, payload); err != nil {
		t.Fatalf("write tic batch: %v", err)
	}
	waitForBufferedRelayFrames(t, srv, 91, 1)

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 91, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}
	if _, _, _, _, err := readHello(vconn); err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}
	if _, _, err := readFrameWithDeadline(vconn, 2*time.Second); err != nil {
		t.Fatalf("read keyframe: %v", err)
	}
	header, gotPayload, err := readFrameWithDeadline(vconn, 2*time.Second)
	if err != nil {
		t.Fatalf("read replayed tic batch: %v", err)
	}
	if header.Type != frameTypeTicBatch {
		t.Fatalf("header=%+v want type=%d", header, frameTypeTicBatch)
	}
	if got := unpackDemoTic(gotPayload[ticBatchOverhead : ticBatchOverhead+4]); got != tc {
		t.Fatalf("tic=%+v want %+v", got, tc)
	}
}

func TestRelayServerDoesNotForwardPeriodicKeyframesToActiveViewer(t *testing.T) {
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
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 92, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	if _, _, _, _, err := readHello(bconn); err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}
	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Tic: 0}, []byte{1}); err != nil {
		t.Fatalf("write initial keyframe: %v", err)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 92, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}
	if _, _, _, _, err := readHello(vconn); err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}
	if _, _, err := readFrame(vconn); err != nil {
		t.Fatalf("read join keyframe: %v", err)
	}

	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Tic: 175}, []byte{2}); err != nil {
		t.Fatalf("write periodic keyframe: %v", err)
	}
	tc := demo.Tic{Forward: 25, AngleTurn: 512}
	payload := make([]byte, ticBatchOverhead+4)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	copy(payload[ticBatchOverhead:], packDemoTic(tc))
	if err := writeFrame(bconn, frameHeader{Type: frameTypeTicBatch, Tic: 176}, payload); err != nil {
		t.Fatalf("write tic batch: %v", err)
	}

	header, gotPayload, err := readFrame(vconn)
	if err != nil {
		t.Fatalf("read forwarded frame: %v", err)
	}
	if header.Type != frameTypeTicBatch {
		t.Fatalf("header=%+v want type=%d", header, frameTypeTicBatch)
	}
	if got := unpackDemoTic(gotPayload[ticBatchOverhead : ticBatchOverhead+4]); got != tc {
		t.Fatalf("tic=%+v want %+v", got, tc)
	}
}

func readFrameWithDeadline(conn net.Conn, d time.Duration) (frameHeader, []byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(d))
	defer conn.SetReadDeadline(time.Time{})
	return readFrame(conn)
}

func waitForBufferedRelayFrames(t *testing.T, srv *Server, sessionID uint64, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		sess := srv.sessions[sessionID]
		srv.mu.Unlock()
		if sess != nil {
			if got := len(srv.backlogFrames(sess)); got >= want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for session %d backlog >= %d", sessionID, want)
}

func TestRelayServerForwardsMandatoryKeyframesToActiveViewer(t *testing.T) {
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
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 93, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	if _, _, _, _, err := readHello(bconn); err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}
	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Tic: 0}, []byte{1}); err != nil {
		t.Fatalf("write initial keyframe: %v", err)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 93, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}
	if _, _, _, _, err := readHello(vconn); err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}
	if _, _, err := readFrame(vconn); err != nil {
		t.Fatalf("read join keyframe: %v", err)
	}

	keyframe := []byte{9, 8, 7}
	if err := writeFrame(bconn, frameHeader{Type: frameTypeKeyframe, Flags: keyframeFlagMandatoryApply, Tic: 200}, keyframe); err != nil {
		t.Fatalf("write mandatory keyframe: %v", err)
	}

	header, payload, err := readFrame(vconn)
	if err != nil {
		t.Fatalf("read forwarded keyframe: %v", err)
	}
	if header.Type != frameTypeKeyframe || header.Tic != 200 || header.Flags != keyframeFlagMandatoryApply {
		t.Fatalf("header=%+v want type=%d tic=200 flags=%d", header, frameTypeKeyframe, keyframeFlagMandatoryApply)
	}
	if string(payload) != string(keyframe) {
		t.Fatalf("payload=%v want=%v", payload, keyframe)
	}
}

func TestRelayServerAssignsSessionIDToBroadcaster(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial broadcaster: %v", err)
	}
	defer conn.Close()
	if err := writeHello(conn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 0, SessionConfig{MapName: "E1M1"}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}
	role, _, sessionID, session, err := readHello(conn)
	if err != nil {
		t.Fatalf("readHello broadcaster ack: %v", err)
	}
	if role != helloRoleServer {
		t.Fatalf("role=%d want=%d", role, helloRoleServer)
	}
	if sessionID == 0 {
		t.Fatal("sessionID=0 want assigned id")
	}
	if session.MapName != "E1M1" {
		t.Fatalf("MapName=%q want=E1M1", session.MapName)
	}
}

func TestRelayServerRejectsViewerForMissingSession(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer conn.Close()
	if err := writeHello(conn, helloRoleViewer, helloFlagGameplayCompactV1, 123, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}

	var buf [1]byte
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Read(buf[:])
	if err == nil {
		t.Fatal("expected closed connection")
	}
	var netErr net.Error
	if !errors.Is(err, io.EOF) && (!errors.As(err, &netErr) || !netErr.Timeout()) {
		t.Fatalf("unexpected read error: %v", err)
	}
}

func TestRelayServerClosesViewerWhenBroadcasterDisconnects(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer() error = %v", err)
	}
	defer srv.Close()

	bconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial broadcaster: %v", err)
	}
	if err := writeHello(bconn, helloRoleBroadcaster, helloFlagGameplayCompactV1, 77, SessionConfig{}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, helloFlagGameplayCompactV1, 77, SessionConfig{}); err != nil {
		t.Fatalf("writeHello viewer: %v", err)
	}
	if _, _, _, _, err := readHello(vconn); err != nil {
		t.Fatalf("readHello viewer response: %v", err)
	}

	_ = bconn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		var buf [1]byte
		_, err := vconn.Read(buf[:])
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("viewer connection remained open")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for viewer disconnect")
	}
}

// dialPeer connects a PlayerPeer to the server and returns the raw conn plus
// the assigned session/player info from the server ack hello.
func dialPeer(t *testing.T, addr string, sessionID uint64, slot int) (net.Conn, uint64, byte) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial peer: %v", err)
	}
	cfg := SessionConfig{WADHash: "abc", MapName: "E1M1", GameMode: "coop", PlayerSlot: slot}
	if err := writeHello(conn, helloRolePlayerPeer, helloFlagGameplayCompactV1, sessionID, cfg); err != nil {
		t.Fatalf("writeHello peer: %v", err)
	}
	role, _, assignedID, ack, err := readHello(conn)
	if err != nil {
		t.Fatalf("readHello peer ack: %v", err)
	}
	if role != helloRoleServer {
		t.Fatalf("peer ack role=%d want=%d", role, helloRoleServer)
	}
	return conn, assignedID, byte(ack.PlayerSlot)
}

func readPeerFrame(t *testing.T, conn net.Conn) (frameHeader, []byte) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	h, p, err := readFrame(conn)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	return h, p
}

func TestPlayerPeerTwoWayTicRouting(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer: %v", err)
	}
	defer srv.Close()

	// First peer creates session (sessionID=0 → assigned).
	conn1, sid, pid1 := dialPeer(t, srv.Addr(), 0, 1)
	defer conn1.Close()
	if sid == 0 {
		t.Fatal("expected non-zero session ID")
	}
	if pid1 != 1 {
		t.Fatalf("peer1 playerID=%d want=1", pid1)
	}

	// First peer receives its own roster update on join.
	h, p := readPeerFrame(t, conn1)
	if h.Type != frameTypeRosterUpdate {
		t.Fatalf("peer1 expected roster frame, got type=%d", h.Type)
	}
	roster1, err := unmarshalRosterUpdatePayload(p)
	if err != nil {
		t.Fatalf("unmarshal roster: %v", err)
	}
	if len(roster1.PlayerIDs) != 1 || roster1.PlayerIDs[0] != 1 {
		t.Fatalf("roster1 = %v want [1]", roster1.PlayerIDs)
	}

	// Second peer joins same session.
	conn2, sid2, pid2 := dialPeer(t, srv.Addr(), sid, 2)
	defer conn2.Close()
	if sid2 != sid {
		t.Fatalf("peer2 sessionID=%d want=%d", sid2, sid)
	}
	if pid2 != 2 {
		t.Fatalf("peer2 playerID=%d want=2", pid2)
	}

	// peer2 gets roster update (has both players).
	h, p = readPeerFrame(t, conn2)
	if h.Type != frameTypeRosterUpdate {
		t.Fatalf("peer2 expected roster frame, got type=%d", h.Type)
	}
	roster2, _ := unmarshalRosterUpdatePayload(p)
	if len(roster2.PlayerIDs) != 2 {
		t.Fatalf("roster2 len=%d want=2", len(roster2.PlayerIDs))
	}

	// peer1 also receives updated roster after peer2 joined.
	h, p = readPeerFrame(t, conn1)
	if h.Type != frameTypeRosterUpdate {
		t.Fatalf("peer1 expected roster update after peer2 join, got type=%d", h.Type)
	}
	roster1b, _ := unmarshalRosterUpdatePayload(p)
	if len(roster1b.PlayerIDs) != 2 {
		t.Fatalf("roster1b len=%d want=2", len(roster1b.PlayerIDs))
	}

	// peer1 sends a tic batch; peer2 should receive it tagged with player_id=1.
	tc := demo.Tic{Forward: 10, Side: -5, AngleTurn: 256, Buttons: 1}
	payload1 := marshalPeerTicBatchPayload(pid1, []demo.Tic{tc})
	if err := writeFrame(conn1, frameHeader{Type: frameTypePeerTicBatch}, payload1); err != nil {
		t.Fatalf("peer1 write tic: %v", err)
	}

	h, p = readPeerFrame(t, conn2)
	if h.Type != frameTypePeerTicBatch {
		t.Fatalf("peer2 got frame type=%d want=%d", h.Type, frameTypePeerTicBatch)
	}
	gotPID, gotTics, err := unmarshalPeerTicBatchPayload(p)
	if err != nil {
		t.Fatalf("unmarshal peer tic batch: %v", err)
	}
	if gotPID != pid1 {
		t.Fatalf("routed playerID=%d want=%d", gotPID, pid1)
	}
	if len(gotTics) != 1 {
		t.Fatalf("tic count=%d want=1", len(gotTics))
	}
	if gotTics[0].Forward != tc.Forward || gotTics[0].Side != tc.Side {
		t.Fatalf("tic mismatch got=%+v want=%+v", gotTics[0], tc)
	}

	// peer2 sends a tic; peer1 should receive it tagged with player_id=2.
	tc2 := demo.Tic{Forward: -3, Side: 7, AngleTurn: -128, Buttons: 0}
	payload2 := marshalPeerTicBatchPayload(pid2, []demo.Tic{tc2})
	if err := writeFrame(conn2, frameHeader{Type: frameTypePeerTicBatch}, payload2); err != nil {
		t.Fatalf("peer2 write tic: %v", err)
	}

	h, p = readPeerFrame(t, conn1)
	if h.Type != frameTypePeerTicBatch {
		t.Fatalf("peer1 got frame type=%d want=%d", h.Type, frameTypePeerTicBatch)
	}
	gotPID2, gotTics2, err := unmarshalPeerTicBatchPayload(p)
	if err != nil {
		t.Fatalf("unmarshal peer tic batch: %v", err)
	}
	if gotPID2 != pid2 {
		t.Fatalf("routed playerID=%d want=%d", gotPID2, pid2)
	}
	if gotTics2[0].Forward != tc2.Forward {
		t.Fatalf("tic2 mismatch got=%+v want=%+v", gotTics2[0], tc2)
	}
}

func TestPlayerPeerSelfTicsNotEchoed(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer: %v", err)
	}
	defer srv.Close()

	conn1, sid, _ := dialPeer(t, srv.Addr(), 0, 1)
	defer conn1.Close()

	// Drain roster frame.
	readPeerFrame(t, conn1)

	conn2, _, _ := dialPeer(t, srv.Addr(), sid, 2)
	defer conn2.Close()
	readPeerFrame(t, conn2) // peer2 roster
	readPeerFrame(t, conn1) // peer1 updated roster

	// peer1 sends tic.
	payload := marshalPeerTicBatchPayload(1, []demo.Tic{{Forward: 1}})
	if err := writeFrame(conn1, frameHeader{Type: frameTypePeerTicBatch}, payload); err != nil {
		t.Fatalf("write tic: %v", err)
	}

	// peer2 should receive it.
	h, _ := readPeerFrame(t, conn2)
	if h.Type != frameTypePeerTicBatch {
		t.Fatalf("peer2 got type=%d want peer tic batch", h.Type)
	}

	// peer1 must NOT receive its own tic back.
	_ = conn1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var buf [1]byte
	n, err := conn1.Read(buf[:])
	_ = conn1.SetReadDeadline(time.Time{})
	if n > 0 || (err != nil && !errors.Is(err, context.DeadlineExceeded) && !isTimeoutErr(err)) {
		t.Fatalf("peer1 received unexpected data n=%d err=%v", n, err)
	}
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout()
	}
	return false
}

func TestPlayerPeerRosterOnDisconnect(t *testing.T) {
	srv, err := ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenServer: %v", err)
	}
	defer srv.Close()

	conn1, sid, _ := dialPeer(t, srv.Addr(), 0, 1)
	defer conn1.Close()
	readPeerFrame(t, conn1) // initial roster

	conn2, _, _ := dialPeer(t, srv.Addr(), sid, 2)
	defer conn2.Close()
	readPeerFrame(t, conn2) // peer2 roster (both players)
	readPeerFrame(t, conn1) // peer1 roster update (both players)

	// Disconnect peer2.
	conn2.Close()

	// peer1 should receive a roster update showing only player 1.
	h, p := readPeerFrame(t, conn1)
	if h.Type != frameTypeRosterUpdate {
		t.Fatalf("peer1 got type=%d want roster update after peer2 disconnect", h.Type)
	}
	roster, _ := unmarshalRosterUpdatePayload(p)
	if len(roster.PlayerIDs) != 1 || roster.PlayerIDs[0] != 1 {
		t.Fatalf("roster after disconnect = %v want [1]", roster.PlayerIDs)
	}
}
