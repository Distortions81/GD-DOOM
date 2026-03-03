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
	soundEventSwitchOn
	soundEventSwitchOff
	soundEventNoWay
	soundEventItemUp
	soundEventWeaponUp
	soundEventPowerUp
	soundEventOof
)

type soundSystem struct {
	ctx     *audio.Context
	bank    SoundBank
	players []*audio.Player
}

var (
	sharedAudioCtx  *audio.Context
	sharedAudioRate int
)

func newSoundSystem(bank SoundBank) *soundSystem {
	rate := firstSampleRate(bank)
	if rate <= 0 {
		return nil
	}
	ctx := sharedOrNewAudioContext(rate)
	if ctx == nil {
		// Keep runtime safe if sample rates differ across maps; no panic, just no sound.
		return nil
	}
	return &soundSystem{
		ctx:  ctx,
		bank: bank,
	}
}

func sharedOrNewAudioContext(rate int) *audio.Context {
	if sharedAudioCtx != nil {
		if sharedAudioRate == rate {
			return sharedAudioCtx
		}
		return nil
	}
	sharedAudioCtx = audio.NewContext(rate)
	sharedAudioRate = rate
	return sharedAudioCtx
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
	if bank.SwitchOn.SampleRate > 0 && len(bank.SwitchOn.Data) > 0 {
		return bank.SwitchOn.SampleRate
	}
	if bank.SwitchOff.SampleRate > 0 && len(bank.SwitchOff.Data) > 0 {
		return bank.SwitchOff.SampleRate
	}
	if bank.NoWay.SampleRate > 0 && len(bank.NoWay.Data) > 0 {
		return bank.NoWay.SampleRate
	}
	if bank.ItemUp.SampleRate > 0 && len(bank.ItemUp.Data) > 0 {
		return bank.ItemUp.SampleRate
	}
	if bank.WeaponUp.SampleRate > 0 && len(bank.WeaponUp.Data) > 0 {
		return bank.WeaponUp.SampleRate
	}
	if bank.PowerUp.SampleRate > 0 && len(bank.PowerUp.Data) > 0 {
		return bank.PowerUp.SampleRate
	}
	if bank.Oof.SampleRate > 0 && len(bank.Oof.Data) > 0 {
		return bank.Oof.SampleRate
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
	case soundEventSwitchOn:
		if len(s.bank.SwitchOn.Data) > 0 {
			return s.bank.SwitchOn, true
		}
		return s.bank.DoorOpen, true
	case soundEventSwitchOff:
		if len(s.bank.SwitchOff.Data) > 0 {
			return s.bank.SwitchOff, true
		}
		return s.bank.SwitchOn, true
	case soundEventNoWay:
		if len(s.bank.NoWay.Data) > 0 {
			return s.bank.NoWay, true
		}
		return s.bank.SwitchOff, true
	case soundEventItemUp:
		if len(s.bank.ItemUp.Data) > 0 {
			return s.bank.ItemUp, true
		}
		return s.bank.SwitchOn, true
	case soundEventWeaponUp:
		if len(s.bank.WeaponUp.Data) > 0 {
			return s.bank.WeaponUp, true
		}
		return s.bank.ItemUp, true
	case soundEventPowerUp:
		if len(s.bank.PowerUp.Data) > 0 {
			return s.bank.PowerUp, true
		}
		return s.bank.ItemUp, true
	case soundEventOof:
		if len(s.bank.Oof.Data) > 0 {
			return s.bank.Oof, true
		}
		return s.bank.NoWay, true
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
