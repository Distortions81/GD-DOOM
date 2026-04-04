package doomruntime

import (
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/runtimecfg"
)

type testLiveTicSink struct {
	tics                []demo.Tic
	intermissionAdvance int
	keyframeTics        []uint32
	keyframeFlags       []byte
	keyframes           [][]byte
}

func (s *testLiveTicSink) BroadcastTic(tc demo.Tic) error {
	s.tics = append(s.tics, tc)
	return nil
}

func (s *testLiveTicSink) BroadcastIntermissionAdvance() error {
	s.intermissionAdvance++
	return nil
}

func (s *testLiveTicSink) BroadcastKeyframe(tic uint32, blob []byte) error {
	return s.BroadcastKeyframeWithFlags(tic, blob, 0)
}

func (s *testLiveTicSink) BroadcastKeyframeWithFlags(tic uint32, blob []byte, flags byte) error {
	s.keyframeTics = append(s.keyframeTics, tic)
	s.keyframeFlags = append(s.keyframeFlags, flags)
	s.keyframes = append(s.keyframes, append([]byte(nil), blob...))
	return nil
}

type testLiveTicSource struct {
	tics                []demo.Tic
	intermissionAdvance int
	keyframes           []runtimecfg.RuntimeKeyframe
}

func (s *testLiveTicSource) PollTic() (demo.Tic, bool, error) {
	if len(s.tics) == 0 {
		return demo.Tic{}, false, nil
	}
	tc := s.tics[0]
	s.tics = s.tics[1:]
	return tc, true, nil
}

func (s *testLiveTicSource) PollIntermissionAdvance() (bool, error) {
	if s.intermissionAdvance <= 0 {
		return false, nil
	}
	s.intermissionAdvance--
	return true, nil
}

func (s *testLiveTicSource) PollRuntimeKeyframe() (runtimecfg.RuntimeKeyframe, bool, error) {
	if len(s.keyframes) == 0 {
		return runtimecfg.RuntimeKeyframe{}, false, nil
	}
	kf := s.keyframes[0]
	s.keyframes = s.keyframes[1:]
	return kf, true, nil
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

func TestUpdateBroadcastModeQuantizesLocalTurnLikeDemo(t *testing.T) {
	g := mustLoadE1M1GameForMapTextureTests(t)
	sink := &testLiveTicSink{}
	g.opts.LiveTicSink = sink
	g.opts.MouseLook = true
	g.input.mouseTurnRawAccum = 129 << 16
	startAngle := g.p.angle

	if err := g.Update(); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got, want := g.p.angle-startAngle, uint32(256<<16); got != want {
		t.Fatalf("angle delta=0x%08x want=0x%08x", got, want)
	}
	if got := len(sink.tics); got != 1 {
		t.Fatalf("broadcast tic count=%d want=1", got)
	}
	if got, want := sink.tics[0].AngleTurn, int16(256); got != want {
		t.Fatalf("broadcast AngleTurn=%d want=%d", got, want)
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
