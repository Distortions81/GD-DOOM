package sessionvoice

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"gddoom/internal/audiofx"
	"gddoom/internal/audioinput"
	"gddoom/internal/netplay"
	"gddoom/internal/voicecodec"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

type VoiceStreamer struct {
	cancel context.CancelFunc
	done   chan error
}

func StartPulseBroadcaster(parent context.Context, broadcaster *netplay.AudioBroadcaster, device string, worldTic func() uint32) (*VoiceStreamer, error) {
	if broadcaster == nil {
		return nil, fmt.Errorf("audio broadcaster is required")
	}
	ctx, cancel := context.WithCancel(parent)
	enc, err := voicecodec.NewOpusEncoder()
	if err != nil {
		cancel()
		return nil, err
	}
	cfg := audioinput.PulseConfig{
		Device:        device,
		SampleRate:    voicecodec.SampleRate,
		Channels:      voicecodec.Channels,
		Format:        "s16le",
		LatencyMillis: voicecodec.FrameDurationMillis,
	}
	reader, err := audioinput.OpenPulseReader(ctx, cfg)
	if err != nil {
		cancel()
		_ = enc.Close()
		return nil, err
	}
	if err := broadcaster.BroadcastAudioConfig(netplay.AudioConfig{
		Codec:        netplayAudioCodecOpus(),
		SampleRate:   voicecodec.SampleRate,
		Channels:     voicecodec.Channels,
		FrameSamples: voicecodec.FrameSamples,
		Bitrate:      voicecodec.DefaultBitrate,
	}); err != nil {
		cancel()
		_ = reader.Close()
		_ = enc.Close()
		return nil, err
	}
	vs := &VoiceStreamer{
		cancel: cancel,
		done:   make(chan error, 1),
	}
	go func() {
		defer close(vs.done)
		defer reader.Close()
		defer enc.Close()
		frameBytes := voicecodec.FrameSamples * voicecodec.Channels * 2
		raw := make([]byte, frameBytes)
		pcm := make([]int16, voicecodec.FrameSamples*voicecodec.Channels)
		var startSample uint64
		for {
			if _, err := io.ReadFull(reader, raw); err != nil {
				if ctx.Err() != nil || err == io.EOF || err == io.ErrClosedPipe {
					vs.done <- nil
					return
				}
				vs.done <- err
				return
			}
			for i := range pcm {
				pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
			}
			packet, err := enc.Encode(pcm)
			if err != nil {
				vs.done <- err
				return
			}
			tic := uint32(0)
			if worldTic != nil {
				tic = worldTic()
			}
			if err := broadcaster.BroadcastAudioChunk(netplay.AudioChunk{
				GameTic:     tic,
				StartSample: startSample,
				Payload:     packet,
			}); err != nil {
				vs.done <- err
				return
			}
			startSample += uint64(voicecodec.FrameSamples)
		}
	}()
	return vs, nil
}

func (s *VoiceStreamer) Close() error {
	if s == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		if err, ok := <-s.done; ok {
			return err
		}
	}
	return nil
}

type VoicePlayer struct {
	viewer *netplay.AudioViewer
	cancel context.CancelFunc
	done   chan error
	player *audio.Player
	source *streamSource
}

func StartViewer(parent context.Context, viewer *netplay.AudioViewer) (*VoicePlayer, error) {
	if viewer == nil {
		return nil, fmt.Errorf("audio viewer is required")
	}
	ctx := audiofx.EnsureSharedAudioContext()
	if ctx == nil {
		return nil, fmt.Errorf("shared audio context is unavailable")
	}
	stream := newStreamSource()
	player, err := ctx.NewPlayer(stream)
	if err != nil {
		stream.Close()
		return nil, err
	}
	player.SetVolume(1)
	player.Play()

	runCtx, cancel := context.WithCancel(parent)
	vp := &VoicePlayer{
		viewer: viewer,
		cancel: cancel,
		done:   make(chan error, 1),
		player: player,
		source: stream,
	}
	go vp.run(runCtx, ctx.SampleRate())
	return vp, nil
}

func (p *VoicePlayer) Close() error {
	if p == nil {
		return nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	if p.player != nil {
		p.player.Pause()
		_ = p.player.Close()
	}
	if p.source != nil {
		p.source.Close()
	}
	if p.done != nil {
		if err, ok := <-p.done; ok {
			return err
		}
	}
	return nil
}

func (p *VoicePlayer) run(ctx context.Context, playbackRate int) {
	defer close(p.done)
	defer p.source.Close()

	decoder, err := voicecodec.NewOpusDecoder()
	if err != nil {
		p.done <- err
		return
	}
	defer decoder.Close()

	var current netplay.AudioConfig
	for {
		select {
		case <-ctx.Done():
			p.done <- nil
			return
		default:
		}
		if cfg, ok, err := p.viewer.PollAudioConfig(); err != nil {
			if ctx.Err() != nil {
				p.done <- nil
				return
			}
			p.done <- err
			return
		} else if ok {
			current = cfg
		}
		chunk, ok, err := p.viewer.PollAudioChunk()
		if err != nil {
			if ctx.Err() != nil || err == io.EOF {
				p.done <- nil
				return
			}
			p.done <- err
			return
		}
		if !ok {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if current.Codec == 0 {
			continue
		}
		if current.Codec != netplayAudioCodecOpus() {
			p.done <- fmt.Errorf("unsupported audio codec %d", current.Codec)
			return
		}
		pcm, err := decoder.Decode(chunk.Payload)
		if err != nil {
			p.done <- err
			return
		}
		pcm = resampleMonoLinear(pcm, current.SampleRate, playbackRate)
		p.source.Write(stereoBytesFromMonoPCM(pcm))
	}
}

type streamSource struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	closed bool
}

func newStreamSource() *streamSource {
	s := &streamSource{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *streamSource) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.buf) == 0 && !s.closed {
		s.cond.Wait()
	}
	if len(s.buf) == 0 && s.closed {
		return 0, io.EOF
	}
	n := copy(p, s.buf)
	s.buf = append(s.buf[:0], s.buf[n:]...)
	return n, nil
}

func (s *streamSource) Write(p []byte) {
	s.mu.Lock()
	s.buf = append(s.buf, p...)
	s.mu.Unlock()
	s.cond.Signal()
}

func (s *streamSource) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.cond.Broadcast()
}

func stereoBytesFromMonoPCM(src []int16) []byte {
	out := make([]byte, len(src)*4)
	for i, sample := range src {
		base := i * 4
		binary.LittleEndian.PutUint16(out[base:base+2], uint16(sample))
		binary.LittleEndian.PutUint16(out[base+2:base+4], uint16(sample))
	}
	return out
}

func resampleMonoLinear(src []int16, fromRate, toRate int) []int16 {
	if fromRate <= 0 || toRate <= 0 || len(src) == 0 || fromRate == toRate {
		return append([]int16(nil), src...)
	}
	dstLen := (len(src)*toRate + fromRate - 1) / fromRate
	if dstLen < 1 {
		dstLen = 1
	}
	out := make([]int16, dstLen)
	if len(src) == 1 {
		for i := range out {
			out[i] = src[0]
		}
		return out
	}
	last := len(src) - 1
	for i := range out {
		posNum := i * fromRate
		idx := posNum / toRate
		if idx >= last {
			out[i] = src[last]
			continue
		}
		fracNum := posNum % toRate
		a := int64(src[idx])
		b := int64(src[idx+1])
		out[i] = int16((a*int64(toRate-fracNum) + b*int64(fracNum)) / int64(toRate))
	}
	return out
}

func netplayAudioCodecOpus() byte {
	return voicecodec.CodecOpus
}
