package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"sync"

	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

type authorityLaunch struct {
	Address, Name string
	Spectator     bool
}

// Retain a successful HTTPS fallback, so discovery, join and reconnection do
// not each wait for an unavailable UDP path before opening the same WebSocket.
type authorityConnector struct {
	mu      sync.Mutex
	address string
}

func (c *authorityConnector) open(ctx context.Context) (netgame.MessageTransport, error) {
	c.mu.Lock()
	address := c.address
	c.mu.Unlock()
	transport, selected, err := openAuthorityTransportSelected(ctx, address)
	if err == nil {
		c.mu.Lock()
		c.address = selected
		c.mu.Unlock()
	}
	return transport, err
}

func openAuthorityTransport(ctx context.Context, address string) (netgame.MessageTransport, error) {
	transport, _, err := openAuthorityTransportSelected(ctx, address)
	return transport, err
}

func openAuthorityTransportSelected(ctx context.Context, address string) (netgame.MessageTransport, string, error) {
	address = strings.TrimSpace(address)
	if strings.HasPrefix(address, "wt://") {
		transport, err := netgame.OpenWebTransport(ctx, address)
		return transport, address, err
	}
	if strings.HasPrefix(address, "https://") {
		transport, datagramErr := netgame.OpenWebTransport(ctx, address)
		if datagramErr == nil {
			return transport, address, nil
		}
		if ctx.Err() != nil {
			return nil, address, ctx.Err()
		}
		fallback, err := url.Parse(address)
		if err != nil {
			return nil, address, err
		}
		fallback.Scheme = "wss"
		transport, streamErr := netgame.OpenWebSocket(ctx, fallback.String())
		if streamErr == nil {
			return transport, fallback.String(), nil
		}
		return nil, address, errors.Join(fmt.Errorf("WebTransport: %w", datagramErr), fmt.Errorf("WebSocket fallback: %w", streamErr))
	}
	if strings.HasPrefix(address, "ws://") || strings.HasPrefix(address, "wss://") {
		transport, err := netgame.OpenWebSocket(ctx, address)
		return transport, address, err
	}
	if runtime.GOOS == "js" {
		return nil, address, fmt.Errorf("browser multiplayer requires an HTTPS or WebSocket server URL")
	}
	if strings.HasPrefix(address, "tls://") {
		transport, err := netgame.OpenTLS(ctx, strings.TrimPrefix(address, "tls://"))
		return transport, address, err
	}
	transport, err := netgame.OpenTCP(ctx, strings.TrimPrefix(address, "tcp://"))
	return transport, address, err
}

func dialAuthority(ctx context.Context, cfg authorityLaunch, paths []string, file *wad.File) (runtimecfg.AuthorityJoinResult, error) {
	hashes, err := launchcatalog.HashWADStackSHA256(paths)
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	return dialAuthorityWithHashes(ctx, cfg, hashes, file)
}

func dialAuthorityWithHashes(ctx context.Context, cfg authorityLaunch, hashes []string, file *wad.File) (runtimecfg.AuthorityJoinResult, error) {
	connector := &authorityConnector{address: cfg.Address}
	query := func(ctx context.Context) (netgame.CompatibilityManifest, error) {
		transport, err := connector.open(ctx)
		if err != nil {
			return netgame.CompatibilityManifest{}, err
		}
		info, err := netgame.QueryServer(ctx, transport)
		if err != nil {
			return netgame.CompatibilityManifest{}, fmt.Errorf("discover match: %w", err)
		}
		manifest := info.Manifest
		serverKey, err := manifest.Key()
		if err != nil {
			return netgame.CompatibilityManifest{}, err
		}
		// Use our own simulation/content identity. Server metadata selects rules,
		// never a filesystem path or unverified replacement for the local WADs.
		manifest.Simulation, manifest.WADHashes = netgame.SimulationVersion, hashes
		key, err := manifest.Key()
		if err != nil {
			return netgame.CompatibilityManifest{}, err
		}
		if key != serverKey {
			return netgame.CompatibilityManifest{}, fmt.Errorf("server requires matching engine and WADs in the same load order")
		}
		return manifest, nil
	}
	manifest, err := query(ctx)
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	key, err := manifest.Key()
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	_, err = manifest.ContentKey()
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	m, err := mapdata.LoadMap(file, mapdata.MapName(manifest.Map))
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	transport, err := connector.open(ctx)
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	initial, err := netgame.Connect(ctx, transport, netgame.Hello{Compatibility: key, Name: cfg.Name, Spectator: cfg.Spectator})
	if err != nil {
		return runtimecfg.AuthorityJoinResult{}, err
	}
	client, err := netgame.NewReconnectingClient(ctx, initial, manifest, func(ctx context.Context, token [32]byte) (*netgame.Client, netgame.CompatibilityManifest, error) {
		next, err := query(ctx)
		if err != nil {
			return nil, next, err
		}
		// A map may rotate during an outage. Engine, content and match rules must
		// still match the session we joined before any new state is accepted.
		expected := manifest
		expected.Map = next.Map
		expectedKey, keyErr := expected.Key()
		nextKey, nextErr := next.Key()
		if keyErr != nil || nextErr != nil || expectedKey != nextKey {
			return nil, next, netgame.ErrProtocol
		}
		transport, err := connector.open(ctx)
		if err != nil {
			return nil, next, err
		}
		resumed, err := netgame.Connect(ctx, transport, netgame.Hello{Compatibility: nextKey, Name: cfg.Name, ResumeToken: token, Spectator: cfg.Spectator})
		return resumed, next, err
	})
	if err != nil {
		initial.Leave()
		return runtimecfg.AuthorityJoinResult{}, err
	}
	result := runtimecfg.AuthorityJoinResult{Client: client, Map: m, Manifest: manifest}
	result.Manifest.WADHashes = append([]string(nil), manifest.WADHashes...)
	result.MapLoader = func(change netgame.MapChange) (*mapdata.Map, error) {
		nextManifest := manifest
		nextManifest.Map = change.Map
		expected, err := nextManifest.Key()
		if err != nil {
			return nil, err
		}
		if expected != change.Compatibility {
			return nil, fmt.Errorf("server map change content/rules mismatch")
		}
		return mapdata.LoadMap(file, mapdata.MapName(change.Map))
	}
	return result, nil
}

// authorityJoiner captures only immutable loaded content; each join receives its
// own connector and lifetime context, so failed or canceled attempts can retry.
func authorityJoiner(paths []string, file *wad.File) func(context.Context, runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
	paths = append([]string(nil), paths...)
	// Keep the loaded stack's identity fixed for this game, even if the files
	// are replaced on disk before a later menu join.
	hashes, hashErr := launchcatalog.HashWADStackSHA256(paths)
	return func(ctx context.Context, request runtimecfg.AuthorityJoinRequest) (runtimecfg.AuthorityJoinResult, error) {
		if err := ctx.Err(); err != nil {
			return runtimecfg.AuthorityJoinResult{}, err
		}
		if hashErr != nil {
			return runtimecfg.AuthorityJoinResult{}, hashErr
		}
		request.Address = strings.TrimSpace(request.Address)
		request.Name = strings.TrimSpace(request.Name)
		if request.Address == "" {
			return runtimecfg.AuthorityJoinResult{}, fmt.Errorf("enter a multiplayer server address")
		}
		if request.Name == "" {
			request.Name = "Player"
		}
		return dialAuthorityWithHashes(ctx, authorityLaunch{Address: request.Address, Name: request.Name, Spectator: request.Spectator}, hashes, file)
	}
}

func connectAuthority(ctx context.Context, cfg authorityLaunch, paths []string, file *wad.File, opts *runtimecfg.Options) (*netgame.ReconnectingClient, *mapdata.Map, error) {
	result, err := dialAuthority(ctx, cfg, paths, file)
	if err != nil {
		return nil, nil, err
	}
	manifest := result.Manifest
	content, _ := manifest.ContentKey() // dialAuthority already validated the manifest.
	client := result.Client.(*netgame.ReconnectingClient)
	local := &runtimecfg.AuthorityLocalRules{
		GameMode: "single", SkillLevel: opts.SkillLevel, PlayerSlot: 1,
		NoMonsters: opts.NoMonsters, FastMonsters: opts.FastMonsters, RespawnMonsters: opts.RespawnMonsters,
		WADHash: opts.WADHash, AllCheats: opts.AllCheats, CheatLevel: opts.CheatLevel, Invulnerable: opts.Invulnerable,
		ShowAllItems: opts.ShowAllItems, ShowNoSkillItems: opts.ShowNoSkillItems,
	}
	opts.AuthorityLocalRules = local
	opts.GameMode, opts.SkillLevel = manifest.Mode, manifest.Skill
	opts.NoMonsters, opts.FastMonsters, opts.RespawnMonsters = manifest.NoMonsters, manifest.FastMonsters, manifest.RespawnMonsters
	opts.WADHash = content
	opts.PlayerSlot = int(client.Welcome().PlayerID)
	opts.AuthorityClient, opts.AuthorityMapLoader = client, result.MapLoader
	return client, result.Map, nil
}
