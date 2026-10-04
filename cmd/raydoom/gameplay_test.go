//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestNativeGameplayConfigAndAliasesKeepExplicitValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("player=2\ncheat_level=3\ninvulnerable=true\nshow_no_skill_items=true\nshow_all_items=true\nno_fps=true\nmouselook_speed=2\nkeyboard_turn_speed=3\nmouse_invert_horizontal=true\ndemo_stop_after_tics=75\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := nativePreferenceFlags(path)
	if err := fs.Parse([]string{"-invuln=false", "-no-monsters", "-cheat-level=0", "-nofps=false", "-mouselook-speed=.5", "-keyboard-turn-speed=1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"player": "2", "cheat-level": "0", "god": "false", "nomonsters": "true", "nofps": "false", "mouse-speed": "0.5", "keyboard-speed": "1", "mouse-invert": "true", "demo-stop-after-tics": "75", "show-no-skill-items": "true", "show-all-items": "true"} {
		if got := fs.Lookup(name).Value.String(); got != want {
			t.Fatalf("%s=%q want%q", name, got, want)
		}
	}
}

func TestNativeGameplayFlagsDriveExistingStartupState(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{0, 1, 2, 3} {
		f := nativeGameplayFlags{player: new(int), cheatLevel: &level, textureCrossfade: new(int), showNoSkillItems: new(bool), showAllItems: new(bool), allCheats: new(bool), noFPS: new(bool), debugEvents: new(bool)}
		*f.player = 2
		opts, err := loadAssets(wf)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Apply(&opts); err != nil {
			t.Fatal(err)
		}
		game := doomruntime.NewNativeMeshGame(m, opts)
		frame := game.Frame(1)
		if opts.PlayerSlot != 2 || opts.GameMode != "single" || !opts.SourcePortMode {
			t.Fatal("startup/network profile disagrees with simulation")
		}
		var player2 *mapdata.Thing
		for i := range m.Things {
			if m.Things[i].Type == 2 {
				player2 = &m.Things[i]
				break
			}
		}
		if player2 == nil || frame.Camera.X != float64(player2.X) || frame.Camera.Y != float64(player2.Y) {
			t.Fatal("-player did not select existing Doom start")
		}
		if level >= 2 && frame.Armor != 200 {
			t.Fatal("startup cheats did not grant main inventory")
		}
	}
}

func TestNativeTextureCrossfadeConfigAndExplicitDisable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("texture_anim_crossfade_frames=5\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := nativePreferenceFlags(path)
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	if fs.Lookup("texture-anim-crossfade-frames").Value.String() != "5" {
		t.Fatal("native launcher ignored the main crossfade preference")
	}
	fs = nativePreferenceFlags(path)
	if err := fs.Parse([]string{"-texture-anim-crossfade-frames=0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	if fs.Lookup("texture-anim-crossfade-frames").Value.String() != "0" {
		t.Fatal("explicit disable was replaced by the saved preference")
	}
	if nativePreferenceFlags("").Lookup("texture-anim-crossfade-frames").Value.String() != "7" {
		t.Fatal("native default differs from main")
	}
}
