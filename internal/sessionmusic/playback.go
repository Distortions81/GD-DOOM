package sessionmusic

import (
	"gddoom/internal/audiofx"
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
)

type Playback struct {
	ctl                *Controller
	mapLoader          func(string) (*music.ParsedMUS, error)
	titleLoader        func() (*music.ParsedMUS, error)
	intermissionLoader func(commercial bool) (*music.ParsedMUS, error)
}

func NewPlayback(volume float64, musPanMax float64, oplVolume float64, preEmphasis bool, backend music.Backend, bank music.PatchBank, soundFont *music.SoundFontBank, pcSpeaker audiofx.PCSpeaker, mapLoader func(string) (*music.ParsedMUS, error), titleLoader func() (*music.ParsedMUS, error), intermissionLoader func(bool) (*music.ParsedMUS, error)) (*Playback, error) {
	ctl, err := New(volume, musPanMax, oplVolume, preEmphasis, backend, bank, soundFont, pcSpeaker)
	if err != nil {
		return nil, err
	}
	return &Playback{
		ctl:                ctl,
		mapLoader:          mapLoader,
		titleLoader:        titleLoader,
		intermissionLoader: intermissionLoader,
	}, nil
}

func (p *Playback) Close() {
	if p == nil || p.ctl == nil {
		return
	}
	p.ctl.Close()
	p.ctl = nil
}

func (p *Playback) StopAndClear() {
	if p == nil || p.ctl == nil {
		return
	}
	p.ctl.StopAndClear()
}

func (p *Playback) SetVolume(v float64) {
	if p == nil || p.ctl == nil {
		return
	}
	p.ctl.SetVolume(v)
}

func (p *Playback) SetOutputGain(v float64) {
	if p == nil || p.ctl == nil {
		return
	}
	p.ctl.SetOutputGain(v)
}

func (p *Playback) Tick() {
	if p == nil || p.ctl == nil {
		return
	}
	p.ctl.Tick()
}

func (p *Playback) PlayTitle(volume float64) {
	if p == nil || p.ctl == nil || p.titleLoader == nil || volume <= 0 {
		return
	}
	p.StopAndClear()
	parsed, err := p.titleLoader()
	if err != nil || parsed == nil {
		return
	}
	p.ctl.PlayParsedOnce(parsed)
}

func (p *Playback) PlayMap(name mapdata.MapName, volume float64) {
	if p == nil || p.ctl == nil || p.mapLoader == nil || volume <= 0 {
		return
	}
	p.StopAndClear()
	parsed, err := p.mapLoader(string(name))
	if err != nil || parsed == nil {
		return
	}
	p.ctl.PlayParsed(parsed)
}

func (p *Playback) PlayData(data []byte, volume float64) {
	if p == nil || p.ctl == nil || volume <= 0 || len(data) == 0 {
		return
	}
	p.StopAndClear()
	p.ctl.PlayMUS(data)
}

func (p *Playback) PlayParsed(parsed *music.ParsedMUS, volume float64) {
	if p == nil || p.ctl == nil || volume <= 0 || parsed == nil {
		return
	}
	p.StopAndClear()
	p.ctl.PlayParsed(parsed)
}

func (p *Playback) PlayIntermission(commercial bool, volume float64) {
	if p == nil || p.ctl == nil || p.intermissionLoader == nil || volume <= 0 {
		return
	}
	p.StopAndClear()
	parsed, err := p.intermissionLoader(commercial)
	if err != nil || parsed == nil {
		return
	}
	p.ctl.PlayParsed(parsed)
}
