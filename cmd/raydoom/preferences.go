//go:build raylib && cgo && !js

package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"gddoom/internal/configfile"
	"gddoom/internal/doomruntime"
	"gddoom/internal/runtimecfg"
)

// Apply saved defaults after parsing while retaining the original explicit flag
// set, so command-line values always win and -map retains its launch semantics.
func applyNativeConfig(fs *flag.FlagSet) (*configfile.Config, error) {
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for alias, original := range nativeFlagAliases {
		if explicit[alias] || explicit[original] {
			explicit[alias], explicit[original] = true, true
		}
	}
	path := strings.TrimSpace(fs.Lookup("config").Value.String())
	cfg := &configfile.Config{}
	if path != "" {
		loaded, err := configfile.Read(path)
		if err != nil && !(errors.Is(err, os.ErrNotExist) && !explicit["config"]) {
			return nil, err
		}
		if loaded != nil {
			cfg = loaded
		}
	}
	set := func(name, value string) error {
		if explicit[name] {
			return nil
		}
		return fs.Set(name, value)
	}
	str := func(name string, value *string) error {
		if value == nil {
			return nil
		}
		return set(name, *value)
	}
	integer := func(name string, value *int) error {
		if value == nil {
			return nil
		}
		return set(name, strconv.Itoa(*value))
	}
	boolean := func(name string, value *bool) error {
		if value == nil {
			return nil
		}
		return set(name, strconv.FormatBool(*value))
	}
	number := func(name string, value *float64) error {
		if value == nil {
			return nil
		}
		return set(name, strconv.FormatFloat(*value, 'g', -1, 64))
	}
	invert := cfg.MouseInvert
	if invert == nil {
		invert = cfg.MouseInvertHorizontal
	}
	for _, err := range []error{
		str("wad", cfg.Wad), str("demo", cfg.Demo), str("record-demo", cfg.RecordDemo), integer("skill", cfg.Skill), number("sfx-volume", cfg.SFXVolume), number("music-volume", cfg.MusicVolume),
		str("music-backend", cfg.MusicBackend), str("soundfont", cfg.SoundFont), number("mus-pan-max", cfg.MUSPanMax), boolean("pc-speaker", cfg.PCSpeaker), number("pc-speaker-volume", cfg.PCSpeakerVolume), str("pc-speaker-variant", cfg.PCSpeakerVariant),
		number("mouse-speed", cfg.MouseLookSpeed), number("keyboard-speed", cfg.KeyboardTurnSpeed), boolean("mouselook", cfg.MouseLook),
		boolean("smooth-camera-yaw", cfg.SmoothCameraYaw),
		integer("gamma-level", cfg.GammaLevel),
		integer("detail-level", cfg.DetailLevelSourcePort), boolean("auto-detail", cfg.AutoDetail),
		boolean("crt-effect", cfg.CRTEffect), boolean("no-aspect-correction", cfg.NoAspectCorrection),
		boolean("no-vsync", cfg.NoVsync),
		boolean("mouse-invert", invert), boolean("always-run", cfg.AlwaysRun), boolean("auto-weapon-switch", cfg.AutoWeaponSwitch),
		integer("player", cfg.Player), integer("cheat-level", cfg.CheatLevel), boolean("god", cfg.Invulnerable), boolean("all-cheats", cfg.AllCheats),
		boolean("show-no-skill-items", cfg.ShowNoSkillItems), boolean("show-all-items", cfg.ShowAllItems), boolean("nofps", cfg.NoFPS), boolean("debug-events", cfg.DebugEvents),
		integer("demo-stop-after-tics", cfg.DemoStopAfterTics),
		integer("texture-anim-crossfade-frames", cfg.TextureAnimCrossfadeFrames),
	} {
		if err != nil {
			return nil, err
		}
	}
	if r := cfg.Raylib; r != nil {
		for _, err := range []error{
			integer("width", r.Width), integer("height", r.Height), integer("fps", r.FPS), integer("texture-scale", r.TextureScale),
			boolean("fullscreen", r.Fullscreen), boolean("msaa", r.MSAA), boolean("debug", r.Debug),
			str("texture-filter", r.TextureFilter), str("lighting", r.Lighting), str("mode", r.MeshMode),
		} {
			if err != nil {
				return nil, err
			}
		}
	}
	if cfg.Keybinds != nil {
		supported := map[string]bool{}
		for _, name := range nativeBindingNames {
			supported[name] = true
		}
		for _, row := range doomruntime.NativeBindingDefinitions(*cfg.Keybinds) {
			for _, name := range row.Value {
				name = strings.ToUpper(strings.TrimSpace(name))
				if name != "" && !supported[name] {
					return nil, fmt.Errorf("unsupported key %q for %s", name, row.Label)
				}
			}
		}
	}
	// Like the main host, disabling VSync also removes the default draw cap.
	// A chosen native limiter (command line or saved preference) still wins.
	if f := fs.Lookup("no-vsync"); f != nil && f.Value.String() == "true" {
		chosenFPS := false
		fs.Visit(func(f *flag.Flag) { chosenFPS = chosenFPS || f.Name == "fps" })
		if !chosenFPS && fs.Lookup("fps") != nil {
			if err := fs.Set("fps", "0"); err != nil {
				return nil, err
			}
		}
	}
	return cfg, nil
}

func saveNativePreferences(path string, s nativeSettings, bindings runtimecfg.InputBindings, width, height, textureScale int, msaa bool, mode string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cfg, err := configfile.Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if cfg == nil {
		cfg = &configfile.Config{}
	}
	cfg.SFXVolume, cfg.MusicVolume = &s.sfxVolume, &s.musicVolume
	noFPS := !s.showFPS
	cfg.NoFPS = &noFPS
	backend := s.musicConfig().backend.String()
	cfg.MusicBackend, cfg.SoundFont = &backend, &s.soundFont
	cfg.MUSPanMax, cfg.PCSpeakerVolume, cfg.PCSpeakerVariant = &s.musicPan, &s.speakerVolume, &s.speakerVariant
	cfg.PCSpeaker = &s.pcSpeaker
	cfg.MouseLookSpeed, cfg.KeyboardTurnSpeed = &s.mouseSensitivity, &s.keyboardSpeed
	cfg.MouseLook, cfg.MouseInvert = &s.mouseLook, &s.mouseInvert
	cfg.SmoothCameraYaw = &s.smoothCameraYaw
	cfg.GammaLevel = &s.gammaLevel
	cfg.CRTEffect = &s.crtEffect
	cfg.NoAspectCorrection = &s.noAspectCorrection
	cfg.NoVsync = &s.noVsync
	cfg.DetailLevelSourcePort, cfg.AutoDetail = &s.detailLevel, &s.autoDetail
	cfg.AlwaysRun, cfg.AutoWeaponSwitch = &s.alwaysRun, &s.autoWeaponSwitch
	normalized := runtimecfg.NormalizeInputBindings(bindings)
	cfg.Keybinds = &normalized
	filter, lighting := string(s.textureFilter), string(s.lighting)
	cfg.Raylib = &runtimecfg.RaylibSettings{
		Messages: &s.messages, ScreenBlocks: &s.screenBlocks, HUDScale: &s.hudScale, Width: &width, Height: &height, FPS: &s.fps, Fullscreen: &s.fullscreen, Debug: &s.debug, MSAA: &msaa,
		TextureScale: &textureScale, TextureFilter: &filter, Lighting: &lighting, MeshMode: &mode,
	}
	return configfile.Write(path, cfg)
}
func validNativeControlSpeed(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
