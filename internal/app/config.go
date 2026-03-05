package app

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gddoom/internal/render/automap"

	"github.com/BurntSushi/toml"
)

type fileConfig struct {
	Wad                        *string  `toml:"wad"`
	Map                        *string  `toml:"map"`
	Render                     *bool    `toml:"render"`
	Debug                      *bool    `toml:"debug"`
	MultiCore                  *bool    `toml:"multi_core"`
	Width                      *int     `toml:"width"`
	Height                     *int     `toml:"height"`
	DetailLevel                *int     `toml:"detail_level"`
	GammaLevel                 *int     `toml:"gamma_level"`
	Zoom                       *float64 `toml:"zoom"`
	Player                     *int     `toml:"player"`
	Skill                      *int     `toml:"skill"`
	GameMode                   *string  `toml:"game_mode"`
	MouseLook                  *bool    `toml:"mouselook"`
	MouseLookSpeed             *float64 `toml:"mouselook_speed"`
	KeyboardTurnSpeed          *float64 `toml:"keyboard_turn_speed"`
	MusicVolume                *float64 `toml:"music_volume"`
	MUSPanMax                  *float64 `toml:"mus_pan_max"`
	OPLVolume                  *float64 `toml:"opl_volume"`
	SFXVolume                  *float64 `toml:"sfx_volume"`
	FastMonsters               *bool    `toml:"fast_monsters"`
	AlwaysRun                  *bool    `toml:"always_run"`
	AutoWeaponSwitch           *bool    `toml:"auto_weapon_switch"`
	CheatLevel                 *int     `toml:"cheat_level"`
	Invulnerable               *bool    `toml:"invulnerable"`
	ImportTextures             *bool    `toml:"import_textures"`
	LineColorMode              *string  `toml:"line_color_mode"`
	SourcePortMode             *bool    `toml:"sourceport_mode"`
	DoomLighting               *bool    `toml:"doom_lighting"`
	KageShader                 *bool    `toml:"kage_shader"`
	GPUSky                     *bool    `toml:"gpu_sky"`
	CRTEffect                  *bool    `toml:"crt_effect"`
	DepthBufferView            *bool    `toml:"depth_buffer_view"`
	DepthOcclusion             *bool    `toml:"depth_occlusion"`
	TextureAnimCrossfadeFrames *int     `toml:"texture_anim_crossfade_frames"`
	AllCheats                  *bool    `toml:"all_cheats"`
	StartInMap                 *bool    `toml:"start_in_map"`
	ImportPCSpeaker            *bool    `toml:"import_pcspeaker"`
	Details                    *bool    `toml:"details"`
	CPUProfile                 *string  `toml:"cpu_profile"`
	Demo                       *string  `toml:"demo"`
	RecordDemo                 *string  `toml:"record_demo"`
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
	return cfg, nil
}

func saveRuntimeSettings(path string, s automap.RuntimeSettings) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cfg := &fileConfig{}
	if loaded, err := loadConfig(path, false); err == nil && loaded != nil {
		cfg = loaded
	} else if err != nil {
		return err
	}
	cfg.DetailLevel = intPtr(s.DetailLevel)
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
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func floatPtr(v float64) *float64 {
	return &v
}
