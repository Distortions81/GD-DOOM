package sessionvoice

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gddoom/internal/audiofx"
	"gddoom/internal/audioinput"
	"gddoom/internal/netplay"
	"gddoom/internal/voicecodec"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const (
	audioStartupBufferFrames  = 2
	audioTargetBufferedFrames = 2
	audioTrimBufferedFrames   = 4
	audioResetBufferedFrames  = 5
	audioPlayerBuffer         = 40 * time.Millisecond
	audioFadeSamples          = 512
	audioCatchupFadeSamples   = 1024
)

type BroadcasterOptions struct {
	Codec             string
	G726BitsPerSample int
	SampleRate        int
	AGCEnabled        bool
	GateEnabled       bool
	GateThreshold     float64
}

type VoiceStreamer struct {
	cancel  context.CancelFunc
	done    chan error
	updates chan netplay.AudioFormat
	agc     *micAGC
	level   atomic.Uint64
	gated   atomic.Bool
}

func defaultAudioFormat() netplay.AudioFormat {
	const defaultVoiceSampleRate = 24000
	packetSamples, err := voicecodec.PacketSamplesFor(defaultVoiceSampleRate, voicecodec.PacketDurationMillis)
	if err != nil {
		packetSamples = 720
	}
	return netplay.AudioFormat{
		Codec:                netplayAudioCodecG72632(),
		BitsPerSample:        4,
		SampleRateChoice:     byte(voicecodec.SampleRateChoice24000),
		SampleRate:           defaultVoiceSampleRate,
		Channels:             voicecodec.Channels,
		PacketDurationMillis: voicecodec.PacketDurationMillis,
		PacketSamples:        packetSamples,
		Bitrate:              voicecodec.G726Bitrate(defaultVoiceSampleRate, voicecodec.Channels, 4),
	}
}

func resolveBroadcasterFormat(opts BroadcasterOptions) (netplay.AudioFormat, error) {
	format := defaultAudioFormat()
	g726Bits := voicecodec.NormalizeG726BitsPerSample(opts.G726BitsPerSample)
	switch codec := strings.TrimSpace(strings.ToLower(opts.Codec)); codec {
	case "":
	case "g726", "g726_32", "g72632":
		format.Codec = netplayAudioCodecG72632()
	case "pcm", "pcm16", "pcm16_mono":
		format.Codec = netplayAudioCodecPCM16Mono()
	default:
		return netplay.AudioFormat{}, fmt.Errorf("unsupported mic codec %q", opts.Codec)
	}
	if opts.SampleRate > 0 {
		format.SampleRate = opts.SampleRate
		format.SampleRateChoice = byte(voicecodec.SampleRateChoiceFromRate(opts.SampleRate))
	}
	packetSamples, err := voicecodec.PacketSamplesFor(format.SampleRate, format.PacketDurationMillis)
	if err != nil {
		return netplay.AudioFormat{}, err
	}
	format.PacketSamples = packetSamples
	switch format.Codec {
	case netplayAudioCodecG72632():
		format.BitsPerSample = byte(g726Bits)
		format.Bitrate = voicecodec.G726Bitrate(format.SampleRate, format.Channels, g726Bits)
	case netplayAudioCodecPCM16Mono():
		format.BitsPerSample = 16
		format.Bitrate = format.SampleRate * format.Channels * 16
	}
	return format, nil
}

func ResolveBroadcasterFormat(opts BroadcasterOptions) (netplay.AudioFormat, error) {
	return resolveBroadcasterFormat(opts)
}

func (s *VoiceStreamer) UpdateFormat(format netplay.AudioFormat) error {
	if s == nil {
		return nil
	}
	if s.updates == nil {
		return fmt.Errorf("voice streamer is not running")
	}
	select {
	case s.updates <- format:
		return nil
	default:
		return fmt.Errorf("voice format update is already pending")
	}
}

func (s *VoiceStreamer) SetSampleRateChoice(choice voicecodec.SampleRateChoice) error {
	format := defaultAudioFormat()
	format.SampleRateChoice = byte(choice)
	format.SampleRate = choice.SampleRate()
	if format.SampleRate <= 0 {
		return fmt.Errorf("sample rate choice %d is not supported", choice)
	}
	packetSamples, err := voicecodec.PacketSamplesFor(format.SampleRate, format.PacketDurationMillis)
	if err != nil {
		return err
	}
	format.PacketSamples = packetSamples
	format.Bitrate = format.SampleRate * format.Channels * 4
	return s.UpdateFormat(format)
}

func (s *VoiceStreamer) UpdateGate(enabled bool, threshold float64) {
	if s == nil || s.agc == nil {
		return
	}
	s.agc.SetGate(enabled, threshold)
}

func (s *VoiceStreamer) UpdateAGC(enabled bool) {
	if s == nil || s.agc == nil {
		return
	}
	s.agc.SetEnabled(enabled)
}

func (s *VoiceStreamer) InputLevel() float64 {
	if s == nil {
		return 0
	}
	return math.Float64frombits(s.level.Load())
}

func (s *VoiceStreamer) InputGateActive() bool {
	if s == nil {
		return false
	}
	return s.gated.Load()
}

func StartPulseBroadcaster(parent context.Context, broadcaster *netplay.AudioBroadcaster, device string, opts BroadcasterOptions, worldTic func() uint32) (*VoiceStreamer, error) {
	if broadcaster == nil {
		return nil, fmt.Errorf("audio broadcaster is required")
	}
	ctx, cancel := context.WithCancel(parent)
	cfg := audioinput.PulseConfig{
		Device:        device,
		SampleRate:    voicecodec.CaptureSampleRate,
		Channels:      voicecodec.Channels,
		Format:        "s16le",
		LatencyMillis: voicecodec.FrameDurationMillis,
	}
	reader, err := audioinput.OpenPulseReader(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	format, err := resolveBroadcasterFormat(opts)
	if err != nil {
		cancel()
		_ = reader.Close()
		return nil, err
	}
	if err := broadcaster.BroadcastAudioFormat(format); err != nil {
		cancel()
		_ = reader.Close()
		return nil, err
	}
	vs := &VoiceStreamer{
		cancel:  cancel,
		done:    make(chan error, 1),
		updates: make(chan netplay.AudioFormat, 1),
	}
	go func() {
		defer close(vs.done)
		defer reader.Close()
		currentFormat := format
		g726Bits := voicecodec.NormalizeG726BitsPerSample(int(currentFormat.BitsPerSample))
		g726Encoder := voicecodec.NewG726Encoder(currentFormat.PacketSamples, g726Bits)
		frameBytes := voicecodec.CaptureFrameSamples * voicecodec.Channels * 2
		raw := make([]byte, frameBytes)
		framePCM := make([]int16, voicecodec.CaptureFrameSamples*voicecodec.Channels)
		hpf := newHighPassFilter(50, voicecodec.CaptureSampleRate)
		agc := newMicAGC()
		agc.SetEnabled(opts.AGCEnabled)
		agc.SetGate(opts.GateEnabled, opts.GateThreshold)
		vs.agc = agc
		var startSample uint64
		for {
			select {
			case next := <-vs.updates:
				if err := broadcaster.BroadcastAudioFormat(next); err != nil {
					continue
				}
				currentFormat = next
				g726Bits = voicecodec.NormalizeG726BitsPerSample(int(currentFormat.BitsPerSample))
				g726Encoder.SetPacketSamples(currentFormat.PacketSamples)
				g726Encoder.SetBitsPerSample(g726Bits)
				startSample = 0
			default:
			}
			if _, err := io.ReadFull(reader, raw); err != nil {
				if ctx.Err() != nil || err == io.EOF || err == io.ErrClosedPipe {
					vs.done <- nil
					return
				}
				vs.done <- err
				return
			}
			for i := range framePCM {
				framePCM[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
			}
			hpf.ProcessInt16(framePCM)
			allSilent := agc.ProcessFrame(framePCM, voicecodec.CaptureSampleRate)
			vs.level.Store(math.Float64bits(micLevel(framePCM)))
			vs.gated.Store(agc.GateActive())
			if !allSilent {
				down := downsampleCaptureToVoice(framePCM, currentFormat.SampleRate)
				var payload []byte
				switch currentFormat.Codec {
				case netplayAudioCodecG72632():
					payload, err = g726Encoder.Encode(down)
				case netplayAudioCodecPCM16Mono():
					payload = monoPCMBytes(down)
				default:
					vs.done <- fmt.Errorf("unsupported audio codec %d", currentFormat.Codec)
					return
				}
				if err != nil {
					switch currentFormat.Codec {
					case netplayAudioCodecG72632():
						g726Encoder.Reset()
					}
					startSample += uint64(currentFormat.PacketSamples)
					continue
				}
				tic := uint32(0)
				if worldTic != nil {
					tic = worldTic()
				}
				if err := broadcaster.BroadcastAudioChunk(netplay.AudioChunk{
					GameTic:     tic,
					StartSample: startSample,
					Payload:     payload,
				}); err != nil {
					switch currentFormat.Codec {
					case netplayAudioCodecG72632():
						g726Encoder.Reset()
					}
					startSample += uint64(currentFormat.PacketSamples)
					continue
				}
			}
			startSample += uint64(currentFormat.PacketSamples)
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
	viewer       *netplay.AudioViewer
	cancel       context.CancelFunc
	done         chan error
	player       *audio.Player
	source       *streamSource
	currentTic   func() uint32
	playbackRate int

	mu           sync.RWMutex
	haveChunkTic bool
	lastChunkTic uint32
	syncSamples  []voiceSyncSample
}

type voiceSyncSample struct {
	at     time.Time
	millis int
}

func StartViewer(parent context.Context, viewer *netplay.AudioViewer, currentTic func() uint32) (*VoicePlayer, error) {
	if viewer == nil {
		return nil, fmt.Errorf("audio viewer is required")
	}
	ctx := audiofx.EnsureSharedAudioContext()
	if ctx == nil {
		return nil, fmt.Errorf("shared audio context is unavailable")
	}
	playbackRate := ctx.SampleRate()
	stream := newStreamSource(playbackRate)
	player, err := ctx.NewPlayer(stream)
	if err != nil {
		stream.Close()
		return nil, err
	}
	player.SetBufferSize(audioPlayerBuffer)
	player.SetVolume(1)
	player.Play()

	runCtx, cancel := context.WithCancel(parent)
	vp := &VoicePlayer{
		viewer:       viewer,
		cancel:       cancel,
		done:         make(chan error, 1),
		player:       player,
		source:       stream,
		currentTic:   currentTic,
		playbackRate: playbackRate,
	}
	go vp.run(runCtx, playbackRate)
	return vp, nil
}

func (p *VoicePlayer) VoiceSyncOffsetMillis() (int, bool) {
	if p == nil || p.source == nil || p.playbackRate <= 0 {
		return 0, false
	}
	bufferedMillis := p.source.BufferedMillis(p.playbackRate)
	p.mu.Lock()
	defer p.mu.Unlock()
	haveChunkTic := p.haveChunkTic
	lastChunkTic := p.lastChunkTic
	if !haveChunkTic {
		return 0, false
	}
	if bufferedMillis <= 0 {
		return 0, false
	}
	currentTic := uint32(0)
	if p.currentTic != nil {
		currentTic = p.currentTic()
	}
	ticDeltaMillis := int(math.Round(float64(int64(currentTic)-int64(lastChunkTic)) * (1000.0 / 35.0)))
	offset := ticDeltaMillis + bufferedMillis + voicecodec.PacketDurationMillis + int(audioPlayerBuffer/time.Millisecond)
	now := time.Now()
	p.syncSamples = append(p.syncSamples, voiceSyncSample{at: now, millis: offset})
	cutoff := now.Add(-1 * time.Second)
	keep := 0
	for _, sample := range p.syncSamples {
		if sample.at.Before(cutoff) {
			continue
		}
		p.syncSamples[keep] = sample
		keep++
	}
	p.syncSamples = p.syncSamples[:keep]
	if len(p.syncSamples) == 0 {
		return 0, false
	}
	sum := 0
	for _, sample := range p.syncSamples {
		sum += sample.millis
	}
	return int(math.Round(float64(sum) / float64(len(p.syncSamples)))), true
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

	var current netplay.AudioFormat
	g726Decoder := voicecodec.NewG726Decoder(voicecodec.PacketSamples, 4)
	var lastStartSample uint64
	haveStartSample := false
	for {
		select {
		case <-ctx.Done():
			p.done <- nil
			return
		default:
		}
		if format, ok, err := p.viewer.PollAudioFormat(); err != nil {
			if ctx.Err() != nil {
				p.done <- nil
				return
			}
			p.done <- err
			return
		} else if ok {
			if format != current {
				g726Decoder.Reset()
				if format.Codec == netplayAudioCodecG72632() {
					g726Decoder.SetBitsPerSample(int(format.BitsPerSample))
				}
				g726Decoder.SetPacketSamples(format.PacketSamples)
				p.source.Reset()
				haveStartSample = false
			}
			current = format
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
		if current.Codec != netplayAudioCodecG72632() && current.Codec != netplayAudioCodecPCM16Mono() {
			p.done <- fmt.Errorf("unsupported audio codec %d", current.Codec)
			return
		}
		if haveStartSample && chunk.StartSample != lastStartSample+uint64(current.PacketSamples) {
			g726Decoder.Reset()
			p.source.Reset()
		}
		var pcm []int16
		if chunk.Silence {
			pcm = make([]int16, current.PacketSamples*current.Channels)
		} else {
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
			case netplayAudioCodecG72632():
				pcm, err = g726Decoder.Decode(chunk.Payload)
				if err != nil {
					g726Decoder.Reset()
					p.source.Reset()
					haveStartSample = false
					continue
				}
			}
		}
		pcm = resampleMonoLinear(pcm, current.SampleRate, playbackRate)
		if !haveStartSample {
			// Fresh decoder state is noticeably rough on the first packet at lower G.726 bitrates.
			// Ease that startup transient in instead of sending the discontinuity straight to output.
			applyFadeInMono16(pcm, max(1, playbackRate/200))
		}
		p.source.Write(stereoBytesFromMonoPCM(pcm))
		lastStartSample = chunk.StartSample
		haveStartSample = true
	}
}

type streamSource struct {
	mu                  sync.Mutex
	buf                 []byte
	fade                []byte
	closed              bool
	started             bool
	fadedOut            bool
	startupBytes        int
	targetBufferedBytes int
	trimBufferedBytes   int
	resetBufferedBytes  int

	lastSample [4]byte
	needFadeIn bool
}

func newStreamSource(playbackRate int) *streamSource {
	if playbackRate <= 0 {
		playbackRate = 44100
	}
	samplesPerPacket := (playbackRate*voicecodec.PacketDurationMillis + 999) / 1000
	if samplesPerPacket < 1 {
		samplesPerPacket = 1
	}
	bytesPerPacket := samplesPerPacket * 4
	return &streamSource{
		needFadeIn:          true,
		startupBytes:        audioStartupBufferFrames * bytesPerPacket,
		targetBufferedBytes: audioTargetBufferedFrames * bytesPerPacket,
		trimBufferedBytes:   audioTrimBufferedFrames * bytesPerPacket,
		resetBufferedBytes:  audioResetBufferedFrames * bytesPerPacket,
	}
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
		return n, nil
	}
	if !s.started {
		if len(s.buf) < s.startupBytes {
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
		if !s.fadedOut {
			s.fade = buildFadeOutStereo16(s.lastSample, audioFadeSamples)
			s.fadedOut = true
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
		clear(p)
		return len(p), nil
	}
	n := copy(p, s.buf)
	copy(s.buf, s.buf[n:])
	s.buf = s.buf[:len(s.buf)-n]
	if n >= 4 {
		copy(s.lastSample[:], p[n-4:n])
	}
	if len(s.buf) == 0 && n == 0 && s.closed {
		return 0, io.EOF
	}
	return n, nil
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
	s.fadedOut = false
	s.buf = append(s.buf, data...)
	s.resetBufferedAudioLocked()
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
	s.fade = buildFadeOutStereo16(s.lastSample, audioCatchupFadeSamples)
	s.buf = s.buf[:0]
	s.started = false
	s.fadedOut = false
	s.needFadeIn = true
}

func (s *streamSource) BufferedMillis(playbackRate int) int {
	if s == nil || playbackRate <= 0 {
		return 0
	}
	s.mu.Lock()
	bytes := len(s.buf) + len(s.fade)
	s.mu.Unlock()
	if bytes <= 0 {
		return 0
	}
	samples := bytes / 4
	return int(math.Round(float64(samples) * 1000 / float64(playbackRate)))
}

func (s *streamSource) resetBufferedAudioLocked() {
	if s.resetBufferedBytes <= 0 || len(s.buf) <= s.resetBufferedBytes {
		return
	}
	before := len(s.buf)
	keep := s.trimBufferedBytes
	if keep <= 0 {
		keep = s.targetBufferedBytes
	}
	if keep < s.startupBytes {
		keep = s.startupBytes
	}
	if keep <= 0 {
		keep = 4
	}
	if keep > len(s.buf) {
		keep = len(s.buf)
	}
	keep -= keep % 4
	if keep <= 0 {
		s.buf = s.buf[:0]
		s.started = false
		s.needFadeIn = true
		return
	}
	start := len(s.buf) - keep
	dropped := start / 4
	kept := keep / 4
	fmt.Printf("voice-skip buffered=%d_samples dropped=%d_samples kept=%d_samples\n", before/4, dropped, kept)
	s.fade = buildFadeOutStereo16(s.lastSample, audioCatchupFadeSamples)
	copy(s.buf, s.buf[start:])
	s.buf = s.buf[:keep]
	applyFadeInStereo16(s.buf, audioCatchupFadeSamples)
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

func monoPCMBytes(src []int16) []byte {
	out := make([]byte, len(src)*2)
	for i, sample := range src {
		base := i * 2
		binary.LittleEndian.PutUint16(out[base:base+2], uint16(sample))
	}
	return out
}

func micLevel(src []int16) float64 {
	if len(src) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range src {
		v := float64(sample)
		sum += v * v
	}
	rms := math.Sqrt(sum / float64(len(src)))
	level := rms / 32767.0
	if level < 0 {
		return 0
	}
	if level > 1 {
		return 1
	}
	return level
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

func netplayAudioCodecPCM16Mono() byte {
	return voicecodec.CodecPCM16Mono
}

func netplayAudioCodecG72632() byte {
	return voicecodec.CodecG72632
}

func applyFadeInMono16(pcm []int16, samples int) {
	if samples <= 0 || len(pcm) == 0 {
		return
	}
	if len(pcm) < samples {
		samples = len(pcm)
	}
	for i := 0; i < samples; i++ {
		pcm[i] = int16(int(pcm[i]) * (i + 1) / samples)
	}
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
