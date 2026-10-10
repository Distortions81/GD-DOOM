package app

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gddoom/internal/launchcatalog"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

const (
	maxSavedAuthorityServers    = 32
	defaultAuthorityCoopAddress = "https://m45sci.xyz:6672/netplay"
	defaultAuthorityDMAddress   = "wss://m45sci.xyz:6672/deathmatch"
)

func cleanAuthorityServers(entries []runtimecfg.AuthorityServerEntry) []runtimecfg.AuthorityServerEntry {
	out := make([]runtimecfg.AuthorityServerEntry, 0, min(len(entries), maxSavedAuthorityServers))
	seen := make(map[string]bool)
	for _, entry := range entries {
		entry.Address, entry.Label = strings.TrimSpace(entry.Address), strings.TrimSpace(entry.Label)
		if entry.Address == "" || len(entry.Address) > 512 || !utf8.ValidString(entry.Address) || strings.IndexFunc(entry.Address, unicode.IsControl) >= 0 || seen[entry.Address] {
			continue
		}
		if len(entry.Label) > 64 || !utf8.ValidString(entry.Label) || strings.IndexFunc(entry.Label, unicode.IsControl) >= 0 {
			entry.Label = ""
		}
		seen[entry.Address] = true
		out = append(out, entry)
		if len(out) == maxSavedAuthorityServers {
			break
		}
	}
	return out
}

func configureAuthorityBrowser(opts *runtimecfg.Options, paths []string, configPath string) {
	address := strings.TrimSpace(opts.AuthorityJoinDefaults.Address)
	if address == "" {
		address = defaultAuthorityCoopAddress
	}
	opts.AuthorityJoinDefaults.Address = address
	entries := []runtimecfg.AuthorityServerEntry{
		{Label: "GD-DOOM Co-op", Address: defaultAuthorityCoopAddress},
		{Label: "GD-DOOM Deathmatch", Address: defaultAuthorityDMAddress},
	}
	if address != defaultAuthorityCoopAddress && address != defaultAuthorityDMAddress {
		entries = append([]runtimecfg.AuthorityServerEntry{{Label: "Default server", Address: address}}, entries...)
	}
	entries = append(entries, loadAuthorityServers(configPath)...)
	opts.AuthorityServers = cleanAuthorityServers(entries)
	opts.OnAuthorityServersChanged = func(entries []runtimecfg.AuthorityServerEntry) error {
		return saveAuthorityServers(configPath, cleanAuthorityServers(entries))
	}
	opts.AuthorityDiscover = authorityDiscoverer(paths)
}

func authorityDiscoverer(paths []string) func(context.Context, string) (runtimecfg.AuthorityServerInfo, error) {
	hashes, hashErr := launchcatalog.HashWADStackSHA256(append([]string(nil), paths...))
	return func(ctx context.Context, address string) (runtimecfg.AuthorityServerInfo, error) {
		info := runtimecfg.AuthorityServerInfo{Players: -1, PlayerLimit: -1, Spectators: -1}
		if hashErr != nil {
			return info, hashErr
		}
		address = strings.TrimSpace(address)
		if address == "" {
			return info, fmt.Errorf("enter a server address")
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		connector := &authorityConnector{address: address}
		started := time.Now()
		transport, err := connector.open(ctx)
		if err != nil {
			return info, err
		}
		status, err := netgame.QueryServerStatus(ctx, transport)
		if err == nil {
			info.Manifest = status.Manifest
			info.Players, info.PlayerLimit, info.Spectators = status.Players, status.PlayerLimit, status.Spectators
		} else {
			if ctx.Err() != nil {
				return info, ctx.Err()
			}
			// Older servers understand Query but not StatusQuery. Reconnect for
			// their original metadata response without taking a gameplay slot.
			transport, err = connector.open(ctx)
			if err != nil {
				return info, err
			}
			legacy, err := netgame.QueryServer(ctx, transport)
			if err != nil {
				return info, err
			}
			info.Manifest = legacy.Manifest
		}
		info.Ping = time.Since(started)
		serverKey, err := info.Manifest.Key()
		if err != nil {
			return info, err
		}
		local := info.Manifest
		local.Simulation, local.WADHashes = netgame.SimulationVersion, hashes
		localKey, err := local.Key()
		if err != nil {
			return info, err
		}
		info.Compatible = localKey == serverKey
		if !info.Compatible {
			info.CompatibilityError = "Requires matching engine and loaded WADs"
		}
		return info, nil
	}
}
