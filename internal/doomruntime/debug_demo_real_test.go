package doomruntime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"gddoom/internal/demo"
	"gddoom/internal/mapdata"
	"gddoom/internal/media"
	"gddoom/internal/wad"
)

func TestDebugRealDemo1FullTrace(t *testing.T) {
	if os.Getenv("GD_RUN_REAL_DEMO_DEBUG") == "" {
		t.Skip("set GD_RUN_REAL_DEMO_DEBUG=1 to run")
	}
	tracePath := os.Getenv("GD_REAL_DEMO_TRACE_PATH")
	if tracePath == "" {
		t.Fatal("GD_REAL_DEMO_TRACE_PATH is required")
	}
	wadPath := findDOOM1WAD(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}
	script, err := demo.Load(filepath.Join("..", "..", "demos", "DOOM1-DEMO1.lmp"))
	if err != nil {
		t.Fatalf("load demo: %v", err)
	}
	m, err := mapdata.LoadMap(wf, "E1M5")
	if err != nil {
		t.Fatalf("load E1M5: %v", err)
	}
	g := newGame(m, Options{
		Width:              320,
		Height:             200,
		SourcePortMode:     true,
		DemoScript:         script,
		DemoQuitOnComplete: true,
		DemoTracePath:      tracePath,
		SoundBank:          media.SoundBank{},
	})
	g.snd = nil
	for tic := 0; tic < 100000; tic++ {
		err := g.Update()
		if err == nil {
			continue
		}
		if errors.Is(err, ebiten.Termination) {
			return
		}
		t.Fatalf("update %d: %v", tic, err)
	}
	t.Fatal("demo did not terminate")
}
