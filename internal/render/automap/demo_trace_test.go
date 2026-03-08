package automap

import (
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestDemoTraceWritesMetaDemoAndTics(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	tracePath := t.TempDir() + "/demo-trace.jsonl"
	g := newGame(base.m, Options{
		Width:  320,
		Height: 200,
		WADHash: "test-wad",
		DemoScript: &DemoScript{
			Path: "demo1",
			Header: DemoHeader{
				Version:      demoVersion109,
				Skill:        2,
				Episode:      1,
				Map:          1,
				PlayerInGame: [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 25}, {Forward: 25}},
		},
		DemoTracePath: tracePath,
	})

	for i := 0; i < 3; i++ {
		if err := g.Update(); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}

	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got, want := len(lines), 4; got != want {
		t.Fatalf("trace lines=%d want=%d\n%s", got, want, data)
	}
	if !strings.Contains(lines[0], `"kind":"meta"`) {
		t.Fatalf("meta line missing: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"kind":"demo"`) {
		t.Fatalf("demo line missing: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"kind":"tic"`) || !strings.Contains(lines[3], `"kind":"tic"`) {
		t.Fatalf("tic lines missing:\n%s", data)
	}
	if !strings.Contains(lines[2], `"mobjs"`) || !strings.Contains(lines[2], `"specials"`) {
		t.Fatalf("tic payload missing state arrays: %s", lines[2])
	}
}

func TestDemoTraceStopsWhenPlayerDies(t *testing.T) {
	base := mustLoadE1M1GameForMapTextureTests(t)
	tracePath := t.TempDir() + "/demo-trace.jsonl"
	g := newGame(base.m, Options{
		Width:  320,
		Height: 200,
		WADHash: "test-wad",
		DemoScript: &DemoScript{
			Path: "demo1",
			Header: DemoHeader{
				Version:      demoVersion109,
				Skill:        2,
				Episode:      1,
				Map:          1,
				PlayerInGame: [4]bool{true},
			},
			Tics: []DemoTic{{Forward: 25}, {Forward: 25}},
		},
		DemoTracePath: tracePath,
	})
	g.isDead = true

	err := g.Update()
	if err != ebiten.Termination {
		t.Fatalf("Update() err=%v want %v", err, ebiten.Termination)
	}

	data, readErr := os.ReadFile(tracePath)
	if readErr != nil {
		t.Fatalf("read trace: %v", readErr)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if got, want := len(lines), 3; got != want {
		t.Fatalf("trace lines=%d want=%d\n%s", got, want, data)
	}
}
