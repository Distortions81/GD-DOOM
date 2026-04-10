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
		SFXVolume:          0.5,
		SoundBank:          media.SoundBank{},
	})
	if g.snd != nil {
		g.snd.player = nil
	}
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

// BenchmarkSimOnly measures the cost of running the Doom simulation with no
// rendering and no audio. This is the per-tic cost relevant to server-side
// authoritative simulation.
//
// Run with:
//
//	GD_RUN_REAL_DEMO_DEBUG=1 go test ./internal/doomruntime/... -run=^$ -bench=BenchmarkSimOnly -benchtime=5s -count=1
//
// The benchmark reports ns/op where one "op" is one demo tic.
func BenchmarkSimOnly(b *testing.B) {
	if os.Getenv("GD_RUN_REAL_DEMO_DEBUG") == "" {
		b.Skip("set GD_RUN_REAL_DEMO_DEBUG=1 to run")
	}
	wadPath := findDOOM1WAD(b)
	wf, err := wad.Open(wadPath)
	if err != nil {
		b.Fatalf("open wad: %v", err)
	}
	script, err := demo.Load(filepath.Join("..", "..", "demos", "DOOM1-DEMO1.lmp"))
	if err != nil {
		b.Fatalf("load demo: %v", err)
	}
	m, err := mapdata.LoadMap(wf, "E1M5")
	if err != nil {
		b.Fatalf("load E1M5: %v", err)
	}

	newHeadlessGame := func() *game {
		g := newGame(m, Options{
			Width:              320,
			Height:             200,
			PlayerSlot:         1,
			SkillLevel:         4,
			DemoScript:         script,
			DemoQuitOnComplete: true,
			SFXVolume:          0,    // no sound player created
			SoundBank:          media.SoundBank{}, // empty bank
		})
		if g.snd != nil {
			g.snd.player = nil // belt-and-suspenders: disable audio output
		}
		return g
	}

	// Warm up: run one pass to fill caches.
	{
		g := newHeadlessGame()
		for i := 0; i < 350; i++ {
			_ = g.Update()
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Each b.N iteration runs one tic. We recreate the game every len(script.Tics)
	// tics so we don't run out of demo input.
	g := newHeadlessGame()
	tic := 0
	for i := 0; i < b.N; i++ {
		err := g.Update()
		tic++
		if errors.Is(err, ebiten.Termination) || tic >= len(script.Tics) {
			// Restart for next batch.
			g = newHeadlessGame()
			tic = 0
		}
	}
}


func TestDebugRealDemo1WeaponWindow(t *testing.T) {
	if os.Getenv("GD_RUN_REAL_DEMO_WEAPON_DEBUG") == "" {
		t.Skip("set GD_RUN_REAL_DEMO_WEAPON_DEBUG=1 to run")
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
		SFXVolume:          0.5,
		SoundBank:          media.SoundBank{},
	})
	if g.snd != nil {
		g.snd.player = nil
	}
	for tic := 0; tic < 2145; tic++ {
		if tic >= 2122 && tic <= 2126 {
			t.Logf("pre tic=%d ready=%d pending=%d wstate=%d wtics=%d wy=%d btn=%d",
				tic, demoTraceWeaponID(g.inventory.ReadyWeapon), func() int {
					if g.inventory.PendingWeapon == 0 {
						return demoTraceWeaponNoChange
					}
					return demoTraceWeaponID(g.inventory.PendingWeapon)
				}(), g.weaponState, g.weaponStateTics, g.weaponPSpriteY, script.Tics[tic].Buttons)
		}
		err := g.Update()
		if tic >= 2122 && tic <= 2139 {
			t.Logf("post tic=%d ready=%d pending=%d wstate=%d wtics=%d wy=%d x=%d y=%d",
				tic, demoTraceWeaponID(g.inventory.ReadyWeapon), func() int {
					if g.inventory.PendingWeapon == 0 {
						return demoTraceWeaponNoChange
					}
					return demoTraceWeaponID(g.inventory.PendingWeapon)
				}(), g.weaponState, g.weaponStateTics, g.weaponPSpriteY, g.p.x, g.p.y)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, ebiten.Termination) {
			return
		}
		t.Fatalf("update %d: %v", tic, err)
	}
}
