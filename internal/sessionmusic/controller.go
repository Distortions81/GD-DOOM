package sessionmusic

import (
	"time"

	"gddoom/internal/music"
	"gddoom/internal/sound"
)

type Controller struct {
	player  *music.ChunkPlayer
	driver  *music.Driver
	backend sound.Backend
	stop    chan struct{}
}

type musStreamFactory func() (*music.StreamRenderer, error)

const (
	impSynthGainRatio = 1.0
)

func New(volume float64, musPanMax float64, synthGain float64, preEmphasis bool, backend sound.Backend, bank music.PatchBank) (*Controller, error) {
	player, err := music.NewChunkPlayer()
	if err != nil {
		return nil, err
	}
	_ = player.SetVolume(volume)
	driver, err := music.NewDriverWithBackend(player.SampleRate(), bank, backend)
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
	if c == nil || c.player == nil || c.driver == nil || len(data) == 0 {
		return
	}
	player := c.player
	c.StopAndClear()
	factory := func() (*music.StreamRenderer, error) {
		return music.NewMUSStreamRenderer(c.driver, data)
	}
	var stream *music.StreamRenderer
	chunk, err := nextLoopChunk(factory, &stream)
	if err != nil || len(chunk) == 0 {
		return
	}
	_ = player.EnqueueBytesS16LE(chunk)
	_ = player.Start()
	stop := make(chan struct{})
	c.stop = stop
	go c.stream(player, stop, factory, stream)
}

func (c *Controller) stopStream() {
	if c == nil || c.stop == nil {
		return
	}
	close(c.stop)
	c.stop = nil
}

func nextLoopChunk(factory musStreamFactory, stream **music.StreamRenderer) ([]byte, error) {
	if factory == nil || stream == nil {
		return nil, nil
	}
	if *stream == nil {
		next, err := factory()
		if err != nil {
			return nil, err
		}
		*stream = next
	}
	chunk, done, err := (*stream).NextChunkS16LE(music.DefaultStreamChunkFrames())
	if err != nil {
		return nil, err
	}
	if done {
		*stream = nil
	}
	return chunk, nil
}

func (c *Controller) stream(player *music.ChunkPlayer, stop <-chan struct{}, factory musStreamFactory, stream *music.StreamRenderer) {
	if c == nil || player == nil || factory == nil {
		return
	}
	const bytesPerFrame = 4
	const checkPeriod = 12 * time.Millisecond
	lookaheadBytes := music.DefaultStreamLookahead() * bytesPerFrame
	ticker := time.NewTicker(checkPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		default:
		}
		for player.BufferedBytes() >= lookaheadBytes {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
		chunk, err := nextLoopChunk(factory, &stream)
		if err != nil {
			return
		}
		if len(chunk) > 0 {
			_ = player.EnqueueBytesS16LE(chunk)
		}
	}
}

func effectiveSynthGain(backend sound.Backend, gain float64) float64 {
	if backend == sound.BackendAuto {
		backend = sound.DefaultBackend()
	}
	return gain * synthGainRatio(backend)
}

func synthGainRatio(backend sound.Backend) float64 {
	switch backend {
	case sound.BackendImpSynth:
		return impSynthGainRatio
	default:
		return 1.0
	}
}
