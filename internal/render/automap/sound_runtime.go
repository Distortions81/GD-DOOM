package automap

import (
	"gddoom/internal/audiofx"

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
	soundEventBarrelExplode
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
	bank   SoundBank
	player *audiofx.SpatialPlayer
}

type MenuSoundPlayer = audiofx.MenuPlayer

const (
	doomSoundMaxVolume    = 127
	doomSoundClippingDist = int64(1200 * fracUnit)
	doomSoundNormalSep    = 128
)

func EnsureSharedAudioContext() *audio.Context {
	return audiofx.EnsureSharedAudioContext()
}

func NewMenuSoundPlayer(bank SoundBank, volume float64) *MenuSoundPlayer {
	return audiofx.NewMenuPlayer(bank, volume)
}

func newSoundSystem(bank SoundBank, sfxVolume float64, sourcePort bool) *soundSystem {
	return &soundSystem{
		bank:   bank,
		player: audiofx.NewSpatialPlayer(sfxVolume, sourcePort),
	}
}

func PrepareSoundBankForSourcePort(bank SoundBank, dstRate int) SoundBank {
	return audiofx.PrepareSoundBankForSourcePort(bank, dstRate)
}

func applySourcePortPresenceBoost(src []int16) []int16 {
	return audiofx.ApplySourcePortPresenceBoost(src)
}

func (s *soundSystem) setSFXVolume(v float64) {
	if s == nil || s.player == nil {
		return
	}
	s.player.SetVolume(v)
}

func (s *soundSystem) playEvent(ev soundEvent) {
	s.playEventSpatial(ev, queuedSoundOrigin{}, 0, 0, 0, false)
}

func (s *soundSystem) playEventSpatial(ev soundEvent, origin queuedSoundOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) {
	if s == nil || s.player == nil {
		return
	}
	sample, ok := s.sampleForEvent(ev)
	if !ok || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	s.player.PlaySampleSpatial(sample, audiofx.SpatialOrigin{
		X:          origin.x,
		Y:          origin.y,
		Positioned: origin.positioned,
	}, listenerX, listenerY, listenerAngle, mapUsesFullClip)
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
	case soundEventBarrelExplode:
		if len(s.bank.BarrelExplode.Data) > 0 {
			return s.bank.BarrelExplode, true
		}
		return s.sampleForEvent(soundEventImpactRocket)
	case soundEventMonsterSeePosit:
		if sample, ok := pickFirstAvailable(soundVariantIndex(3),
			s.bank.SeePosit1,
			s.bank.SeePosit2,
			s.bank.SeePosit3,
		); ok {
			return sample, true
		}
		return s.sampleForEvent(soundEventMonsterActivePosit)
	case soundEventMonsterSeeImp:
		if sample, ok := pickFirstAvailable(soundVariantIndex(2),
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
		if sample, ok := pickFirstAvailable(soundVariantIndex(3),
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
		if sample, ok := pickFirstAvailable(soundVariantIndex(3),
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
		if sample, ok := pickFirstAvailable(soundVariantIndex(3),
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
		if sample, ok := pickFirstAvailable(soundVariantIndex(2),
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

func soundVariantIndex(n int) int {
	return audiofx.SoundVariantIndex(n)
}

func pickFirstAvailable(start int, samples ...PCMSample) (PCMSample, bool) {
	return audiofx.PickFirstAvailable(start, samples...)
}

func (s *soundSystem) tick() {
	if s != nil && s.player != nil {
		s.player.Tick()
	}
}

func (s *soundSystem) stopAll() {
	if s != nil && s.player != nil {
		s.player.StopAll()
	}
}

func pcmMonoU8ToStereoS16LE(src []byte) []byte {
	return pcmMonoU8ToStereoS16LESpatial(src, 1, 1)
}

func pcmMonoU8ToMonoS16(src []byte) []int16 {
	return audiofx.PCMMonoU8ToMonoS16(src)
}

func pcmMonoS16ToStereoS16LESpatial(src []int16, leftGain, rightGain float64) []byte {
	return audiofx.PCMMonoS16ToStereoS16LESpatial(src, leftGain, rightGain)
}

func pcmMonoU8ToStereoS16LESpatial(src []byte, leftGain, rightGain float64) []byte {
	return audiofx.PCMMonoU8ToStereoS16LESpatial(src, leftGain, rightGain)
}

func pcmMonoU8ToStereoS16LEResampled(src []byte, srcRate, dstRate int) []byte {
	return audiofx.PCMMonoU8ToStereoS16LEResampled(src, srcRate, dstRate)
}

func pcmMonoU8ToStereoS16LESpatialResampled(src []byte, srcRate, dstRate int, leftGain, rightGain float64) []byte {
	return audiofx.PCMMonoU8ToStereoS16LESpatialResampled(src, srcRate, dstRate, leftGain, rightGain)
}

func doomAdjustSoundParams(listenerX, listenerY int64, listenerAngle uint32, sourceX, sourceY int64, baseVol int, mapUsesFullClip bool) (vol, sep int, ok bool) {
	return audiofx.DoomAdjustSoundParams(listenerX, listenerY, listenerAngle, sourceX, sourceY, baseVol, mapUsesFullClip)
}

func doomSeparationVolumes(vol, sep int) (left, right int) {
	return audiofx.DoomSeparationVolumes(vol, sep)
}
