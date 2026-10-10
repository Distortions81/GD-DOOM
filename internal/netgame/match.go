package netgame

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"unicode/utf8"

	"gddoom/internal/demo"
)

// World is the simulation boundary. All calls belong to one match owner.
// Step must advance exactly one world tic, regardless of the number of players.
type World interface {
	Tic() uint32
	AddPlayer(id byte) error
	RemovePlayer(id byte)
	Step(commands map[byte]demo.Tic) error
}

// SnapshotSource produces an owned immutable baseline suitable for a client.
// Save-game files or diagnostic summaries are not a substitute for a baseline.
type SnapshotSource interface {
	Snapshot(viewer byte) ([]byte, error)
}

// WorldCompletion reports an ordinary terminal match result, independently of
// simulation errors. The final successful tic is snapshotted before a transport
// closes the session. Worlds without this interface remain open-ended.
type WorldCompletion interface {
	MatchCompletion() (complete bool, reason string)
}

type MatchConfig struct {
	Epoch            uint64
	Compatibility    string
	Manifest         *CompatibilityManifest // optional public discovery metadata
	PlayerLimit      int
	InputLead        uint16
	FutureTicks      uint32
	HoldTicks        uint32
	DisconnectTicks  uint32
	SnapshotInterval uint32
	ResumeGraceTicks uint32 // zero disables resumable transport loss
}

// ConnectionID is never reused within a match. It binds an authenticated
// transport session to a slot, so a stale socket cannot control a replacement.
type ConnectionID uint64

type matchPlayer struct {
	id                  byte
	name                string
	chatRate            chatRateLimit
	spectator           bool
	viewer              byte
	input               *InputBuffer
	lastActivity        uint32
	lastSnapshot        uint32
	ackedSnapshot       uint32
	resumeToken         [32]byte
	previousResumeToken [32]byte // retained until new connection confirms activity
	suspended           bool
	resumeTicks         uint32
}

type Match struct {
	config             MatchConfig
	world              World
	snapshots          SnapshotSource
	players            map[ConnectionID]*matchPlayer
	nextConnection     ConnectionID
	snapshotID         uint32
	chatID             uint64
	err                error
	completed          bool
	completionReason   string
	completionReported bool
}

var (
	ErrUnknownConnection = errors.New("unknown multiplayer connection")
	ErrSessionEpoch      = errors.New("multiplayer session epoch mismatch")
	ErrCompatibility     = errors.New("multiplayer content or simulation mismatch")
	ErrMatchFull         = errors.New("multiplayer match is full")
	ErrMatchCompleted    = errors.New("multiplayer match has completed")
)

// NewMatch leaves scheduling to its owner. The transport must never invoke
// these methods from reader goroutines or execute socket writes in them.
func NewMatch(world World, snapshots SnapshotSource, config MatchConfig) (*Match, error) {
	if world == nil || config.Epoch == 0 || len(config.Compatibility) == 0 || len(config.Compatibility) > 256 || !utf8.ValidString(config.Compatibility) ||
		config.PlayerLimit < 1 || config.PlayerLimit > MaxPlayers || config.InputLead > TickRate ||
		config.FutureTicks > MaxInputWindow || config.FutureTicks < uint32(config.InputLead) ||
		config.HoldTicks > MaxInputWindow || config.DisconnectTicks <= config.HoldTicks ||
		config.SnapshotInterval == 0 {
		return nil, errors.New("invalid authoritative match configuration")
	}
	if config.ResumeGraceTicks > 5*60*TickRate {
		return nil, errors.New("resume grace exceeds five minutes")
	}
	if config.Manifest != nil {
		key, err := config.Manifest.Key()
		if err != nil || key != config.Compatibility {
			return nil, ErrCompatibility
		}
		manifest := cloneCompatibilityManifest(*config.Manifest)
		config.Manifest = &manifest
	}
	return &Match{config: config, world: world, snapshots: snapshots, players: make(map[ConnectionID]*matchPlayer)}, nil
}

func (m *Match) Join(hello Hello) (ConnectionID, Welcome, error) {
	if m.err != nil {
		return 0, Welcome{}, m.err
	}
	if m.pollCompletion() {
		return 0, Welcome{}, ErrMatchCompleted
	}
	if hello.Compatibility != m.config.Compatibility {
		return 0, Welcome{}, ErrCompatibility
	}
	if _, err := validateMessage(hello); err != nil {
		return 0, Welcome{}, err
	}
	if hello.ResumeToken != ([32]byte{}) {
		if hello.Spectator {
			return 0, Welcome{}, ErrResumeUnavailable
		}
		return m.resume(hello.ResumeToken)
	}
	if hello.Spectator {
		return m.joinSpectator(hello)
	}
	if m.PlayerCount() >= m.config.PlayerLimit {
		return 0, Welcome{}, ErrMatchFull
	}
	if m.world.Tic() == math.MaxUint32 || m.nextConnection == ConnectionID(math.MaxUint64) {
		return 0, Welcome{}, ErrInputEpochExhausted
	}
	var occupied [MaxPlayers + 1]bool
	for _, p := range m.players {
		occupied[p.id] = true
	}
	id := byte(1)
	for occupied[id] {
		id++
	}
	buffer, err := NewInputBuffer(InputBufferConfig{FirstTick: m.world.Tic() + 1, FutureTicks: m.config.FutureTicks, HoldTicks: m.config.HoldTicks})
	if err != nil {
		return 0, Welcome{}, err
	}
	token, err := m.newResumeToken()
	if err != nil {
		return 0, Welcome{}, err
	}
	if err := m.world.AddPlayer(id); err != nil {
		return 0, Welcome{}, err
	}
	m.nextConnection++
	handle := m.nextConnection
	p := &matchPlayer{id: id, name: hello.Name, input: buffer, lastActivity: m.world.Tic(), resumeToken: token}
	m.players[handle] = p
	return handle, m.welcome(p), nil
}

func (m *Match) Leave(handle ConnectionID) {
	if p, ok := m.players[handle]; ok {
		delete(m.players, handle)
		if !p.spectator {
			m.world.RemovePlayer(p.id)
		}
	}
}

// Submit accepts redundant input packets without treating an already finalized
// command as a reason to wait or disconnect. Invalid batches are atomic.
func (m *Match) Submit(handle ConnectionID, batch InputBatch) error {
	if m.err != nil {
		return m.err
	}
	p, ok := m.players[handle]
	if !ok || p.suspended {
		return ErrUnknownConnection
	}
	if batch.Epoch != m.config.Epoch {
		return ErrSessionEpoch
	}
	if p.spectator && len(batch.Inputs) != 0 {
		return ErrProtocol
	}
	if _, err := validateMessage(batch); err != nil {
		return err
	}
	if len(batch.Inputs) > MaxInputBatch || batch.SnapshotAck > p.lastSnapshot {
		return ErrProtocol
	}
	// Clone the small bounded buffer so one malformed command cannot partially
	// alter this player's timeline before the caller handles the protocol error.
	next := *p.input
	next.pending = make(map[uint32]Input, len(p.input.pending)+len(batch.Inputs))
	for tick, in := range p.input.pending {
		next.pending[tick] = in
	}
	for _, in := range batch.Inputs {
		err := next.Submit(in)
		if err != nil && !errors.Is(err, ErrDuplicateInput) && !errors.Is(err, ErrStaleInput) {
			return err
		}
	}
	p.input = &next
	p.lastActivity = m.world.Tic()
	p.previousResumeToken = [32]byte{}
	if batch.SnapshotAck > p.ackedSnapshot {
		p.ackedSnapshot = batch.SnapshotAck
	}
	return nil
}

type TickResult struct {
	Tick             uint32
	Snapshots        map[ConnectionID]Snapshot
	Dropped          []ConnectionID
	Completed        bool
	CompletionReason string
}

// Step never waits for inputs. A missing player command is synthesized by its
// InputBuffer. An empty match pauses. Simulation or snapshot errors are fatal:
// retrying a partially completed tic would otherwise grant extra world time.
func (m *Match) Step() (TickResult, error) {
	result := TickResult{Tick: m.world.Tic()}
	if m.err != nil {
		return result, m.err
	}
	if m.completed && m.completionReported {
		result.Completed, result.CompletionReason = true, m.completionReason
		return result, nil
	}
	// A rules change may finish the match between tics. Do not attempt one more
	// Step against a world that has already stopped; still send its final state.
	if m.pollCompletion() {
		result.Completed, result.CompletionReason = true, m.completionReason
		return m.snapshotResult(result)
	}
	if m.PlayerCount() == 0 {
		return result, nil
	}
	if m.world.Tic() == math.MaxUint32 {
		m.err = ErrInputEpochExhausted
		return result, m.err
	}
	tick := m.world.Tic() + 1
	handles := m.orderedPlayers()
	commands := make(map[byte]demo.Tic, len(handles))
	for _, handle := range handles {
		p := m.players[handle]
		if p.spectator {
			continue
		}
		if !p.suspended && uint64(tick)-uint64(p.lastActivity) > uint64(m.config.DisconnectTicks) {
			m.Suspend(handle)
			result.Dropped = append(result.Dropped, handle)
			if m.players[handle] == nil {
				continue
			}
		}
		if p.suspended {
			p.resumeTicks--
			commands[p.id] = demo.Tic{}
			continue
		}
		in, err := p.input.Consume(tick)
		if err != nil {
			m.err = err
			return result, err
		}
		commands[p.id] = in.Command
	}
	if len(commands) == 0 {
		return result, nil
	}
	if err := m.world.Step(commands); err != nil {
		m.err = err
		return result, err
	}
	if m.world.Tic() != tick {
		m.err = fmt.Errorf("simulation advanced to %d, expected %d", m.world.Tic(), tick)
		return result, m.err
	}
	result.Tick = tick
	for _, handle := range handles {
		if p := m.players[handle]; p != nil && p.suspended && p.resumeTicks == 0 {
			m.Leave(handle)
		}
	}
	if m.pollCompletion() {
		result.Completed, result.CompletionReason = true, m.completionReason
	}
	if result.Completed || tick%m.config.SnapshotInterval == 0 {
		return m.snapshotResult(result)
	}
	return result, nil
}

func (m *Match) pollCompletion() bool {
	if !m.completed {
		if world, ok := m.world.(WorldCompletion); ok {
			m.completed, m.completionReason = world.MatchCompletion()
		}
	}
	return m.completed
}

func (m *Match) snapshotResult(result TickResult) (TickResult, error) {
	if m.snapshots != nil {
		if m.snapshotID == math.MaxUint32 {
			m.err = ErrInputEpochExhausted
			return result, m.err
		}
		m.snapshotID++
		result.Snapshots = make(map[ConnectionID]Snapshot, len(m.players))
		states := make(map[byte][]byte, MaxPlayers)
		for _, handle := range m.orderedPlayers() {
			p := m.players[handle]
			if p.suspended {
				continue
			}
			viewer := p.id
			if p.spectator {
				viewer = m.spectatorView(p)
				if viewer == 0 {
					continue
				}
			}
			state := states[viewer]
			if state == nil {
				var err error
				state, err = m.snapshots.Snapshot(viewer)
				if err != nil {
					m.err = err
					return result, err
				}
				states[viewer] = state
			}
			if len(state) == 0 || len(state) > MaxSnapshotBytes {
				m.err = ErrProtocol
				return result, m.err
			}
			result.Snapshots[handle] = Snapshot{Epoch: m.config.Epoch, ID: m.snapshotID, Tick: result.Tick, Finalized: p.input.Ack(), State: state}
			p.lastSnapshot = m.snapshotID
		}
	}
	if result.Completed {
		m.completionReported = true
	}
	return result, nil
}

func (m *Match) orderedPlayers() []ConnectionID {
	out := make([]ConnectionID, 0, len(m.players))
	for handle := range m.players {
		out = append(out, handle)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := m.players[out[i]].id, m.players[out[j]].id
		if a == b {
			return out[i] < out[j]
		}
		return a < b
	})
	return out
}

func (m *Match) PlayerCount() int {
	count := 0
	for _, p := range m.players {
		if !p.spectator {
			count++
		}
	}
	return count
}
func (m *Match) Tic() uint32 { return m.world.Tic() }
