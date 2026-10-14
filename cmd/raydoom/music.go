//go:build raylib && cgo && !js

package main

import (
	"encoding/binary"
	"fmt"
	gobeep86 "github.com/Distortions81/GoBeep86"
	"io"
	"strings"

	"gddoom/internal/audiofx"
	"gddoom/internal/music"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const nativeMusicFrames = 4096

type nativeMusic struct {
	wad             *wad.File
	driver          nativeMusicDriver
	config          nativeMusicConfig
	bank            music.PatchBank
	font            *music.SoundFontBank
	speaker         *gobeep86.Source
	sharedSpeaker   nativeSpeaker
	speakerSequence []sound.PCSpeakerTone
	speakerRate     int
	bytes           []byte
	parsed          *music.ParsedMUS
	rawParsed       *music.ParsedMUS
	renderer        *music.StreamRenderer
	stream          rl.AudioStream
	pcm             []int16
	track           string
	trackKey        string
	paused          bool
	volume          float32
}

// nativeMusicDriver satisfies the shared MUS stream renderer for either synth.
type nativeMusicDriver interface {
	Reset()
	ApplyEvent(music.Event)
	GenerateStereoS16(int) []int16
	SampleRate() int
	TicRate() int
	SetMUSPanMax(float64)
	SetOutputGain(float64)
	SetPreEmphasis(bool)
	RenderMUSS16LE([]byte) ([]byte, error)
}

type nativeMusicConfig struct {
	backend                   music.Backend
	soundFont, speakerVariant string
	panMax                    float64
	volumeCompression         float64
}

func defaultNativeMusicConfig() nativeMusicConfig {
	return nativeMusicConfig{backend: music.DefaultBackend(), panMax: .8, volumeCompression: music.DefaultMUSVolumeCompression, speakerVariant: "paper-speaker"}
}
func newNativeMusicState(wf *wad.File, cfg nativeMusicConfig, bank music.PatchBank, font *music.SoundFontBank) (*nativeMusic, error) {
	cfg.volumeCompression = music.NormalizeMUSVolumeCompression(cfg.volumeCompression)
	if err := music.ValidateBackend(cfg.backend); err != nil {
		return nil, err
	}
	m := &nativeMusic{wad: wf, config: cfg, bank: bank, font: font, volume: 1, pcm: make([]int16, nativeMusicFrames*2)}
	if m.bank == nil {
		if lump, ok := wf.LumpByName("GENMIDI"); ok {
			data, err := wf.LumpData(lump)
			if err != nil {
				return nil, err
			}
			m.bank, err = music.ParseGENMIDIOP2PatchBank(data)
			if err != nil {
				return nil, err
			}
		} else {
			m.bank = music.DefaultPatchBank{}
		}
	}
	var err error
	switch music.ResolveBackend(cfg.backend) {
	case music.BackendMeltySynth:
		if m.font == nil {
			m.font, err = music.ParseSoundFontFile(cfg.soundFont)
			if err != nil {
				return nil, err
			}
		}
		m.driver, err = music.NewMeltySynthDriver(music.OutputSampleRate, m.font)
	case music.BackendPCSpeaker:
		m.bytes = make([]byte, nativeMusicFrames*4)
		m.speaker = gobeep86.NewSource(gobeep86.ParseVariant(cfg.speakerVariant))
	default:
		m.driver, err = music.NewDriverWithBackend(music.OutputSampleRate, m.bank, cfg.backend)
	}
	if err != nil {
		return nil, err
	}
	if m.driver != nil {
		m.driver.SetMUSPanMax(cfg.panMax)
		m.driver.SetOutputGain(2.5)
	}
	return m, nil
}
func newNativeMusic(wf *wad.File, volume float32) (*nativeMusic, error) {
	return newNativeMusicWithConfig(wf, volume, defaultNativeMusicConfig())
}
func newNativeMusicWithConfig(wf *wad.File, volume float32, cfg nativeMusicConfig) (*nativeMusic, error) {
	m, err := newNativeMusicState(wf, cfg, nil, nil)
	if err != nil {
		return nil, err
	}
	rl.SetAudioStreamBufferSizeDefault(nativeMusicFrames)
	m.stream = rl.LoadAudioStream(music.OutputSampleRate, 16, 2)
	if !rl.IsAudioStreamValid(m.stream) {
		return nil, fmt.Errorf("music: audio stream could not initialize")
	}
	m.SetVolume(volume)
	return m, nil
}

// Keep the unmodified score so changing synths or compression never compounds
// a previous compression pass, and disabling compression restores the source.
func (m *nativeMusic) prepareRawScore(parsed *music.ParsedMUS, name string) error {
	m.rawParsed = parsed
	return m.prepareScore(music.ApplyMUSVolumeCompression(parsed, m.config.volumeCompression), name)
}

func (m *nativeMusic) prepareScore(parsed *music.ParsedMUS, name string) error {
	m.parsed, m.track = parsed, name
	if m.speaker != nil {
		seq, rate := music.RenderParsedMUSToPCSpeaker(m.bank, parsed)
		if len(seq) == 0 {
			return fmt.Errorf("music: score has no audio duration")
		}
		m.speakerSequence, m.speakerRate = seq, rate
		tones := make([]gobeep86.Tone, len(seq))
		for i, tone := range seq {
			tones[i] = gobeep86.Tone{Active: tone.Active, Divisor: tone.ToneDivisor()}
		}
		m.speaker.SetGain(float64(m.volume))
		m.speaker.SetMusic(tones, music.OutputSampleRate, rate, true)
	}
	// Render before changing the live stream, including rejecting zero-duration MUS.
	_, err := m.nextPCM()
	return err
}
func (m *nativeMusic) adopt(candidate *nativeMusic) error {
	candidate.sharedSpeaker = m.sharedSpeaker
	if candidate.speaker != nil && candidate.sharedSpeaker != nil {
		if m.paused && m.parsed != nil {
			rl.ResumeAudioStream(m.stream)
		}
		rl.StopAudioStream(m.stream)
		candidate.stream, candidate.paused = m.stream, m.paused
		candidate.SetVolume(m.volume)
		candidate.sharedSpeaker.SetVariant(audiofx.ParsePCSpeakerVariant(candidate.config.speakerVariant))
		candidate.sharedSpeaker.SetMusic(candidate.speakerSequence, candidate.speakerRate, true)
		candidate.sharedSpeaker.SetPaused(candidate.paused)
		*m = *candidate
		return nil
	}
	// Raylib StopAudioStream does not reset paused or inactive buffers. Prepare a
	// fresh stream before replacing the old one, so no previous score can leak.
	candidate.stream = rl.LoadAudioStream(music.OutputSampleRate, 16, 2)
	if !rl.IsAudioStreamValid(candidate.stream) {
		return fmt.Errorf("music: audio stream could not initialize")
	}
	candidate.SetVolume(m.volume)
	candidate.paused = m.paused
	// Play initializes Raylib's processed flags. Pause before priming both halves
	// so Play cannot subsequently mark the newly written score as consumed.
	rl.PlayAudioStream(candidate.stream)
	rl.PauseAudioStream(candidate.stream)
	rl.UpdateAudioStream(candidate.stream, candidate.pcm[:nativeMusicFrames])
	if err := candidate.fill(); err != nil {
		rl.UnloadAudioStream(candidate.stream)
		return err
	}
	if m.speaker != nil && m.sharedSpeaker != nil {
		m.sharedSpeaker.ClearMusic()
	}
	rl.UnloadAudioStream(m.stream)
	*m = *candidate
	if !m.paused {
		rl.ResumeAudioStream(m.stream)
	}
	return nil
}

func (m *nativeMusic) Configure(cfg nativeMusicConfig) error {
	if m == nil {
		return fmt.Errorf("music: audio device unavailable")
	}
	cfg.volumeCompression = music.NormalizeMUSVolumeCompression(cfg.volumeCompression)
	if cfg == m.config {
		return nil
	}
	previous := m.config
	previous.speakerVariant = cfg.speakerVariant
	if previous == cfg && music.ResolveBackend(cfg.backend) != music.BackendPCSpeaker {
		m.config = cfg
		return nil
	}
	font := m.font
	if cfg.soundFont != m.config.soundFont {
		font = nil
	}
	next, err := newNativeMusicState(m.wad, cfg, m.bank, font)
	if err != nil {
		return err
	}
	next.volume = m.volume
	if m.parsed != nil {
		next.trackKey = m.trackKey
		if err := next.prepareRawScore(m.rawParsed, m.track); err != nil {
			return err
		}
		return m.adopt(next)
	}
	next.stream, next.paused, next.volume, next.sharedSpeaker = m.stream, m.paused, m.volume, m.sharedSpeaker
	*m = *next
	return nil
}

func (m *nativeMusic) PlayMap(name string) error {
	lump, ok := music.MapLumpName(name)
	if !ok {
		return fmt.Errorf("music: no track for %s", name)
	}
	return m.PlayLump(lump)
}

func (m *nativeMusic) PlayTitle(mapName string) error {
	name := "D_INTRO"
	if strings.HasPrefix(strings.ToUpper(mapName), "MAP") {
		name = "D_DM2TTL"
	}
	return m.PlayLump(name)
}

func (m *nativeMusic) PlayLump(name string) error {
	lump, ok := m.wad.LumpByName(name)
	if !ok {
		return fmt.Errorf("music: missing %s", name)
	}
	data, err := m.wad.LumpData(lump)
	if err != nil {
		return err
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		return err
	}
	return m.PlayParsed(parsed, name)
}

func (m *nativeMusic) PlayParsed(parsed *music.ParsedMUS, name string) error {
	next, err := newNativeMusicState(m.wad, m.config, m.bank, m.font)
	if err != nil {
		return err
	}
	next.volume = m.volume
	if err := next.prepareRawScore(parsed, name); err != nil {
		return err
	}
	return m.adopt(next)
}
func (m *nativeMusic) PlayData(data []byte, name string) error {
	if m == nil {
		return fmt.Errorf("music: audio device unavailable")
	}
	parsed, err := music.ParseMUSData(data)
	if err != nil {
		return err
	}
	return m.PlayParsed(parsed, name)
}

func (m *nativeMusic) PlayCheat(current, code string) (bool, error) {
	if m == nil {
		return false, nil
	}
	target, lump, ok := music.CheatSelection(current, code)
	if !ok {
		return false, nil
	}
	if target == "" && lump == "" {
		m.Stop()
		return true, nil
	}
	if target != "" {
		lump, ok = music.MapLumpName(target)
		if !ok {
			return false, nil
		}
	}
	if _, ok := m.wad.LumpByName(lump); !ok {
		return false, nil
	}
	if err := m.PlayLump(lump); err != nil {
		return false, err
	}
	return true, nil
}

// nextPCM fills one stereo buffer, joining the tail and start without silence.
// Limit loop attempts so an empty/malformed zero-duration score cannot hang.
func (m *nativeMusic) nextPCM() ([]int16, error) {
	if m.speaker != nil {
		_, err := io.ReadFull(m.speaker, m.bytes)
		if err != nil {
			return nil, err
		}
		for i := range m.pcm {
			m.pcm[i] = int16(binary.LittleEndian.Uint16(m.bytes[2*i : 2*i+2]))
		}
		return m.pcm, nil
	}
	out := m.pcm[:0]
	empty := 0
	for len(out) < nativeMusicFrames*2 {
		if m.renderer == nil {
			var err error
			m.renderer, err = music.NewParsedMUSStreamRenderer(m.driver, m.parsed)
			if err != nil {
				return nil, err
			}
		}
		data, done, err := m.renderer.NextChunkS16LE(nativeMusicFrames - len(out)/2)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(data); i += 2 {
			out = append(out, int16(binary.LittleEndian.Uint16(data[i:i+2])))
		}
		if done {
			m.renderer = nil
		}
		if len(data) == 0 {
			empty++
			if empty > 1 {
				return nil, fmt.Errorf("music: score has no audio duration")
			}
		} else {
			empty = 0
		}
	}
	return out, nil
}

func (m *nativeMusic) fill() error {
	// Raylib has two sub-buffers; bound refills even with a fast/null device.
	for i := 0; i < 2 && rl.IsAudioStreamProcessed(m.stream); i++ {
		pcm, err := m.nextPCM()
		if err != nil {
			return err
		}
		// v0.60.1's cgo wrapper passes len(data) as C's *frame count*. Keep the
		// stereo backing array intact but shorten the slice count to frames.
		rl.UpdateAudioStream(m.stream, pcm[:len(pcm)/2])
	}
	return nil
}

func (m *nativeMusic) Update() error {
	if m == nil || m.parsed == nil || m.paused {
		return nil
	}
	if m.speaker != nil && m.sharedSpeaker != nil {
		return nil
	}
	if err := m.fill(); err != nil {
		m.Stop()
		return err
	}
	return nil
}
func (m *nativeMusic) SetVolume(v float32) {
	if m != nil {
		m.volume = v
		// The shared PC-speaker player applies volume both to its physical speaker
		// source and to the output mixer. Retain that response in this host.
		if m.speaker != nil {
			m.speaker.SetGain(float64(v))
			if m.sharedSpeaker != nil {
				m.sharedSpeaker.SetVolume(float64(v))
			}
		}
		rl.SetAudioStreamVolume(m.stream, v)
	}
}
func (m *nativeMusic) SetPaused(paused bool) {
	if m == nil || m.paused == paused {
		return
	}
	m.paused = paused
	if m.parsed == nil {
		return
	}
	if m.speaker != nil && m.sharedSpeaker != nil {
		m.sharedSpeaker.SetPaused(paused)
		return
	}
	if paused {
		rl.PauseAudioStream(m.stream)
	} else {
		rl.ResumeAudioStream(m.stream)
	}
}
func (m *nativeMusic) Stop() {
	if m == nil {
		return
	}
	// Resume then stop allows Raylib to clear queued sub-buffers when paused.
	if m.paused && m.parsed != nil {
		rl.ResumeAudioStream(m.stream)
	}
	rl.StopAudioStream(m.stream)
	m.parsed, m.renderer, m.track, m.trackKey = nil, nil, "", ""
	m.rawParsed = nil
	if m.speaker != nil {
		m.speaker.ClearMusic()
		if m.sharedSpeaker != nil {
			m.sharedSpeaker.ClearMusic()
		}
	}
}

// UseSharedSpeaker must precede playback. The native audio owner pumps this
// output for both DP effects and PC-speaker music through the shared player.
func (m *nativeMusic) UseSharedSpeaker(speaker nativeSpeaker) {
	if m != nil {
		m.sharedSpeaker = speaker
	}
}
func (m *nativeMusic) Close() {
	if m != nil {
		m.Stop()
		rl.UnloadAudioStream(m.stream)
	}
}
