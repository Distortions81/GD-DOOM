package sound

import extimpsynth "github.com/Distortions81/impsynth"

type ImpSynth = extimpsynth.Synth

func NewImpSynth(sampleRate int) *ImpSynth {
	return extimpsynth.New(sampleRate)
}
