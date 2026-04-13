package sessionmusic

import (
	"fmt"

	"gddoom/internal/audiofx"
	"gddoom/internal/music"
)

type Controller struct {
	player    *music.ChunkPlayer
	driver    musicEventDriver
	backend   music.Backend
	pcSpeaker audiofx.PCSpeaker
	patchBank music.PatchBank
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
	impSynthGainRatio  = 1.0
	pcSpeakerMusicRate = 11025
)

func New(volume float64, musPanMax float64, synthGain float64, preEmphasis bool, backend music.Backend, bank music.PatchBank, soundFont *music.SoundFontBank, pcSpeaker audiofx.PCSpeaker) (*Controller, error) {
	if music.ResolveBackend(backend) == music.BackendPCSpeaker {
		if pcSpeaker == nil {
			return nil, fmt.Errorf("pcspeaker backend requires a shared PC speaker player")
		}
		driver, err := newMusicDriver(pcSpeakerMusicRate, backend, bank, soundFont)
		if err != nil {
			return nil, err
		}
		driver.SetMUSPanMax(musPanMax)
		driver.SetOutputGain(effectiveSynthGain(backend, synthGain))
		driver.SetPreEmphasis(preEmphasis)
		return &Controller{
			backend:   backend,
			driver:    driver,
			pcSpeaker: pcSpeaker,
			patchBank: bank,
		}, nil
	}
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
	if c != nil && c.pcSpeaker != nil {
		c.pcSpeaker.ClearMusic()
	}
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.Close()
	c.player = nil
}

func (c *Controller) StopAndClear() {
	c.stopStream()
	if c != nil && c.pcSpeaker != nil {
		c.pcSpeaker.ClearMusic()
	}
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.ResetPlayback()
}

func (c *Controller) SetVolume(v float64) {
	if c != nil && c.pcSpeaker != nil {
		return
	}
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.SetVolume(v)
}

func (c *Controller) SetOutputGain(v float64) {
	if c == nil || c.driver == nil || c.pcSpeaker != nil {
		return
	}
	c.driver.SetOutputGain(effectiveSynthGain(c.backend, v))
}

func (c *Controller) PlayMUS(data []byte) {
	c.playMUS(data, true)
}

func (c *Controller) PlayParsed(parsed *music.ParsedMUS) {
	c.playParsed(parsed, true)
}

func (c *Controller) PlayMUSOnce(data []byte) {
	c.playMUSOnce(data)
}

func (c *Controller) PlayParsedOnce(parsed *music.ParsedMUS) {
	c.playParsed(parsed, false)
}

func (c *Controller) Tick() {
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.Tick()
}

func (c *Controller) playParsed(parsed *music.ParsedMUS, loop bool) {
	if c != nil && c.pcSpeaker != nil {
		if parsed == nil {
			return
		}
		seq, tickRate := music.RenderParsedMUSToPCSpeaker(c.patchBank, parsed)
		c.StopAndClear()
		if len(seq) == 0 {
			c.pcSpeaker.ClearMusic()
			return
		}
		c.pcSpeaker.SetMusic(seq, tickRate, loop)
		return
	}
	if c == nil || c.player == nil || c.driver == nil || parsed == nil {
		return
	}
	chunkFrames := music.DefaultStreamChunkFramesForBackend(c.backend)
	enqueueFrames := music.DefaultStreamEnqueueFramesForBackend(c.backend)
	player := c.player
	c.StopAndClear()
	factory := func() (*music.StreamRenderer, error) {
		return music.NewParsedMUSStreamRenderer(c.driver, parsed)
	}
	lookaheadFrames := music.DefaultStreamLookaheadForBackend(c.backend)
	_ = player.PlayStream(factory, loop, chunkFrames, enqueueFrames, lookaheadFrames)
}

func (c *Controller) playMUS(data []byte, loop bool) {
	if len(data) == 0 {
		return
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil || parsed == nil {
		return
	}
	c.playParsed(parsed, loop)
}

func (c *Controller) playMUSOnce(data []byte) {
	if c == nil || c.player == nil || c.driver == nil || len(data) == 0 {
		return
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil || parsed == nil {
		return
	}
	c.playParsedOnce(parsed)
}

func (c *Controller) playParsedOnce(parsed *music.ParsedMUS) {
	if c != nil && c.pcSpeaker != nil {
		if parsed == nil {
			return
		}
		seq, tickRate := music.RenderParsedMUSToPCSpeaker(c.patchBank, parsed)
		c.StopAndClear()
		if len(seq) == 0 {
			c.pcSpeaker.ClearMusic()
			return
		}
		c.pcSpeaker.SetMusic(seq, tickRate, false)
		return
	}
	if c == nil || c.player == nil || c.driver == nil || parsed == nil {
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
	if c == nil || c.player == nil {
		return
	}
	_ = c.player.StopStream()
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
