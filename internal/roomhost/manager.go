package roomhost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"gddoom/internal/lobby"
)

type Config struct {
	WorkerPath string
	PublicURL  string
	Packs      []ContentPack
	WebOrigins []string
	// WebTransport advertises HTTPS game URLs; the operator must serve UDP at
	// PublicURL and retain WSS at the same routes for automatic fallback.
	WebTransport bool
	// TrustedProxies contains explicit literal loopback addresses of gateways
	// that replace X-Forwarded-For with the actual client IP.
	TrustedProxies  []string
	MaxRooms        int
	IdleTimeout     time.Duration
	StartupTimeout  time.Duration
	PollInterval    time.Duration
	ShutdownTimeout time.Duration
	TempDir         string
	Log             io.Writer
	UploadDir       string
	UploadQuota     int64
	Redistribution  []RedistributionApproval
	DownloadQuota   int64
}

type Manager struct {
	config         Config
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	closed         bool
	rooms          map[string]*managedRoom
	requests       map[string]string
	rates          map[string]creationRate
	packs          map[string]ContentPack
	wg             sync.WaitGroup
	uploadSlot     chan struct{}
	uploadBytes    int64
	uploadLock     *os.File
	downloadMu     sync.Mutex
	downloads      map[string]downloadFile
	approvals      map[string]RedistributionApproval
	downloadDir    string
	downloadBytes  int64
	downloadSlots  chan struct{}
	trustedProxies map[netip.Addr]struct{}
}

type managedRoom struct {
	room        lobby.Room
	request     lobby.CreateRequest
	result      chan struct{}
	proxy       http.Handler
	finished    time.Time
	err         error
	ctx         context.Context
	tcpAddress  string
	connections chan struct{}
}

type creationRate struct {
	tokens  float64
	updated time.Time
}

type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string         { return e.message }
func reject(status int, message string) error { return &requestError{status, message} }

func New(ctx context.Context, config Config) (*Manager, error) {
	trustedProxies, err := parseTrustedProxies(config.TrustedProxies)
	if err != nil {
		return nil, err
	}
	config.TrustedProxies = slices.Clone(config.TrustedProxies)
	if config.MaxRooms == 0 {
		config.MaxRooms = 8
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 10 * time.Minute
	}
	if config.StartupTimeout == 0 {
		config.StartupTimeout = 20 * time.Second
	}
	if config.PollInterval == 0 {
		config.PollInterval = 2 * time.Second
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 5 * time.Second
	}
	if config.MaxRooms < 1 || config.MaxRooms > lobby.MaxRooms || config.IdleTimeout < time.Millisecond || config.StartupTimeout < time.Millisecond || config.PollInterval < time.Millisecond || config.ShutdownTimeout < time.Millisecond {
		return nil, fmt.Errorf("invalid room capacity or lifecycle duration")
	}
	normalized, err := lobby.NormalizeAddress(config.PublicURL)
	if err != nil {
		return nil, err
	}
	public, err := url.Parse(normalized)
	if err != nil || public.Host == "" || public.User != nil || (public.Path != "" && public.Path != "/") || public.RawQuery != "" || public.ForceQuery || public.Fragment != "" || (public.Scheme != "https" && public.Scheme != "http") {
		return nil, fmt.Errorf("public URL must be an HTTP(S) origin without a path, query, or credentials")
	}
	if public.Scheme == "http" {
		ip := net.ParseIP(public.Hostname())
		if public.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("plain HTTP public URL requires localhost or a loopback IP")
		}
	}
	config.PublicURL = public.Scheme + "://" + public.Host
	if config.WebTransport && public.Scheme != "https" {
		return nil, errors.New("WebTransport rooms require an HTTPS public URL")
	}
	worker, err := exec.LookPath(config.WorkerPath)
	if err != nil {
		return nil, fmt.Errorf("worker executable: %w", err)
	}
	config.WorkerPath = worker
	if config.Log == nil {
		config.Log = io.Discard
	}
	config.Log = &lockedWriter{writer: config.Log}
	config.WebOrigins = append(slices.Clone(config.WebOrigins), config.PublicURL)
	if len(config.Packs) == 0 || len(config.Packs) > lobby.MaxPacks {
		return nil, fmt.Errorf("invalid content pack count")
	}
	packs := make(map[string]ContentPack)
	config.Packs = slices.Clone(config.Packs)
	for i, pack := range config.Packs {
		if err := lobby.ValidatePack(pack.Pack); err != nil {
			return nil, err
		}
		if _, exists := packs[pack.Pack.ID]; exists || len(pack.Paths) != len(pack.Pack.WADHashes) {
			return nil, fmt.Errorf("duplicate or incomplete content pack %q", pack.Pack.ID)
		}
		for _, path := range pack.Paths {
			// gdserver's ordered -file flag is comma-separated.
			if path == "" || strings.Contains(path, ",") {
				return nil, fmt.Errorf("WAD path is empty or contains a comma")
			}
		}
		pack.Paths = slices.Clone(pack.Paths)
		pack.Pack = clonePack(pack.Pack)
		packs[pack.Pack.ID], config.Packs[i] = pack, pack
	}
	lifetime, cancel := context.WithCancel(ctx)
	m := &Manager{config: config, ctx: lifetime, cancel: cancel, rooms: make(map[string]*managedRoom), requests: make(map[string]string), rates: make(map[string]creationRate), packs: packs, uploadSlot: make(chan struct{}, 1), trustedProxies: trustedProxies}
	if err := m.initContent(); err != nil {
		cancel()
		if m.uploadLock != nil {
			_ = m.uploadLock.Close()
		}
		_ = os.RemoveAll(m.downloadDir)
		return nil, err
	}
	return m, nil
}

func clonePack(p lobby.Pack) lobby.Pack {
	p.WADHashes = slices.Clone(p.WADHashes)
	p.Maps = slices.Clone(p.Maps)
	p.Files = slices.Clone(p.Files)
	return p
}
func cloneRoom(r lobby.Room) lobby.Room {
	r.Manifest.WADHashes = slices.Clone(r.Manifest.WADHashes)
	return r
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
	m.mu.Lock()
	if m.uploadLock != nil {
		_ = m.uploadLock.Close()
		m.uploadLock = nil
	}
	if m.downloadDir != "" {
		_ = os.RemoveAll(m.downloadDir)
		m.downloadDir = ""
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) State() lobby.State {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := lobby.State{Version: lobby.APIVersion, MaxRooms: m.config.MaxRooms, UploadsEnabled: m.config.UploadDir != "", Packs: make([]lobby.Pack, 0, len(m.config.Packs)), Rooms: []lobby.Room{}}
	for _, p := range m.config.Packs {
		state.Packs = append(state.Packs, clonePack(p.Pack))
	}
	for _, r := range m.rooms {
		state.Rooms = append(state.Rooms, cloneRoom(r.room))
	}
	sort.Slice(state.Rooms, func(i, j int) bool {
		a, b := state.Rooms[i], state.Rooms[j]
		activeA, activeB := activeRoom(a.State), activeRoom(b.State)
		if activeA != activeB {
			return activeA
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	if len(state.Rooms) > lobby.MaxRooms {
		state.Rooms = state.Rooms[:lobby.MaxRooms]
	}
	return state
}

func activeRoom(state string) bool {
	return state == "starting" || state == "ready" || state == "stopping"
}

// Create owns startup independently of the HTTP request. Retrying the same ID
// returns the same room; canceled requests cannot orphan an unmanaged child.
func (m *Manager) Create(ctx context.Context, remote string, request lobby.CreateRequest) (lobby.Room, error) {
	if err := ctx.Err(); err != nil {
		return lobby.Room{}, err
	}
	if err := lobby.ValidateCreateRequest(request); err != nil {
		return lobby.Room{}, reject(http.StatusBadRequest, err.Error())
	}
	m.mu.Lock()
	pack, exists := m.packs[request.Settings.PackID]
	m.mu.Unlock()
	if !exists {
		return lobby.Room{}, reject(http.StatusBadRequest, "unknown content pack")
	}
	manifest, err := lobby.ValidateSettings(request.Settings, pack.Pack)
	if err != nil {
		return lobby.Room{}, reject(http.StatusBadRequest, err.Error())
	}
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil {
		m.mu.Unlock()
		return lobby.Room{}, reject(http.StatusServiceUnavailable, "lobby is stopping")
	}
	now := time.Now()
	m.pruneLocked(now)
	if id, found := m.requests[request.RequestID]; found {
		r := m.rooms[id]
		if r.request != request {
			m.mu.Unlock()
			return lobby.Room{}, reject(http.StatusConflict, "request ID was already used with different settings")
		}
		m.mu.Unlock()
		return m.awaitRoom(ctx, r)
	}
	active := 0
	for _, r := range m.rooms {
		if activeRoom(r.room.State) {
			active++
		}
	}
	if active >= m.config.MaxRooms || len(m.rooms) >= 256 {
		m.mu.Unlock()
		return lobby.Room{}, reject(http.StatusServiceUnavailable, "all room slots are in use")
	}
	if !m.allowCreateLocked(remote, now) {
		m.mu.Unlock()
		return lobby.Room{}, reject(http.StatusTooManyRequests, "please wait before creating another room")
	}
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		m.mu.Unlock()
		return lobby.Room{}, err
	}
	id := hex.EncodeToString(idBytes[:])
	r := &managedRoom{request: request, result: make(chan struct{}), connections: make(chan struct{}, 32), room: lobby.Room{ID: id, Name: request.Name, State: "starting", Settings: request.Settings, Manifest: manifest, PlayerLimit: request.Settings.PlayerLimit, CreatedAt: now.UTC()}}
	m.rooms[id], m.requests[request.RequestID] = r, id
	m.wg.Add(1)
	go m.runWorker(r, pack)
	m.mu.Unlock()
	return m.awaitRoom(ctx, r)
}

func (m *Manager) awaitRoom(ctx context.Context, r *managedRoom) (lobby.Room, error) {
	select {
	case <-ctx.Done():
		return lobby.Room{}, ctx.Err()
	case <-m.ctx.Done():
		return lobby.Room{}, reject(http.StatusServiceUnavailable, "lobby is stopping")
	case <-r.result:
		m.mu.Lock()
		defer m.mu.Unlock()
		if r.err != nil {
			return cloneRoom(r.room), r.err
		}
		if r.room.State != "ready" {
			return cloneRoom(r.room), reject(http.StatusGone, "room has ended")
		}
		return cloneRoom(r.room), nil
	}
}

func (m *Manager) pruneLocked(now time.Time) {
	for id, r := range m.rooms {
		if !r.finished.IsZero() && now.Sub(r.finished) > 10*time.Minute {
			delete(m.requests, r.request.RequestID)
			delete(m.rooms, id)
		}
	}
	for remote, rate := range m.rates {
		if now.Sub(rate.updated) > 10*time.Minute {
			delete(m.rates, remote)
		}
	}
}

func (m *Manager) allowCreateLocked(remote string, now time.Time) bool {
	rate, found := m.rates[remote]
	if !found {
		if len(m.rates) >= 4096 {
			return false
		}
		rate = creationRate{tokens: 2, updated: now}
	}
	rate.tokens += now.Sub(rate.updated).Seconds() / 30
	if rate.tokens > 2 {
		rate.tokens = 2
	}
	rate.updated = now
	allowed := rate.tokens >= 1
	if allowed {
		rate.tokens--
	}
	m.rates[remote] = rate
	return allowed
}

func (m *Manager) logRoom(r *managedRoom, err error, output []byte) {
	if err != nil {
		fmt.Fprintf(m.config.Log, "room %s: %v\n%s", r.room.ID, err, output)
	}
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(data)
}

// snapshotReadyFile accepts only a private worker's bounded structured output.
func snapshotReadyFile(filename string) (WorkerReady, error) {
	f, err := os.Open(filename)
	if err != nil {
		return WorkerReady{}, err
	}
	defer f.Close()
	var ready WorkerReady
	dec := json.NewDecoder(io.LimitReader(f, 16385))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ready); err != nil {
		return ready, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ready, errors.New("invalid worker readiness record")
	}
	return ready, nil
}
