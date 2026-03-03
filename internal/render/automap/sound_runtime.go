package automap

import (
	"github.com/hajimehoshi/ebiten/v2/audio"
)

type soundEvent int

const (
	soundEventDoorOpen soundEvent = iota
	soundEventDoorClose
	soundEventBlazeOpen
	soundEventBlazeClose
)

type soundSystem struct {
	ctx     *audio.Context
	bank    SoundBank
	players []*audio.Player
}

func newSoundSystem(bank SoundBank) *soundSystem {
	rate := firstSampleRate(bank)
	if rate <= 0 {
		return nil
	}
	return &soundSystem{
		ctx:  audio.NewContext(rate),
		bank: bank,
	}
}

func firstSampleRate(bank SoundBank) int {
	if bank.DoorOpen.SampleRate > 0 && len(bank.DoorOpen.Data) > 0 {
		return bank.DoorOpen.SampleRate
	}
	if bank.DoorClose.SampleRate > 0 && len(bank.DoorClose.Data) > 0 {
		return bank.DoorClose.SampleRate
	}
	if bank.BlazeOpen.SampleRate > 0 && len(bank.BlazeOpen.Data) > 0 {
		return bank.BlazeOpen.SampleRate
	}
	if bank.BlazeClose.SampleRate > 0 && len(bank.BlazeClose.Data) > 0 {
		return bank.BlazeClose.SampleRate
	}
	return 0
}

func (s *soundSystem) playEvent(ev soundEvent) {
	if s == nil || s.ctx == nil {
		return
	}
	sample, ok := s.sampleForEvent(ev)
	if !ok || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	if sample.SampleRate != s.ctx.SampleRate() {
		// Keep runtime simple for now: single-rate context.
		return
	}
	pcm := pcmMonoU8ToStereoS16LE(sample.Data)
	p := audio.NewPlayerFromBytes(s.ctx, pcm)
	p.SetVolume(0.65)
	p.Play()
	s.players = append(s.players, p)
}

func (s *soundSystem) sampleForEvent(ev soundEvent) (PCMSample, bool) {
	switch ev {
	case soundEventDoorOpen:
		return s.bank.DoorOpen, true
	case soundEventDoorClose:
		return s.bank.DoorClose, true
	case soundEventBlazeOpen:
		if len(s.bank.BlazeOpen.Data) > 0 {
			return s.bank.BlazeOpen, true
		}
		return s.bank.DoorOpen, true
	case soundEventBlazeClose:
		if len(s.bank.BlazeClose.Data) > 0 {
			return s.bank.BlazeClose, true
		}
		return s.bank.DoorClose, true
	default:
		return PCMSample{}, false
	}
}

func (s *soundSystem) tick() {
	if s == nil || len(s.players) == 0 {
		return
	}
	keep := s.players[:0]
	for _, p := range s.players {
		if p.IsPlaying() {
			keep = append(keep, p)
			continue
		}
		_ = p.Close()
	}
	s.players = keep
}

func pcmMonoU8ToStereoS16LE(src []byte) []byte {
	out := make([]byte, len(src)*4)
	oi := 0
	for _, u := range src {
		v := int16(int(u)-128) << 8
		lo := byte(v)
		hi := byte(v >> 8)
		// left
		out[oi] = lo
		out[oi+1] = hi
		// right
		out[oi+2] = lo
		out[oi+3] = hi
		oi += 4
	}
	return out
}
