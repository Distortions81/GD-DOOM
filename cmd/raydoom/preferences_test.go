//go:build raylib && cgo && !js

package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
	"gddoom/internal/music"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
)

func nativePreferenceFlags(path string) *flag.FlagSet {
	fs := flag.NewFlagSet("native preferences test", flag.ContinueOnError)
	fs.String("config", path, "")
	fs.String("wad", "DOOM1.WAD", "")
	fs.String("map", "", "")
	fs.String("demo", "", "")
	fs.String("record-demo", "", "")
	fs.Bool("nomonsters", false, "")
	fs.Bool("god", false, "")
	fs.Int("demo-stop-after-tics", 0, "")
	fs.String("music-backend", "impsynth", "")
	fs.String("soundfont", "", "")
	fs.Float64("mus-pan-max", .8, "")
	fs.Float64("pc-speaker-volume", 1, "")
	fs.Bool("pc-speaker", false, "")
	fs.String("pc-speaker-variant", "paper-speaker", "")
	fs.Int("skill", 3, "")
	fs.Int("width", 1280, "")
	fs.Int("height", 800, "")
	fs.Int("fps", 144, "")
	fs.Int("texture-scale", 2, "")
	fs.Float64("mouse-speed", runtimecfg.DefaultMouseLookSpeed, "")
	fs.Float64("keyboard-speed", runtimecfg.DefaultKeyboardTurnSpeed, "")
	fs.Float64("music-volume", runtimecfg.DefaultMusicVolume, "")
	fs.Float64("sfx-volume", runtimecfg.DefaultSFXVolume, "")
	fs.Bool("smooth-camera-yaw", runtimecfg.DefaultSmoothCameraYaw, "")
	fs.Int("gamma-level", -1, "")
	fs.Int("detail-level", 0, "")
	fs.Bool("auto-detail", false, "")
	fs.Bool("crt-effect", false, "")
	fs.Bool("no-aspect-correction", false, "")
	fs.Bool("no-vsync", false, "")
	fs.Bool("mouselook", true, "")
	fs.Bool("mouse-invert", false, "")
	fs.Bool("always-run", runtimecfg.DefaultAlwaysRun, "")
	fs.Bool("auto-weapon-switch", true, "")
	fs.Bool("fullscreen", false, "")
	fs.Bool("debug", false, "")
	fs.Bool("msaa", true, "")
	fs.String("texture-filter", "anisotropic", "")
	fs.String("lighting", "doom", "")
	fs.String("mode", "textured", "")
	registerNativeGameplayFlags(fs)
	registerNativeFlagAliases(fs)
	return fs
}
func TestNativePreferencesRoundTripPreservesMainAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("map=\"E1M3\"\nskill=4\nmusic_volume=0.4\nmouselook=true\nalways_run=true\ndetail_level_faithful=2\nmusic_backend=\"impsynth\"\nsoundfont=\"custom.sf2\"\n[raylib]\nwidth=640\nheight=400\nfps=0\nmsaa=false\ntexture_filter=\"nearest\"\nlighting=\"fullbright\"\n[keybinds]\nfire=[\"MB2\",\"LCTRL\"]\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	fs := nativePreferenceFlags(path)
	if err := fs.Parse([]string{"-fps=120", "-mouselook=false", "-music-volume=.9", "-music-backend=pcspeaker"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := applyNativeConfig(fs)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"music-backend": "pcspeaker", "soundfont": "custom.sf2", "map": "", "skill": "4", "width": "640", "fps": "120", "mouselook": "false", "msaa": "false", "music-volume": "0.9", "always-run": "true", "texture-filter": "nearest"} {
		if got := fs.Lookup(name).Value.String(); got != want {
			t.Fatalf("%s=%s want %s", name, got, want)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("loading native preferences rewrote config")
	}
	s := nativeSettings{messages: true, showFPS: true, screenBlocks: 2, hudScale: 6, pcSpeaker: true, musicBackend: music.BackendMeltySynth, soundFont: "changed.sf2", musicPan: .6, speakerVolume: .3, speakerVariant: "small-buzzer", sfxVolume: .6, musicVolume: .2, mouseSensitivity: 2, keyboardSpeed: 1.5, mouseLook: false, mouseInvert: true, alwaysRun: true, autoWeaponSwitch: false, fullscreen: true, fps: 240, textureFilter: raymesh.Trilinear, lighting: raymesh.SectorLighting}
	binds := runtimecfg.NormalizeInputBindings(*cfg.Keybinds)
	binds.MoveForward = runtimecfg.KeyBinding{"I", "UP"}
	if err := saveNativePreferences(path, s, binds, 1600, 1000, 2, true, "wireframe"); err != nil {
		t.Fatal(err)
	}
	loaded, err := configfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if *loaded.MusicBackend != "meltysynth" || *loaded.SoundFont != "changed.sf2" || *loaded.MUSPanMax != .6 || *loaded.PCSpeakerVolume != .3 || *loaded.PCSpeakerVariant != "small-buzzer" || *loaded.DetailLevelFaithful != 2 || *loaded.Map != "E1M3" {
		t.Fatal("native save destroyed unrelated main settings")
	}
	if *loaded.MusicVolume != .2 || *loaded.MouseLookSpeed != 2 || *loaded.KeyboardTurnSpeed != 1.5 || !*loaded.MouseInvert || *loaded.AutoWeaponSwitch || loaded.Keybinds.MoveForward != binds.MoveForward {
		t.Fatal("common preferences did not round trip")
	}
	if loaded.Raylib.Messages == nil || !*loaded.Raylib.Messages || *loaded.Raylib.ScreenBlocks != 2 || *loaded.Raylib.HUDScale != 6 || *loaded.NoFPS {
		t.Fatal("menu display preferences did not persist")
	}
	if loaded.PCSpeaker == nil || !*loaded.PCSpeaker {
		t.Fatal("PC-speaker effect preference did not persist")
	}
	fs = nativePreferenceFlags(path)
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"fps": "240", "width": "1600", "height": "1000", "mode": "wireframe", "texture-filter": "trilinear", "lighting": "sector", "fullscreen": "true", "music-backend": "meltysynth", "soundfont": "changed.sf2", "mus-pan-max": "0.6", "pc-speaker-volume": "0.3", "pc-speaker-variant": "small-buzzer", "mouse-speed": "2", "keyboard-speed": "1.5"} {
		if got := fs.Lookup(name).Value.String(); got != want {
			t.Fatalf("reloaded %s=%s want %s", name, got, want)
		}
	}
}
func TestNativePreferencesMissingDisabledAndInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	fs := nativePreferenceFlags(path)
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal("missing default config should use defaults", err)
	}
	fs = nativePreferenceFlags(path)
	fs.Set("config", path)
	if _, err := applyNativeConfig(fs); err == nil {
		t.Fatal("missing explicit config accepted")
	}
	fs = nativePreferenceFlags(path)
	fs.Set("config", "")
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal("disabled config did not work", err)
	}
	if err := saveNativePreferences("", nativeSettings{}, runtimecfg.InputBindings{}, 1280, 800, 2, true, "textured"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled persistence created config")
	}
	s := nativeSettings{mouseSensitivity: 1, keyboardSpeed: 1, fps: 144, textureFilter: raymesh.Anisotropic, lighting: raymesh.DoomLighting}
	if err := saveNativePreferences(path, s, runtimecfg.DefaultInputBindings(), 1280, 800, 2, true, "textured"); err != nil {
		t.Fatal("first preference write failed", err)
	}
	if _, err := applyNativeConfig(nativePreferenceFlags(path)); err != nil {
		t.Fatal("created config cannot be loaded", err)
	}
	for _, bad := range []string{"malformed = [", "[keybinds]\nfire=[\"UNKNOWN\",\"\"]\n"} {
		os.WriteFile(path, []byte(bad), 0644)
		if _, err := applyNativeConfig(nativePreferenceFlags(path)); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
	broken := []byte("malformed = [")
	os.WriteFile(path, broken, 0644)
	if err := saveNativePreferences(path, nativeSettings{}, runtimecfg.InputBindings{}, 1280, 800, 2, true, "textured"); err == nil {
		t.Fatal("corrupt config overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, broken) {
		t.Fatal("failed write mutated corrupt config")
	}
}

func TestNativeAlwaysRunDefaultsAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		args         []string
		want         string
	}{
		{name: "no saved preference", want: "true"},
		{name: "saved walk", config: "always_run=false\n", want: "false"},
		{name: "saved run", config: "always_run=true\n", want: "true"},
		{name: "explicit run", config: "always_run=false\n", args: []string{"-always-run=true"}, want: "true"},
		{name: "explicit walk", config: "always_run=true\n", args: []string{"-always-run=false"}, want: "false"},
		{name: "walk without config", args: []string{"-always-run=false"}, want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := ""
			if tc.config != "" {
				path = filepath.Join(t.TempDir(), "config.toml")
				if err := os.WriteFile(path, []byte(tc.config), 0644); err != nil {
					t.Fatal(err)
				}
			}
			fs := nativePreferenceFlags(path)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if _, err := applyNativeConfig(fs); err != nil {
				t.Fatal(err)
			}
			if got := fs.Lookup("always-run").Value.String(); got != tc.want {
				t.Fatalf("always-run=%s want %s", got, tc.want)
			}
		})
	}
}
