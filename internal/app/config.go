package app

import (
	"fmt"
	"os"
	"strings"

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
	Zoom                       *float64 `toml:"zoom"`
	Player                     *int     `toml:"player"`
	Skill                      *int     `toml:"skill"`
	FastMonsters               *bool    `toml:"fast_monsters"`
	CheatLevel                 *int     `toml:"cheat_level"`
	Invulnerable               *bool    `toml:"invulnerable"`
	ImportTextures             *bool    `toml:"import_textures"`
	LineColorMode              *string  `toml:"line_color_mode"`
	SourcePortMode             *bool    `toml:"sourceport_mode"`
	TextureAnimCrossfadeFrames *int     `toml:"texture_anim_crossfade_frames"`
	AllCheats                  *bool    `toml:"all_cheats"`
	StartInMap                 *bool    `toml:"start_in_map"`
	ImportPCSpeaker            *bool    `toml:"import_pcspeaker"`
	Details                    *bool    `toml:"details"`
	CPUProfile                 *string  `toml:"cpu_profile"`
	Demo                       *string  `toml:"demo"`
	RecordDemo                 *string  `toml:"record_demo"`
	NoVsync                    *bool    `toml:"no_vsync"`
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
