package doomruntime

import (
	"testing"

	"gddoom/internal/demo"
)

type testLiveTicSink struct {
	tics []demo.Tic
}

func (s *testLiveTicSink) BroadcastTic(tc demo.Tic) error {
	s.tics = append(s.tics, tc)
	return nil
}

type testLiveTicSource struct {
	tics []demo.Tic
}

func (s *testLiveTicSource) PollTic() (demo.Tic, bool, error) {
	if len(s.tics) == 0 {
		return demo.Tic{}, false, nil
	}
	tc := s.tics[0]
	s.tics = s.tics[1:]
	return tc, true, nil
}

func TestUpdateBroadcastModeAdvancesWorldAndEmitsTic(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	sink := &testLiveTicSink{}
	g.opts.LiveTicSink = sink

	if err := g.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if g.worldTic != 1 {
		t.Fatalf("worldTic=%d want=1", g.worldTic)
	}
	if got := len(sink.tics); got != 1 {
		t.Fatalf("broadcast tic count=%d want=1", got)
	}
}

func TestUpdateWatchModeConsumesLiveTicAndAdvancesWorld(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	g.opts.LiveTicSource = &testLiveTicSource{
		tics: []demo.Tic{{Forward: 25}},
	}

	if err := g.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if g.worldTic != 1 {
		t.Fatalf("worldTic=%d want=1", g.worldTic)
	}
}
