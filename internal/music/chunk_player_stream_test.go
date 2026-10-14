package music

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"
)

// The fake backend deliberately consumes the entire source queue without
// declaring itself stopped: a real backend can still have those samples queued.
type streamTestAudioPlayer struct {
	src                   *pcmChunkBuffer
	plays, pauses, closes int
	pcm                   []byte
}

func (p *streamTestAudioPlayer) Play()             { p.plays++ }
func (p *streamTestAudioPlayer) Pause()            { p.pauses++ }
func (p *streamTestAudioPlayer) SetVolume(float64) {}
func (p *streamTestAudioPlayer) Close() error      { p.closes++; return nil }

func (p *streamTestAudioPlayer) drain(t *testing.T) int {
	t.Helper()
	n := p.src.BufferedBytes()
	if n == 0 {
		return 0
	}
	chunk := make([]byte, n)
	if _, err := io.ReadFull(p.src, chunk); err != nil {
		t.Fatalf("backend drain: %v", err)
	}
	p.pcm = append(p.pcm, chunk...)
	return n
}

type streamTestRenderer struct {
	now           *time.Time
	renderCost    time.Duration
	frames        int
	calls, resets int
}

func (d *streamTestRenderer) Reset()                { d.frames = 0; d.resets++ }
func (d *streamTestRenderer) ApplyEvent(Event)      {}
func (d *streamTestRenderer) SampleRate() int       { return OutputSampleRate }
func (d *streamTestRenderer) TicRate() int          { return OutputSampleRate }
func (d *streamTestRenderer) SetMUSPanMax(float64)  {}
func (d *streamTestRenderer) SetOutputGain(float64) {}
func (d *streamTestRenderer) SetPreEmphasis(bool)   {}
func (d *streamTestRenderer) RenderMUSS16LE([]byte) ([]byte, error) {
	return nil, errors.New("test renderer requires streaming")
}

func (d *streamTestRenderer) GenerateStereoS16(frames int) []int16 {
	d.calls++
	*d.now = d.now.Add(d.renderCost)
	pcm := make([]int16, frames*2)
	for i := range frames {
		sample := int16((d.frames+i)%30000 + 1)
		pcm[i*2], pcm[i*2+1] = sample, -sample
	}
	d.frames += frames
	return pcm
}

func newStreamTestPlayer(t *testing.T) (*ChunkPlayer, *streamTestAudioPlayer, *streamTestRenderer, *time.Time) {
	t.Helper()
	now := time.Unix(100, 0)
	src := newPCMChunkBuffer()
	backend := &streamTestAudioPlayer{src: src}
	driver := &streamTestRenderer{now: &now}
	cp := &ChunkPlayer{player: backend, src: src, inline: true, done: make(chan struct{}), now: func() time.Time { return now }}
	t.Cleanup(func() { src.Close() })
	return cp, backend, driver, &now
}

func streamTestFactory(driver *streamTestRenderer, frames int, calls *int) StreamFactory {
	return func() (*StreamRenderer, error) {
		*calls++
		// Bound even a broken empty-loop implementation without wall-clock waits.
		if *calls > 3 {
			return nil, errors.New("unexpected repeated renderer creation")
		}
		return NewParsedMUSStreamRenderer(driver, &ParsedMUS{events: []Event{{Type: EventEnd, DeltaTics: uint32(frames)}}})
	}
}

func assertStreamTestPCM(t *testing.T, pcm []byte, frames int) {
	t.Helper()
	if len(pcm) != frames*4 {
		t.Fatalf("PCM bytes=%d want=%d", len(pcm), frames*4)
	}
	for frame := range frames {
		want := int16(frame%30000 + 1)
		left := int16(binary.LittleEndian.Uint16(pcm[frame*4:]))
		right := int16(binary.LittleEndian.Uint16(pcm[frame*4+2:]))
		if left != want || right != -want {
			t.Fatalf("PCM frame %d=(%d,%d) want=(%d,%d): samples were dropped, repeated or changed", frame, left, right, want, -want)
		}
	}
}

func finishStreamTest(t *testing.T, cp *ChunkPlayer, backend *streamTestAudioPlayer, now *time.Time) {
	t.Helper()
	for i := 0; cp.stream != nil && i < 100; i++ {
		backend.drain(t)
		*now = now.Add(20 * time.Millisecond)
		if err := cp.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	if cp.stream != nil {
		t.Fatal("finite stream did not finish")
	}
	if cp.src.blockOnStarve || cp.src.refillPending {
		t.Fatal("completed stream left its tail behind a blocking prefill gate")
	}
	backend.drain(t)
}

func readStreamTestWithoutBlocking(t *testing.T, src *pcmChunkBuffer, dst []byte) int {
	t.Helper()
	type result struct {
		n   int
		err error
	}
	read := make(chan result, 1)
	go func() {
		n, err := src.Read(dst)
		read <- result{n, err}
	}()
	select {
	case got := <-read:
		if got.err != nil {
			t.Fatalf("stream Read error: %v", got.err)
		}
		return got.n
	case <-time.After(time.Second):
		t.Fatal("music Read blocked the shared backend source reader")
		return 0
	}
}

func TestChunkPlayerActiveStreamReadYieldsAndResumesWithoutInsertedSilence(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	const frames = 16384 + 37
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, frames, &creations), false, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	backend.drain(t)
	untouched := bytes.Repeat([]byte{0xA5}, 128*4)
	dst := bytes.Clone(untouched)
	// Oto visits music and SFX sources sequentially. A dry music source must
	// yield that reader without queuing silence ahead of the next real music.
	if n := readStreamTestWithoutBlocking(t, cp.src, dst); n != 0 || !bytes.Equal(dst, untouched) {
		t.Fatalf("dry active music inserted audio: n=%d modified=%t", n, !bytes.Equal(dst, untouched))
	}
	*now = now.Add(20 * time.Millisecond)
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	backend.drain(t)
	assertStreamTestPCM(t, backend.pcm, driver.frames)
	finishStreamTest(t, cp, backend, now)
	assertStreamTestPCM(t, backend.pcm, frames)
	// Completion releases the special dry-read mode after the final tail.
	// Normal starvation then supplies a finite fade/silence to the backend.
	if n := readStreamTestWithoutBlocking(t, cp.src, dst); n != len(dst) {
		t.Fatalf("completed track retained active dry-read mode: n=%d want=%d", n, len(dst))
	}
	if backend.plays != 1 || backend.pauses != 0 || creations != 1 {
		t.Fatal("dry source recovery restarted or paused the track")
	}
}

func TestChunkPlayerStreamBackendDrainDoesNotPauseOrRestart(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	const frames = 16384
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, frames, &creations), false, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	if backend.drain(t) != 4096*4 {
		t.Fatal("startup did not prime the full minimum reserve")
	}
	*now = now.Add(20 * time.Millisecond)
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	if cp.BufferedBytes() == 0 || backend.plays != 1 || backend.pauses != 0 {
		t.Fatalf("source refill disturbed active backend: bytes=%d plays=%d pauses=%d", cp.BufferedBytes(), backend.plays, backend.pauses)
	}
	finishStreamTest(t, cp, backend, now)
	assertStreamTestPCM(t, backend.pcm, frames)
	if creations != 1 || driver.resets != 1 || backend.plays != 1 || backend.pauses != 0 || backend.closes != 0 {
		t.Fatalf("refill reset playback: creations=%d resets=%d plays=%d pauses=%d closes=%d", creations, driver.resets, backend.plays, backend.pauses, backend.closes)
	}
}

func TestChunkPlayerShortOneShotPlaysAndDrainsBelowPrefill(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	const frames = 37
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, frames, &creations), false, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	if cp.stream != nil || backend.plays != 1 || backend.pauses != 0 {
		t.Fatalf("short track did not start and hand off its tail: active=%t plays=%d pauses=%d", cp.stream != nil, backend.plays, backend.pauses)
	}
	finishStreamTest(t, cp, backend, now)
	assertStreamTestPCM(t, backend.pcm, frames)
	*now = now.Add(time.Second)
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	if backend.plays != 1 || backend.pauses != 0 || creations != 1 {
		t.Fatal("completed short track restarted or paused")
	}
}

func TestChunkPlayerLongOneShotLetsFinalPartialChunkDrain(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	const frames = 4096*3 + 37
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, frames, &creations), false, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	finishStreamTest(t, cp, backend, now)
	assertStreamTestPCM(t, backend.pcm, frames)
	if backend.plays != 1 || backend.pauses != 0 || backend.closes != 0 || creations != 1 {
		t.Fatalf("one-shot completion interrupted queued audio: plays=%d pauses=%d closes=%d creations=%d", backend.plays, backend.pauses, backend.closes, creations)
	}
}

func TestChunkPlayerDelayedTickGrowsReserveWithoutChangingPCM(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	const frames = OutputSampleRate * 2
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, frames, &creations), false, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	initialReserve := cp.buffering.reserve()
	backend.drain(t)
	*now = now.Add(200 * time.Millisecond)
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	if cp.buffering.reserve() <= initialReserve || cp.BufferedBytes() <= 4096*4 {
		t.Fatalf("delayed service did not grow reserve: before=%s after=%s queued=%d", initialReserve, cp.buffering.reserve(), cp.BufferedBytes())
	}
	finishStreamTest(t, cp, backend, now)
	assertStreamTestPCM(t, backend.pcm, frames)
	if creations != 1 || driver.resets != 1 || backend.plays != 1 || backend.pauses != 0 || backend.closes != 0 {
		t.Fatal("adaptive reserve growth reset or restarted the stream")
	}
}

func TestChunkPlayerExpensiveChunksBoundEachTickWork(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	driver.renderCost = 3 * time.Millisecond
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, OutputSampleRate*2, &creations), false, 256, 2048, 4096); err != nil {
		t.Fatal(err)
	}
	if backend.drain(t) != 4096*4 {
		t.Fatal("startup budget prevented minimum prefill")
	}
	*now = now.Add(20 * time.Millisecond)
	beforeTime, beforeCalls := *now, driver.calls
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	calls, cost := driver.calls-beforeCalls, now.Sub(beforeTime)
	if calls < 1 || calls > 2 || cost > musicRefillBudget+driver.renderCost {
		t.Fatalf("Tick exceeded its chunk-boundary budget: calls=%d cost=%s", calls, cost)
	}
	if cp.stream == nil || backend.plays != 1 || backend.pauses != 0 {
		t.Fatal("bounded refill interrupted playback")
	}
	backend.drain(t)
	assertStreamTestPCM(t, backend.pcm, 4096+calls*256)
}

func TestChunkPlayerEmptyLoopTerminatesWithoutStartingBackend(t *testing.T) {
	cp, backend, driver, now := newStreamTestPlayer(t)
	creations := 0
	if err := cp.PlayStream(streamTestFactory(driver, 0, &creations), true, 256, 1024, 4096); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	if err := cp.Tick(); err != nil {
		t.Fatal(err)
	}
	if cp.stream != nil || creations != 1 || backend.plays != 0 || backend.pauses != 0 || cp.BufferedBytes() != 0 || cp.src.blockOnStarve {
		t.Fatalf("empty loop failed to terminate cleanly: active=%t creations=%d plays=%d pauses=%d bytes=%d blocking=%t", cp.stream != nil, creations, backend.plays, backend.pauses, cp.BufferedBytes(), cp.src.blockOnStarve)
	}
}
