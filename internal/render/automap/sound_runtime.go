package automap

import (
	"math"
	"reflect"

	"gddoom/internal/doomrand"
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
	soundEventSwitchExit
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
	soundEventMonsterAttackClaw
	soundEventMonsterAttackSgt
	soundEventMonsterAttackSkull
	soundEventImpactFire
	soundEventImpactRocket
	soundEventMonsterSeePosit
	soundEventMonsterSeeImp
	soundEventMonsterSeeDemon
	soundEventMonsterSeeCaco
	soundEventMonsterSeeBaron
	soundEventMonsterSeeKnight
	soundEventMonsterSeeSpider
	soundEventMonsterSeeArachnotron
	soundEventMonsterSeeCyber
	soundEventMonsterSeePainElemental
	soundEventMonsterSeeWolfSS
	soundEventMonsterSeeArchvile
	soundEventMonsterSeeRevenant
	soundEventMonsterActivePosit
	soundEventMonsterActiveImp
	soundEventMonsterActiveDemon
	soundEventMonsterActiveArachnotron
	soundEventMonsterActiveArchvile
	soundEventMonsterActiveRevenant
	soundEventMonsterPainHumanoid
	soundEventMonsterPainDemon
	soundEventDeathZombie
	soundEventDeathShotgunGuy
	soundEventDeathChaingunner
	soundEventDeathImp
	soundEventDeathDemon
	soundEventDeathCaco
	soundEventDeathBaron
	soundEventDeathKnight
	soundEventDeathCyber
	soundEventDeathSpider
	soundEventDeathArachnotron
	soundEventDeathLostSoul
	soundEventDeathMancubus
	soundEventDeathRevenant
	soundEventDeathPainElemental
	soundEventDeathWolfSS
	soundEventDeathArchvile
	soundEventMonsterDeath
	soundEventPlayerDeath
	soundEventIntermissionTick
	soundEventIntermissionDone
)

type soundSystem struct {
	ctx        *audio.Context
	bank       SoundBank
	volume     float64
	players    []*audio.Player
	sourcePort bool
}

type MenuSoundPlayer struct {
	ctx     *audio.Context
	volume  float64
	move    PCMSample
	confirm PCMSample
	back    PCMSample
	quit1   []PCMSample
	quit2   []PCMSample
	players []*audio.Player
}

const (
	doomSoundMaxVolume    = 127
	doomSoundClippingDist = int64(1200 * fracUnit)
	doomSoundCloseDist    = int64(160 * fracUnit)
	doomSoundStereoSwing  = int64(96 * fracUnit)
	doomSoundAttenuator   = (doomSoundClippingDist - doomSoundCloseDist) / fracUnit
	doomSoundNormalSep    = 128
	doomSoundSepRange     = 256
)

var (
	sharedAudioCtx  *audio.Context
	sharedAudioRate int
)

func EnsureSharedAudioContext() *audio.Context {
	rate := music.OutputSampleRate
	if rate <= 0 {
		return nil
	}
	return sharedOrNewAudioContext(rate)
}

func NewMenuSoundPlayer(bank SoundBank, volume float64) *MenuSoundPlayer {
	ctx := EnsureSharedAudioContext()
	if ctx == nil {
		return nil
	}
	return &MenuSoundPlayer{
		ctx:     ctx,
		volume:  clampVolume(volume),
		move:    firstMenuSample(bank.MenuCursor, bank.SwitchOn),
		confirm: firstMenuSample(bank.ShootPistol, bank.SwitchOn),
		back:    firstMenuSample(bank.SwitchOff, bank.NoWay),
		quit1: []PCMSample{
			firstMenuSample(bank.PlayerDeath, bank.MonsterDeath),
			firstMenuSample(bank.MonsterPainDemon, bank.MonsterPainHumanoid),
			firstMenuSample(bank.MonsterPainHumanoid, bank.Pain),
			firstMenuSample(bank.ImpactRocket, bank.Oof),
			firstMenuSample(bank.PowerUp, bank.SwitchOn),
			firstMenuSample(bank.SeePosit1, bank.SeePosit2),
			firstMenuSample(bank.SeePosit3, bank.SeePosit1),
			firstMenuSample(bank.AttackSgt, bank.ShootShotgun),
		},
		quit2: []PCMSample{
			firstMenuSample(bank.ActiveVilAct, bank.SeeVileSit),
			firstMenuSample(bank.PowerUp, bank.ItemUp),
			firstMenuSample(bank.SeeCyberSit, bank.SeeBruiserSit),
			firstMenuSample(bank.ImpactRocket, bank.Oof),
			firstMenuSample(bank.AttackClaw, bank.AttackSkull),
			firstMenuSample(bank.DeathKnight, bank.DeathBaron),
			firstMenuSample(bank.ActiveBSPAct, bank.ActiveDMAct),
			firstMenuSample(bank.AttackSgt, bank.ShootShotgun),
		},
		players: make([]*audio.Player, 0, 8),
	}
}

func firstMenuSample(samples ...PCMSample) PCMSample {
	for _, sample := range samples {
		if sample.SampleRate > 0 && len(sample.Data) > 0 {
			return sample
		}
	}
	return PCMSample{}
}

func (p *MenuSoundPlayer) PlayMove() {
	if p != nil {
		p.playSample(p.move)
	}
}

func (p *MenuSoundPlayer) PlayConfirm() {
	if p != nil {
		p.playSample(p.confirm)
	}
}

func (p *MenuSoundPlayer) PlayBack() {
	if p != nil {
		p.playSample(p.back)
	}
}

func (p *MenuSoundPlayer) PlayQuit(commercial bool, seq int) {
	if p == nil {
		return
	}
	set := p.quit1
	if commercial {
		set = p.quit2
	}
	if len(set) == 0 {
		return
	}
	if seq < 0 {
		seq = 0
	}
	p.playSample(set[seq%len(set)])
}

func (p *MenuSoundPlayer) StopAll() {
	if p == nil {
		return
	}
	for _, player := range p.players {
		if player == nil {
			continue
		}
		player.Pause()
		_ = player.Close()
	}
	p.players = p.players[:0]
}

func (p *MenuSoundPlayer) playSample(sample PCMSample) {
	if p == nil || p.ctx == nil || p.volume <= 0 || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	var pcm []byte
	if sample.SampleRate == p.ctx.SampleRate() {
		pcm = pcmMonoU8ToStereoS16LE(sample.Data)
	} else {
		pcm = pcmMonoU8ToStereoS16LEResampled(sample.Data, sample.SampleRate, p.ctx.SampleRate())
	}
	if len(pcm) == 0 {
		return
	}
	player := audio.NewPlayerFromBytes(p.ctx, pcm)
	player.SetVolume(p.volume)
	player.Play()
	p.players = append(p.players, player)
}

func newSoundSystem(bank SoundBank, sfxVolume float64, sourcePort bool) *soundSystem {
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
		ctx:        ctx,
		bank:       bank,
		volume:     clampVolume(sfxVolume),
		sourcePort: sourcePort,
	}
}

func PrepareSoundBankForSourcePort(bank SoundBank, dstRate int) SoundBank {
	if dstRate <= 0 {
		return bank
	}
	rv := reflect.ValueOf(&bank).Elem()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		sample, ok := field.Addr().Interface().(*PCMSample)
		if !ok || sample == nil {
			continue
		}
		prepareSampleForSourcePort(sample, dstRate)
	}
	return bank
}

func prepareSampleForSourcePort(sample *PCMSample, dstRate int) {
	if sample == nil || len(sample.Data) == 0 || sample.SampleRate <= 0 || dstRate <= 0 {
		return
	}
	mono := pcmMonoU8ToMonoS16(sample.Data)
	if sample.SampleRate != dstRate {
		mono = resampleMonoS16Polyphase(mono, sample.SampleRate, dstRate)
	}
	mono = applySourcePortPresenceBoost(mono)
	sample.PreparedRate = dstRate
	sample.PreparedMono = mono
}

func applySourcePortPresenceBoost(src []int16) []int16 {
	if len(src) == 0 {
		return nil
	}
	out := make([]int16, len(src))
	out[0] = src[0]
	prevHP1 := 0.0
	prevHP2 := 0.0
	prevX := float64(src[0])
	const hpAlpha1 = 0.54
	const hpAlpha2 = 0.36
	const boost1 = 1.15
	const boost2 = 0.65
	for i := 1; i < len(src); i++ {
		x := float64(src[i])
		hp1 := hpAlpha1 * (prevHP1 + x - prevX)
		hp2 := hpAlpha2 * (prevHP2 + hp1 - prevHP1)
		y := x + boost1*hp1 + boost2*hp2
		out[i] = int16(clampFloat(math.Round(y), -32768, 32767))
		prevHP1 = hp1
		prevHP2 = hp2
		prevX = x
	}
	return out
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
	if bank.AttackClaw.SampleRate > 0 && len(bank.AttackClaw.Data) > 0 {
		return bank.AttackClaw.SampleRate
	}
	if bank.AttackSgt.SampleRate > 0 && len(bank.AttackSgt.Data) > 0 {
		return bank.AttackSgt.SampleRate
	}
	if bank.AttackSkull.SampleRate > 0 && len(bank.AttackSkull.Data) > 0 {
		return bank.AttackSkull.SampleRate
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
	s.playEventSpatial(ev, queuedSoundOrigin{}, 0, 0, 0, false)
}

func (s *soundSystem) playEventSpatial(ev soundEvent, origin queuedSoundOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) {
	if s == nil || s.ctx == nil || s.volume <= 0 {
		return
	}
	sample, ok := s.sampleForEvent(ev)
	if !ok || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	leftGain, rightGain := s.eventStereoGains(origin, listenerX, listenerY, listenerAngle, mapUsesFullClip)
	var pcm []byte
	if s.sourcePort && sample.PreparedRate == s.ctx.SampleRate() && len(sample.PreparedMono) > 0 {
		pcm = pcmMonoS16ToStereoS16LESpatial(sample.PreparedMono, leftGain, rightGain)
	} else if sample.SampleRate != s.ctx.SampleRate() {
		pcm = pcmMonoU8ToStereoS16LESpatialResampled(sample.Data, sample.SampleRate, s.ctx.SampleRate(), leftGain, rightGain)
	} else {
		pcm = pcmMonoU8ToStereoS16LESpatial(sample.Data, leftGain, rightGain)
	}
	if len(pcm) == 0 {
		return
	}
	p := audio.NewPlayerFromBytes(s.ctx, pcm)
	p.SetVolume(1)
	p.Play()
	s.players = append(s.players, p)
}

func (s *soundSystem) eventStereoGains(origin queuedSoundOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) (float64, float64) {
	baseVol := int(math.Round(s.volume * doomSoundMaxVolume))
	if baseVol < 0 {
		baseVol = 0
	}
	if baseVol > doomSoundMaxVolume {
		baseVol = doomSoundMaxVolume
	}
	if !origin.positioned {
		gain := float64(baseVol) / doomSoundMaxVolume
		return gain, gain
	}
	vol, sep, ok := doomAdjustSoundParams(listenerX, listenerY, listenerAngle, origin.x, origin.y, baseVol, mapUsesFullClip)
	if !ok || vol <= 0 {
		return 0, 0
	}
	left, right := doomSeparationVolumes(vol, sep)
	return float64(left) / doomSoundMaxVolume, float64(right) / doomSoundMaxVolume
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
	case soundEventSwitchExit:
		if len(s.bank.SwitchOff.Data) > 0 {
			return s.bank.SwitchOff, true
		}
		return s.bank.SwitchOn, true
	case soundEventSwitchOff:
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
	case soundEventMonsterAttackClaw:
		if len(s.bank.AttackClaw.Data) > 0 {
			return s.bank.AttackClaw, true
		}
		return s.sampleForEvent(soundEventShootFireball)
	case soundEventMonsterAttackSgt:
		if len(s.bank.AttackSgt.Data) > 0 {
			return s.bank.AttackSgt, true
		}
		return s.sampleForEvent(soundEventShootShotgun)
	case soundEventMonsterAttackSkull:
		if len(s.bank.AttackSkull.Data) > 0 {
			return s.bank.AttackSkull, true
		}
		return s.sampleForEvent(soundEventShootFireball)
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
	case soundEventMonsterSeePosit:
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%3,
			s.bank.SeePosit1,
			s.bank.SeePosit2,
			s.bank.SeePosit3,
		); ok {
			return sample, true
		}
		return s.sampleForEvent(soundEventMonsterActivePosit)
	case soundEventMonsterSeeImp:
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%2,
			s.bank.SeeBGSit1,
			s.bank.SeeBGSit2,
		); ok {
			return sample, true
		}
		return s.sampleForEvent(soundEventMonsterActiveImp)
	case soundEventMonsterSeeDemon:
		if len(s.bank.SeeSgtSit.Data) > 0 {
			return s.bank.SeeSgtSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeeCaco:
		if len(s.bank.SeeCacoSit.Data) > 0 {
			return s.bank.SeeCacoSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeeBaron:
		if len(s.bank.SeeBruiserSit.Data) > 0 {
			return s.bank.SeeBruiserSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeeKnight:
		if len(s.bank.SeeKnightSit.Data) > 0 {
			return s.bank.SeeKnightSit, true
		}
		return s.sampleForEvent(soundEventMonsterSeeBaron)
	case soundEventMonsterSeeSpider:
		if len(s.bank.SeeSpiderSit.Data) > 0 {
			return s.bank.SeeSpiderSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeeArachnotron:
		if len(s.bank.SeeBabySit.Data) > 0 {
			return s.bank.SeeBabySit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveArachnotron)
	case soundEventMonsterSeeCyber:
		if len(s.bank.SeeCyberSit.Data) > 0 {
			return s.bank.SeeCyberSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeePainElemental:
		if len(s.bank.SeePainSit.Data) > 0 {
			return s.bank.SeePainSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterSeeWolfSS:
		if len(s.bank.SeeSSSit.Data) > 0 {
			return s.bank.SeeSSSit, true
		}
		return s.sampleForEvent(soundEventMonsterSeePosit)
	case soundEventMonsterSeeArchvile:
		if len(s.bank.SeeVileSit.Data) > 0 {
			return s.bank.SeeVileSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveArchvile)
	case soundEventMonsterSeeRevenant:
		if len(s.bank.SeeSkeSit.Data) > 0 {
			return s.bank.SeeSkeSit, true
		}
		return s.sampleForEvent(soundEventMonsterActiveRevenant)
	case soundEventMonsterActivePosit:
		if len(s.bank.ActivePosAct.Data) > 0 {
			return s.bank.ActivePosAct, true
		}
		return s.sampleForEvent(soundEventMonsterSeePosit)
	case soundEventMonsterActiveImp:
		if len(s.bank.ActiveBGAct.Data) > 0 {
			return s.bank.ActiveBGAct, true
		}
		return s.sampleForEvent(soundEventMonsterSeeImp)
	case soundEventMonsterActiveDemon:
		if len(s.bank.ActiveDMAct.Data) > 0 {
			return s.bank.ActiveDMAct, true
		}
		return s.sampleForEvent(soundEventMonsterSeeDemon)
	case soundEventMonsterActiveArachnotron:
		if len(s.bank.ActiveBSPAct.Data) > 0 {
			return s.bank.ActiveBSPAct, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterActiveArchvile:
		if len(s.bank.ActiveVilAct.Data) > 0 {
			return s.bank.ActiveVilAct, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
	case soundEventMonsterActiveRevenant:
		if len(s.bank.ActiveSkeAct.Data) > 0 {
			return s.bank.ActiveSkeAct, true
		}
		return s.sampleForEvent(soundEventMonsterActiveDemon)
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
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%3,
			s.bank.DeathPodth1,
			s.bank.DeathPodth2,
			s.bank.DeathPodth3,
		); ok {
			return sample, true
		}
		if len(s.bank.DeathZombie.Data) > 0 {
			return s.bank.DeathZombie, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathShotgunGuy:
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%3,
			s.bank.DeathPodth1,
			s.bank.DeathPodth2,
			s.bank.DeathPodth3,
		); ok {
			return sample, true
		}
		if len(s.bank.DeathShotgunGuy.Data) > 0 {
			return s.bank.DeathShotgunGuy, true
		}
		return s.sampleForEvent(soundEventDeathZombie)
	case soundEventDeathChaingunner:
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%3,
			s.bank.DeathPodth1,
			s.bank.DeathPodth2,
			s.bank.DeathPodth3,
		); ok {
			return sample, true
		}
		if len(s.bank.DeathChaingunner.Data) > 0 {
			return s.bank.DeathChaingunner, true
		}
		return s.sampleForEvent(soundEventDeathZombie)
	case soundEventDeathImp:
		if sample, ok := pickFirstAvailable(doomrand.PRandom()%2,
			s.bank.DeathBgdth1,
			s.bank.DeathBgdth2,
		); ok {
			return sample, true
		}
		if len(s.bank.DeathImp.Data) > 0 {
			return s.bank.DeathImp, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathDemon:
		if len(s.bank.DeathSgtDth.Data) > 0 {
			return s.bank.DeathSgtDth, true
		}
		if len(s.bank.DeathDemon.Data) > 0 {
			return s.bank.DeathDemon, true
		}
		return s.sampleForEvent(soundEventDeathImp)
	case soundEventDeathCaco:
		if len(s.bank.DeathCacoRaw.Data) > 0 {
			return s.bank.DeathCacoRaw, true
		}
		if len(s.bank.DeathCaco.Data) > 0 {
			return s.bank.DeathCaco, true
		}
		return s.sampleForEvent(soundEventDeathDemon)
	case soundEventDeathBaron:
		if len(s.bank.DeathBaronRaw.Data) > 0 {
			return s.bank.DeathBaronRaw, true
		}
		if len(s.bank.DeathBaron.Data) > 0 {
			return s.bank.DeathBaron, true
		}
		return s.sampleForEvent(soundEventDeathDemon)
	case soundEventDeathKnight:
		if len(s.bank.DeathKnightRaw.Data) > 0 {
			return s.bank.DeathKnightRaw, true
		}
		if len(s.bank.DeathKnight.Data) > 0 {
			return s.bank.DeathKnight, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathCyber:
		if len(s.bank.DeathCyberRaw.Data) > 0 {
			return s.bank.DeathCyberRaw, true
		}
		if len(s.bank.DeathCyber.Data) > 0 {
			return s.bank.DeathCyber, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathSpider:
		if len(s.bank.DeathSpiderRaw.Data) > 0 {
			return s.bank.DeathSpiderRaw, true
		}
		if len(s.bank.DeathSpider.Data) > 0 {
			return s.bank.DeathSpider, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathArachnotron:
		if len(s.bank.DeathArachRaw.Data) > 0 {
			return s.bank.DeathArachRaw, true
		}
		if len(s.bank.DeathArachnotron.Data) > 0 {
			return s.bank.DeathArachnotron, true
		}
		return s.sampleForEvent(soundEventDeathSpider)
	case soundEventDeathLostSoul:
		if len(s.bank.DeathLostSoulRaw.Data) > 0 {
			return s.bank.DeathLostSoulRaw, true
		}
		if len(s.bank.DeathLostSoul.Data) > 0 {
			return s.bank.DeathLostSoul, true
		}
		return s.sampleForEvent(soundEventImpactFire)
	case soundEventDeathMancubus:
		if len(s.bank.DeathMancubusRaw.Data) > 0 {
			return s.bank.DeathMancubusRaw, true
		}
		if len(s.bank.DeathMancubus.Data) > 0 {
			return s.bank.DeathMancubus, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
	case soundEventDeathRevenant:
		if len(s.bank.DeathRevenantRaw.Data) > 0 {
			return s.bank.DeathRevenantRaw, true
		}
		if len(s.bank.DeathRevenant.Data) > 0 {
			return s.bank.DeathRevenant, true
		}
		return s.sampleForEvent(soundEventDeathDemon)
	case soundEventDeathPainElemental:
		if len(s.bank.DeathPainElemRaw.Data) > 0 {
			return s.bank.DeathPainElemRaw, true
		}
		if len(s.bank.DeathPainElemental.Data) > 0 {
			return s.bank.DeathPainElemental, true
		}
		return s.sampleForEvent(soundEventDeathCaco)
	case soundEventDeathWolfSS:
		if len(s.bank.DeathWolfSSRaw.Data) > 0 {
			return s.bank.DeathWolfSSRaw, true
		}
		if len(s.bank.DeathWolfSS.Data) > 0 {
			return s.bank.DeathWolfSS, true
		}
		return s.sampleForEvent(soundEventDeathZombie)
	case soundEventDeathArchvile:
		if len(s.bank.DeathArchvileRaw.Data) > 0 {
			return s.bank.DeathArchvileRaw, true
		}
		if len(s.bank.DeathArchvile.Data) > 0 {
			return s.bank.DeathArchvile, true
		}
		return s.sampleForEvent(soundEventDeathBaron)
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

func pickFirstAvailable(start int, samples ...PCMSample) (PCMSample, bool) {
	if len(samples) == 0 {
		return PCMSample{}, false
	}
	if start < 0 {
		start = 0
	}
	start %= len(samples)
	for i := 0; i < len(samples); i++ {
		s := samples[(start+i)%len(samples)]
		if len(s.Data) > 0 && s.SampleRate > 0 {
			return s, true
		}
	}
	return PCMSample{}, false
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
	return pcmMonoU8ToStereoS16LESpatial(src, 1, 1)
}

func pcmMonoU8ToMonoS16(src []byte) []int16 {
	if len(src) == 0 {
		return nil
	}
	out := make([]int16, len(src))
	for i, u := range src {
		out[i] = (int16(u) - 128) << 8
	}
	return out
}

func pcmMonoS16ToStereoS16LESpatial(src []int16, leftGain, rightGain float64) []byte {
	out := make([]byte, len(src)*4)
	oi := 0
	for _, base := range src {
		left := int16(clampFloat(float64(base)*leftGain, -32768, 32767))
		right := int16(clampFloat(float64(base)*rightGain, -32768, 32767))
		out[oi] = byte(left)
		out[oi+1] = byte(left >> 8)
		out[oi+2] = byte(right)
		out[oi+3] = byte(right >> 8)
		oi += 4
	}
	return out
}

func pcmMonoU8ToStereoS16LESpatial(src []byte, leftGain, rightGain float64) []byte {
	return pcmMonoS16ToStereoS16LESpatial(pcmMonoU8ToMonoS16(src), leftGain, rightGain)
}

func pcmMonoU8ToStereoS16LEResampled(src []byte, srcRate, dstRate int) []byte {
	return pcmMonoU8ToStereoS16LESpatialResampled(src, srcRate, dstRate, 1, 1)
}

func pcmMonoU8ToStereoS16LESpatialResampled(src []byte, srcRate, dstRate int, leftGain, rightGain float64) []byte {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		return pcmMonoU8ToStereoS16LESpatial(src, leftGain, rightGain)
	}
	return pcmMonoS16ToStereoS16LESpatial(resampleMonoS16Polyphase(pcmMonoU8ToMonoS16(src), srcRate, dstRate), leftGain, rightGain)
}

func resampleMonoS16Polyphase(src []int16, srcRate, dstRate int) []int16 {
	if len(src) == 0 || srcRate <= 0 || dstRate <= 0 {
		return nil
	}
	if srcRate == dstRate {
		out := make([]int16, len(src))
		copy(out, src)
		return out
	}
	const phases = 128
	const taps = 24
	center := float64(taps-1) * 0.5
	cutoff := math.Min(1.0, float64(dstRate)/float64(srcRate)) * 0.995
	const kaiserBeta = 8.6
	kaiserDen := besselI0(kaiserBeta)
	kernels := make([][taps]float64, phases)
	for p := 0; p < phases; p++ {
		frac := float64(p) / phases
		sum := 0.0
		for i := 0; i < taps; i++ {
			x := (float64(i) - center) - frac
			ratio := (2*float64(i))/float64(taps-1) - 1
			arg := 1 - ratio*ratio
			if arg < 0 {
				arg = 0
			}
			w := besselI0(kaiserBeta*math.Sqrt(arg)) / kaiserDen
			v := cutoff * sinc(cutoff*x) * w
			kernels[p][i] = v
			sum += v
		}
		if sum != 0 {
			for i := 0; i < taps; i++ {
				kernels[p][i] /= sum
			}
		}
	}
	dstLen := int(math.Ceil(float64(len(src)) * float64(dstRate) / float64(srcRate)))
	if dstLen < 1 {
		dstLen = 1
	}
	out := make([]int16, dstLen)
	for i := 0; i < dstLen; i++ {
		pos := float64(i) * float64(srcRate) / float64(dstRate)
		base := int(math.Floor(pos))
		frac := pos - float64(base)
		phase := int(math.Round(frac * phases))
		if phase >= phases {
			phase = 0
			base++
		}
		start := base - taps/2 + 1
		acc := 0.0
		for k := 0; k < taps; k++ {
			idx := start + k
			if idx < 0 {
				idx = 0
			} else if idx >= len(src) {
				idx = len(src) - 1
			}
			acc += float64(src[idx]) * kernels[phase][k]
		}
		out[i] = int16(clampFloat(math.Round(acc), -32768, 32767))
	}
	return out
}

func sinc(x float64) float64 {
	if math.Abs(x) < 1e-12 {
		return 1
	}
	x *= math.Pi
	return math.Sin(x) / x
}

func besselI0(x float64) float64 {
	sum := 1.0
	term := 1.0
	y := (x * x) * 0.25
	for k := 1; k < 32; k++ {
		term *= y / (float64(k) * float64(k))
		sum += term
		if term < 1e-12*sum {
			break
		}
	}
	return sum
}

func doomAdjustSoundParams(listenerX, listenerY int64, listenerAngle uint32, sourceX, sourceY int64, baseVol int, mapUsesFullClip bool) (vol, sep int, ok bool) {
	adx := abs64(listenerX - sourceX)
	ady := abs64(listenerY - sourceY)
	approxDist := adx + ady - min64(adx, ady)/2
	if !mapUsesFullClip && approxDist > doomSoundClippingDist {
		return 0, doomSoundNormalSep, false
	}
	angle := math.Atan2(float64(sourceY-listenerY), float64(sourceX-listenerX)) - angleToRadians(listenerAngle)
	sep = doomSoundNormalSep - int(math.Round((float64(doomSoundStereoSwing)/float64(fracUnit))*math.Sin(angle)))
	if sep < 0 {
		sep = 0
	}
	if sep > 255 {
		sep = 255
	}
	if approxDist < doomSoundCloseDist {
		vol = baseVol
		return vol, sep, vol > 0
	}
	if mapUsesFullClip {
		if approxDist > doomSoundClippingDist {
			approxDist = doomSoundClippingDist
		}
		vol = 15 + ((baseVol-15)*int((doomSoundClippingDist-approxDist)/fracUnit))/int(doomSoundAttenuator)
	} else {
		vol = (baseVol * int((doomSoundClippingDist-approxDist)/fracUnit)) / int(doomSoundAttenuator)
	}
	return vol, sep, vol > 0
}

func doomSeparationVolumes(vol, sep int) (left, right int) {
	sep++
	left = vol - (vol*sep*sep)/(doomSoundSepRange*doomSoundSepRange)
	sep -= 257
	right = vol - (vol*sep*sep)/(doomSoundSepRange*doomSoundSepRange)
	if left < 0 {
		left = 0
	}
	if left > doomSoundMaxVolume {
		left = doomSoundMaxVolume
	}
	if right < 0 {
		right = 0
	}
	if right > doomSoundMaxVolume {
		right = doomSoundMaxVolume
	}
	return left, right
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
