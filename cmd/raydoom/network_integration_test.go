//go:build raylib && cgo && integration && !js

package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gddoom/internal/doomruntime"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/mapdata"
	"gddoom/internal/netplay"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

func TestNativeNetworkLauncherWithLocalRelay(t *testing.T) {
	if os.Getenv("GD_RAYLIB_NETWORK_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_NETWORK_INTEGRATION=1 after building build/raydoom")
	}
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	opts.WADHash = fmt.Sprintf("%x", sha1.Sum(bytes))
	opts.SkillLevel, opts.NoMonsters, opts.PlayerSlot = 4, false, 1
	srv, err := netplay.ListenServer("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	common := []string{"-config=", "-wad=../../DOOM1.WAD", "-sound=false", "-music=false", "-width=640", "-height=400"}
	t.Run("watch_adopts_host_map_and_late_join_state", func(t *testing.T) {
		m, err := mapdata.LoadMap(wf, "E1M3")
		if err != nil {
			t.Fatal(err)
		}
		b, err := netplay.DialRelayBroadcaster(srv.Addr(), 0, launchcatalog.BroadcastSessionConfig(m.Name, opts))
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		b.SetLowLatency(true)
		hostOpts := opts
		hostOpts.LiveTicSink = b
		c := doomruntime.NewNativeCampaign(doomruntime.NewNativeMeshGame(m, hostOpts), hostOpts, nil)
		if err := c.BroadcastInitialKeyframe(); err != nil {
			t.Fatal(err)
		}
		for range 210 {
			if err := c.Tick(doomruntime.NativeMeshInput{Forward: 1, Turn: 1, Run: true}, false); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		args := append(append([]string(nil), common...), "-watch="+srv.Addr(), fmt.Sprintf("-watch-session=%d", b.SessionID()), "-frames=90", "-fps=35", "-capture=../../build/raylib-captures/native-network-watch.png")
		log, err := os.CreateTemp(t.TempDir(), "watch-*.log")
		if err != nil {
			t.Fatal(err)
		}
		defer log.Close()
		cmd := exec.CommandContext(ctx, "../../build/raydoom", args...)
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		waited := false
		defer func() {
			cancel()
			if !waited {
				<-done
			}
		}()
		deadline := time.Now().Add(5 * time.Second)
		connected := false
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(log.Name())
			if strings.Contains(string(data), "watch: keyframe loaded tic=175") {
				connected = true
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !connected {
			t.Fatal("watch did not load initial keyframe")
		}
		if err := b.SendRuntimeChat(runtimecfg.ChatMessage{Name: "P1", Text: "native relay chat"}); err != nil {
			t.Fatal(err)
		}
		for time.Now().Before(deadline) {
			if err := c.PollNetwork(); err != nil {
				t.Fatal(err)
			}
			if len(c.ChatPatches(640, 336)) > 0 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		err = <-done
		waited = true
		if err != nil {
			out, _ := os.ReadFile(log.Name())
			t.Fatalf("watch launch: %v\n%s", err, out)
		}
		out, _ := os.ReadFile(log.Name())
		if !strings.Contains(string(out), "watch: keyframe loaded tic=175") || !strings.Contains(string(out), "map=E1M3 frames=90 world-tic=210") {
			t.Fatalf("watch did not adopt host map or stopped before stream end:\n%s", out)
		}
		file, err := os.Open("../../build/raylib-captures/native-network-watch.png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		img, err := png.Decode(file)
		if err != nil {
			t.Fatal(err)
		}
		checked, differences := 0, 0
		for _, patch := range c.ChatPatches(640, 336) {
			for y := range patch.Texture.Height {
				for x := range patch.Texture.Width {
					i := (y*patch.Texture.Width + x) * 4
					if patch.Texture.RGBA[i+3] != 255 {
						continue
					}
					r, g, b, _ := img.At(int(patch.X)+x, int(patch.Y)+y).RGBA()
					checked++
					if byte(r>>8) != patch.Texture.RGBA[i] || byte(g>>8) != patch.Texture.RGBA[i+1] || byte(b>>8) != patch.Texture.RGBA[i+2] {
						differences++
					}
				}
			}
		}
		if checked < 100 || differences > checked/100 {
			t.Fatalf("chat glyphs absent or misplaced in native framebuffer: %d differences/%d pixels", differences, checked)
		}
	})
	t.Run("native_broadcast_has_main_wire_profile_and_commands", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log, err := os.CreateTemp(t.TempDir(), "broadcast-*.log")
		if err != nil {
			t.Fatal(err)
		}
		defer log.Close()
		args := append(append([]string(nil), common...), "-broadcast="+srv.Addr(), "-map=E1M3", "-skill=4", "-frames=100", "-fps=35", "-low-latency", "-capture=../../build/raylib-captures/native-network-broadcast.png")
		cmd := exec.CommandContext(ctx, "../../build/raydoom", args...)
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		defer func() { cancel(); <-done }()
		var id uint64
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(log.Name())
			match := regexp.MustCompile(`broadcast: session id (\d+)`).FindSubmatch(data)
			if len(match) == 2 {
				id, _ = strconv.ParseUint(string(match[1]), 10, 64)
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if id == 0 {
			data, _ := os.ReadFile(log.Name())
			t.Fatalf("native did not publish a session:\n%s", data)
		}
		v, err := netplay.DialRelayViewer(srv.Addr(), id, opts.WADHash)
		if err != nil {
			t.Fatal(err)
		}
		defer v.Close()
		if v.Session().MapName != "E1M3" || v.Session().SkillLevel != 4 || v.Session().NoMonsters {
			t.Fatalf("native advertised wrong session: %+v", v.Session())
		}
		var blob []byte
		commands := 0
		for time.Now().Before(deadline) {
			kf, ready, err := v.PollKeyframe()
			if err != nil {
				t.Fatal(err)
			}
			if ready {
				blob = kf.Blob
			}
			_, ready, err = v.PollTic()
			if err != nil {
				t.Fatal(err)
			}
			if ready {
				commands++
			}
			if len(blob) > 0 && commands >= 40 {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if len(blob) == 0 || commands < 40 {
			t.Fatalf("native stream missing snapshot/commands: blob=%d commands=%d", len(blob), commands)
		}
	})
}
