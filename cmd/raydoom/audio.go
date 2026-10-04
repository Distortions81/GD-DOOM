//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

type soundVoice struct {
	sound rl.Sound
	event doomruntime.NativeSound
}

type nativeAudio struct {
	sources map[string]rl.Sound
	voices  []soundVoice
	volume  float32
}

func newNativeAudio(wf *wad.File, volume float32) *nativeAudio {
	rl.InitAudioDevice()
	if !rl.IsAudioDeviceReady() {
		return nil
	}
	a := &nativeAudio{sources: make(map[string]rl.Sound), volume: volume}
	for _, s := range sound.ImportDigitalSounds(wf).Sounds {
		if len(s.Samples) == 0 {
			continue
		}
		// Preserve the main game's vanilla base-pitch semantics: DMX headers
		// can carry other rates, but Doom's SFX mixer uses 11025 Hz.
		wave := rl.NewWave(uint32(len(s.Samples)), 11025, 8, 1, s.Samples)
		source := rl.LoadSoundFromWave(wave)
		// The Wave borrows Go bytes; only the copied Raylib Sound is owned.
		if rl.IsSoundValid(source) {
			a.sources[s.Name] = source
		}
	}
	return a
}

func (a *nativeAudio) Play(game *doomruntime.NativeMeshGame, event doomruntime.NativeSound) {
	if a == nil {
		return
	}
	source, ok := a.sources[event.Name]
	if !ok {
		return
	}
	if event.Group != "" {
		for _, v := range a.voices {
			if v.event.Group == event.Group {
				rl.StopSound(v.sound)
			}
		}
	}
	a.Update(game)
	if len(a.voices) >= 32 {
		rl.StopSound(a.voices[0].sound)
		rl.UnloadSoundAlias(a.voices[0].sound)
		a.voices = a.voices[1:]
	}
	voice := rl.LoadSoundAlias(source)
	vol, pan := game.SoundParams(event)
	rl.SetSoundVolume(voice, vol*a.volume)
	rl.SetSoundPan(voice, pan)
	rl.SetSoundPitch(voice, event.Pitch)
	rl.PlaySound(voice)
	a.voices = append(a.voices, soundVoice{voice, event})
}

func (a *nativeAudio) Update(game *doomruntime.NativeMeshGame) {
	if a == nil {
		return
	}
	active := a.voices[:0]
	for _, v := range a.voices {
		if !rl.IsSoundPlaying(v.sound) {
			rl.UnloadSoundAlias(v.sound)
			continue
		}
		vol, pan := game.SoundParams(v.event)
		rl.SetSoundVolume(v.sound, vol*a.volume)
		rl.SetSoundPan(v.sound, pan)
		active = append(active, v)
	}
	a.voices = active
}

func (a *nativeAudio) Stop() {
	if a == nil {
		return
	}
	for _, v := range a.voices {
		rl.StopSound(v.sound)
		rl.UnloadSoundAlias(v.sound)
	}
	a.voices = a.voices[:0]
}

func (a *nativeAudio) Close() {
	if a == nil {
		return
	}
	a.Stop()
	for _, s := range a.sources {
		rl.UnloadSound(s)
	}
	rl.CloseAudioDevice()
}
