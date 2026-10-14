//go:build raylib && cgo && integration && !js

package main

import (
	"gddoom/internal/launchcatalog"
	"gddoom/internal/music"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"runtime"
	"testing"
)

func TestNativeControlsAndBindingMenusGPU(t *testing.T) {
	if os.Getenv("GD_RAYLIB_CONTROLS_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_CONTROLS_INTEGRATION=1 for menu framebuffer checks")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	rl.SetConfigFlags(rl.FlagWindowHidden | rl.FlagMsaa4xHint)
	rl.InitWindow(1280, 800, "Native controls regression")
	defer rl.CloseWindow()
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	p, err := raymesh.NewPresentation(raymesh.TextureOptions{Scale: 2, Filter: raymesh.Anisotropic})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	opts.MusicPlayerCatalog, opts.MusicPlayerTrackLoader = launchcatalog.BuildMusicPlayerCatalog("../../DOOM1.WAD")
	opts.MusicSoundFontChoices = []string{"soundfonts/general-midi.sf2"}
	opts.Episodes = []int{1, 2, 3}
	m := newNativeMenu(opts, nil, "")
	m.frontend = true
	s := nativeSettings{messages: true, screenBlocks: 1, hudScale: 4, sfxVolume: .7, speakerVolume: 1, speakerVariant: "paper-speaker", mouseLook: true, autoWeaponSwitch: true, mouseSensitivity: 1.5, keyboardSpeed: 1, bindings: runtimecfg.DefaultInputBindings()}
	draw := func(name string) {
		patches := m.draw(s, 0)
		// All glyphs, footer and values must stay within the logical menu page.
		for _, patch := range patches {
			if patch.X < 0 || patch.Y < 0 || patch.X+patch.W > 320 || patch.Y+patch.H > 200 {
				t.Fatalf("menu glyph outside page: %+v", patch)
			}
		}
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		p.DrawUI(patches, 1280, 800)
		img, err := raymesh.CaptureWindow()
		if err != nil {
			t.Fatal(err)
		}
		colors := rl.LoadImageColors(img)
		red := 0
		for _, pixel := range colors {
			if pixel.R > 80 && pixel.R > pixel.G {
				red++
			}
		}
		rl.UnloadImageColors(colors)
		rl.UnloadImage(img)
		if red < 15000 {
			t.Fatal("menu artwork absent from framebuffer")
		}
		if err := captureScreen("../../build/raylib-captures/" + name + ".png"); err != nil {
			t.Fatal(err)
		}
		rl.EndDrawing()
	}
	for _, page := range []struct {
		page menuPage
		name string
	}{
		{menuMain, "native-main-menu"}, {menuOptions, "native-options"},
		{menuNewGame, "native-episode"}, {menuSkill, "native-skill"},
		{menuHelp, "native-read-this"}, {menuGraphics, "native-raylib-options"},
		{menuSpeaker, "native-speaker-options"},
	} {
		m.open(page.page)
		draw(page.name)
	}
	m.open(menuMain)
	m.frontend = false
	draw("native-pause-menu")
	m.open(menuQuit)
	draw("native-quit")
	m.update(menuInput{quitNo: true, mouseRow: -1}, &s)
	m.open(menuControls)
	draw("native-controls")
	m.open(menuBindings)
	m.row = 6
	m.bindingSlot = 1
	draw("native-bindings")
	m.row = 19
	m.capture = true
	draw("native-bindings-capture")
	s.musicBackend, s.soundFont, s.musicVolume = music.BackendMeltySynth, "soundfonts/general-midi.sf2", .5
	m.open(menuSound)
	draw("native-sound")
	m.open(menuMusicPlayer)
	m.syncMusicSelection("", "D_E1M4")
	m.nowPlaying = "SONG: KITCHEN ACE (AND TAKING NAMES)"
	draw("native-music-player")
}
