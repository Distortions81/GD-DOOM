package music

import (
	"fmt"

	"github.com/sinshu/go-meltysynth/meltysynth"
)

type eventRenderer interface {
	Reset()
	ApplyEvent(Event)
	GenerateStereoS16(frames int) []int16
	SampleRate() int
	TicRate() int
	SetMUSPanMax(float64)
	SetOutputGain(float64)
	SetPreEmphasis(bool)
	RenderMUSS16LE(musData []byte) ([]byte, error)
}

type MeltySynthDriver struct {
	soundFont  *SoundFontBank
	sampleRate int
	ticRate    int
	outputGain float64
	synth      *meltysynth.Synthesizer
	left       []float32
	right      []float32
	pcm        []int16
}

func NewMeltySynthDriver(sampleRate int, soundFont *SoundFontBank) (*MeltySynthDriver, error) {
	if soundFont == nil || soundFont.font == nil {
		return nil, fmt.Errorf("music: meltysynth backend requires a soundfont")
	}
	if sampleRate <= 0 {
		sampleRate = OutputSampleRate
	}
	d := &MeltySynthDriver{
		soundFont:  soundFont,
		sampleRate: sampleRate,
		ticRate:    defaultTicRate,
		outputGain: DefaultOutputGain,
	}
	d.Reset()
	return d, nil
}

func (d *MeltySynthDriver) Reset() {
	if d == nil || d.soundFont == nil || d.soundFont.font == nil {
		return
	}
	if d.synth != nil && d.synth.SoundFont == d.soundFont.font && int(d.synth.SampleRate) == d.sampleRate {
		d.synth.Reset()
		return
	}
	settings := meltysynth.NewSynthesizerSettings(int32(d.sampleRate))
	synth, err := meltysynth.NewSynthesizer(d.soundFont.font, settings)
	if err != nil {
		d.synth = nil
		return
	}
	d.synth = synth
}

func (d *MeltySynthDriver) ApplyEvent(ev Event) {
	if d == nil || d.synth == nil {
		return
	}
	channel := int32(ev.Channel & 0x0F)
	switch ev.Type {
	case EventProgramChange:
		d.synth.ProcessMidiMessage(channel, 0xC0, int32(ev.A), 0)
	case EventControlChange:
		d.synth.ProcessMidiMessage(channel, 0xB0, int32(ev.A), int32(ev.B))
	case EventPitchBend:
		d.synth.ProcessMidiMessage(channel, 0xE0, int32(ev.A), int32(ev.B))
	case EventNoteOn:
		d.synth.ProcessMidiMessage(channel, 0x90, int32(ev.A), int32(ev.B))
	case EventNoteOff:
		d.synth.ProcessMidiMessage(channel, 0x80, int32(ev.A), 0)
	}
}

func (d *MeltySynthDriver) GenerateStereoS16(frames int) []int16 {
	if d == nil || d.synth == nil || frames <= 0 {
		return nil
	}
	if cap(d.left) < frames {
		d.left = make([]float32, frames)
		d.right = make([]float32, frames)
	} else {
		d.left = d.left[:frames]
		d.right = d.right[:frames]
		for i := range d.left {
			d.left[i] = 0
			d.right[i] = 0
		}
	}
	d.synth.Render(d.left, d.right)
	need := frames * 2
	if cap(d.pcm) < need {
		d.pcm = make([]int16, need)
	} else {
		d.pcm = d.pcm[:need]
	}
	gain := clampOutputGain(d.outputGain)
	for i := 0; i < frames; i++ {
		d.pcm[i*2] = float32ToS16(d.left[i] * float32(gain))
		d.pcm[i*2+1] = float32ToS16(d.right[i] * float32(gain))
	}
	return d.pcm
}

func (d *MeltySynthDriver) SampleRate() int {
	if d == nil || d.sampleRate <= 0 {
		return OutputSampleRate
	}
	return d.sampleRate
}

func (d *MeltySynthDriver) TicRate() int {
	if d == nil || d.ticRate <= 0 {
		return defaultTicRate
	}
	return d.ticRate
}

func (d *MeltySynthDriver) SetMUSPanMax(float64) {}

func (d *MeltySynthDriver) SetOutputGain(gain float64) {
	if d == nil {
		return
	}
	d.outputGain = clampOutputGain(gain)
}

func (d *MeltySynthDriver) SetPreEmphasis(bool) {}

func (d *MeltySynthDriver) RenderMUSS16LE(musData []byte) ([]byte, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, err
	}
	return d.RenderParsedMUSS16LE(parsed)
}

func (d *MeltySynthDriver) RenderParsedMUSS16LE(parsed *ParsedMUS) ([]byte, error) {
	return renderParsedMUSS16LE(d, parsed)
}

func float32ToS16(v float32) int16 {
	if v > 1 {
		v = 1
	} else if v < -1 {
		v = -1
	}
	if v >= 0 {
		return int16(v * 32767)
	}
	return int16(v * 32768)
}
