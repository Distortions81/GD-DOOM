//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/app"
	"gddoom/internal/audiofx"
	"gddoom/internal/doomruntime"
	"gddoom/internal/media"
	"gddoom/internal/sessionaudio"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
)

type soundVoice struct {
	owned bool
	sound rl.Sound
	event doomruntime.NativeSound
}

type nativeAudio struct {
	rawSamples     map[string]media.PCMSample
	sectorLoops    audiofx.LoopSequencer
	sources        map[string]rl.Sound
	voices         []soundVoice
	volume         float32
	pcMode         bool
	pcBank         map[string][]sound.PCSpeakerTone
	speaker        nativeSpeaker
	paused         bool
	speakerBackend *nativeSpeakerBackend
	speakerError   error
	attached       *doomruntime.NativeMeshGame
	attachedPC     bool
	quitSources    [2][8]string
}

func newNativeAudio(wf *wad.File, volume float32) *nativeAudio {
	return newNativeAudioWithOutput(wf, volume, audiofx.PCSpeakerOutputEmulated)
}

func newNativeAudioWithOutput(wf *wad.File, volume float32, output audiofx.PCSpeakerOutput) *nativeAudio {
	rl.InitAudioDevice()
	if !rl.IsAudioDeviceReady() {
		return nil
	}
	a := &nativeAudio{sources: make(map[string]rl.Sound), rawSamples: make(map[string]media.PCMSample), volume: volume}
	a.pcBank = sound.BuildPCSpeakerBank(sound.ImportPCSpeakerSounds(wf))
	a.speaker, a.speakerBackend, a.speakerError = newNativeSpeakerOutput(output, 1, audiofx.PCSpeakerVariantSmallSpeaker, os.Stderr)
	report := sound.ImportDigitalSounds(wf)
	a.quitSources = nativeQuitSoundSources(report)
	for _, s := range report.Sounds {
		if len(s.Samples) == 0 {
			continue
		}
		// Preserve the main game's vanilla base-pitch semantics: DMX headers
		// can carry other rates, but Doom's SFX mixer uses 11025 Hz.
		a.rawSamples[s.Name] = media.PCMSample{SampleRate: 11025, Data: s.Samples}
		wave := rl.NewWave(uint32(len(s.Samples)), 11025, 8, 1, s.Samples)
		source := rl.LoadSoundFromWave(wave)
		// The Wave borrows Go bytes; only the copied Raylib Sound is owned.
		if rl.IsSoundValid(source) {
			if previous, ok := a.sources[s.Name]; ok {
				rl.UnloadSound(previous)
			}
			a.sources[s.Name] = source
		}
	}
	return a
}

func (a *nativeAudio) Play(game *doomruntime.NativeMeshGame, event doomruntime.NativeSound) {
	if a == nil {
		return
	}
	if a.pcMode {
		if a.volume > 0 && a.speaker != nil && !a.paused {
			a.speaker.Play(a.pcBank[event.Name])
		}
		return
	}
	source, ok := a.sources[event.Name]
	if !ok {
		return
	}
	var chunk media.PCMSample
	if event.LoopTics > 0 {
		var start bool
		chunk, event.Pitch, start = a.sectorLoops.Chunk(event.Group, event.LoopTics, event.Tick, a.rawSamples[event.Name], event.Pitch)
		if !start {
			return
		}
	} else {
		a.sectorLoops.Chunk(event.Group, 0, event.Tick, media.PCMSample{}, 1)
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
		a.voices[0].unload()
		a.voices = a.voices[1:]
	}
	voice := rl.Sound{}
	owned := event.LoopTics > 0 && len(chunk.Data) > 0
	if owned {
		voice = rl.LoadSoundFromWave(rl.NewWave(uint32(len(chunk.Data)), uint32(chunk.SampleRate), 8, 1, chunk.Data))
	} else {
		voice = rl.LoadSoundAlias(source)
	}
	vol, pan := game.SoundParams(event)
	rl.SetSoundVolume(voice, vol*a.volume)
	rl.SetSoundPan(voice, pan)
	rl.SetSoundPitch(voice, event.Pitch)
	rl.PlaySound(voice)
	a.voices = append(a.voices, soundVoice{sound: voice, event: event, owned: owned})
}

func (a *nativeAudio) SetPaused(paused bool) {
	if a == nil {
		return
	}
	a.paused = paused
	if a.speaker != nil {
		a.speaker.SetPaused(paused)
	}
}

func (a *nativeAudio) Update(game *doomruntime.NativeMeshGame) {
	if a == nil {
		return
	}
	if err := a.speakerBackend.Update(); err != nil {
		game.Notify(err.Error())
	}
	active := a.voices[:0]
	for _, v := range a.voices {
		if !rl.IsSoundPlaying(v.sound) {
			v.unload()
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
	if a != nil {
		a.sectorLoops.Clear()
	}
	if a == nil {
		return
	}
	for _, v := range a.voices {
		rl.StopSound(v.sound)
		v.unload()
	}
	a.voices = a.voices[:0]
	if a.speaker != nil {
		a.speaker.ClearEffects()
	}
}

func (a *nativeAudio) ConfigureSpeaker(enabled bool, volume float64, variant string) error {
	if a == nil {
		return nil
	}
	if enabled && a.speakerError != nil {
		return a.speakerError
	}
	mode := enabled && len(a.pcBank) > 0
	if a.pcMode != mode {
		a.Stop()
	}
	a.pcMode = mode
	if a.speaker != nil {
		a.speaker.SetVariant(audiofx.ParsePCSpeakerVariant(variant))
		a.speaker.SetVolume(volume)
	}
	return nil
}

func (a *nativeAudio) Attach(game *doomruntime.NativeMeshGame) {
	if a == nil {
		return
	}
	if a.attached != game || a.attachedPC != a.pcMode {
		game.SetSoundSink(func(event doomruntime.NativeSound) { a.Play(game, event) })
		a.attached, a.attachedPC = game, a.pcMode
	}
	if a.pcMode && a.speaker != nil {
		game.SetPCSpeakerSound(a.pcBank, a.speaker, float64(a.volume))
	} else {
		game.SetPCSpeakerSound(nil, nil, float64(a.volume))
	}
}

func (a *nativeAudio) Close() {
	if a == nil {
		return
	}
	a.Stop()
	if a.speaker != nil {
		_ = a.speaker.Close()
	}
	for _, s := range a.sources {
		rl.UnloadSound(s)
	}
	rl.CloseAudioDevice()
}

func nativeQuitSoundSources(report sound.DigitalImportReport) [2][8]string {
	bank := app.BuildRuntimeSoundAliases(report)
	var names [2][8]string
	for game := range names {
		for index, sample := range audiofx.MenuQuitSamples(bank, game == 1) {
			if len(sample.Data) == 0 {
				continue
			}
			for _, imported := range report.Sounds {
				if len(imported.Samples) > 0 && &sample.Data[0] == &imported.Samples[0] {
					names[game][index] = imported.Name
					break
				}
			}
		}
	}
	return names
}

func (a *nativeAudio) PlayQuit(game *doomruntime.NativeMeshGame, commercial bool, seq int) {
	if a == nil {
		return
	}
	name := sessionaudio.PCSpeakerQuitSoundName(commercial, seq)
	if !a.pcMode {
		bank := 0
		if commercial {
			bank = 1
		}
		name = a.quitSources[bank][max(0, seq)%8]
	}
	if name != "" {
		a.Play(game, doomruntime.NativeSound{Name: name, Pitch: 1})
	}
}

func (v soundVoice) unload() {
	if v.owned {
		rl.UnloadSound(v.sound)
	} else {
		rl.UnloadSoundAlias(v.sound)
	}
}
