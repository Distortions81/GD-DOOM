package doomruntime

import (
	"fmt"
	"gddoom/internal/audiofx"
	"gddoom/internal/doomrand"
	"math"

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
	soundEventTink
	soundEventItemUp
	soundEventWeaponUp
	soundEventPowerUp
	soundEventTeleport
	soundEventBossBrainSpit
	soundEventBossBrainCube
	soundEventBossBrainAwake
	soundEventBossBrainPain
	soundEventBossBrainDeath
	soundEventOof
	soundEventPain
	soundEventShootPistol
	soundEventShootShotgun
	soundEventShootSuperShotgun
	soundEventShootPlasma
	soundEventShootBFG
	soundEventPunch
	soundEventShootFireball
	soundEventShootRocket
	soundEventSawUp
	soundEventSawIdle
	soundEventSawFull
	soundEventSawHit
	soundEventShotgunOpen
	soundEventShotgunLoad
	soundEventShotgunClose
	soundEventMonsterAttackClaw
	soundEventMonsterAttackSgt
	soundEventMonsterAttackSkull
	soundEventMonsterAttackArchvile
	soundEventMonsterAttackMancubus
	soundEventImpactFire
	soundEventImpactRocket
	soundEventBarrelExplode
	soundEventMonsterSeePosit1
	soundEventMonsterSeePosit2
	soundEventMonsterSeePosit3
	soundEventMonsterSeeImp1
	soundEventMonsterSeeImp2
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
	soundEventDeathPodth1
	soundEventDeathPodth2
	soundEventDeathPodth3
	soundEventDeathBgdth1
	soundEventDeathBgdth2
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
	bank          SoundBank
	player        *audiofx.SpatialPlayer
	rand          uint32
	vanillaVolume int
	pitchShift    bool
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

func newSoundSystem(bank SoundBank, sfxVolume float64, sourcePort bool, pitchShift bool) *soundSystem {
	var player *audiofx.SpatialPlayer
	if clampVolume(sfxVolume) > 0 {
		player = audiofx.NewSpatialPlayer(sfxVolume, sourcePort)
	}
	return &soundSystem{
		bank:          bank,
		player:        player,
		rand:          0x1f123bb5,
		vanillaVolume: vanillaSFXVolume(sfxVolume),
		pitchShift:    pitchShift,
	}
}

func vanillaSFXVolume(v float64) int {
	v = clampVolume(v)
	return int(v*15 + 0.5)
}

func PrepareSoundBankForSourcePort(bank SoundBank, dstRate int) SoundBank {
	return audiofx.PrepareSoundBankForSourcePort(bank, dstRate)
}

func applySourcePortPresenceBoost(src []int16) []int16 {
	return audiofx.ApplySourcePortPresenceBoost(src)
}

func (s *soundSystem) setSFXVolume(v float64) {
	if s == nil {
		return
	}
	s.vanillaVolume = vanillaSFXVolume(v)
	if s.player != nil {
		s.player.SetVolume(v)
	}
}

func (s *soundSystem) playEvent(ev soundEvent) {
	s.playEventSpatial(ev, queuedSoundOrigin{}, 0, 0, 0, false)
}

func (s *soundSystem) playEventSpatial(ev soundEvent, origin queuedSoundOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) {
	if !vanillaSoundWouldStart(s, origin, listenerX, listenerY, listenerAngle, mapUsesFullClip) {
		return
	}
	pitch := vanillaPitchForEvent(ev, s != nil && s.pitchShift)
	if s == nil || s.player == nil {
		return
	}
	audioOrigin := audiofx.SpatialOrigin{
		X:          origin.x,
		Y:          origin.y,
		Positioned: origin.positioned,
	}
	sample, ok := s.sampleForEvent(ev)
	if !ok || sample.SampleRate <= 0 || len(sample.Data) == 0 {
		return
	}
	sample = applyVanillaPitch(sample, pitch)
	s.player.PlaySampleSpatialDelayed(sample, audioOrigin, listenerX, listenerY, listenerAngle, mapUsesFullClip, s.monsterVocalPreDelaySamples(ev))
}

func (s *soundSystem) nextRandByte() int {
	if s == nil {
		return 0
	}
	if s.rand == 0 {
		s.rand = 0x1f123bb5
	}
	s.rand = s.rand*1664525 + 1013904223
	return int((s.rand >> 24) & 0xff)
}

func (s *soundSystem) monsterVocalPreDelaySamples(ev soundEvent) float64 {
	if s == nil || s.player == nil || !isMonsterVocalSound(ev) {
		return 0
	}
	ctx := audiofx.EnsureSharedAudioContext()
	if ctx == nil {
		return 0
	}
	// Audio-only pre-delay is a GD-DOOM effect, not a vanilla gameplay RNG consumer.
	delayMS := float64(s.nextRandByte() % 26)
	return delayMS * float64(ctx.SampleRate()) / 1000.0
}

func vanillaSoundWouldStart(s *soundSystem, origin queuedSoundOrigin, listenerX, listenerY int64, listenerAngle uint32, mapUsesFullClip bool) bool {
	if !origin.positioned {
		return true
	}
	baseVol := 15
	if s != nil {
		baseVol = s.vanillaVolume
	}
	_, _, ok := doomAdjustSoundParams(listenerX, listenerY, listenerAngle, origin.x, origin.y, baseVol, mapUsesFullClip)
	return ok
}

type vanillaPitchMode int

const (
	vanillaPitchNone vanillaPitchMode = iota
	vanillaPitchDefault
	vanillaPitchSaw
)

func vanillaPitchModeForEvent(ev soundEvent) vanillaPitchMode {
	switch ev {
	case soundEventSawUp, soundEventSawIdle, soundEventSawFull, soundEventSawHit:
		return vanillaPitchSaw
	case soundEventItemUp, soundEventTink:
		return vanillaPitchNone
	default:
		return vanillaPitchDefault
	}
}

func vanillaPitchForEvent(ev soundEvent, enabled bool) int {
	pitch := 128
	if !enabled {
		return pitch
	}
	switch vanillaPitchModeForEvent(ev) {
	case vanillaPitchSaw:
		debugLogVanillaPitch(ev, "saw")
		pitch += 8 - (doomrand.MRandom() & 15)
	case vanillaPitchNone:
		return pitch
	default:
		debugLogVanillaPitch(ev, "default")
		pitch += 16 - (doomrand.MRandom() & 31)
	}
	if pitch < 0 {
		pitch = 0
	} else if pitch > 255 {
		pitch = 255
	}
	return pitch
}

func applyVanillaPitch(sample PCMSample, pitch int) PCMSample {
	return sample
}

func doomPitchStep(pitch int) int {
	if pitch < 0 {
		pitch = 0
	} else if pitch > 255 {
		pitch = 255
	}
	return int(math.Pow(2.0, (float64(pitch)-128.0)/64.0) * 65536.0)
}

func vanillaPitchAdjustedSample(ev soundEvent, sample PCMSample, enabled bool) PCMSample {
	return applyVanillaPitch(sample, vanillaPitchForEvent(ev, enabled))
}

func debugLogVanillaPitch(ev soundEvent, mode string) {
	if runtimeDebugEnv("GD_DEBUG_RNG_SOUND") == "" {
		return
	}
	fmt.Printf("doomrand-sound side=gd event=%s mode=%s\n", soundEventDebugName(ev), mode)
}

func soundEventDebugName(ev soundEvent) string {
	switch ev {
	case soundEventItemUp:
		return "ItemUp"
	case soundEventTink:
		return "Tink"
	case soundEventPowerUp:
		return "PowerUp"
	case soundEventShootPistol:
		return "ShootPistol"
	case soundEventShootShotgun:
		return "ShootShotgun"
	case soundEventMonsterAttackClaw:
		return "MonsterAttackClaw"
	case soundEventMonsterAttackSgt:
		return "MonsterAttackSgt"
	case soundEventMonsterAttackSkull:
		return "MonsterAttackSkull"
	case soundEventMonsterAttackArchvile:
		return "MonsterAttackArchvile"
	case soundEventMonsterAttackMancubus:
		return "MonsterAttackMancubus"
	case soundEventMonsterSeePosit1:
		return "MonsterSeePosit1"
	case soundEventMonsterSeePosit2:
		return "MonsterSeePosit2"
	case soundEventMonsterSeePosit3:
		return "MonsterSeePosit3"
	case soundEventMonsterSeeImp1:
		return "MonsterSeeImp1"
	case soundEventMonsterSeeImp2:
		return "MonsterSeeImp2"
	case soundEventMonsterSeeDemon:
		return "MonsterSeeDemon"
	case soundEventMonsterSeeCaco:
		return "MonsterSeeCaco"
	case soundEventMonsterSeeBaron:
		return "MonsterSeeBaron"
	case soundEventMonsterSeeKnight:
		return "MonsterSeeKnight"
	case soundEventMonsterSeeSpider:
		return "MonsterSeeSpider"
	case soundEventMonsterSeeCyber:
		return "MonsterSeeCyber"
	case soundEventMonsterActivePosit:
		return "MonsterActivePosit"
	case soundEventMonsterActiveImp:
		return "MonsterActiveImp"
	case soundEventMonsterActiveDemon:
		return "MonsterActiveDemon"
	case soundEventMonsterActiveArachnotron:
		return "MonsterActiveArachnotron"
	case soundEventMonsterActiveArchvile:
		return "MonsterActiveArchvile"
	case soundEventMonsterActiveRevenant:
		return "MonsterActiveRevenant"
	default:
		return fmt.Sprintf("soundEvent(%d)", ev)
	}
}

func isMonsterVocalSound(ev soundEvent) bool {
	switch ev {
	case soundEventMonsterSeePosit1,
		soundEventMonsterSeePosit2,
		soundEventMonsterSeePosit3,
		soundEventMonsterSeeImp1,
		soundEventMonsterSeeImp2,
		soundEventMonsterSeePosit,
		soundEventMonsterSeeImp,
		soundEventMonsterSeeDemon,
		soundEventMonsterSeeCaco,
		soundEventMonsterSeeBaron,
		soundEventMonsterSeeKnight,
		soundEventMonsterSeeSpider,
		soundEventMonsterSeeArachnotron,
		soundEventMonsterSeeCyber,
		soundEventMonsterSeePainElemental,
		soundEventMonsterSeeWolfSS,
		soundEventMonsterSeeArchvile,
		soundEventMonsterSeeRevenant,
		soundEventMonsterPainHumanoid,
		soundEventMonsterPainDemon,
		soundEventDeathZombie,
		soundEventDeathShotgunGuy,
		soundEventDeathChaingunner,
		soundEventDeathImp,
		soundEventDeathDemon,
		soundEventDeathCaco,
		soundEventDeathBaron,
		soundEventDeathKnight,
		soundEventDeathCyber,
		soundEventDeathSpider,
		soundEventDeathArachnotron,
		soundEventDeathLostSoul,
		soundEventDeathMancubus,
		soundEventDeathRevenant,
		soundEventDeathPainElemental,
		soundEventDeathWolfSS,
		soundEventDeathArchvile,
		soundEventMonsterDeath:
		return true
	default:
		return false
	}
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
	case soundEventTink:
		if len(s.bank.Tink.Data) > 0 {
			return s.bank.Tink, true
		}
		return s.bank.NoWay, true
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
	case soundEventTeleport:
		if len(s.bank.Teleport.Data) > 0 {
			return s.bank.Teleport, true
		}
		return s.bank.NoWay, true
	case soundEventBossBrainSpit:
		if len(s.bank.BossBrainSpit.Data) > 0 {
			return s.bank.BossBrainSpit, true
		}
		return s.sampleForEvent(soundEventShootFireball)
	case soundEventBossBrainCube:
		if len(s.bank.BossBrainCube.Data) > 0 {
			return s.bank.BossBrainCube, true
		}
		return s.sampleForEvent(soundEventBossBrainSpit)
	case soundEventBossBrainAwake:
		if len(s.bank.BossBrainAwake.Data) > 0 {
			return s.bank.BossBrainAwake, true
		}
		return s.sampleForEvent(soundEventBossBrainSpit)
	case soundEventBossBrainPain:
		if len(s.bank.BossBrainPain.Data) > 0 {
			return s.bank.BossBrainPain, true
		}
		return s.sampleForEvent(soundEventMonsterPainHumanoid)
	case soundEventBossBrainDeath:
		if len(s.bank.BossBrainDeath.Data) > 0 {
			return s.bank.BossBrainDeath, true
		}
		return s.sampleForEvent(soundEventImpactRocket)
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
	case soundEventShootSuperShotgun:
		if len(s.bank.ShootSuperShotgun.Data) > 0 {
			return s.bank.ShootSuperShotgun, true
		}
		return s.sampleForEvent(soundEventShootShotgun)
	case soundEventShootPlasma:
		if len(s.bank.ShootPlasma.Data) > 0 {
			return s.bank.ShootPlasma, true
		}
		return s.sampleForEvent(soundEventShootFireball)
	case soundEventShootBFG:
		if len(s.bank.ShootBFG.Data) > 0 {
			return s.bank.ShootBFG, true
		}
		return s.sampleForEvent(soundEventShootRocket)
	case soundEventPunch:
		if len(s.bank.Punch.Data) > 0 {
			return s.bank.Punch, true
		}
		return s.sampleForEvent(soundEventMonsterAttackClaw)
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
	case soundEventSawUp:
		if len(s.bank.SawUp.Data) > 0 {
			return s.bank.SawUp, true
		}
		return s.sampleForEvent(soundEventWeaponUp)
	case soundEventSawIdle:
		if len(s.bank.SawIdle.Data) > 0 {
			return s.bank.SawIdle, true
		}
		return s.sampleForEvent(soundEventSawUp)
	case soundEventSawFull:
		if len(s.bank.SawFull.Data) > 0 {
			return s.bank.SawFull, true
		}
		return s.sampleForEvent(soundEventSawIdle)
	case soundEventSawHit:
		if len(s.bank.SawHit.Data) > 0 {
			return s.bank.SawHit, true
		}
		return s.sampleForEvent(soundEventSawFull)
	case soundEventShotgunOpen:
		if len(s.bank.ShotgunOpen.Data) > 0 {
			return s.bank.ShotgunOpen, true
		}
		return s.sampleForEvent(soundEventWeaponUp)
	case soundEventShotgunLoad:
		if len(s.bank.ShotgunLoad.Data) > 0 {
			return s.bank.ShotgunLoad, true
		}
		return s.sampleForEvent(soundEventShotgunOpen)
	case soundEventShotgunClose:
		if len(s.bank.ShotgunClose.Data) > 0 {
			return s.bank.ShotgunClose, true
		}
		return s.sampleForEvent(soundEventShotgunOpen)
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
	case soundEventMonsterAttackArchvile:
		if len(s.bank.AttackArchvile.Data) > 0 {
			return s.bank.AttackArchvile, true
		}
		return s.sampleForEvent(soundEventBarrelExplode)
	case soundEventMonsterAttackMancubus:
		if len(s.bank.AttackMancubus.Data) > 0 {
			return s.bank.AttackMancubus, true
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
	case soundEventMonsterSeePosit1:
		if len(s.bank.SeePosit1.Data) > 0 {
			return s.bank.SeePosit1, true
		}
		return s.sampleForEvent(soundEventMonsterActivePosit)
	case soundEventMonsterSeePosit2:
		if len(s.bank.SeePosit2.Data) > 0 {
			return s.bank.SeePosit2, true
		}
		return s.sampleForEvent(soundEventMonsterSeePosit1)
	case soundEventMonsterSeePosit3:
		if len(s.bank.SeePosit3.Data) > 0 {
			return s.bank.SeePosit3, true
		}
		return s.sampleForEvent(soundEventMonsterSeePosit1)
	case soundEventMonsterSeeImp1:
		if len(s.bank.SeeBGSit1.Data) > 0 {
			return s.bank.SeeBGSit1, true
		}
		return s.sampleForEvent(soundEventMonsterActiveImp)
	case soundEventMonsterSeeImp2:
		if len(s.bank.SeeBGSit2.Data) > 0 {
			return s.bank.SeeBGSit2, true
		}
		return s.sampleForEvent(soundEventMonsterSeeImp1)
	case soundEventMonsterSeePosit:
		return s.sampleForEvent(soundEventMonsterSeePosit1)
	case soundEventMonsterSeeImp:
		return s.sampleForEvent(soundEventMonsterSeeImp1)
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
	case soundEventDeathPodth1:
		if len(s.bank.DeathPodth1.Data) > 0 {
			return s.bank.DeathPodth1, true
		}
		if len(s.bank.DeathZombie.Data) > 0 {
			return s.bank.DeathZombie, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathPodth2:
		if len(s.bank.DeathPodth2.Data) > 0 {
			return s.bank.DeathPodth2, true
		}
		return s.sampleForEvent(soundEventDeathPodth1)
	case soundEventDeathPodth3:
		if len(s.bank.DeathPodth3.Data) > 0 {
			return s.bank.DeathPodth3, true
		}
		return s.sampleForEvent(soundEventDeathPodth1)
	case soundEventDeathBgdth1:
		if len(s.bank.DeathBgdth1.Data) > 0 {
			return s.bank.DeathBgdth1, true
		}
		if len(s.bank.DeathImp.Data) > 0 {
			return s.bank.DeathImp, true
		}
		return s.sampleForEvent(soundEventMonsterDeath)
	case soundEventDeathBgdth2:
		if len(s.bank.DeathBgdth2.Data) > 0 {
			return s.bank.DeathBgdth2, true
		}
		return s.sampleForEvent(soundEventDeathBgdth1)
	case soundEventDeathZombie:
		return s.sampleForEvent(soundEventDeathPodth1)
	case soundEventDeathShotgunGuy:
		return s.sampleForEvent(soundEventDeathPodth2)
	case soundEventDeathChaingunner:
		return s.sampleForEvent(soundEventDeathPodth2)
	case soundEventDeathImp:
		return s.sampleForEvent(soundEventDeathBgdth1)
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
