package netplay

import (
	"context"
	"io"
	"testing"
	"time"

	"gddoom/internal/demo"
)

func TestBroadcastWatchRoundTrip(t *testing.T) {
	b, err := Listen("127.0.0.1:0", SessionConfig{
		WADHash:          "abc123",
		MapName:          "E1M1",
		PlayerSlot:       1,
		SkillLevel:       3,
		GameMode:         "single",
		AutoWeaponSwitch: true,
	})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer b.Close()

	v, err := Dial(b.Addr(), "abc123")
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer v.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.WaitForViewer(ctx); err != nil {
		t.Fatalf("WaitForViewer() error = %v", err)
	}

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

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok, err := v.PollTic()
		if err != nil && err != io.EOF {
			t.Fatalf("PollTic() error = %v", err)
		}
		if ok {
			if got != want {
				t.Fatalf("PollTic() = %+v want %+v", got, want)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for tic")
}

func TestWatchRejectsWADHashMismatch(t *testing.T) {
	b, err := Listen("127.0.0.1:0", SessionConfig{WADHash: "host"})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer b.Close()

	if _, err := Dial(b.Addr(), "local"); err == nil {
		t.Fatal("Dial() error = nil want mismatch")
	}
}

func TestBroadcasterStopAcceptingAfterFirstViewer(t *testing.T) {
	b, err := Listen("127.0.0.1:0", SessionConfig{})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer b.Close()

	v, err := Dial(b.Addr(), "")
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer v.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.WaitForViewer(ctx); err != nil {
		t.Fatalf("WaitForViewer() error = %v", err)
	}
	if err := b.StopAccepting(); err != nil {
		t.Fatalf("StopAccepting() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)
}
