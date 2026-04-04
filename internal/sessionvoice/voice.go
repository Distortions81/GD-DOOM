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

const (
	audioLateDropTics        = 2
	audioStartupBufferFrames = 2
	audioFadeSamples       = 256
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
		return nil, err
	}
	if err := broadcaster.BroadcastAudioConfig(netplay.AudioConfig{
		Codec:        netplayAudioCodecPCM16Mono(),
		SampleRate:   voicecodec.SampleRate,
		Channels:     voicecodec.Channels,
		FrameSamples: voicecodec.FrameSamples,
		Bitrate:      voicecodec.SampleRate * voicecodec.Channels * 16,
	}); err != nil {
		cancel()
		_ = reader.Close()
		return nil, err
	}
	vs := &VoiceStreamer{
		cancel: cancel,
		done:   make(chan error, 1),
	}
	go func() {
		defer close(vs.done)
		defer reader.Close()
		frameBytes := voicecodec.FrameSamples * voicecodec.Channels * 2
		raw := make([]byte, frameBytes)
		pcm := make([]int16, voicecodec.FrameSamples*voicecodec.Channels)
		agc := newMicAGC()
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
			agc.ProcessFrame(pcm, voicecodec.SampleRate)
			for i, sample := range pcm {
				binary.LittleEndian.PutUint16(raw[i*2:i*2+2], uint16(sample))
			}
			tic := uint32(0)
			if worldTic != nil {
				tic = worldTic()
			}
			if err := broadcaster.BroadcastAudioChunk(netplay.AudioChunk{
				GameTic:     tic,
				StartSample: startSample,
				Payload:     append([]byte(nil), raw...),
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
	currentTic func() uint32
}

func StartViewer(parent context.Context, viewer *netplay.AudioViewer, currentTic func() uint32) (*VoicePlayer, error) {
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
		currentTic: currentTic,
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

	var current netplay.AudioConfig
	var lastStartSample uint64
	haveStartSample := false
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
		if current.Codec != netplayAudioCodecOpus() && current.Codec != netplayAudioCodecPCM16Mono() {
			p.done <- fmt.Errorf("unsupported audio codec %d", current.Codec)
			return
		}
		if p.currentTic != nil && chunk.GameTic+audioLateDropTics < p.currentTic() {
			p.source.Reset()
			haveStartSample = false
			continue
		}
		if haveStartSample && chunk.StartSample != lastStartSample+uint64(voicecodec.FrameSamples) {
			p.source.Reset()
		}
		var pcm []int16
		switch current.Codec {
		case netplayAudioCodecPCM16Mono():
			if len(chunk.Payload)%2 != 0 {
				p.done <- fmt.Errorf("raw pcm payload len=%d must be even", len(chunk.Payload))
				return
			}
			pcm = make([]int16, len(chunk.Payload)/2)
			for i := range pcm {
				pcm[i] = int16(binary.LittleEndian.Uint16(chunk.Payload[i*2 : i*2+2]))
			}
		case netplayAudioCodecOpus():
			decoder, err := voicecodec.NewOpusDecoder()
			if err != nil {
				p.done <- err
				return
			}
			pcm, err = decoder.Decode(chunk.Payload)
			_ = decoder.Close()
			if err != nil {
				p.done <- err
				return
			}
		}
		pcm = resampleMonoLinear(pcm, current.SampleRate, playbackRate)
		p.source.Write(stereoBytesFromMonoPCM(pcm))
		lastStartSample = chunk.StartSample
		haveStartSample = true
	}
}

type streamSource struct {
	mu     sync.Mutex
	buf    []byte
	fade   []byte
	closed bool
	started bool

	lastSample [4]byte
	needFadeIn bool
}

func newStreamSource() *streamSource {
	return &streamSource{needFadeIn: true}
}

func (s *streamSource) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if len(s.fade) > 0 {
		n := copy(p, s.fade)
		copy(s.fade, s.fade[n:])
		s.fade = s.fade[:len(s.fade)-n]
		if n < len(p) {
			clear(p[n:])
		}
		return len(p), nil
	}
	if !s.started {
		if len(s.buf) < audioStartupBufferFrames*voicecodec.FrameSamples*4 {
			if s.closed {
				return 0, io.EOF
			}
			clear(p)
			return len(p), nil
		}
		s.started = true
	}
	if len(s.buf) == 0 {
		if s.closed {
			return 0, io.EOF
		}
		s.fade = buildFadeOutStereo16(s.lastSample, audioFadeSamples)
		if len(s.fade) > 0 {
			n := copy(p, s.fade)
			copy(s.fade, s.fade[n:])
			s.fade = s.fade[:len(s.fade)-n]
			if n < len(p) {
				clear(p[n:])
			}
		} else {
			clear(p)
		}
		s.needFadeIn = true
		return len(p), nil
	}
	n := copy(p, s.buf)
	copy(s.buf, s.buf[n:])
	s.buf = s.buf[:len(s.buf)-n]
	if n >= 4 {
		copy(s.lastSample[:], p[n-4:n])
	}
	if n < len(p) {
		clear(p[n:])
		s.needFadeIn = true
	}
	if len(s.buf) == 0 && n == 0 && s.closed {
		return 0, io.EOF
	}
	return len(p), nil
}

func (s *streamSource) Write(p []byte) {
	s.mu.Lock()
	if len(p) == 0 {
		s.mu.Unlock()
		return
	}
	data := append([]byte(nil), p...)
	if s.needFadeIn {
		applyFadeInStereo16(data, audioFadeSamples)
		s.needFadeIn = false
	}
	s.buf = append(s.buf, data...)
	s.mu.Unlock()
}

func (s *streamSource) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

func (s *streamSource) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fade = buildFadeOutStereo16(s.lastSample, audioFadeSamples)
	s.buf = s.buf[:0]
	s.started = false
	s.needFadeIn = true
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

func netplayAudioCodecPCM16Mono() byte {
	return voicecodec.CodecPCM16Mono
}

func applyFadeInStereo16(p []byte, samples int) {
	if samples <= 0 {
		return
	}
	total := len(p) / 4
	if total < samples {
		samples = total
	}
	for i := 0; i < samples; i++ {
		scaleNum := i + 1
		base := i * 4
		left := int16(binary.LittleEndian.Uint16(p[base : base+2]))
		right := int16(binary.LittleEndian.Uint16(p[base+2 : base+4]))
		left = int16(int(left) * scaleNum / samples)
		right = int16(int(right) * scaleNum / samples)
		binary.LittleEndian.PutUint16(p[base:base+2], uint16(left))
		binary.LittleEndian.PutUint16(p[base+2:base+4], uint16(right))
	}
}

func buildFadeOutStereo16(last [4]byte, samples int) []byte {
	left := int16(binary.LittleEndian.Uint16(last[0:2]))
	right := int16(binary.LittleEndian.Uint16(last[2:4]))
	if left == 0 && right == 0 || samples <= 0 {
		return nil
	}
	out := make([]byte, samples*4)
	for i := 0; i < samples; i++ {
		scaleNum := samples - i - 1
		base := i * 4
		l := int16(int(left) * scaleNum / samples)
		r := int16(int(right) * scaleNum / samples)
		binary.LittleEndian.PutUint16(out[base:base+2], uint16(l))
		binary.LittleEndian.PutUint16(out[base+2:base+4], uint16(r))
	}
	return out
}
