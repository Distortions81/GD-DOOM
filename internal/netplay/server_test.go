package netplay

import (
	"context"
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
	if err := writeHello(bconn, helloRoleBroadcaster, 0, 99, SessionConfig{
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
	if err := writeHello(vconn, helloRoleViewer, 0, 99, SessionConfig{}); err != nil {
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
	payload[0] = 1
	copy(payload[ticBatchOverhead:], packDemoTic(tc))
	if err := writeFrame(bconn, frameHeader{Type: frameTypeTicBatch, Tic: 7}, payload); err != nil {
		t.Fatalf("writeFrame broadcaster: %v", err)
	}

	header, gotPayload, err := readFrame(vconn)
	if err != nil {
		t.Fatalf("readFrame viewer: %v", err)
	}
	if header.Type != frameTypeTicBatch || header.Tic != 7 {
		t.Fatalf("header=%+v want type=%d tic=7", header, frameTypeTicBatch)
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
	if err := writeHello(bconn, helloRoleBroadcaster, 0, 88, SessionConfig{MapName: "E1M1"}); err != nil {
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
	if err := writeHello(vconn, helloRoleViewer, 0, 88, SessionConfig{}); err != nil {
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
	if err := writeHello(conn, helloRoleBroadcaster, 0, 0, SessionConfig{MapName: "E1M1"}); err != nil {
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
	if err := writeHello(conn, helloRoleViewer, 0, 123, SessionConfig{}); err != nil {
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
	if err := writeHello(bconn, helloRoleBroadcaster, 0, 77, SessionConfig{}); err != nil {
		t.Fatalf("writeHello broadcaster: %v", err)
	}

	vconn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatalf("dial viewer: %v", err)
	}
	defer vconn.Close()
	if err := writeHello(vconn, helloRoleViewer, 0, 77, SessionConfig{}); err != nil {
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
