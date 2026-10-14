//go:build !js

package netgame

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSpectatorTCPAndWebSocketWaitFollowLeaveAndChangeMap(t *testing.T) {
	for _, websocketObserver := range []bool{false, true} {
		t.Run(map[bool]string{false: "TCP observer", true: "WebSocket observer"}[websocketObserver], func(t *testing.T) {
			m, _ := newResumeTestMatch(t, 35)
			world := &completingTestWorld{completeAt: 16}
			m.world, m.snapshots, m.config.DisconnectTicks = world, world, 350
			s, err := NewServer(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetTransitionHandler(func() (*MapTransition, error) {
				world.tick, world.complete, world.completeAt = 0, false, 1000
				return &MapTransition{Epoch: 100, Map: "E1M2", Compatibility: "next-content"}, nil
			}); err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ready := &readyListener{Listener: listener, ready: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- s.Serve(ctx, ready) }()
			<-ready.ready
			httpServer := httptest.NewServer(s.WebSocketHandler(WebSocketOptions{}))
			defer func() { cancel(); httpServer.Close(); waitServerStopped(t, done) }()
			tcpURL, wsURL := listener.Addr().String(), "ws"+strings.TrimPrefix(httpServer.URL, "http")
			dial := func(observer, ws bool, name string) *Client {
				t.Helper()
				hello := Hello{Compatibility: "test-content", Name: name, Spectator: observer}
				var c *Client
				var err error
				if ws {
					c, err = DialWebSocket(ctx, wsURL, hello)
				} else {
					c, err = DialTCP(ctx, tcpURL, hello)
				}
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { c.Close() })
				return c
			}
			observer := dial(true, websocketObserver, "watcher")
			if observer.Welcome().PlayerID != 0 {
				t.Fatal("spectator got a player slot")
			}
			// The first peer is a spectator: no invalid viewer0 snapshot and no
			// body-driven world tic is possible before a player arrives.
			time.Sleep(2 * time.Second / TickRate)
			if _, ok, err := observer.PollSnapshot(); ok || err != nil {
				t.Fatal("empty lobby produced a camera snapshot or disconnected")
			}
			a := dial(false, !websocketObserver, "player one")
			b := dial(false, websocketObserver, "player two")
			if a.Welcome().PlayerID != 1 || a.Welcome().ServerTick != 0 || b.Welcome().PlayerID != 2 {
				t.Fatal("spectator affected player admission")
			}
			waitCamera := func(id byte) {
				t.Helper()
				for {
					snapshot := nextWebSocketTestSnapshot(t, observer)
					if len(snapshot.State) == 2 && snapshot.State[1] == id {
						return
					}
				}
			}
			waitCamera(1)
			if err := observer.FollowPlayer(0); err != nil {
				t.Fatal(err)
			}
			waitCamera(2)
			// Explicit quit releases the body despite configured reconnect grace.
			if err := b.Leave(); err != nil {
				t.Fatal(err)
			}
			waitCamera(1)
			deadline := time.Now().Add(2 * time.Second)
			for {
				change, ok, err := observer.PollTransition()
				if err != nil {
					t.Fatal(err)
				}
				if ok {
					if change.Map != "E1M2" || change.Welcome.PlayerID != 0 || change.Welcome.Epoch != 100 {
						t.Fatal("spectator epoch control incorrect")
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("spectator map change missing")
				}
				time.Sleep(time.Millisecond)
			}
			snapshot := nextWebSocketTestSnapshot(t, observer)
			if snapshot.Epoch != 100 || snapshot.State[1] != 1 || snapshot.Finalized.HasTick {
				t.Fatal("spectator new-map camera baseline invalid")
			}
		})
	}
}
