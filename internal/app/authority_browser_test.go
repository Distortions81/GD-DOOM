package app

import (
	"context"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gddoom/internal/configfile"
	"gddoom/internal/netgame"
	"gddoom/internal/runtimecfg"
)

func TestAuthorityBrowserStatusDoesNotJoinAndReportsCompatibility(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		name := "matching"
		if mismatch {
			name = "mismatch"
		}
		t.Run(name, func(t *testing.T) {
			f := newAuthorityJoinFixture(t, mismatch)
			discover := authorityDiscoverer(f.paths)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			info, err := discover(ctx, f.address)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info.Manifest, f.manifest) || info.Compatible == mismatch || info.Players != 0 || info.PlayerLimit != 1 || info.Spectators != 0 || info.Ping <= 0 {
				t.Fatalf("unexpected browser status: %+v", info)
			}
			players, _, adds, removes := f.world.state()
			if len(players) != 0 || adds != 0 || removes != 0 {
				t.Fatal("discovery changed player admission")
			}
			if mismatch {
				if info.CompatibilityError == "" {
					t.Fatal("mismatch needs an explanation")
				}
				return
			}
			joined, err := authorityJoiner(f.paths, f.file)(ctx, runtimecfg.AuthorityJoinRequest{Address: f.address, Name: "browser-test"})
			if err != nil {
				t.Fatal(err)
			}
			defer joinedClientLeave(t, joined.Client)()
			info, err = discover(ctx, f.address)
			if err != nil || info.Players != 1 || info.PlayerLimit != 1 {
				t.Fatalf("full server cannot be browsed: %+v %v", info, err)
			}
			_, _, adds, _ = f.world.state()
			if adds != 1 {
				t.Fatal("full server query admitted another player")
			}
		})
	}
}

func TestAuthorityBrowserFallsBackToLegacyQuery(t *testing.T) {
	f := newAuthorityJoinFixture(t, false)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		for i := range 2 {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			msg, err := netgame.ReadMessage(conn)
			if err == nil && i == 0 {
				if _, ok := msg.(netgame.StatusQuery); !ok {
					err = netgame.ErrProtocol
				}
			} else if err == nil {
				if _, ok := msg.(netgame.Query); !ok {
					err = netgame.ErrProtocol
				}
				if err == nil {
					frame, marshalErr := netgame.MarshalMessage(netgame.ServerInfo{Manifest: f.manifest})
					err = marshalErr
					if err == nil {
						_, err = conn.Write(frame)
					}
				}
			}
			_ = conn.Close()
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	info, err := authorityDiscoverer(f.paths)(ctx, listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Compatible || !reflect.DeepEqual(info.Manifest, f.manifest) || info.Players != -1 || info.PlayerLimit != -1 || info.Spectators != -1 {
		t.Fatalf("unexpected legacy metadata: %+v", info)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestAuthorityBrowserSavedServersPreservePreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	volume, auto := 0.7, true
	if err := configfile.Write(path, &configfile.Config{MusicVolume: &volume, AutoDetail: &auto}); err != nil {
		t.Fatal(err)
	}
	opts := runtimecfg.Options{}
	configureAuthorityBrowser(&opts, []string{filepath.Join("..", "..", "DOOM1.WAD")}, path)
	if opts.AuthorityJoinDefaults.Address != defaultAuthorityCoopAddress || len(opts.AuthorityServers) != 2 || opts.AuthorityDiscover == nil || opts.OnAuthorityServersChanged == nil {
		t.Fatal("browser has no default server or callbacks")
	}
	entries := []runtimecfg.AuthorityServerEntry{{Label: " Friends ", Address: " ws://localhost:1234/netplay "}, {Label: "duplicate", Address: "ws://localhost:1234/netplay"}, {Address: ""}, {Address: "bad\naddress"}, {Label: strings.Repeat("x", 65), Address: "localhost:5678"}}
	if err := opts.OnAuthorityServersChanged(entries); err != nil {
		t.Fatal(err)
	}
	cfg, err := configfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MusicVolume == nil || *cfg.MusicVolume != volume || cfg.AutoDetail == nil || !*cfg.AutoDetail {
		t.Fatal("saving servers changed other preferences")
	}
	expected := []runtimecfg.AuthorityServerEntry{{Label: "Friends", Address: "ws://localhost:1234/netplay"}, {Address: "localhost:5678"}}
	if got := loadAuthorityServers(path); !reflect.DeepEqual(got, expected) {
		t.Fatalf("saved servers: %+v", got)
	}
	configureAuthorityBrowser(&opts, nil, path)
	if len(opts.AuthorityServers) != 4 || opts.AuthorityServers[2] != expected[0] {
		t.Fatal("relaunch lost saved server list")
	}
}

func TestAuthorityBrowserPublicRoomsPreservePreferredAndSavedServers(t *testing.T) {
	public := []runtimecfg.AuthorityServerEntry{
		{Label: "GD-DOOM Co-op", Address: "https://m45sci.xyz:6672/netplay"},
		{Label: "GD-DOOM Deathmatch", Address: "wss://m45sci.xyz:6672/deathmatch"},
	}
	custom := runtimecfg.AuthorityServerEntry{Label: "Default server", Address: "wss://friends.example/netplay"}
	saved := runtimecfg.AuthorityServerEntry{Label: "Weekend game", Address: "localhost:6670"}
	for _, preferred := range []string{"", public[0].Address, public[1].Address, " " + custom.Address + " "} {
		t.Run(preferred, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			// Built-in rows and the preferred address may also have been saved by
			// an earlier session. Each address should still appear exactly once.
			entries := []runtimecfg.AuthorityServerEntry{public[1], public[0], saved}
			if strings.TrimSpace(preferred) == custom.Address {
				entries = append(entries, custom)
			}
			if err := saveAuthorityServers(path, entries); err != nil {
				t.Fatal(err)
			}
			opts := runtimecfg.Options{AuthorityJoinDefaults: runtimecfg.AuthorityJoinRequest{Address: preferred, Name: "Marine", Spectator: true}}
			configureAuthorityBrowser(&opts, nil, path)
			wantAddress := strings.TrimSpace(preferred)
			if wantAddress == "" {
				wantAddress = public[0].Address
			}
			if opts.AuthorityJoinDefaults.Address != wantAddress || opts.AuthorityJoinDefaults.Name != "Marine" || !opts.AuthorityJoinDefaults.Spectator {
				t.Fatalf("preferred join settings changed: %+v", opts.AuthorityJoinDefaults)
			}
			want := append(append([]runtimecfg.AuthorityServerEntry(nil), public...), saved)
			if wantAddress == custom.Address {
				want = append([]runtimecfg.AuthorityServerEntry{custom}, want...)
			}
			if !reflect.DeepEqual(opts.AuthorityServers, want) {
				t.Fatalf("server catalog: got %+v want %+v", opts.AuthorityServers, want)
			}
			if got := loadAuthorityServers(path); !reflect.DeepEqual(got, entries) {
				t.Fatal("opening the browser rewrote saved entries")
			}
		})
	}
}
