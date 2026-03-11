package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/doomsession"
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

func TestExplicitMapStartInMap(t *testing.T) {
	if !explicitMapStartInMap(false, true) {
		t.Fatal("explicit CLI map should force start-in-map")
	}
	if !explicitMapStartInMap(true, false) {
		t.Fatal("explicit start-in-map flag should still start in map")
	}
	if explicitMapStartInMap(false, false) {
		t.Fatal("neither flag should not force start-in-map")
	}
}

func TestRunParseUsesPositionalWADArgument(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{wadPath, "-render=false"}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if strings.Contains(errb.String(), "open wad:") {
		t.Fatalf("stderr %q unexpectedly contains wad open error", errb.String())
	}
}

func TestRunParseTreatsPositionalWADAsExplicitAndSkipsPicker(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	code := RunParse([]string{"missing-from-cli.wad", "-render=false"}, &out, &errb)
	if code != 1 {
		t.Fatalf("RunParse() code=%d want=1 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "open wad:") {
		t.Fatalf("stderr %q does not contain open wad error", errb.String())
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

func TestRunParseDemoOverridesSelectedMapFromHeader(t *testing.T) {
	td := t.TempDir()
	demoPath := filepath.Join(td, "demo.lmp")
	data, err := demo.Format(&demo.Script{
		Header: demo.Header{
			Version:      110,
			Skill:        2,
			Episode:      1,
			Map:          2,
			PlayerInGame: [4]bool{true},
		},
		Tics: []demo.Tic{{Forward: 25}},
	})
	if err != nil {
		t.Fatalf("FormatDemoScript() error = %v", err)
	}
	if err := os.WriteFile(demoPath, data, 0o644); err != nil {
		t.Fatalf("write demo: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-render=false", "-map", "E1M1", "-demo", demoPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
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

func TestRunParseSourcePortDefaultsEnableGPUSky(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-render=false", "-sourceport-mode"}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=") {
		t.Fatalf("stdout %q missing map output", out.String())
	}
}

func TestRunParseSourcePortDefaultsPreserveExplicitNearestSkyUpscale(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{
		"-wad", wadPath,
		"-render=false",
		"-sourceport-mode",
		"-gpu-sky=false",
		"-sky-upscale", "nearest",
	}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=") {
		t.Fatalf("stdout %q missing map output", out.String())
	}
}

func TestLoadConfigParsesSkyUpscaleMode(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("sourceport_mode = true\nsky_upscale = \"sharp\"\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if loaded.SkyUpscaleMode == nil || *loaded.SkyUpscaleMode != "sharp" {
		t.Fatalf("sky_upscale=%v want sharp", loaded.SkyUpscaleMode)
	}
}

func TestRunParseLoadsDoomLightingFromConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\ndoom_lighting = false\n")
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

func TestRunParseLoadsWallOcclusionFromConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\nwall_occlusion = false\n")
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

func TestLoadConfigParsesSourcePortSectorLighting(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("sourceport_mode = true\nsourceport_sector_lighting = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if loaded.SourcePortMode == nil || !*loaded.SourcePortMode {
		t.Fatalf("sourceport_mode=%v want true", loaded.SourcePortMode)
	}
	if loaded.SourcePortSectorLighting == nil || !*loaded.SourcePortSectorLighting {
		t.Fatalf("sourceport_sector_lighting=%v want true", loaded.SourcePortSectorLighting)
	}
}

func TestLoadConfigParsesSourcePortThingRenderMode(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("sourceport_mode = true\nsourceport_thing_render_mode = \"sprites\"\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if loaded.SourcePortThingRenderMode == nil || *loaded.SourcePortThingRenderMode != "sprites" {
		t.Fatalf("sourceport_thing_render_mode=%v want sprites", loaded.SourcePortThingRenderMode)
	}
}

func TestLoadConfigParsesSourcePortThingBlendFrames(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("sourceport_mode = true\nsourceport_thing_blend_frames = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if loaded.SourcePortThingBlendFrames == nil || !*loaded.SourcePortThingBlendFrames {
		t.Fatalf("sourceport_thing_blend_frames=%v want true", loaded.SourcePortThingBlendFrames)
	}
}

func TestLoadConfigParsesItemSpawnOverrides(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("show_no_skill_items = true\nshow_all_items = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if loaded.ShowNoSkillItems == nil || !*loaded.ShowNoSkillItems {
		t.Fatalf("show_no_skill_items=%v want true", loaded.ShowNoSkillItems)
	}
	if loaded.ShowAllItems == nil || !*loaded.ShowAllItems {
		t.Fatalf("show_all_items=%v want true", loaded.ShowAllItems)
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
	in := doomsession.RuntimeSettings{
		DetailLevel:      2,
		GammaLevel:       5,
		MusicVolume:      1.0,
		MUSPanMax:        0.8,
		OPLVolume:        2.0,
		SFXVolume:        0.25,
		MouseLook:        false,
		AlwaysRun:        true,
		AutoWeaponSwitch: false,
		LineColorMode:    "doom",
		CRTEffect:        true,
	}
	if err := saveRuntimeSettings(cfgPath, in, true); err != nil {
		t.Fatalf("saveRuntimeSettings() error: %v", err)
	}
	cfg, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if cfg.DetailLevelSourcePort == nil || *cfg.DetailLevelSourcePort != in.DetailLevel {
		t.Fatalf("detail_level_sourceport=%v want %d", cfg.DetailLevelSourcePort, in.DetailLevel)
	}
	if cfg.GammaLevel == nil || *cfg.GammaLevel != in.GammaLevel {
		t.Fatalf("gamma_level=%v want %d", cfg.GammaLevel, in.GammaLevel)
	}
	if cfg.MusicVolume == nil || *cfg.MusicVolume != in.MusicVolume {
		t.Fatalf("music_volume=%v want %v", cfg.MusicVolume, in.MusicVolume)
	}
	if cfg.MUSPanMax == nil || *cfg.MUSPanMax != in.MUSPanMax {
		t.Fatalf("mus_pan_max=%v want %v", cfg.MUSPanMax, in.MUSPanMax)
	}
	if cfg.OPLVolume == nil || *cfg.OPLVolume != in.OPLVolume {
		t.Fatalf("opl_volume=%v want %v", cfg.OPLVolume, in.OPLVolume)
	}
	if cfg.SFXVolume == nil || *cfg.SFXVolume != in.SFXVolume {
		t.Fatalf("sfx_volume=%v want %v", cfg.SFXVolume, in.SFXVolume)
	}
	if cfg.MouseLook == nil || *cfg.MouseLook != in.MouseLook {
		t.Fatalf("mouselook=%v want %v", cfg.MouseLook, in.MouseLook)
	}
	if cfg.AlwaysRun == nil || *cfg.AlwaysRun != in.AlwaysRun {
		t.Fatalf("always_run=%v want %v", cfg.AlwaysRun, in.AlwaysRun)
	}
	if cfg.AutoWeaponSwitch == nil || *cfg.AutoWeaponSwitch != in.AutoWeaponSwitch {
		t.Fatalf("auto_weapon_switch=%v want %v", cfg.AutoWeaponSwitch, in.AutoWeaponSwitch)
	}
	if cfg.LineColorMode == nil || *cfg.LineColorMode != in.LineColorMode {
		t.Fatalf("line_color_mode=%v want %v", cfg.LineColorMode, in.LineColorMode)
	}
	if cfg.CRTEffect == nil || *cfg.CRTEffect != in.CRTEffect {
		t.Fatalf("crt_effect=%v want %v", cfg.CRTEffect, in.CRTEffect)
	}
	if _, err := os.Stat(cfgPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover tmp file, stat err=%v", err)
	}
}

func TestSaveRuntimeSettingsWritesFaithfulDetailSeparately(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "config.toml")
	if err := saveRuntimeSettings(cfgPath, doomsession.RuntimeSettings{DetailLevel: 1}, false); err != nil {
		t.Fatalf("saveRuntimeSettings() faithful error: %v", err)
	}
	if err := saveRuntimeSettings(cfgPath, doomsession.RuntimeSettings{DetailLevel: 3}, true); err != nil {
		t.Fatalf("saveRuntimeSettings() sourceport error: %v", err)
	}
	cfg, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if cfg.DetailLevelFaithful == nil || *cfg.DetailLevelFaithful != 1 {
		t.Fatalf("detail_level_faithful=%v want 1", cfg.DetailLevelFaithful)
	}
	if cfg.DetailLevelSourcePort == nil || *cfg.DetailLevelSourcePort != 3 {
		t.Fatalf("detail_level_sourceport=%v want 3", cfg.DetailLevelSourcePort)
	}
}

func TestConfiguredDetailLevelForModeUsesModeSpecificOnly(t *testing.T) {
	cfg := &fileConfig{
		DetailLevelFaithful:   intPtr(1),
		DetailLevelSourcePort: intPtr(3),
	}
	if got := configuredDetailLevelForMode(cfg, false); got != 1 {
		t.Fatalf("configuredDetailLevelForMode(faithful)=%d want 1", got)
	}
	if got := configuredDetailLevelForMode(cfg, true); got != 3 {
		t.Fatalf("configuredDetailLevelForMode(sourceport)=%d want 3", got)
	}
	cfg.DetailLevelFaithful = nil
	cfg.DetailLevelSourcePort = nil
	if got := configuredDetailLevelForMode(cfg, false); got != -1 {
		t.Fatalf("configuredDetailLevelForMode(fallback faithful)=%d want -1", got)
	}
	if got := configuredDetailLevelForMode(cfg, true); got != -1 {
		t.Fatalf("configuredDetailLevelForMode(fallback sourceport)=%d want -1", got)
	}
}

func TestLoadConfigRewritesFileAtomically(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	original := "render=true\nmap=\"E1M2\"\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig(cfgPath, true)
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}
	if cfg.Map == nil || *cfg.Map != "E1M2" {
		t.Fatalf("map=%v want E1M2", cfg.Map)
	}
	if cfg.Render == nil || !*cfg.Render {
		t.Fatalf("render=%v want true", cfg.Render)
	}

	rewritten, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	if string(rewritten) == original {
		t.Fatalf("expected config to be rewritten, content unchanged: %q", rewritten)
	}
	if _, err := os.Stat(cfgPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover tmp file, stat err=%v", err)
	}
}
