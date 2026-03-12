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

const (
	pureGoOPLGainRatio = 1.0
	nukedOPLGainRatio  = 1.0
)

func New(volume float64, musPanMax float64, oplVolume float64, preEmphasis bool, backend sound.Backend, bank music.PatchBank) (*Controller, error) {
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
	driver.SetOutputGain(effectiveOPLGain(backend, oplVolume))
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
	_ = c.player.Stop()
	_ = c.player.ClearBuffer()
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
	c.driver.SetOutputGain(effectiveOPLGain(c.backend, v))
}

func (c *Controller) PlayMUS(data []byte) {
	if c == nil || c.player == nil || c.driver == nil || len(data) == 0 {
		return
	}
	c.StopAndClear()
	stream, err := music.NewMUSStreamRenderer(c.driver, data)
	if err != nil {
		return
	}
	chunk, done, err := stream.NextChunkS16LE(music.DefaultStreamChunkFrames)
	if err != nil || len(chunk) == 0 {
		return
	}
	_ = c.player.EnqueueBytesS16LE(chunk)
	_ = c.player.Start()
	if done {
		return
	}
	stop := make(chan struct{})
	c.stop = stop
	go c.stream(stop, stream)
}

func (c *Controller) stopStream() {
	if c == nil || c.stop == nil {
		return
	}
	close(c.stop)
	c.stop = nil
}

func (c *Controller) stream(stop <-chan struct{}, stream *music.StreamRenderer) {
	if c == nil || c.player == nil || stream == nil {
		return
	}
	const bytesPerFrame = 4
	const checkPeriod = 12 * time.Millisecond
	lookaheadBytes := music.DefaultStreamLookahead * bytesPerFrame
	ticker := time.NewTicker(checkPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		default:
		}
		for c.player.BufferedBytes() >= lookaheadBytes {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
		chunk, done, err := stream.NextChunkS16LE(music.DefaultStreamChunkFrames)
		if err != nil {
			return
		}
		if len(chunk) > 0 {
			_ = c.player.EnqueueBytesS16LE(chunk)
		}
		if done {
			return
		}
	}
}

func effectiveOPLGain(backend sound.Backend, gain float64) float64 {
	if backend == sound.BackendAuto {
		backend = sound.DefaultBackend()
	}
	return gain * oplGainRatio(backend)
}

func oplGainRatio(backend sound.Backend) float64 {
	switch backend {
	case sound.BackendPureGo:
		return pureGoOPLGainRatio
	case sound.BackendNuked:
		return nukedOPLGainRatio
	default:
		return 1.0
	}
}
