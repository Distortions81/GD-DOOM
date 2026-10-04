package doomruntime

import (
	"gddoom/internal/audiofx"
	"gddoom/internal/sound"
	"math"
)

// SetPCSpeakerSound uses the shared sound-event policy, including excluded DP
// effects and the no-pitch-RNG PC-speaker path. The native host owns the output.
func (n *NativeMeshGame) SetPCSpeakerSound(bank map[string][]sound.PCSpeakerTone, speaker audiofx.PCSpeaker, sfxVolume float64) {
	s := n.g.snd
	if sfxVolume <= 0 {
		speaker = nil
	}
	s.pcSpeaker, s.pcSpeakerBank = speaker, bank
	s.vanillaVolume = vanillaSFXVolume(sfxVolume)
}

// NativeSound describes a sound selected by the original Doom sound queue.
// Coordinates are map units; Pitch is a playback multiplier (1 = normal).
type NativeSound struct {
	Name, Group string
	X, Y        float64
	Positioned  bool
	Pitch       float32
}

func (n *NativeMeshGame) SetSoundSink(sink func(NativeSound)) {
	s := n.g.snd
	if sink == nil {
		s.nativePlay = nil
		s.vanillaVolume = 0
		return
	}
	s.vanillaVolume = 15
	s.nativePlay = func(ev soundEvent, origin queuedSoundOrigin, _ int64, _ int64, _ uint32, _ bool, pitch int) {
		name, ok := soundEventDSName(ev)
		if !ok {
			switch ev {
			case soundEventMonsterSeePosit:
				name = "DSPOSIT1"
			case soundEventMonsterSeeImp, soundEventMonsterSeeImp1:
				name = "DSBGSIT1"
			case soundEventMonsterSeeImp2:
				name = "DSBGSIT2"
			case soundEventMonsterSeeDemon:
				name = "DSSGTSIT"
			default:
				return
			}
		}
		sink(NativeSound{Name: name, Group: singularSoundGroup(ev), X: float64(origin.x) / fracUnit, Y: float64(origin.y) / fracUnit, Positioned: origin.positioned, Pitch: float32(doomPitchStep(pitch)) / 65536})
	}
}

// SoundParams also lets a native mixer update already-playing voices as the
// listener moves. Pan uses Raylib's -1 = left, 0 = center, 1 = right.
func (n *NativeMeshGame) SoundParams(sound NativeSound) (volume, pan float32) {
	if !sound.Positioned {
		return 1, 0
	}
	g := n.g
	vol, sep, ok := doomAdjustSoundParams(g.p.x, g.p.y, g.p.angle, int64(math.Round(sound.X*fracUnit)), int64(math.Round(sound.Y*fracUnit)), 15, soundMapUsesFullClip(g.m.Name))
	if !ok {
		return 0, 0
	}
	return float32(vol) / 15, float32(sep-128) / 128
}
