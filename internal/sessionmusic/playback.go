package sessionmusic

import (
	"gddoom/internal/mapdata"
	"gddoom/internal/music"
)

type Playback struct {
	ctl         *Controller
	mapLoader   func(string) ([]byte, error)
	titleLoader func() ([]byte, error)
}

func NewPlayback(volume float64, musPanMax float64, oplVolume float64, bank music.PatchBank, mapLoader func(string) ([]byte, error), titleLoader func() ([]byte, error)) (*Playback, error) {
	ctl, err := New(volume, musPanMax, oplVolume, bank)
	if err != nil {
		return nil, err
	}
	return &Playback{
		ctl:         ctl,
		mapLoader:   mapLoader,
		titleLoader: titleLoader,
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

func (p *Playback) PlayTitle(volume float64) {
	if p == nil || p.ctl == nil || p.titleLoader == nil || volume <= 0 {
		return
	}
	p.StopAndClear()
	data, err := p.titleLoader()
	if err != nil || len(data) == 0 {
		return
	}
	p.ctl.PlayMUS(data)
}

func (p *Playback) PlayMap(name mapdata.MapName, volume float64) {
	if p == nil || p.ctl == nil || p.mapLoader == nil || volume <= 0 {
		return
	}
	p.StopAndClear()
	data, err := p.mapLoader(string(name))
	if err != nil || len(data) == 0 {
		return
	}
	p.ctl.PlayMUS(data)
}

func (p *Playback) PlayData(data []byte, volume float64) {
	if p == nil || p.ctl == nil || volume <= 0 || len(data) == 0 {
		return
	}
	p.StopAndClear()
	p.ctl.PlayMUS(data)
}
