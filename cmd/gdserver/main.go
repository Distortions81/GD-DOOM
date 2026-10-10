// gdserver runs one authoritative Doom match without a display or audio device.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/netgame"
	"gddoom/internal/sessionflow"
	"gddoom/internal/wad"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "gdserver: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("gdserver", flag.ContinueOnError)
	fs.SetOutput(errOut)
	listen := fs.String("listen", "127.0.0.1:6671", "TCP listen address")
	udpListen := fs.String("udp-listen", "", "optional authenticated WebTransport/QUIC UDP listen address; requires TLS certificate; route /netplay")
	webListen := fs.String("web-listen", "", "optional HTTP/WebSocket listen address; game route /netplay")
	var webProxies webProxyFlags
	fs.Var(&webProxies, "web-proxy", "additional exact WebSocket route=loopback HTTP URL; repeatable, e.g. /deathmatch=http://127.0.0.1:6674/netplay")
	webOrigins := fs.String("web-origins", "", "comma-separated permitted browser origins; same host allowed by default")
	tlsCert := fs.String("tls-cert", "", "PEM certificate for TLS TCP, HTTPS/WSS, and WebTransport listeners")
	tlsKey := fs.String("tls-key", "", "PEM key for TLS TCP, HTTPS/WSS, and WebTransport listeners")
	base := fs.String("wad", "DOOM1.WAD", "base WAD")
	addons := fs.String("file", "", "ordered comma-separated add-on WADs")
	mapName := fs.String("map", "E1M1", "starting map")
	mode := fs.String("mode", "coop", "game mode: coop or deathmatch")
	skill := fs.Int("skill", 3, "skill 1..5")
	noMonsters := fs.Bool("no-monsters", false, "disable monsters")
	friendlyFire := fs.Bool("friendly-fire", false, "allow co-op player damage")
	fragLimit := fs.Int("frag-limit", 20, "deathmatch frag limit, 0 disables")
	timeLimit := fs.Uint("time-limit", 0, "deathmatch time limit in seconds, 0 disables")
	players := fs.Int("players", 4, "maximum active players, 1..4")
	rotationFlag := fs.String("rotation", "", "comma-separated deathmatch map rotation; defaults to repeating starting map")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if len(webProxies) != 0 && *webListen == "" {
		return fmt.Errorf("-web-proxy requires -web-listen")
	}
	if *timeLimit > uint(^uint32(0))/netgame.TickRate {
		return fmt.Errorf("time limit too large")
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		return fmt.Errorf("-tls-cert and -tls-key must be provided together")
	}
	if *udpListen != "" && *tlsCert == "" {
		return fmt.Errorf("-udp-listen requires -tls-cert and -tls-key")
	}
	var tlsConfig *tls.Config
	if *tlsCert != "" {
		certificate, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			return err
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	}
	paths := []string{*base}
	for _, path := range strings.Split(*addons, ",") {
		if path = strings.TrimSpace(path); path != "" {
			paths = append(paths, path)
		}
	}
	files := make([]*wad.File, 0, len(paths))
	hashes := make([]string, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := wad.OpenData(path, data)
		if err != nil {
			return err
		}
		files = append(files, file)
		hash := sha256.Sum256(data)
		hashes = append(hashes, hex.EncodeToString(hash[:]))
	}
	manifest := netgame.CompatibilityManifest{Simulation: netgame.SimulationVersion, WADHashes: hashes, Map: strings.ToUpper(*mapName), Mode: *mode, Skill: *skill, NoMonsters: *noMonsters, FriendlyFire: *friendlyFire, RespawnDelayTics: 35, FragLimit: *fragLimit, TimeLimitTics: uint32(*timeLimit) * netgame.TickRate}
	compat, err := manifest.Key()
	if err != nil {
		return err
	}
	file := wad.Merge(files...)
	m, err := mapdata.LoadMap(file, mapdata.MapName(manifest.Map))
	if err != nil {
		return err
	}
	contentKey, err := manifest.ContentKey()
	if err != nil {
		return err
	}
	authority, err := doomruntime.NewAuthority(m, doomruntime.Options{GameMode: *mode, SkillLevel: *skill, NoMonsters: *noMonsters, WADHash: contentKey, SFXVolume: 1})
	if err != nil {
		return err
	}
	if err := authority.SetRules(doomruntime.AuthorityRules{FragLimit: *fragLimit, TimeLimitTics: manifest.TimeLimitTics, RespawnDelayTics: manifest.RespawnDelayTics, FriendlyFire: *friendlyFire}); err != nil {
		return err
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	epoch := binary.LittleEndian.Uint64(random[:])
	if epoch == 0 {
		epoch = 1
	}
	match, err := netgame.NewMatch(authority, authority, netgame.MatchConfig{Epoch: epoch, Compatibility: compat, Manifest: &manifest, ResumeGraceTicks: 30 * netgame.TickRate, PlayerLimit: *players, InputLead: 3, FutureTicks: 35, HoldTicks: 2, DisconnectTicks: netgame.TickRate * 10, SnapshotInterval: 2})
	if err != nil {
		return err
	}
	server, err := netgame.NewServer(match)
	if err != nil {
		return err
	}
	rotation := []mapdata.MapName{m.Name}
	if strings.TrimSpace(*rotationFlag) != "" {
		if *mode != "deathmatch" {
			return fmt.Errorf("-rotation is for deathmatch; co-op follows map exits")
		}
		rotation = nil
		for _, name := range strings.Split(*rotationFlag, ",") {
			name = strings.ToUpper(strings.TrimSpace(name))
			if _, err := mapdata.LoadMap(file, mapdata.MapName(name)); err != nil {
				return fmt.Errorf("rotation map %s: %w", name, err)
			}
			rotation = append(rotation, mapdata.MapName(name))
		}
	}
	rotationIndex := -1
	for i, name := range rotation {
		if name == m.Name {
			rotationIndex = i
			break
		}
	}
	if err := server.SetTransitionHandler(func() (*netgame.MapTransition, error) {
		var nextName mapdata.MapName
		if *mode == "deathmatch" {
			rotationIndex = (rotationIndex + 1) % len(rotation)
			nextName = rotation[rotationIndex]
		} else {
			current := authority.MapName()
			if _, finale := sessionflow.StartFinale(current, authority.SecretExit()); finale || current == "MAP30" {
				return nil, nil
			}
			var err error
			nextName, err = mapdata.NextMapName(file, current, authority.SecretExit())
			if err != nil {
				return nil, err
			}
		}
		if epoch == ^uint64(0) {
			return nil, fmt.Errorf("session epoch exhausted")
		}
		next, err := mapdata.LoadMap(file, nextName)
		if err != nil {
			return nil, err
		}
		nextManifest := manifest
		nextManifest.Map = string(nextName)
		nextKey, err := nextManifest.Key()
		if err != nil {
			return nil, err
		}
		if err := authority.AdvanceMap(next); err != nil {
			return nil, err
		}
		epoch++
		manifest = nextManifest
		fmt.Fprintf(out, "gdserver: map %s, epoch %d\n", nextName, epoch)
		return &netgame.MapTransition{Epoch: epoch, Map: string(nextName), Compatibility: nextKey}, nil
	}); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer ln.Close()
	if tlsConfig != nil {
		ln = tls.NewListener(ln, tlsConfig)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var origins []string
	for _, origin := range strings.Split(*webOrigins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	var udpListener net.PacketConn
	var udpServer *netgame.WebTransportServer
	if *udpListen != "" {
		udpListener, err = net.ListenPacket("udp", *udpListen)
		if err != nil {
			return err
		}
		defer udpListener.Close()
		udpServer, err = server.NewWebTransportServer(tlsConfig, netgame.WebTransportOptions{OriginPatterns: origins})
		if err != nil {
			return err
		}
		defer udpServer.Close()
	}
	var httpServer *http.Server
	var webListener net.Listener
	if *webListen != "" {
		webListener, err = net.Listen("tcp", *webListen)
		if err != nil {
			return err
		}
		defer webListener.Close()
		mux := http.NewServeMux()
		mux.Handle("/netplay", server.WebSocketHandler(netgame.WebSocketOptions{OriginPatterns: origins}))
		for _, proxy := range webProxies {
			mux.Handle(proxy.route, proxy.handler(ctx, errOut))
		}
		httpServer = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
		if tlsConfig != nil {
			webListener = tls.NewListener(webListener, tlsConfig.Clone())
		}
	}
	fmt.Fprintf(out, "gdserver: %s on %s, map %s, %d Hz\ncompatibility: %s\n", *mode, ln.Addr(), manifest.Map, netgame.TickRate, compat)
	if webListener != nil {
		scheme := "ws"
		if tlsConfig != nil {
			scheme = "wss"
		}
		fmt.Fprintf(out, "websocket: %s://%s/netplay\n", scheme, webListener.Addr())
		for _, proxy := range webProxies {
			fmt.Fprintf(out, "websocket proxy: %s://%s%s -> %s\n", scheme, webListener.Addr(), proxy.route, proxy.upstream)
		}
	}
	if udpListener != nil {
		fmt.Fprintf(out, "webtransport: https://%s/netplay\n", udpListener.LocalAddr())
	}
	fmt.Fprintln(out, "reconnect grace: 30 seconds")
	ownerDone := make(chan error, 1)
	serviceDone := make(chan error, 2)
	services := 0
	go func() { ownerDone <- server.Serve(ctx, ln) }()
	if httpServer != nil {
		services++
		go func() { serviceDone <- httpServer.Serve(webListener) }()
	}
	if udpServer != nil {
		services++
		go func() { serviceDone <- udpServer.Serve(udpListener) }()
	}
	ownerFinished := false
	select {
	case err = <-ownerDone:
		ownerFinished = true
	case err = <-serviceDone:
		services--
	}
	cancel()
	if httpServer != nil {
		_ = httpServer.Close()
	}
	// The owner waits for connection writers' bounded terminal drains. Closing
	// QUIC first would reset successfully queued final/query/map-change bytes.
	if !ownerFinished {
		ownerErr := <-ownerDone
		if err == nil {
			err = ownerErr
		}
	}
	if udpServer != nil {
		_ = udpServer.Close()
		_ = udpListener.Close()
	}
	for range services {
		<-serviceDone
	}
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
