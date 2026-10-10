package roomhost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gddoom/internal/netgame"
)

// Keep only the tail of a worker's diagnostics; stdout cannot grow with room
// lifetime or block the child on an undrained pipe.
type workerLog struct {
	mu   sync.Mutex
	data []byte
}

func (b *workerLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	const limit = 32 << 10
	n := len(p)
	if len(p) >= limit {
		b.data = append(b.data[:0], p[len(p)-limit:]...)
		return n, nil
	}
	if remove := len(b.data) + len(p) - limit; remove > 0 {
		copy(b.data, b.data[remove:])
		b.data = b.data[:len(b.data)-remove]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func (b *workerLog) bytes() []byte { b.mu.Lock(); defer b.mu.Unlock(); return slices.Clone(b.data) }

func (m *Manager) workerArgs(r *managedRoom, pack ContentPack, readyFile string) []string {
	s := r.request.Settings
	args := []string{"-listen", "127.0.0.1:0", "-web-listen", "127.0.0.1:0", "-ready-file", readyFile,
		"-wad", pack.Paths[0], "-map", s.Map, "-mode", s.Mode, "-skill", strconv.Itoa(s.Skill),
		"-players", strconv.Itoa(s.PlayerLimit), "-frag-limit", strconv.Itoa(s.FragLimit), "-time-limit", strconv.Itoa(s.TimeLimitSeconds),
		"-web-origins", strings.Join(m.config.WebOrigins, ",")}
	if len(pack.Paths) > 1 {
		args = append(args, "-file", strings.Join(pack.Paths[1:], ","))
	}
	for _, flag := range []struct {
		name    string
		enabled bool
	}{
		{"-no-monsters", s.NoMonsters}, {"-fast-monsters", s.FastMonsters}, {"-respawn-monsters", s.RespawnMonsters}, {"-friendly-fire", s.FriendlyFire},
	} {
		if flag.enabled {
			args = append(args, flag.name)
		}
	}
	return args
}

func (m *Manager) runWorker(r *managedRoom, pack ContentPack) {
	defer m.wg.Done()
	ctx, cancel := context.WithCancel(m.ctx)
	var cmd *exec.Cmd
	var exited chan error
	var logs workerLog
	announced, reaped := false, false
	finalState := "failed"
	var finalErr error
	defer func() {
		cancel() // also closes any upgraded sockets through this room's proxy
		if cmd != nil && cmd.Process != nil && !reaped {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			timer := time.NewTimer(m.config.ShutdownTimeout)
			select {
			case <-exited:
			case <-timer.C:
				_ = cmd.Process.Kill()
				<-exited
			}
			timer.Stop()
		}
		m.logRoom(r, finalErr, logs.bytes())
		m.mu.Lock()
		r.room.State, r.room.Players, r.room.Spectators, r.room.ReservedPlayers = finalState, 0, 0, 0
		r.proxy, r.finished = nil, time.Now()
		if finalErr != nil {
			r.err = reject(502, "game server could not start or stopped unexpectedly")
		}
		if !announced {
			close(r.result)
		}
		m.mu.Unlock()
	}()
	directory, err := os.MkdirTemp(m.config.TempDir, "gddoom-room-")
	if err != nil {
		finalErr = err
		return
	}
	defer os.RemoveAll(directory)
	readyFile := filepath.Join(directory, "ready.json")
	cmd = exec.Command(m.config.WorkerPath, m.workerArgs(r, pack, readyFile)...)
	configureWorkerProcess(cmd)
	cmd.Stdout, cmd.Stderr = &logs, &logs
	if err := cmd.Start(); err != nil {
		finalErr = err
		return
	}
	exited = make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	startup, stopStartup := context.WithTimeout(ctx, m.config.StartupTimeout)
	defer stopStartup()
	readyTick := time.NewTicker(25 * time.Millisecond)
	defer readyTick.Stop()
	var ready WorkerReady
	var upstream *url.URL
	for {
		ready, err = snapshotReadyFile(readyFile)
		if err == nil {
			upstream, err = validateWorkerReady(ready, r.room.Manifest)
			if err != nil {
				finalErr = err
				return
			}
			status, probeErr := probeWorker(startup, ready.TCPAddress)
			if probeErr == nil {
				if err = validateWorkerStatus(status, r.room.Manifest, pack, true, r.room.PlayerLimit); err != nil {
					finalErr = err
					return
				}
				break
			}
		}
		select {
		case err := <-exited:
			reaped = true
			finalErr = fmt.Errorf("worker exited before readiness: %v", err)
			return
		case <-startup.Done():
			if ctx.Err() != nil {
				finalState = "ended"
			} else {
				finalErr = fmt.Errorf("worker readiness deadline exceeded")
			}
			return
		case <-readyTick.C:
		}
	}
	stopStartup()
	readyTick.Stop()
	m.mu.Lock()
	r.room.State = "ready"
	r.room.Address = "ws" + strings.TrimPrefix(m.config.PublicURL, "http") + "/rooms/" + r.room.ID + "/netplay"
	r.proxy = roomProxy(ctx, upstream, m.config.Log)
	close(r.result)
	announced = true
	m.mu.Unlock()
	poll := time.NewTicker(m.config.PollInterval)
	defer poll.Stop()
	idleSince := time.Now()
	failedProbes := 0
	for {
		select {
		case <-ctx.Done():
			m.setRoomStopping(r)
			finalState = "ended"
			return
		case err := <-exited:
			reaped = true
			if err == nil {
				finalState = "ended"
			} else {
				finalErr = err
			}
			return
		case <-poll.C:
			status, err := probeWorker(ctx, ready.TCPAddress)
			if err != nil {
				failedProbes++
				if failedProbes >= 3 {
					finalErr = fmt.Errorf("worker health probe: %w", err)
					m.setRoomStopping(r)
					return
				}
				continue
			}
			failedProbes = 0
			if err := validateWorkerStatus(status, ready.Manifest, pack, false, r.request.Settings.PlayerLimit); err != nil {
				finalErr = err
				m.setRoomStopping(r)
				return
			}
			m.mu.Lock()
			r.room.Manifest = status.Manifest
			r.room.Players, r.room.Spectators, r.room.ReservedPlayers = status.Players, status.Spectators, status.ReservedPlayers
			m.mu.Unlock()
			if status.Players+status.Spectators > 0 {
				idleSince = time.Now()
			}
			if time.Since(idleSince) >= m.config.IdleTimeout {
				m.setRoomStopping(r)
				finalState = "ended"
				return
			}
		}
	}
}

func (m *Manager) setRoomStopping(r *managedRoom) {
	m.mu.Lock()
	r.room.State = "stopping"
	r.proxy = nil
	m.mu.Unlock()
}

func validateWorkerReady(ready WorkerReady, expected netgame.CompatibilityManifest) (*url.URL, error) {
	want, err := expected.Key()
	if err != nil {
		return nil, err
	}
	got, err := ready.Manifest.Key()
	if err != nil || got != want || ready.Version != 1 || !loopbackAddress(ready.TCPAddress) {
		return nil, errors.New("worker readiness identity or TCP endpoint mismatch")
	}
	u, err := url.Parse(ready.WebURL)
	if err != nil || u.Scheme != "http" || !loopbackAddress(u.Host) || u.User != nil || u.Opaque != "" || u.Path != "/netplay" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("worker readiness requires an exact HTTP loopback /netplay endpoint")
	}
	return u, nil
}

func loopbackAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	return ip != nil && ip.IsLoopback() && err == nil && n > 0 && n <= 65535
}

func probeWorker(ctx context.Context, address string) (netgame.ServerStatus, error) {
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	transport, err := netgame.OpenTCP(probe, address)
	if err != nil {
		return netgame.ServerStatus{}, err
	}
	return netgame.QueryServerStatus(probe, transport)
}

func validateWorkerStatus(status netgame.ServerStatus, expected netgame.CompatibilityManifest, pack ContentPack, initial bool, playerLimit int) error {
	if !initial {
		if !slices.Contains(pack.Pack.Maps, status.Manifest.Map) {
			return errors.New("worker changed to a map outside the content catalog")
		}
		expected.Map = status.Manifest.Map
	}
	want, wantErr := expected.Key()
	got, gotErr := status.Manifest.Key()
	if wantErr != nil || gotErr != nil || want != got || status.PlayerLimit != playerLimit {
		return errors.New("worker status does not match requested content and rules")
	}
	return nil
}
