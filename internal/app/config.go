package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gddoom/internal/doomsession"

	"github.com/BurntSushi/toml"
)

type fileConfig struct {
	Wad                        *string  `toml:"wad"`
	Map                        *string  `toml:"map"`
	Render                     *bool    `toml:"render"`
	Debug                      *bool    `toml:"debug"`
	DebugEvents                *bool    `toml:"debug_events"`
	Width                      *int     `toml:"width"`
	Height                     *int     `toml:"height"`
	DetailLevelFaithful        *int     `toml:"detail_level_faithful"`
	DetailLevelSourcePort      *int     `toml:"detail_level_sourceport"`
	GammaLevel                 *int     `toml:"gamma_level"`
	Zoom                       *float64 `toml:"zoom"`
	Player                     *int     `toml:"player"`
	Skill                      *int     `toml:"skill"`
	GameMode                   *string  `toml:"game_mode"`
	ShowNoSkillItems           *bool    `toml:"show_no_skill_items"`
	ShowAllItems               *bool    `toml:"show_all_items"`
	MouseLook                  *bool    `toml:"mouselook"`
	MouseLookSpeed             *float64 `toml:"mouselook_speed"`
	KeyboardTurnSpeed          *float64 `toml:"keyboard_turn_speed"`
	MusicVolume                *float64 `toml:"music_volume"`
	MUSPanMax                  *float64 `toml:"mus_pan_max"`
	OPLVolume                  *float64 `toml:"opl_volume"`
	AudioPreEmphasis           *bool    `toml:"audio_preemphasis"`
	OPL3Backend                *string  `toml:"opl3_backend"`
	OPLBank                    *string  `toml:"opl_bank"`
	SFXVolume                  *float64 `toml:"sfx_volume"`
	SFXPitchShift              *bool    `toml:"sfx_pitch_shift"`
	FastMonsters               *bool    `toml:"fast_monsters"`
	AlwaysRun                  *bool    `toml:"always_run"`
	AutoWeaponSwitch           *bool    `toml:"auto_weapon_switch"`
	CheatLevel                 *int     `toml:"cheat_level"`
	Invulnerable               *bool    `toml:"invulnerable"`
	ImportTextures             *bool    `toml:"import_textures"`
	LineColorMode              *string  `toml:"line_color_mode"`
	SourcePortMode             *bool    `toml:"sourceport_mode"`
	SourcePortThingRenderMode  *string  `toml:"sourceport_thing_render_mode"`
	SourcePortThingBlendFrames *bool    `toml:"sourceport_thing_blend_frames"`
	SourcePortItemSprites      *bool    `toml:"sourceport_item_sprites"`
	SourcePortSectorLighting   *bool    `toml:"sourceport_sector_lighting"`
	DoomLighting               *bool    `toml:"doom_lighting"`
	KageShader                 *bool    `toml:"kage_shader"`
	GPUSky                     *bool    `toml:"gpu_sky"`
	SkyUpscaleMode             *string  `toml:"sky_upscale"`
	CRTEffect                  *bool    `toml:"crt_effect"`
	WallOcclusion              *bool    `toml:"wall_occlusion"`
	WallSpanReject             *bool    `toml:"wall_span_reject"`
	WallSpanClip               *bool    `toml:"wall_span_clip"`
	WallSliceOcclusion         *bool    `toml:"wall_slice_occlusion"`
	BillboardClipping          *bool    `toml:"billboard_clipping"`
	RendererWorkers            *int     `toml:"renderer_workers"`
	TextureAnimCrossfadeFrames *int     `toml:"texture_anim_crossfade_frames"`
	AllCheats                  *bool    `toml:"all_cheats"`
	StartInMap                 *bool    `toml:"start_in_map"`
	ImportPCSpeaker            *bool    `toml:"import_pcspeaker"`
	Details                    *bool    `toml:"details"`
	CPUProfile                 *string  `toml:"cpu_profile"`
	MemProfile                 *string  `toml:"mem_profile"`
	ExecTrace                  *string  `toml:"exec_trace"`
	Demo                       *string  `toml:"demo"`
	RecordDemo                 *string  `toml:"record_demo"`
	DemoStopAfterTics          *int     `toml:"demo_stop_after_tics"`
	NoVsync                    *bool    `toml:"no_vsync"`
	NoFPS                      *bool    `toml:"no_fps"`
	NoAspectCorrection         *bool    `toml:"no_aspect_correction"`
}

func resolveConfigPath(args []string) (path string, explicit bool) {
	path = "config.toml"
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-config=") {
			return strings.TrimPrefix(a, "-config="), true
		}
		if a == "-config" {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
	}
	return path, false
}

func loadConfig(path string, explicit bool) (*fileConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	if !configFileAccessSupported() {
		if explicit {
			return nil, fmt.Errorf("config files are not supported on js/wasm")
		}
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &fileConfig{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := writeConfigAtomic(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func configuredDetailLevelForMode(cfg *fileConfig, sourcePortMode bool) int {
	if cfg == nil {
		return -1
	}
	if sourcePortMode {
		if cfg.DetailLevelSourcePort != nil {
			return *cfg.DetailLevelSourcePort
		}
	} else {
		if cfg.DetailLevelFaithful != nil {
			return *cfg.DetailLevelFaithful
		}
	}
	return -1
}

func saveRuntimeSettings(path string, s doomsession.RuntimeSettings, sourcePortMode bool) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if !configFileAccessSupported() {
		return nil
	}
	cfg := &fileConfig{}
	if loaded, err := loadConfig(path, false); err == nil && loaded != nil {
		cfg = loaded
	} else if err != nil {
		return err
	}
	if sourcePortMode {
		cfg.DetailLevelSourcePort = intPtr(s.DetailLevel)
	} else {
		cfg.DetailLevelFaithful = intPtr(s.DetailLevel)
	}
	cfg.GammaLevel = intPtr(s.GammaLevel)
	cfg.MusicVolume = floatPtr(s.MusicVolume)
	cfg.MUSPanMax = floatPtr(s.MUSPanMax)
	cfg.OPLVolume = floatPtr(s.OPLVolume)
	cfg.SFXVolume = floatPtr(s.SFXVolume)
	cfg.MouseLook = boolPtr(s.MouseLook)
	cfg.AlwaysRun = boolPtr(s.AlwaysRun)
	cfg.AutoWeaponSwitch = boolPtr(s.AutoWeaponSwitch)
	cfg.LineColorMode = strPtr(s.LineColorMode)
	cfg.CRTEffect = boolPtr(s.CRTEffect)
	return writeConfigAtomic(path, cfg)
}

func writeConfigAtomic(path string, cfg *fileConfig) error {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := writeBytesAtomic(path, b.Bytes()); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func writeBytesAtomic(path string, data []byte) error {
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func floatPtr(v float64) *float64 {
	return &v
}
