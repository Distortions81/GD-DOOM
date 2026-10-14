package netgame

import (
	"errors"
	"unicode/utf8"
)

// ResetEpoch binds a freshly initialized level to the existing authenticated
// connections. The owner first observes completion, then advances the World,
// then calls ResetEpoch before any new input or simulation tick is processed.
// Connection handles and player slots survive; pending commands, input sequence
// history, snapshot IDs/acks, timeouts, and completion state do not.
// A published manifest requires nextMap so its new compatibility key can be
// validated before any mutation. Suspended players retain their remaining grace.
func (m *Match) ResetEpoch(epoch uint64, compatibility string, nextMap ...string) (map[ConnectionID]Welcome, error) {
	if m.err != nil {
		return nil, m.err
	}
	if !m.completed || epoch <= m.config.Epoch || m.world.Tic() != 0 || len(compatibility) == 0 || len(compatibility) > 256 || !utf8.ValidString(compatibility) {
		return nil, errors.New("invalid authoritative level epoch transition")
	}
	if len(nextMap) > 1 || (len(nextMap) == 1 && !validManifestMapName(nextMap[0])) {
		return nil, ErrCompatibility
	}
	var manifest *CompatibilityManifest
	if m.config.Manifest != nil {
		if len(nextMap) != 1 {
			return nil, ErrCompatibility
		}
		next := cloneCompatibilityManifest(*m.config.Manifest)
		next.Map = nextMap[0]
		key, err := next.Key()
		if err != nil || key != compatibility {
			return nil, ErrCompatibility
		}
		manifest = &next
	}
	if world, ok := m.world.(WorldCompletion); ok {
		if ended, _ := world.MatchCompletion(); ended {
			return nil, errors.New("new authoritative level is already complete")
		}
	}
	buffers := make(map[ConnectionID]*InputBuffer, len(m.players))
	welcomes := make(map[ConnectionID]Welcome, len(m.players))
	for handle, p := range m.players {
		buffer, err := NewInputBuffer(InputBufferConfig{FirstTick: 1, FutureTicks: m.config.FutureTicks, HoldTicks: m.config.HoldTicks})
		if err != nil {
			return nil, err
		}
		buffers[handle] = buffer
		welcome := m.welcome(p)
		welcome.Epoch = epoch
		welcomes[handle] = welcome
	}
	m.config.Epoch, m.config.Compatibility = epoch, compatibility
	m.config.Manifest = manifest
	m.snapshotID = 0
	m.completed, m.completionReported, m.completionReason = false, false, ""
	for handle, p := range m.players {
		p.input = buffers[handle]
		p.lastActivity, p.lastSnapshot, p.ackedSnapshot = 0, 0, 0
	}
	return welcomes, nil
}

func (m *Match) Epoch() uint64 { return m.config.Epoch }
