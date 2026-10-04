//go:build raylib && cgo && integration && !js

package main

import (
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"gddoom/internal/launchcatalog"
	"gddoom/internal/music"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativePWADSoundAndMusicWithAudioDevice(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_AUDIO_INTEGRATION=1 with an audio device or null PCM")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	base, first, last := nativeWADOverlayFixture(t)
	wf, _, err := launchcatalog.OpenWADStack(base, []string{first, last})
	if err != nil {
		t.Fatal(err)
	}
	a := newNativeAudio(wf, .5)
	if a == nil {
		t.Fatal("native audio device unavailable")
	}
	defer a.Close()
	baseFile, err := wad.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	var expectedFrames uint32
	for _, s := range sound.ImportDigitalSounds(baseFile).Sounds {
		if s.Name == "DSSHOTGN" {
			expectedFrames = uint32(len(s.Samples))
		}
	}
	// Raylib resamples DMX data to the device rate. The unchanged shotgun
	// source is the reference for this deliberately identical replacement.
	if source := a.sources["DSPISTOL"]; expectedFrames == 0 || source.FrameCount != a.sources["DSSHOTGN"].FrameCount {
		t.Fatalf("Raylib pistol source has %d frames, expected resampled shotgun %d", source.FrameCount, a.sources["DSSHOTGN"].FrameCount)
	}
	m, err := newNativeMusicWithConfig(wf, 1, defaultNativeMusicConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.PlayLump("D_E1M3"); err != nil {
		t.Fatal(err)
	}
	lump, _ := baseFile.LumpByName("D_E1M1")
	data, err := baseFile.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.parsed, music.ApplyMUSVolumeCompression(parsed, music.DefaultMUSVolumeCompression)) {
		t.Fatal("native music ignored overlaid score")
	}
	if err := m.Update(); err != nil {
		t.Fatal(err)
	}
}

func TestNativePWADLauncherAndFramebuffer(t *testing.T) {
	if os.Getenv("GD_RAYLIB_WAD_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_WAD_INTEGRATION=1 after building build/raydoom")
	}
	base, first, last := nativeWADOverlayFixture(t)
	wf, _, err := launchcatalog.OpenWADStack(base, []string{first, last})
	if err != nil {
		t.Fatal(err)
	}
	palette, err := doomtex.LoadPaletteRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	capture, err := filepath.Abs("../../build/raylib-captures/native-pwad-overlay.png")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.CommandContext(ctx, "../../build/raydoom", "-config=", "-wad="+base, "-file="+first+","+last, "-menu=false", "-player=2", "-cheat-level=2", "-no-monsters", "-invuln", "-nofps", "-mouselook-speed=.05", "-keyboard-turn-speed=6", "-sound=false", "-music=false", "-frames=3", "-fps=35", "-width=640", "-height=400", "-capture="+capture).CombinedOutput()
	if err != nil {
		t.Fatalf("PWAD launch: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "raylib-mesh map=E1M3 frames=3") {
		t.Fatalf("native ignored overlay's start map:\n%s", out)
	}
	file, err := os.Open(capture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(2, 390).RGBA()
	want := palette[160*4 : 160*4+3]
	if byte(r>>8) != want[0] || byte(g>>8) != want[1] || byte(b>>8) != want[2] {
		t.Fatalf("last PWAD's HUD absent from native framebuffer: %v want %v", []byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)}, want)
	}
}
