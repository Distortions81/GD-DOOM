package doomsession

import (
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/runtimehost"
	"gddoom/internal/session"

	"github.com/hajimehoshi/ebiten/v2"
)

type stubRuntime struct {
	update func() error
}

type stubHostRuntime struct {
	stubRuntime
	samples, pumps int
}

func (s *stubHostRuntime) SampleInput() { s.samples++ }
func (s *stubHostRuntime) UpdateHostFrame() error {
	s.pumps++
	return nil
}

func TestSessionForwardsHostFramePumpAtFixedSessionCadence(t *testing.T) {
	updates := 0
	runtime := &stubHostRuntime{stubRuntime: stubRuntime{update: func() error { updates++; return nil }}}
	host := session.New(&Session{game: runtime})
	for range 140 {
		if err := host.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.samples != 140 || runtime.pumps != 105 || updates != 35 {
		t.Fatalf("forwarded samples=%d pumps=%d updates=%d; want140/105/35", runtime.samples, runtime.pumps, updates)
	}
}

func (s *stubRuntime) Update() error {
	if s.update == nil {
		return nil
	}
	return s.update()
}
func (s *stubRuntime) Draw(_ *ebiten.Image) {}
func (s *stubRuntime) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type stubKeyframeSink struct {
	tics  []uint32
	blobs [][]byte
}

func (s *stubKeyframeSink) BroadcastTic(demo.Tic) error { return nil }

func (s *stubKeyframeSink) BroadcastKeyframe(tic uint32, blob []byte) error {
	s.tics = append(s.tics, tic)
	s.blobs = append(s.blobs, append([]byte(nil), blob...))
	return nil
}

func TestSessionUpdateBroadcastsPeriodicKeyframeAtCadence(t *testing.T) {
	sink := &stubKeyframeSink{}
	worldTic := periodicKeyframeIntervalTics
	sess := &Session{
		game: &stubRuntime{},
		meta: runtimehost.Meta{
			Options: func() runtimecfg.Options {
				return runtimecfg.Options{LiveTicSink: sink}
			},
			CurrentWorldTic: func() int { return worldTic },
			CaptureKeyframe: func() ([]byte, error) { return []byte{1, 2, 3}, nil },
		},
	}

	if err := sess.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(sink.tics) != 1 || sink.tics[0] != uint32(worldTic) {
		t.Fatalf("broadcast tics=%v want [%d]", sink.tics, worldTic)
	}
}

func TestSessionUpdateDoesNotRepeatPeriodicKeyframeForSameTic(t *testing.T) {
	sink := &stubKeyframeSink{}
	worldTic := periodicKeyframeIntervalTics
	sess := &Session{
		game: &stubRuntime{},
		meta: runtimehost.Meta{
			Options: func() runtimecfg.Options {
				return runtimecfg.Options{LiveTicSink: sink}
			},
			CurrentWorldTic: func() int { return worldTic },
			CaptureKeyframe: func() ([]byte, error) { return []byte{1}, nil },
		},
	}

	if err := sess.Update(); err != nil {
		t.Fatalf("first Update() error = %v", err)
	}
	if err := sess.Update(); err != nil {
		t.Fatalf("second Update() error = %v", err)
	}
	if len(sink.tics) != 1 {
		t.Fatalf("broadcast count=%d want=1", len(sink.tics))
	}
}

func TestSessionUpdateSkipsPeriodicKeyframeOffCadence(t *testing.T) {
	sink := &stubKeyframeSink{}
	worldTic := periodicKeyframeIntervalTics - 1
	sess := &Session{
		game: &stubRuntime{},
		meta: runtimehost.Meta{
			Options: func() runtimecfg.Options {
				return runtimecfg.Options{LiveTicSink: sink}
			},
			CurrentWorldTic: func() int { return worldTic },
			CaptureKeyframe: func() ([]byte, error) { return []byte{1}, nil },
		},
	}

	if err := sess.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(sink.tics) != 0 {
		t.Fatalf("broadcast count=%d want=0", len(sink.tics))
	}
}
