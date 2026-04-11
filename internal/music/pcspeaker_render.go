package music

import (
	"slices"

	"gddoom/internal/sound"
)

const pcSpeakerMusicSubstepsPerTick = 8
const pcSpeakerMusicTickRate = defaultTicRate * pcSpeakerMusicSubstepsPerTick

type nullSynth struct{}

func (nullSynth) Reset()                        {}
func (nullSynth) WriteReg(uint16, uint8)        {}
func (nullSynth) GenerateStereoS16(int) []int16 { return nil }
func (nullSynth) GenerateMonoU8(int) []byte     { return nil }

type pcSpeakerRenderer struct {
	driver  *Driver
	arpStep int
}

type pcSpeakerCandidate struct {
	velocity uint8
	order    int
	divisor  uint16
}

func RenderMUSToPCSpeaker(bank PatchBank, musData []byte) ([]sound.PCSpeakerTone, int, error) {
	parsed, err := ParseMUSData(musData)
	if err != nil {
		return nil, 0, err
	}
	seq, tickRate := RenderParsedMUSToPCSpeaker(bank, parsed)
	return seq, tickRate, nil
}

func RenderParsedMUSToPCSpeaker(bank PatchBank, parsed *ParsedMUS) ([]sound.PCSpeakerTone, int) {
	if parsed == nil {
		return nil, pcSpeakerMusicTickRate
	}
	r := newPCSpeakerRenderer(bank)
	out := make([]sound.PCSpeakerTone, 0, (parsed.estimatedPCMBytes/1260)*pcSpeakerMusicSubstepsPerTick)
	for _, ev := range parsed.events {
		for i := uint32(0); i < ev.DeltaTics; i++ {
			for step := 0; step < pcSpeakerMusicSubstepsPerTick; step++ {
				out = append(out, r.toneForSubTick(step))
			}
		}
		r.applyEvent(ev)
	}
	return out, pcSpeakerMusicTickRate
}

func newPCSpeakerRenderer(bank PatchBank) *pcSpeakerRenderer {
	if bank == nil {
		bank = DefaultPatchBank{}
	}
	d := &Driver{
		synth:        nullSynth{},
		sampleRate:   OutputSampleRate,
		ticRate:      defaultTicRate,
		musPanMax:    defaultMUSPanMax,
		outputGain:   DefaultOutputGain,
		bank:         bank,
		voices:       make([]voiceState, defaultVoices),
		freeList:     make([]int, 0, defaultVoices),
		allocList:    make([]int, 0, defaultVoices),
		allocScratch: make([]int, 0, defaultVoices),
	}
	for i := range d.voices {
		d.voices[i].synthCh = i
		d.freeList = append(d.freeList, i)
	}
	for i := range d.ch {
		d.ch[i] = channelState{
			volume:     defaultChanVol,
			expression: defaultChanExpr,
			pan:        defaultChanPan,
		}
	}
	d.Reset()
	return &pcSpeakerRenderer{driver: d}
}

func (r *pcSpeakerRenderer) applyEvent(ev Event) {
	if r == nil || r.driver == nil {
		return
	}
	r.driver.applyEvent(ev)
}

func (r *pcSpeakerRenderer) toneForSubTick(subTick int) sound.PCSpeakerTone {
	candidates := r.interleaveCandidates()
	if len(candidates) == 0 {
		return sound.PCSpeakerTone{}
	}
	pattern := comboPattern(candidates)
	if len(pattern) == 0 {
		return sound.PCSpeakerTone{}
	}
	idx := pattern[r.arpStep%len(pattern)]
	r.arpStep++
	divisor := candidates[idx].divisor
	if divisor == 0 {
		return sound.PCSpeakerTone{}
	}
	return sound.PCSpeakerTone{Active: true, Divisor: divisor}
}

func (r *pcSpeakerRenderer) interleaveCandidates() []pcSpeakerCandidate {
	if r == nil || r.driver == nil || len(r.driver.allocList) == 0 {
		return nil
	}
	candidates := make([]pcSpeakerCandidate, 0, len(r.driver.allocList))
	for order, vi := range r.driver.allocList {
		v := r.driver.voices[vi]
		if !v.active || v.velocity == 0 {
			continue
		}
		divisor := sound.PITDivisorForFrequency(oplFreqWordFrequency(v.freqWord))
		if divisor == 0 {
			continue
		}
		candidates = append(candidates, pcSpeakerCandidate{
			velocity: v.velocity,
			order:    order,
			divisor:  divisor,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b pcSpeakerCandidate) int {
		if a.velocity != b.velocity {
			if a.velocity > b.velocity {
				return -1
			}
			return 1
		}
		if a.order > b.order {
			return -1
		}
		if a.order < b.order {
			return 1
		}
		if a.divisor < b.divisor {
			return -1
		}
		if a.divisor > b.divisor {
			return 1
		}
		return 0
	})
	const maxInterleaveVoices = 4
	if len(candidates) > maxInterleaveVoices {
		candidates = candidates[:maxInterleaveVoices]
	}
	deduped := candidates[:0]
	seen := map[uint16]struct{}{}
	for _, c := range candidates {
		if _, ok := seen[c.divisor]; ok {
			continue
		}
		seen[c.divisor] = struct{}{}
		deduped = append(deduped, c)
	}
	slices.SortFunc(deduped, func(a, b pcSpeakerCandidate) int {
		if a.divisor != b.divisor {
			if a.divisor > b.divisor {
				return -1
			}
			return 1
		}
		if a.velocity != b.velocity {
			if a.velocity > b.velocity {
				return -1
			}
			return 1
		}
		if a.order > b.order {
			return -1
		}
		if a.order < b.order {
			return 1
		}
		return 0
	})
	return deduped
}

func oplFreqWordFrequency(freqWord uint16) float64 {
	fnum := float64(freqWord & 0x03ff)
	if fnum <= 0 {
		return 0
	}
	block := int((freqWord >> 10) & 0x07)
	return fnum * 49716.0 * float64(uint32(1)<<(block+2)) / float64(uint32(1)<<19)
}

func comboPattern(candidates []pcSpeakerCandidate) []int {
	switch len(candidates) {
	case 0:
		return nil
	case 1:
		return []int{0}
	case 2:
		return []int{0, 0, 1, 0, 0, 1}
	case 3:
		return []int{0, 0, 1, 0, 2, 1, 0, 0, 1}
	default:
		return []int{0, 0, 1, 0, 2, 1, 0, 3, 1, 0, 2, 1}
	}
}
