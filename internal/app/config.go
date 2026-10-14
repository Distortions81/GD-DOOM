package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gddoom/internal/configfile"
	"gddoom/internal/doomsession"
	"gddoom/internal/runtimecfg"
)

type fileConfig = configfile.Config

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
	cfg, err := configfile.Read(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return nil, nil
		}
		return nil, err
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

// A saved AUTO level is the last performance adjustment, not a manual quality
// preference. Start a new launch at full detail and let AUTO measure it again.
// Explicit command-line levels remain useful for profiling and comparisons.
func startupDetailLevel(level int, auto, explicit bool) int {
	if auto && !explicit {
		return 0
	}
	return level
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
	cfg.AutoDetail = boolPtr(s.AutoDetail)
	cfg.GammaLevel = intPtr(s.GammaLevel)
	cfg.MusicVolume = floatPtr(s.MusicVolume)
	cfg.MUSPanMax = floatPtr(s.MUSPanMax)
	cfg.MusicBackend = strPtr(strings.TrimSpace(s.MusicBackend))
	cfg.SoundFont = strPtr(strings.TrimSpace(s.MusicSoundFontPath))
	cfg.SFXVolume = floatPtr(s.SFXVolume)
	cfg.PCSpeakerVolume = floatPtr(s.PCSpeakerVolume)
	cfg.MouseLook = boolPtr(s.MouseLook)
	cfg.MouseInvert = boolPtr(s.MouseInvert)
	cfg.AlwaysRun = boolPtr(s.AlwaysRun)
	cfg.AutoWeaponSwitch = boolPtr(s.AutoWeaponSwitch)
	cfg.CRTEffect = boolPtr(s.CRTEffect)
	return writeConfigAtomic(path, cfg)
}

func saveInputBindings(path string, binds runtimecfg.InputBindings) error {
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
	normalized := runtimecfg.NormalizeInputBindings(binds)
	cfg.Keybinds = &normalized
	return writeConfigAtomic(path, cfg)
}

func writeConfigAtomic(path string, cfg *fileConfig) error { return configfile.Write(path, cfg) }
func writeBytesAtomic(path string, data []byte) error      { return configfile.WriteBytesAtomic(path, data) }

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func floatPtr(v float64) *float64 {
	return &v
}
