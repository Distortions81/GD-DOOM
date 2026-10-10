package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gddoom/internal/netgame"
	"gddoom/internal/roomhost"
)

func TestRunWorkerReadyRulesAndCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	filename := filepath.Join(t.TempDir(), "ready.json")
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-wad", filepath.Join("..", "..", "DOOM1.WAD"), "-listen", "127.0.0.1:0", "-web-listen", "127.0.0.1:0", "-ready-file", filename, "-fast-monsters", "-respawn-monsters"}, io.Discard, io.Discard)
	}()
	var ready roomhost.WorkerReady
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filename); err == nil {
			if err := json.Unmarshal(data, &ready); err != nil {
				t.Fatal(err)
			}
			break
		}
		select {
		case err := <-done:
			t.Fatalf("worker failed before ready: %v", err)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	if ready.Version != 1 || ready.TCPAddress == "" || ready.WebURL == "" || !ready.Manifest.FastMonsters || !ready.Manifest.RespawnMonsters {
		t.Fatalf("incomplete readiness metadata: %+v", ready)
	}
	transport, err := netgame.OpenTCP(ctx, ready.TCPAddress)
	if err != nil {
		t.Fatal(err)
	}
	status, err := netgame.QueryServerStatus(ctx, transport)
	if err != nil || !status.Manifest.FastMonsters || !status.Manifest.RespawnMonsters || status.Players != 0 {
		t.Fatalf("ready worker status: %+v %v", status, err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not shut down")
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Fatalf("stale readiness file remains: %v", err)
	}
}
