package automap

import (
	"gddoom/internal/music"

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
	soundEventPain
	soundEventShootPistol
	soundEventShootShotgun
	soundEventShootFireball
	soundEventShootRocket
	soundEventImpactFire
	soundEventImpactRocket
	soundEventMonsterPainHumanoid
	soundEventMonsterPainDemon
	soundEventDeathZombie
	soundEventDeathShotgunGuy
	soundEventDeathImp
	soundEventDeathDemon
	soundEventDeathCaco
	soundEventDeathBaron
	soundEventDeathCyber
	soundEventDeathSpider
	soundEventDeathLostSoul
	soundEventMonsterDeath
	soundEventPlayerDeath
	soundEventIntermissionTick
	soundEventIntermissionDone
)

type soundSystem struct {
	ctx     *audio.Context
	bank    SoundBank
	volume  float64
	players []*audio.Player
}

var (
	sharedAudioCtx  *audio.Context
	sharedAudioRate int
)

func newSoundSystem(bank SoundBank, sfxVolume float64) *soundSystem {
	rate := music.OutputSampleRate
	if rate <= 0 {
		return nil
	}
	ctx := sharedOrNewAudioContext(rate)
	if ctx == nil {
		// Keep runtime safe if sample rates differ across maps; no panic, just no sound.
		return nil
	}
	return &soundSystem{
		ctx:    ctx,
		bank:   bank,
		volume: clampVolume(sfxVolume),
	}
}

func (s *soundSystem) setSFXVolume(v float64) {
	if s == nil {
		return
	}
	s.volume = clampVolume(v)
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
	if bank.Pain.SampleRate > 0 && len(bank.Pain.Data) > 0 {
		return bank.Pain.SampleRate
	}
	if bank.ShootPistol.SampleRate > 0 && len(bank.ShootPistol.Data) > 0 {
		return bank.ShootPistol.SampleRate
	}
	if bank.ShootShotgun.SampleRate > 0 && len(bank.ShootShotgun.Data) > 0 {
		return bank.ShootShotgun.SampleRate
	}
	if bank.ShootFireball.SampleRate > 0 && len(bank.ShootFireball.Data) > 0 {
		return bank.ShootFireball.SampleRate
	}
	if bank.ShootRocket.SampleRate > 0 && len(bank.ShootRocket.Data) > 0 {
		return bank.ShootRocket.SampleRate
	}
	if bank.ImpactFire.SampleRate > 0 && len(bank.ImpactFire.Data) > 0 {
		return bank.ImpactFire.SampleRate
	}
	if bank.ImpactRocket.SampleRate > 0 && len(bank.ImpactRocket.Data) > 0 {
		return bank.ImpactRocket.SampleRate
	}
	if bank.MonsterPainHumanoid.SampleRate > 0 && len(bank.MonsterPainHumanoid.Data) > 0 {
		return bank.MonsterPainHumanoid.SampleRate
	}
	if bank.MonsterPainDemon.SampleRate > 0 && len(bank.MonsterPainDemon.Data) > 0 {
		return bank.MonsterPainDemon.SampleRate
	}
	if bank.DeathZombie.SampleRate > 0 && len(bank.DeathZombie.Data) > 0 {
		return bank.DeathZombie.SampleRate
	}
	if bank.DeathShotgunGuy.SampleRate > 0 && len(bank.DeathShotgunGuy.Data) > 0 {
		return bank.DeathShotgunGuy.SampleRate
	}
	if bank.DeathImp.SampleRate > 0 && len(bank.DeathImp.Data) > 0 {
		return bank.DeathImp.SampleRate
	}
	if bank.DeathDemon.SampleRate > 0 && len(bank.DeathDemon.Data) > 0 {
		return bank.DeathDemon.SampleRate
	}
	if bank.DeathCaco.SampleRate > 0 && len(bank.DeathCaco.Data) > 0 {
		return bank.DeathCaco.SampleRate
	}
	if bank.DeathBaron.SampleRate > 0 && len(bank.DeathBaron.Data) > 0 {
		return bank.DeathBaron.SampleRate
	}
	if bank.DeathCyber.SampleRate > 0 && len(bank.DeathCyber.Data) > 0 {
		return bank.DeathCyber.SampleRate
	}
	if bank.DeathSpider.SampleRate > 0 && len(bank.DeathSpider.Data) > 0 {
		return bank.DeathSpider.SampleRate
	}
	if bank.DeathLostSoul.SampleRate > 0 && len(bank.DeathLostSoul.Data) > 0 {
		return bank.DeathLostSoul.SampleRate
	}
	if bank.MonsterDeath.SampleRate > 0 && len(bank.MonsterDeath.Data) > 0 {
		return bank.MonsterDeath.SampleRate
	}
	if bank.PlayerDeath.SampleRate > 0 && len(bank.PlayerDeath.Data) > 0 {
		return bank.PlayerDeath.SampleRate
	}
	if bank.InterTick.SampleRate > 0 && len(bank.InterTick.Data) > 0 {
		return bank.InterTick.SampleRate
	}
	if bank.InterDone.SampleRate > 0 && len(bank.InterDone.Data) > 0 {
		return bank.InterDone.SampleRate
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
	pcm := pcmMonoU8ToStereoS16LE(sample.Data)
	if sample.SampleRate != s.ctx.SampleRate() {
		pcm = pcmMonoU8ToStereoS16LEResampled(sample.Data, sample.SampleRate, s.ctx.SampleRate())
	}
	if len(pcm) == 0 {
		return
	}
	p := audio.NewPlayerFromBytes(s.ctx, pcm)
	p.SetVolume(s.volume)
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
	case soundEventPain:
		if len(s.bank.Pain.Data) > 0 {
			return s.bank.Pain, true
		}
		if len(s.bank.Oof.Data) > 0 {
			return s.bank.Oof, true
		}
		return s.bank.NoWay, true
	case soundEventShootPistol:
		if len(s.bank.ShootPistol.Data) > 0 {
			return s.bank.ShootPistol, true
		}
		return s.bank.SwitchOn, true
	case soundEventShootShotgun:
		if len(s.bank.ShootShotgun.Data) > 0 {
			return s.bank.ShootShotgun, true
		}
		if len(s.bank.ShootPistol.Data) > 0 {
			return s.bank.ShootPistol, true
		}
		return s.bank.SwitchOn, true
	case soundEventShootFireball:
		if len(s.bank.ShootFireball.Data) > 0 {
			return s.bank.ShootFireball, true
		}
		if len(s.bank.ShootPistol.Data) > 0 {
			return s.bank.ShootPistol, true
		}
		return s.bank.SwitchOn, true
	case soundEventShootRocket:
		if len(s.bank.ShootRocket.Data) > 0 {
			return s.bank.ShootRocket, true
		}
		if len(s.bank.ShootShotgun.Data) > 0 {
			return s.bank.ShootShotgun, true
		}
		return s.bank.SwitchOn, true
	case soundEventImpactFire:
		if len(s.bank.ImpactFire.Data) > 0 {
			return s.bank.ImpactFire, true
		}
		if len(s.bank.ShootFireball.Data) > 0 {
			return s.bank.ShootFireball, true
		}
		return s.bank.SwitchOn, true
	case soundEventImpactRocket:
		if len(s.bank.ImpactRocket.Data) > 0 {
			return s.bank.ImpactRocket, true
		}
		if len(s.bank.ImpactFire.Data) > 0 {
			return s.bank.ImpactFire, true
		}
		return s.bank.SwitchOn, true
	case soundEventMonsterPainHumanoid:
		if len(s.bank.MonsterPainHumanoid.Data) > 0 {
			return s.bank.MonsterPainHumanoid, true
		}
		if len(s.bank.Pain.Data) > 0 {
			return s.bank.Pain, true
		}
		if len(s.bank.Oof.Data) > 0 {
			return s.bank.Oof, true
		}
		return s.bank.NoWay, true
	case soundEventMonsterPainDemon:
		if len(s.bank.MonsterPainDemon.Data) > 0 {
			return s.bank.MonsterPainDemon, true
		}
		if len(s.bank.MonsterPainHumanoid.Data) > 0 {
			return s.bank.MonsterPainHumanoid, true
		}
		if len(s.bank.Pain.Data) > 0 {
			return s.bank.Pain, true
		}
		if len(s.bank.Oof.Data) > 0 {
			return s.bank.Oof, true
		}
		return s.bank.NoWay, true
	case soundEventDeathZombie:
		if len(s.bank.DeathZombie.Data) > 0 {
			return s.bank.DeathZombie, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathShotgunGuy:
		if len(s.bank.DeathShotgunGuy.Data) > 0 {
			return s.bank.DeathShotgunGuy, true
		}
		return s.sampleForEvent(soundEventDeathZombie)
	case soundEventDeathImp:
		if len(s.bank.DeathImp.Data) > 0 {
			return s.bank.DeathImp, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathDemon:
		if len(s.bank.DeathDemon.Data) > 0 {
			return s.bank.DeathDemon, true
		}
		return s.sampleForEvent(soundEventDeathImp)
	case soundEventDeathCaco:
		if len(s.bank.DeathCaco.Data) > 0 {
			return s.bank.DeathCaco, true
		}
		return s.sampleForEvent(soundEventDeathDemon)
	case soundEventDeathBaron:
		if len(s.bank.DeathBaron.Data) > 0 {
			return s.bank.DeathBaron, true
		}
		return s.sampleForEvent(soundEventDeathDemon)
	case soundEventDeathCyber:
		if len(s.bank.DeathCyber.Data) > 0 {
			return s.bank.DeathCyber, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathSpider:
		if len(s.bank.DeathSpider.Data) > 0 {
			return s.bank.DeathSpider, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathLostSoul:
		if len(s.bank.DeathLostSoul.Data) > 0 {
			return s.bank.DeathLostSoul, true
		}
		return s.sampleForEvent(soundEventImpactFire)
	case soundEventMonsterDeath:
		if len(s.bank.MonsterDeath.Data) > 0 {
			return s.bank.MonsterDeath, true
		}
		if len(s.bank.ImpactFire.Data) > 0 {
			return s.bank.ImpactFire, true
		}
		return s.bank.SwitchOn, true
	case soundEventPlayerDeath:
		if len(s.bank.PlayerDeath.Data) > 0 {
			return s.bank.PlayerDeath, true
		}
		if len(s.bank.Pain.Data) > 0 {
			return s.bank.Pain, true
		}
		return s.bank.NoWay, true
	case soundEventIntermissionTick:
		if len(s.bank.InterTick.Data) > 0 {
			return s.bank.InterTick, true
		}
		return s.bank.SwitchOn, true
	case soundEventIntermissionDone:
		if len(s.bank.InterDone.Data) > 0 {
			return s.bank.InterDone, true
		}
		return s.bank.PowerUp, true
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

func (s *soundSystem) stopAll() {
	if s == nil || len(s.players) == 0 {
		return
	}
	for _, p := range s.players {
		if p == nil {
			continue
		}
		p.Pause()
		_ = p.Close()
	}
	s.players = s.players[:0]
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

func pcmMonoU8ToStereoS16LEResampled(src []byte, srcRate, dstRate int) []byte {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		return pcmMonoU8ToStereoS16LE(src)
	}
	// Keep runtime cheap; nearest-neighbor is sufficient for short SFX.
	dstLen := len(src) * dstRate / srcRate
	if dstLen <= 0 {
		dstLen = 1
	}
	out := make([]byte, dstLen*4)
	for i := 0; i < dstLen; i++ {
		si := i * srcRate / dstRate
		if si >= len(src) {
			si = len(src) - 1
		}
		v := int16(int(src[si])-128) << 8
		oi := i * 4
		lo := byte(v)
		hi := byte(v >> 8)
		out[oi] = lo
		out[oi+1] = hi
		out[oi+2] = lo
		out[oi+3] = hi
	}
	return out
}
