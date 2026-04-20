package audiofx

import (
	"sync"

	"gddoom/internal/music"
	"gddoom/internal/sound"
	gobeep86 "github.com/Distortions81/GoBeep86"
)

const pcSpeakerEmulatedMixTickRate = gobeep86.DefaultMixTickRate

var pcSpeakerToneInterleaveTargetHz = 140.0

func SetPCSpeakerInterleaveHz(hz float64) {
	if hz < 10 {
		hz = 10
	} else if hz > 1000 {
		hz = 1000
	}
	pcSpeakerToneInterleaveTargetHz = hz
	gobeep86.SetInterleaveHz(hz)
}

type PCSpeakerPlayer struct {
	mu     sync.Mutex
	player ebitenPlayer
	src    *gobeep86.Source
	volume float64
}

type PCSpeakerVariant = gobeep86.Variant

const (
	PCSpeakerVariantClean        = gobeep86.VariantClean
	PCSpeakerVariantSmallSpeaker = gobeep86.VariantSmallSpeaker
	PCSpeakerVariantPiezo        = gobeep86.VariantPiezo
)

func ParsePCSpeakerVariant(s string) PCSpeakerVariant {
	return gobeep86.ParseVariant(s)
}

func NewPCSpeakerPlayer(volume float64, variant PCSpeakerVariant) *PCSpeakerPlayer {
	ctx := sharedOrNewAudioContext(music.OutputSampleRate)
	if ctx == nil {
		return nil
	}
	src := gobeep86.NewSource(variant)
	src.SetGain(volume)
	ap, err := ctx.NewPlayer(src)
	if err != nil {
		return nil
	}
	ap.SetBufferSize(pcSpeakerPlayerBufferDuration())
	return &PCSpeakerPlayer{player: ap, src: src, volume: clampVolume(volume)}
}

func (p *PCSpeakerPlayer) Play(seq []sound.PCSpeakerTone) {
	if p == nil || p.player == nil || len(seq) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tones := toLibTones(seq)
	if p.src.MusicIsActive() {
		p.src.SetEffectMixed(tones, music.OutputSampleRate, 140)
		p.player.SetVolume(p.volume)
		if !p.player.IsPlaying() {
			p.player.Play()
		}
		return
	}
	p.player.Pause()
	p.src.Load(tones, music.OutputSampleRate)
	if err := p.player.Rewind(); err != nil {
		return
	}
	p.player.SetVolume(p.volume)
	p.player.Play()
}

func (p *PCSpeakerPlayer) SetMusic(seq []sound.PCSpeakerTone, tickRate int, loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.SetMusic(toLibTones(seq), music.OutputSampleRate, tickRate, loop)
	if err := p.player.Rewind(); err != nil {
		return
	}
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) SetMusicPCM(pcm []byte, loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.SetMusicPCM(pcm, music.OutputSampleRate, loop)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) BeginMusicPCM(loop bool) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.BeginMusicPCM(music.OutputSampleRate, loop)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) AppendMusicPCM(pcm []byte) {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.AppendMusicPCM(pcm)
	p.player.SetVolume(p.volume)
	if !p.player.IsPlaying() {
		p.player.Play()
	}
}

func (p *PCSpeakerPlayer) FinishMusicPCM() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.FinishMusicPCM()
}

func (p *PCSpeakerPlayer) BufferedMusicPCMBytes() int {
	if p == nil || p.player == nil {
		return 0
	}
	return p.src.BufferedMusicPCMBytes()
}

func (p *PCSpeakerPlayer) ClearMusic() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.src.ClearMusic()
}

func (p *PCSpeakerPlayer) SetEffectVolume(v float64) { p.SetVolume(v) }
func (p *PCSpeakerPlayer) SetMusicVolume(v float64)  { p.SetVolume(v) }

func (p *PCSpeakerPlayer) Stop() {
	if p == nil || p.player == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.player.Pause()
}

func (p *PCSpeakerPlayer) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.player == nil {
		return nil
	}
	p.player.Pause()
	err := p.player.Close()
	p.player = nil
	p.src = nil
	return err
}

func (p *PCSpeakerPlayer) SetVolume(v float64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = clampVolume(v)
	if p.src != nil {
		p.src.SetGain(p.volume)
	}
	if p.player != nil {
		p.player.SetVolume(p.volume)
	}
}

func InterleavePCSpeakerSequences(effectSeq []sound.PCSpeakerTone, effectTickRate int, musicSeq []sound.PCSpeakerTone, musicTickRate int) ([]sound.PCSpeakerTone, int) {
	out, tickRate := gobeep86.InterleaveSequences(toLibTones(effectSeq), effectTickRate, toLibTones(musicSeq), musicTickRate)
	return fromLibTones(out), tickRate
}

func RenderPCSpeakerSequenceToPCM(seq []sound.PCSpeakerTone, tickRate int, variant PCSpeakerVariant) ([]int16, error) {
	return gobeep86.RenderSequenceToPCM(toLibTones(seq), tickRate, variant)
}

func RenderMixedPCSpeakerSequencesToPCM(effectSeq []sound.PCSpeakerTone, effectTickRate int, musicSeq []sound.PCSpeakerTone, musicTickRate int, variant PCSpeakerVariant) ([]int16, error) {
	return gobeep86.RenderMixedSequencesToPCM(toLibTones(effectSeq), effectTickRate, toLibTones(musicSeq), musicTickRate, variant)
}

func toLibTones(seq []sound.PCSpeakerTone) []gobeep86.Tone {
	if len(seq) == 0 {
		return nil
	}
	out := make([]gobeep86.Tone, len(seq))
	for i, tone := range seq {
		out[i] = gobeep86.Tone{Active: tone.Active, Divisor: tone.ToneDivisor()}
	}
	return out
}

func fromLibTones(seq []gobeep86.Tone) []sound.PCSpeakerTone {
	if len(seq) == 0 {
		return nil
	}
	out := make([]sound.PCSpeakerTone, len(seq))
	for i, tone := range seq {
		out[i] = sound.PCSpeakerTone{Active: tone.Active, Divisor: tone.Divisor}
	}
	return out
}
