package sessionmusic

import (
	"time"

	"gddoom/internal/music"
)

type Controller struct {
	player  *music.ChunkPlayer
	driver  musicEventDriver
	backend music.Backend
	stop    chan struct{}
}

type musStreamFactory func() (*music.StreamRenderer, error)
type musicEventDriver interface {
	Reset()
	ApplyEvent(music.Event)
	GenerateStereoS16(int) []int16
	SampleRate() int
	TicRate() int
	SetMUSPanMax(float64)
	SetOutputGain(float64)
	SetPreEmphasis(bool)
	RenderMUSS16LE([]byte) ([]byte, error)
	RenderParsedMUSS16LE(*music.ParsedMUS) ([]byte, error)
}

const (
	impSynthGainRatio = 1.0
)

func New(volume float64, musPanMax float64, synthGain float64, preEmphasis bool, backend music.Backend, bank music.PatchBank, soundFont *music.SoundFontBank) (*Controller, error) {
	player, err := music.NewChunkPlayer()
	if err != nil {
		return nil, err
	}
	_ = player.SetVolume(volume)
	driver, err := newMusicDriver(player.SampleRate(), backend, bank, soundFont)
	if err != nil {
		_ = player.Close()
		return nil, err
	}
	driver.SetMUSPanMax(musPanMax)
	driver.SetOutputGain(effectiveSynthGain(backend, synthGain))
	driver.SetPreEmphasis(preEmphasis)
	return &Controller{
		player:  player,
		driver:  driver,
		backend: backend,
	}, nil
}

func newMusicDriver(sampleRate int, backend music.Backend, bank music.PatchBank, soundFont *music.SoundFontBank) (musicEventDriver, error) {
	switch music.ResolveBackend(backend) {
	case music.BackendImpSynth:
		return music.NewDriverWithBackend(sampleRate, bank, music.ResolveBackend(backend))
	case music.BackendMeltySynth:
		return music.NewMeltySynthDriver(sampleRate, soundFont)
	default:
		return music.NewDriverWithBackend(sampleRate, bank, music.BackendImpSynth)
	}
}

func (c *Controller) Close() {
	c.stopStream()
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.Close()
	c.player = nil
}

func (c *Controller) StopAndClear() {
	c.stopStream()
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.ResetPlayback()
}

func (c *Controller) SetVolume(v float64) {
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.SetVolume(v)
}

func (c *Controller) SetOutputGain(v float64) {
	if c == nil || c.driver == nil {
		return
	}
	c.driver.SetOutputGain(effectiveSynthGain(c.backend, v))
}

func (c *Controller) PlayMUS(data []byte) {
	c.playMUS(data, true)
}

func (c *Controller) PlayMUSOnce(data []byte) {
	c.playMUSOnce(data)
}

func (c *Controller) playMUS(data []byte, loop bool) {
	if c == nil || c.player == nil || c.driver == nil || len(data) == 0 {
		return
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil || parsed == nil {
		return
	}
	const bytesPerFrame = 4
	chunkFrames := music.DefaultStreamChunkFramesForBackend(c.backend)
	lookaheadFrames := music.DefaultStreamLookaheadForBackend(c.backend)
	targetBytes := startupPrefillBytes(lookaheadFrames * bytesPerFrame)
	player := c.player
	c.StopAndClear()
	player.SetBlockingPrefill(targetBytes)
	factory := func() (*music.StreamRenderer, error) {
		return music.NewParsedMUSStreamRenderer(c.driver, parsed)
	}
	var stream *music.StreamRenderer
	buffered, err := prefillStream(player, factory, &stream, loop, chunkFrames, targetBytes)
	if err != nil {
		return
	}
	if err := player.Sync(); err != nil {
		return
	}
	if buffered == 0 {
		return
	}
	started := buffered >= targetBytes
	if started {
		_ = player.Start()
	}
	stop := make(chan struct{})
	c.stop = stop
	go c.stream(player, stop, factory, stream, loop, buffered, started)
}

func (c *Controller) playMUSOnce(data []byte) {
	if c == nil || c.player == nil || c.driver == nil || len(data) == 0 {
		return
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil || parsed == nil {
		return
	}
	player := c.player
	c.StopAndClear()
	player.DisableBlockingPrefill()
	pcm, err := c.driver.RenderParsedMUSS16LE(parsed)
	if err != nil || len(pcm) == 0 {
		return
	}
	_ = player.EnqueueBytesS16LE(pcm)
	if err := player.Sync(); err != nil {
		return
	}
	_ = player.Start()
}

func (c *Controller) stopStream() {
	if c == nil || c.stop == nil {
		return
	}
	close(c.stop)
	c.stop = nil
}

func nextChunk(factory musStreamFactory, stream **music.StreamRenderer, loop bool, frames int) ([]byte, error) {
	return nextChunkFrames(factory, stream, loop, frames)
}

func nextChunkFrames(factory musStreamFactory, stream **music.StreamRenderer, loop bool, frames int) ([]byte, error) {
	if factory == nil || stream == nil {
		return nil, nil
	}
	for {
		if *stream == nil {
			next, err := factory()
			if err != nil {
				return nil, err
			}
			*stream = next
		}
		chunk, done, err := (*stream).NextChunkS16LE(frames)
		if err != nil {
			return nil, err
		}
		if done {
			*stream = nil
			if !loop || len(chunk) > 0 {
				return chunk, nil
			}
			continue
		}
		return chunk, nil
	}
}

func prefillStream(player *music.ChunkPlayer, factory musStreamFactory, stream **music.StreamRenderer, loop bool, chunkFrames int, targetBytes int) (int, error) {
	if player == nil || factory == nil || stream == nil {
		return 0, nil
	}
	if targetBytes < 0 {
		targetBytes = 0
	}
	buffered := 0
	for buffered < targetBytes {
		chunk, err := nextChunk(factory, stream, loop, chunkFrames)
		if err != nil {
			return buffered, err
		}
		if len(chunk) > 0 {
			_ = player.EnqueueBytesS16LE(chunk)
			buffered += len(chunk)
		}
		if *stream == nil {
			break
		}
		if len(chunk) == 0 {
			break
		}
	}
	if targetBytes == 0 && buffered == 0 {
		chunk, err := nextChunk(factory, stream, loop, chunkFrames)
		if err != nil {
			return buffered, err
		}
		if len(chunk) > 0 {
			_ = player.EnqueueBytesS16LE(chunk)
			buffered += len(chunk)
		}
	}
	return buffered, nil
}

func startupPrefillBytes(lookaheadBytes int) int {
	if lookaheadBytes <= 0 {
		return 0
	}
	return lookaheadBytes
}

func (c *Controller) stream(player *music.ChunkPlayer, stop <-chan struct{}, factory musStreamFactory, stream *music.StreamRenderer, loop bool, buffered int, started bool) {
	if c == nil || player == nil || factory == nil {
		return
	}
	const bytesPerFrame = 4
	const checkPeriod = 12 * time.Millisecond
	chunkFrames := music.DefaultStreamChunkFramesForBackend(c.backend)
	lookaheadBytes := music.DefaultStreamLookaheadForBackend(c.backend) * bytesPerFrame
	targetBytes := startupPrefillBytes(lookaheadBytes)
	ticker := time.NewTicker(checkPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		default:
		}
		if !started {
			if buffered < targetBytes {
				chunk, err := nextChunk(factory, &stream, loop, chunkFrames)
				if err != nil {
					return
				}
				if stream == nil && len(chunk) == 0 {
					if buffered == 0 {
						return
					}
				}
				if len(chunk) > 0 {
					_ = player.EnqueueBytesS16LE(chunk)
					buffered += len(chunk)
				}
				if buffered < targetBytes {
					select {
					case <-stop:
						return
					case <-ticker.C:
					}
					continue
				}
			}
			if err := player.Sync(); err != nil {
				return
			}
			if buffered == 0 {
				return
			}
			_ = player.Start()
			started = true
			continue
		}
		buffered = player.BufferedBytes()
		if buffered == 0 {
			_ = player.Stop()
			if err := player.Sync(); err != nil {
				return
			}
			started = false
			continue
		}
		for player.BufferedBytes() >= lookaheadBytes {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
		chunk, err := nextChunk(factory, &stream, loop, chunkFrames)
		if err != nil {
			return
		}
		if stream == nil && len(chunk) == 0 {
			return
		}
		if len(chunk) > 0 {
			_ = player.EnqueueBytesS16LE(chunk)
			buffered += len(chunk)
		}
		if stream == nil && !loop {
			return
		}
	}
}

func effectiveSynthGain(backend music.Backend, gain float64) float64 {
	backend = music.ResolveBackend(backend)
	return gain * synthGainRatio(backend)
}

func synthGainRatio(backend music.Backend) float64 {
	switch backend {
	case music.BackendImpSynth:
		return impSynthGainRatio
	default:
		return 1.0
	}
}
