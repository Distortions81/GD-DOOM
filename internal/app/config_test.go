package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/render/automap"
)

func TestRunParseLoadsConfigDefaults(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
	}
}

func TestRunParseCLIOverridesConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath, "-map", "E1M1"}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M1 ") {
		t.Fatalf("stdout %q does not contain map=E1M1", out.String())
	}
}

func TestRunParseRejectsDemoAndRecordDemoTogether(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{
		"-wad", wadPath,
		"-render=false",
		"-demo", "bench.demo",
		"-record-demo", "out.demo",
	}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "mutually exclusive") {
		t.Fatalf("stderr %q does not mention mutual exclusion", errb.String())
	}
}

func TestRunParseLoadsNoVsyncFromConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\nno_vsync = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
	}
}

func TestRunParseLoadsGPUSkyFromConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\ngpu_sky = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
	}
}

func TestRunParseRejectsInvalidGameMode(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-game-mode", "bad-mode", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -game-mode") {
		t.Fatalf("stderr %q does not mention invalid game mode", errb.String())
	}
}

func TestRunParseRejectsInvalidKeyboardTurnSpeed(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-keyboard-turn-speed", "0", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -keyboard-turn-speed") {
		t.Fatalf("stderr %q does not mention invalid keyboard turn speed", errb.String())
	}
}

func TestRunParseRejectsInvalidMouseLookSpeed(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-mouselook-speed", "0", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -mouselook-speed") {
		t.Fatalf("stderr %q does not mention invalid mouselook speed", errb.String())
	}
}

func TestRunParseRejectsInvalidMusicVolume(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-music-volume", "1.1", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -music-volume") {
		t.Fatalf("stderr %q does not mention invalid music volume", errb.String())
	}
}

func TestRunParseRejectsInvalidSFXVolume(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-sfx-volume", "-0.1", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -sfx-volume") {
		t.Fatalf("stderr %q does not mention invalid sfx volume", errb.String())
	}
}

func TestRunParseRejectsInvalidMUSPanMax(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-mus-pan-max", "1.1", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -mus-pan-max") {
		t.Fatalf("stderr %q does not mention invalid mus pan max", errb.String())
	}
}

func TestRunParseRejectsInvalidOPLVolume(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"-opl-volume", "4.1", "-render=false"}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "invalid -opl-volume") {
		t.Fatalf("stderr %q does not mention invalid opl volume", errb.String())
	}
}

func TestSaveRuntimeSettingsWritesConfigValues(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "config.toml")
	if err := saveRuntimeSettings(cfgPath, automap.RuntimeSettings{
		DetailLevel:      2,
		GammaLevel:       5,
		MusicVolume:      1.0,
		MUSPanMax:        0.8,
		OPLVolume:        2.0,
		SFXVolume:        0.5,
		MouseLook:        false,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		LineColorMode:    "doom",
		CRTEffect:        true,
	}); err != nil {
		t.Fatalf("saveRuntimeSettings() error: %v", err)
	}
	cfg, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if cfg.DetailLevel == nil || *cfg.DetailLevel != 2 {
		t.Fatalf("detail_level=%v want 2", cfg.DetailLevel)
	}
	if cfg.GammaLevel == nil || *cfg.GammaLevel != 5 {
		t.Fatalf("gamma_level=%v want 5", cfg.GammaLevel)
	}
	if cfg.MusicVolume == nil || *cfg.MusicVolume != 1.0 {
		t.Fatalf("music_volume=%v want 1.0", cfg.MusicVolume)
	}
	if cfg.MUSPanMax == nil || *cfg.MUSPanMax != 0.8 {
		t.Fatalf("mus_pan_max=%v want 0.8", cfg.MUSPanMax)
	}
	if cfg.OPLVolume == nil || *cfg.OPLVolume != 2.0 {
		t.Fatalf("opl_volume=%v want 2.0", cfg.OPLVolume)
	}
	if cfg.SFXVolume == nil || *cfg.SFXVolume != 0.66 {
		t.Fatalf("sfx_volume=%v want 0.66", cfg.SFXVolume)
	}
	if cfg.MouseLook == nil || *cfg.MouseLook {
		t.Fatalf("mouselook=%v want false", cfg.MouseLook)
	}
	if cfg.AlwaysRun == nil || !*cfg.AlwaysRun {
		t.Fatalf("always_run=%v want true", cfg.AlwaysRun)
	}
	if cfg.AutoWeaponSwitch == nil || *cfg.AutoWeaponSwitch {
		t.Fatalf("auto_weapon_switch=%v want false", cfg.AutoWeaponSwitch)
	}
	if cfg.LineColorMode == nil || *cfg.LineColorMode != "doom" {
		t.Fatalf("line_color_mode=%v want doom", cfg.LineColorMode)
	}
	if cfg.CRTEffect == nil || !*cfg.CRTEffect {
		t.Fatalf("crt_effect=%v want true", cfg.CRTEffect)
	}
}
