package sessionmusic

import (
	"bytes"
	"encoding/binary"
	"testing"

	"gddoom/internal/music"
)

func TestEffectiveSynthGainUsesImpSynthRatio(t *testing.T) {
	want := 3.5 * impSynthGainRatio
	if got := effectiveSynthGain(music.BackendImpSynth, 3.5); got != want {
		t.Fatalf("effectiveSynthGain(impsynth)=%.2f want %.2f", got, want)
	}
}

func TestEffectiveSynthGainUsesDefaultForAuto(t *testing.T) {
	want := 2.25
	if music.DefaultBackend() == music.BackendImpSynth {
		want *= impSynthGainRatio
	}
	if got := effectiveSynthGain(music.BackendAuto, 2.25); got != want {
		t.Fatalf("effectiveSynthGain(auto)=%.2f want %.2f", got, want)
	}
}

func TestNextLoopChunkRestartsAfterDone(t *testing.T) {
	driver, err := music.NewDriverWithBackend(44100, nil, music.BackendAuto)
	if err != nil {
		t.Fatalf("NewDriverWithBackend() error: %v", err)
	}
	score := []byte{
		0x40, 0, 0,
		0x10, 60,
		0x80, 60,
		0x08,
		0x60,
	}
	musData := buildMUSTestLump(score)
	factory := func() (*music.StreamRenderer, error) {
		return music.NewMUSStreamRenderer(driver, musData)
	}

	var stream *music.StreamRenderer
	first, err := nextChunk(factory, &stream, true)
	if err != nil {
		t.Fatalf("first nextLoopChunk() error: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected initial chunk")
	}

	doneSeen := false
	for i := 0; i < 64; i++ {
		_, err := nextChunk(factory, &stream, true)
		if err != nil {
			t.Fatalf("nextLoopChunk() error: %v", err)
		}
		if stream == nil {
			doneSeen = true
			break
		}
	}
	if !doneSeen {
		t.Fatal("stream never reached done state")
	}

	looped, err := nextChunk(factory, &stream, true)
	if err != nil {
		t.Fatalf("loop restart nextLoopChunk() error: %v", err)
	}
	if len(looped) == 0 {
		t.Fatal("expected chunk after loop restart")
	}
	if stream == nil {
		t.Fatal("stream should be active again after loop restart")
	}
}

func TestNextChunkStopsAtDoneWhenNotLooping(t *testing.T) {
	driver, err := music.NewDriverWithBackend(44100, nil, music.BackendAuto)
	if err != nil {
		t.Fatalf("NewDriverWithBackend() error: %v", err)
	}
	score := []byte{
		0x40, 0, 0,
		0x10, 60,
		0x80, 60,
		0x08,
		0x60,
	}
	musData := buildMUSTestLump(score)
	factory := func() (*music.StreamRenderer, error) {
		return music.NewMUSStreamRenderer(driver, musData)
	}

	var stream *music.StreamRenderer
	doneSeen := false
	for i := 0; i < 64; i++ {
		chunk, err := nextChunk(factory, &stream, false)
		if err != nil {
			t.Fatalf("nextChunk() error: %v", err)
		}
		if len(chunk) == 0 {
			t.Fatal("expected non-empty chunk before stop")
		}
		if stream == nil {
			doneSeen = true
			break
		}
	}
	if !doneSeen {
		t.Fatal("non-looping stream never reached done state")
	}
	chunk, err := nextChunk(factory, &stream, false)
	if err != nil {
		t.Fatalf("post-done nextChunk() error: %v", err)
	}
	if len(chunk) == 0 {
		t.Fatal("expected first chunk when explicitly started again")
	}
}

func TestNextChunkFramesLoopsAcrossExactChunkBoundary(t *testing.T) {
	driver, err := music.NewDriverWithBackend(44100, nil, music.BackendAuto)
	if err != nil {
		t.Fatalf("NewDriverWithBackend() error: %v", err)
	}
	score := []byte{
		0x90, 60, 0x04,
		0x80, 60, 0x00,
		0x60,
	}
	musData := buildMUSTestLump(score)
	factory := func() (*music.StreamRenderer, error) {
		return music.NewMUSStreamRenderer(driver, musData)
	}

	var stream *music.StreamRenderer
	first, err := nextChunkFrames(factory, &stream, true, 1260)
	if err != nil {
		t.Fatalf("first nextChunkFrames() error: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected initial chunk")
	}

	looped, err := nextChunkFrames(factory, &stream, true, 1260)
	if err != nil {
		t.Fatalf("loop boundary nextChunkFrames() error: %v", err)
	}
	if len(looped) == 0 {
		t.Fatal("expected chunk after exact-boundary loop restart")
	}
	if stream == nil {
		t.Fatal("stream should remain active after exact-boundary loop restart")
	}
}

func buildMUSTestLump(score []byte) []byte {
	var b bytes.Buffer
	b.WriteString("MUS\x1a")
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(score)))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	b.Write(score)
	return b.Bytes()
}
