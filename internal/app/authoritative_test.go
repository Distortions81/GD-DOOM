package app

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

func TestAuthorityLaunchDiscoversRulesAndLoadsServerMap(t *testing.T) {
	path := filepath.Join("..", "..", "DOOM1.WAD")
	file, paths, err := openWADStack(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := launchcatalog.HashWADStackSHA256(paths)
	if err != nil {
		t.Fatal(err)
	}
	manifest := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: hashes, Map: "E1M2", Mode: "coop", Skill: 4, NoMonsters: true, RespawnDelayTics: 35}
	key, err := manifest.Key()
	if err != nil {
		t.Fatal(err)
	}
	content, err := manifest.ContentKey()
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(file, "E1M2")
	if err != nil {
		t.Fatal(err)
	}
	world, err := doomruntime.NewAuthority(m, doomruntime.Options{GameMode: "coop", SkillLevel: 4, NoMonsters: true, WADHash: content, SFXVolume: 1})
	if err != nil {
		t.Fatal(err)
	}
	match, err := netgame.NewMatch(world, world, netgame.MatchConfig{Epoch: 1, Compatibility: key, Manifest: &manifest, PlayerLimit: 4, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: 350, SnapshotInterval: 2})
	if err != nil {
		t.Fatal(err)
	}
	server, err := netgame.NewServer(match)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	}()
	opts := runtimecfg.Options{GameMode: "single", SkillLevel: 1}
	client, joined, err := connectAuthority(ctx, authorityLaunch{Address: listener.Addr().String(), Name: "browser-compatible"}, paths, file, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if joined.Name != "E1M2" || opts.GameMode != "coop" || opts.SkillLevel != 4 || !opts.NoMonsters || opts.PlayerSlot != 1 || opts.WADHash != content || opts.AuthorityClient != client {
		t.Fatalf("server settings not applied: map=%s mode=%s skill=%d", joined.Name, opts.GameMode, opts.SkillLevel)
	}
	next := manifest
	next.Map = "E1M3"
	nextKey, _ := next.Key()
	loaded, err := opts.AuthorityMapLoader(netgame.MapChange{Map: "E1M3", Compatibility: nextKey})
	if err != nil || loaded.Name != "E1M3" {
		t.Fatalf("map transition load: %v", err)
	}
	if _, err := opts.AuthorityMapLoader(netgame.MapChange{Map: "E1M3", Compatibility: "wrong"}); err == nil {
		t.Fatal("unverified map change accepted")
	}
}
