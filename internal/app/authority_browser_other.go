//go:build !js || !wasm

package app

import (
	"strings"

	"gddoom/internal/configfile"
	"gddoom/internal/runtimecfg"
)

func loadAuthorityServers(path string) []runtimecfg.AuthorityServerEntry {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cfg, err := configfile.Read(path)
	if err != nil {
		return nil
	}
	return cleanAuthorityServers(cfg.MultiplayerServers)
}

func saveAuthorityServers(path string, entries []runtimecfg.AuthorityServerEntry) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cfg, err := loadConfig(path, false)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &fileConfig{}
	}
	cfg.MultiplayerServers = cleanAuthorityServers(entries)
	return writeConfigAtomic(path, cfg)
}
