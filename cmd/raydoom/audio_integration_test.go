//go:build raylib && cgo && integration && !js

package main

import (
	"gddoom/internal/music"
	"os"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"gddoom/internal/audiofx"
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestNativeAudioLoadsDMXAndPlaysAliases(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_AUDIO_INTEGRATION=1 with an audio device or ALSA null PCM")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapdata.LoadMap(wf, "E1M1")
	if err != nil {
		t.Fatal(err)
	}
	opts, err := loadAssets(wf)
	if err != nil {
		t.Fatal(err)
	}
	opts.NoMonsters = true
	game := doomruntime.NewNativeMeshGame(m, opts)
	a := newNativeAudio(wf, 0.7)
	if a == nil {
		t.Fatal("audio device did not initialize")
	}
	defer a.Close()
	if len(a.sources) < 10 {
		t.Fatal("DMX sound bank was not loaded")
	}
	a.Play(game, doomruntime.NativeSound{Name: "DSPISTOL", Pitch: 1})
	if len(a.voices) != 1 || !rl.IsSoundPlaying(a.voices[0].sound) {
		t.Fatal("native sound alias did not start")
	}
	a.Play(game, doomruntime.NativeSound{Name: "DSPISTOL", Pitch: 1})
	if len(a.voices) != 2 {
		t.Fatal("overlapping effects did not get independent voices")
	}
	a.Stop()
	a.PlayQuit(game, false, 0)
	if len(a.voices) != 1 || a.voices[0].event.Name != "DSPLDETH" || !rl.IsSoundPlaying(a.voices[0].sound) {
		t.Fatal("confirmed quit did not start the Doom quit sound")
	}
	a.Stop()
	if len(a.voices) != 0 {
		t.Fatal("restart/pause did not release playing aliases")
	}
}

func TestNativeMusicSharesPhysicalSpeakerInterface(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("requires native audio")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	a := newNativeAudio(wf, .7)
	if a == nil {
		t.Fatal("audio unavailable")
	}
	defer a.Close()
	if a.speakerError != nil {
		t.Fatal(a.speakerError)
	}
	if err := a.speaker.Close(); err != nil {
		t.Fatal(err)
	}
	physical := &nativeSpeakerProbe{}
	a.speaker, a.speakerBackend = physical, nil
	if err := a.ConfigureSpeaker(true, .75, "passthrough"); err != nil {
		t.Fatal(err)
	}
	cfg := defaultNativeMusicConfig()
	cfg.backend = music.BackendPCSpeaker
	m, err := newNativeMusicWithConfig(wf, .75, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.UseSharedSpeaker(a.speaker)
	if err := m.PlayMap("E1M1"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(physical.music, m.speakerSequence) || physical.rate != m.speakerRate || !physical.loop || rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("speaker music bypassed the physical output or also started PCM")
	}
	m.SetPaused(true)
	if !physical.paused {
		t.Fatal("music pause did not reach the physical output")
	}
	m.SetPaused(false)
	if physical.paused {
		t.Fatal("physical output did not resume")
	}
	effect := []sound.PCSpeakerTone{{Active: true, Divisor: 1000}}
	physical.Play(effect)
	a.Stop()
	if len(physical.effects) != 0 || len(physical.music) == 0 {
		t.Fatal("clearing effects lost physical music")
	}
	physical.Play(effect)
	cfg.backend = music.BackendImpSynth
	if err := m.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if len(physical.music) != 0 || !slices.Equal(physical.effects, effect) || !rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("switch to digital synth lost an effect or retained physical music")
	}
	// Updating physical output has no PCM transport to pump.
	a.Update(nil)
	if physical.variant != audiofx.ParsePCSpeakerVariant("paper-speaker") {
		t.Fatal("shared model setting was not routed")
	}
}

func TestNativeMusicPlaysPausesChangesAndStops(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("set GD_RAYLIB_AUDIO_INTEGRATION=1 with an audio device or ALSA null PCM")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	audio := newNativeAudio(wf, .7)
	if audio == nil {
		t.Fatal("audio device unavailable")
	}
	defer audio.Close()
	m, err := newNativeMusic(wf, .5)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.PlayTitle("E1M1"); err != nil {
		t.Fatal(err)
	}
	if m.track != "D_INTRO" || !rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("title music did not start")
	}
	if err := m.PlayMap("E1M1"); err != nil {
		t.Fatal(err)
	}
	if !rl.IsAudioStreamPlaying(m.stream) || m.track != "D_E1M1" {
		t.Fatal("map music did not start")
	}
	m.SetPaused(true)
	if rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("paused music is still playing")
	}
	m.SetPaused(false)
	if !rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("music did not resume")
	}
	if ok, err := m.PlayCheat("E1M1", "12"); !ok || err != nil || m.track != "D_E1M2" {
		t.Fatalf("IDMUS selection failed: %t %v %s", ok, err, m.track)
	}
	if ok, _ := m.PlayCheat("E1M1", "99"); ok || m.track != "D_E1M2" {
		t.Fatal("invalid IDMUS changed current track")
	}
	m.SetVolume(0)
	if err := m.Update(); err != nil {
		t.Fatal(err)
	}
	m.Stop()
	if rl.IsAudioStreamPlaying(m.stream) || m.parsed != nil {
		t.Fatal("music stop retained playback")
	}
}

func TestNativeMusicSwitchesSynthsWithoutLosingPausedTrack(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("requires native audio")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	audio := newNativeAudio(wf, .7)
	if audio == nil {
		t.Fatal("audio unavailable")
	}
	defer audio.Close()
	m, err := newNativeMusic(wf, .5)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.PlayMap("E1M1"); err != nil {
		t.Fatal(err)
	}
	m.trackKey = "doom.wad"
	m.SetPaused(true)
	for _, backend := range []music.Backend{music.BackendPCSpeaker, music.BackendMeltySynth, music.BackendImpSynth} {
		cfg := defaultNativeMusicConfig()
		cfg.backend = backend
		cfg.soundFont = "../../soundfonts/general-midi.sf2"
		if err := m.Configure(cfg); err != nil {
			t.Fatal(err)
		}
		if rl.IsAudioStreamProcessed(m.stream) || m.track != "D_E1M1" || m.trackKey != "doom.wad" || m.parsed == nil || !m.paused || rl.IsAudioStreamPlaying(m.stream) {
			t.Fatal("switch lost current track or resumed paused playback")
		}
		m.SetPaused(false)
		if !rl.IsAudioStreamPlaying(m.stream) {
			t.Fatal("new synth cannot resume")
		}
		if err := m.Update(); err != nil {
			t.Fatal(err)
		}
		m.SetPaused(true)
	}
	originalDriver, originalPCM := m.driver, append([]int16(nil), m.pcm...)
	cfg := defaultNativeMusicConfig()
	cfg.backend = music.BackendMeltySynth
	cfg.soundFont = "not-a-soundfont.sf2"
	if err := m.Configure(cfg); err == nil {
		t.Fatal("invalid font accepted")
	}
	if m.driver != originalDriver || !slices.Equal(m.pcm, originalPCM) || m.track != "D_E1M1" || !m.paused {
		t.Fatal("failed switch mutated live playback")
	}
	if err := m.PlayData([]byte("bad score"), "BROKEN"); err == nil || m.track != "D_E1M1" {
		t.Fatal("invalid track replaced current score")
	}
	m.Stop()
	if !rl.IsAudioStreamProcessed(m.stream) {
		t.Fatal("paused stop retained old audio buffers")
	}
	cfg = defaultNativeMusicConfig()
	cfg.backend = music.BackendPCSpeaker
	if err := m.Configure(cfg); err != nil || m.parsed != nil || rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("changing synth restarted stopped music")
	}
	if err := m.PlayMap("E1M2"); err != nil {
		t.Fatal(err)
	}
	m.SetPaused(false)
	if m.speaker == nil || m.track != "D_E1M2" || !rl.IsAudioStreamPlaying(m.stream) {
		t.Fatal("speaker synth cannot play after stop")
	}
}

func TestNativeMusicReconfiguresCompressionFromOriginalScore(t *testing.T) {
	if os.Getenv("GD_RAYLIB_AUDIO_INTEGRATION") == "" {
		t.Skip("requires native audio")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rl.SetTraceLogLevel(rl.LogWarning)
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	audio := newNativeAudio(wf, .7)
	if audio == nil {
		t.Fatal("audio unavailable")
	}
	defer audio.Close()
	m, err := newNativeMusic(wf, .5)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.PlayMap("E1M1"); err != nil {
		t.Fatal(err)
	}
	raw := m.rawParsed
	m.trackKey = "doom.wad"
	m.SetPaused(true)
	for _, backend := range []music.Backend{music.BackendImpSynth, music.BackendPCSpeaker, music.BackendMeltySynth} {
		for _, ratio := range []float64{1, 3, 8, 1} {
			cfg := defaultNativeMusicConfig()
			cfg.backend, cfg.volumeCompression = backend, ratio
			cfg.soundFont = "../../soundfonts/general-midi.sf2"
			if err := m.Configure(cfg); err != nil {
				t.Fatal(err)
			}
			want := music.ApplyMUSVolumeCompression(raw, ratio)
			if m.rawParsed != raw || !reflect.DeepEqual(m.parsed, want) {
				t.Fatalf("backend %s ratio %g compounded or retained stale compression", backend, ratio)
			}
			if m.track != "D_E1M1" || m.trackKey != "doom.wad" || !m.paused || m.volume != .5 || rl.IsAudioStreamPlaying(m.stream) {
				t.Fatal("compression change lost playback state")
			}
			stream := m.stream
			if err := m.Configure(cfg); err != nil || m.stream != stream {
				t.Fatal("unchanged compression replaced the stream")
			}
		}
	}
	m.Stop()
	if m.rawParsed != nil {
		t.Fatal("stopping music retained the original score")
	}
	if err := m.PlayData([]byte("bad score"), "BROKEN"); err == nil || m.rawParsed != nil {
		t.Fatal("invalid score resumed stopped music")
	}
}
