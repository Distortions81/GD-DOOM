// Package configfile shares the on-disk preferences model between window hosts.
package configfile

import (
	"bytes"
	"fmt"
	"gddoom/internal/runtimecfg"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
)

type Config struct {
	Wad                        *string                           `toml:"wad"`
	Map                        *string                           `toml:"map"`
	Render                     *bool                             `toml:"render"`
	Debug                      *bool                             `toml:"debug"`
	DebugEvents                *bool                             `toml:"debug_events"`
	Width                      *int                              `toml:"width"`
	Height                     *int                              `toml:"height"`
	DetailLevelFaithful        *int                              `toml:"detail_level_faithful"`
	DetailLevelSourcePort      *int                              `toml:"detail_level_sourceport"`
	AutoDetail                 *bool                             `toml:"auto_detail"`
	MultiplayerServers         []runtimecfg.AuthorityServerEntry `toml:"multiplayer_servers"`
	GammaLevel                 *int                              `toml:"gamma_level"`
	Player                     *int                              `toml:"player"`
	Skill                      *int                              `toml:"skill"`
	ShowNoSkillItems           *bool                             `toml:"show_no_skill_items"`
	ShowAllItems               *bool                             `toml:"show_all_items"`
	MouseLook                  *bool                             `toml:"mouselook"`
	MouseInvert                *bool                             `toml:"mouse_invert"`
	MouseInvertHorizontal      *bool                             `toml:"mouse_invert_horizontal"`
	SmoothCameraYaw            *bool                             `toml:"smooth_camera_yaw"`
	MouseLookSpeed             *float64                          `toml:"mouselook_speed"`
	KeyboardTurnSpeed          *float64                          `toml:"keyboard_turn_speed"`
	MusicVolume                *float64                          `toml:"music_volume"`
	MUSPanMax                  *float64                          `toml:"mus_pan_max"`
	MusicBackend               *string                           `toml:"music_backend"`
	SoundFont                  *string                           `toml:"soundfont"`
	SFXVolume                  *float64                          `toml:"sfx_volume"`
	PCSpeakerVolume            *float64                          `toml:"pc_speaker_volume"`
	AlwaysRun                  *bool                             `toml:"always_run"`
	AutoWeaponSwitch           *bool                             `toml:"auto_weapon_switch"`
	CheatLevel                 *int                              `toml:"cheat_level"`
	Invulnerable               *bool                             `toml:"invulnerable"`
	SourcePortMode             *bool                             `toml:"sourceport_mode"`
	CRTEffect                  *bool                             `toml:"crt_effect"`
	RendererWorkers            *int                              `toml:"renderer_workers"`
	TextureAnimCrossfadeFrames *int                              `toml:"texture_anim_crossfade_frames"`
	AllCheats                  *bool                             `toml:"all_cheats"`
	Details                    *bool                             `toml:"details"`
	CPUProfile                 *string                           `toml:"cpu_profile"`
	MemProfile                 *string                           `toml:"mem_profile"`
	ExecTrace                  *string                           `toml:"exec_trace"`
	Demo                       *string                           `toml:"demo"`
	RecordDemo                 *string                           `toml:"record_demo"`
	DemoStopAfterTics          *int                              `toml:"demo_stop_after_tics"`
	PCSpeaker                  *bool                             `toml:"pc_speaker"`
	PCSpeakerVariant           *string                           `toml:"pc_speaker_variant"`
	NoVsync                    *bool                             `toml:"no_vsync"`
	NoFPS                      *bool                             `toml:"no_fps"`
	NoAspectCorrection         *bool                             `toml:"no_aspect_correction"`
	Keybinds                   *runtimecfg.InputBindings         `toml:"keybinds"`
	Raylib                     *runtimecfg.RaylibSettings        `toml:"raylib"`
}

func Read(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
func Write(path string, cfg *Config) error {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := WriteBytesAtomic(path, b.Bytes()); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

func WriteBytesAtomic(path string, data []byte) error {
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
