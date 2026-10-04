//go:build raylib && cgo && integration && !js

package main

import (
	"os"
	"runtime"
	"testing"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeAudioLoadsDMXAndPlaysAliases(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_AUDIO_INTEGRATION=1 with an audio device or ALSA null PCM")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.NoMonsters = true
	game := doomruntime.NewNativeMeshGame(m, opts)
	a := newNativeAudio(wf, 0.7)
	if a == nil {
		t.Fatal("audio device did not initialize")
	}
	defer a.Close()
	if len(a.sources) < 10 {
		t.Fatal("DMX sound bank was not loaded")
	}
	a.Play(game, doomruntime.NativeSound{Name: "DSPISTOL", Pitch: 1})
	if len(a.voices) != 1 || !rl.IsSoundPlaying(a.voices[0].sound) {
		t.Fatal("native sound alias did not start")
	}
	a.Play(game, doomruntime.NativeSound{Name: "DSPISTOL", Pitch: 1})
	if len(a.voices) != 2 {
		t.Fatal("overlapping effects did not get independent voices")
	}
	a.Stop()
	if len(a.voices) != 0 {
		t.Fatal("restart/pause did not release playing aliases")
	}
}
