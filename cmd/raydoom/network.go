//go:build raylib && cgo && !js

package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netplay"
)

func nativeNetworkArgs(args []string) []string {
	out := make([]string, 0, len(args)+2)
	for i, a := range args {
		out = append(out, a)
		if (a == "-broadcast" || a == "-watch") && (i+1 == len(args) || strings.HasPrefix(args[i+1], "-")) {
			out = append(out, "")
		}
	}
	return out
}

func nativeRelayAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:6670"
	}
	if !strings.Contains(addr, ":") {
		return addr + ":6670"
	}
	return addr
}

func nativeNetworkFlags(fs *flag.FlagSet) (broadcast, watch string, err error) {
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if explicit["broadcast"] {
		broadcast = nativeRelayAddress(fs.Lookup("broadcast").Value.String())
	}
	if explicit["watch"] {
		watch = nativeRelayAddress(fs.Lookup("watch").Value.String())
	}
	if broadcast != "" && watch != "" {
		return "", "", fmt.Errorf("-broadcast and -watch are mutually exclusive")
	}
	if watch != "" && fs.Lookup("watch-session").Value.String() == "0" {
		return "", "", fmt.Errorf("-watch requires -watch-session")
	}
	if broadcast != "" || watch != "" {
		for _, name := range []string{"demo", "record-demo", "trace-demo-state"} {
			if strings.TrimSpace(fs.Lookup(name).Value.String()) != "" {
				return "", "", fmt.Errorf("-broadcast and -watch do not support demo playback, recording or tracing")
			}
		}
	}
	return
}

type nativeNetwork struct {
	watcher     *netplay.Viewer
	broadcaster *netplay.RelayBroadcaster
	initial     []byte
}

func (n *nativeNetwork) Close() {
	if n.watcher != nil {
		n.watcher.Close()
	}
	if n.broadcaster != nil {
		n.broadcaster.Close()
	}
}

func (n *nativeNetwork) Connect(broadcast, watch string, id uint64, lowLatency bool, opts *doomruntime.Options, name mapdata.MapName) (mapdata.MapName, error) {
	if watch != "" {
		v, err := netplay.DialRelayViewer(watch, id, opts.WADHash)
		if err != nil {
			return "", err
		}
		n.watcher = v
		launchcatalog.ApplyWatchSession(opts, v.Session())
		opts.LiveTicSource, opts.NetBandwidthMeter = v, v
		opts.WatchStartupBufferTics = 3
		if lowLatency {
			opts.WatchStartupBufferTics = 0
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			kf, ready, err := v.PollKeyframe()
			if err != nil {
				return "", err
			}
			if ready {
				n.initial = kf.Blob
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		return mapdata.MapName(strings.ToUpper(v.Session().MapName)), nil
	}
	if broadcast != "" {
		b, err := netplay.DialRelayBroadcaster(broadcast, 0, launchcatalog.BroadcastSessionConfig(name, *opts))
		if err != nil {
			return "", err
		}
		n.broadcaster = b
		b.SetLowLatency(lowLatency)
		opts.LiveTicSink, opts.NetBandwidthMeter = b, b
		fmt.Printf("broadcast: publishing to relay %s\nbroadcast: session id %d\n", broadcast, b.SessionID())
	}
	return name, nil
}
