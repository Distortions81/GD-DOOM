package main

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gddoom/internal/netgame"
)

type serverOutput struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *serverOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}
func (b *serverOutput) value() string { b.mu.Lock(); defer b.mu.Unlock(); return b.Buffer.String() }

func TestRunCrossTransportDeathmatchRotationAndShutdown(t *testing.T) {
	for _, rotation := range []string{"", "E1M1,E1M2"} {
		name := "default-progression"
		if rotation != "" {
			name = "explicit-rotation"
		}
		t.Run(name, func(t *testing.T) { testRunDeathmatchTransition(t, rotation) })
	}
}

func testRunDeathmatchTransition(t *testing.T, rotation string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output serverOutput
	done := make(chan error, 1)
	go func() {
		args := []string{"-listen", "127.0.0.1:0", "-web-listen", "127.0.0.1:0", "-wad", filepath.Join("..", "..", "DOOM1.WAD"), "-mode", "deathmatch", "-time-limit", "1", "-no-monsters"}
		if rotation != "" {
			args = append(args, "-rotation", rotation)
		}
		done <- run(ctx, args, &output, io.Discard)
	}()
	var address, webURL, compatibility string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(output.value(), "\n") {
			if strings.HasPrefix(line, "gdserver: deathmatch on ") {
				address = strings.Split(strings.TrimPrefix(line, "gdserver: deathmatch on "), ",")[0]
			}
			if strings.HasPrefix(line, "compatibility: ") {
				compatibility = strings.TrimPrefix(line, "compatibility: ")
			}
			if strings.HasPrefix(line, "websocket: ") {
				webURL = strings.TrimPrefix(line, "websocket: ")
			}
		}
		if address != "" && webURL != "" && compatibility != "" {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("server startup: %v", err)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	if address == "" || webURL == "" {
		t.Fatalf("missing startup addresses: %s", output.value())
	}
	hello := netgame.Hello{Compatibility: compatibility, Name: "test"}
	native, err := netgame.DialTCP(ctx, address, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer native.Close()
	web, err := netgame.DialWebSocket(ctx, webURL, hello)
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	initial := native.Welcome()
	if initial.Epoch != web.Welcome().Epoch || initial.PlayerID == web.Welcome().PlayerID {
		t.Fatal("clients were not assigned one match and distinct players")
	}
	clients := []*netgame.Client{native, web}
	changed := [2]bool{}
	baselines := [2]bool{}
	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && (!baselines[0] || !baselines[1]) {
		for i, client := range clients {
			change, ok, err := client.PollTransition()
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				if change.Map != "E1M2" || change.PreviousEpoch != initial.Epoch || change.Welcome.Epoch <= initial.Epoch {
					t.Fatalf("unexpected transition: %+v", change)
				}
				changed[i] = true
			}
			snapshot, ok, err := client.PollSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				if snapshot.Epoch != client.Welcome().Epoch || len(snapshot.State) == 0 {
					t.Fatal("wrong/empty epoch baseline")
				}
				if changed[i] {
					baselines[i] = true
				}
				if err := client.SendInputs(netgame.InputBatch{Epoch: snapshot.Epoch, SnapshotAck: snapshot.ID}); err != nil {
					t.Fatal(err)
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	if !baselines[0] || !baselines[1] {
		t.Fatalf("map rotation did not reach both clients: %v %s", baselines, output.value())
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}
