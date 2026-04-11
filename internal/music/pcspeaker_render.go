package music

import (
	"math"
	"slices"

	"gddoom/internal/sound"
)

const pcSpeakerMusicSubstepsPerTick = 8
const pcSpeakerMusicTickRate = defaultTicRate * pcSpeakerMusicSubstepsPerTick
const pcSpeakerPercussionBurstSubsteps = 1
const pcSpeakerPercussionDivisor = 96
const pcSpeakerMinNote = 48
const pcSpeakerMaxNote = 84
const pcSpeakerGapHoldSubsteps = 6

type nullSynth struct{}

func (nullSynth) Reset()                        {}
func (nullSynth) WriteReg(uint16, uint8)        {}
func (nullSynth) GenerateStereoS16(int) []int16 { return nil }
func (nullSynth) GenerateMonoU8(int) []byte     { return nil }

type pcSpeakerRenderer struct {
	driver          *Driver
	percussionBurst int
}

type pcSpeakerCandidate struct {
	velocity uint8
	order    int
	note     uint8
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
	if ev.Type == EventNoteOn && (ev.Channel&0x0f) == 9 && ev.B > 0 {
		r.percussionBurst = pcSpeakerPercussionBurstSubsteps
	}
	r.driver.applyEvent(ev)
}

func (r *pcSpeakerRenderer) toneForSubTick(subTick int) sound.PCSpeakerTone {
	candidate, ok := r.topCandidate()
	if r.percussionBurst > 0 {
		r.percussionBurst--
		if !ok {
			return sound.PCSpeakerTone{Active: true, Divisor: pcSpeakerPercussionDivisor}
		}
	}
	if !ok {
		return sound.PCSpeakerTone{}
	}
	if candidate.divisor == 0 {
		return sound.PCSpeakerTone{}
	}
	return sound.PCSpeakerTone{Active: true, Divisor: candidate.divisor}
}

func (r *pcSpeakerRenderer) activeCandidates() []pcSpeakerCandidate {
	if r == nil || r.driver == nil || len(r.driver.allocList) == 0 {
		return nil
	}
	candidates := make([]pcSpeakerCandidate, 0, len(r.driver.allocList))
	for order, vi := range r.driver.allocList {
		v := r.driver.voices[vi]
		if !v.active || v.velocity == 0 {
			continue
		}
		if (v.ch & 0x0f) == 9 {
			continue
		}
		note := normalizePCSpeakerNote(v.playNote)
		bend := r.driver.ch[v.ch&0x0f].pitchBend + v.fineTune
		divisor := sound.PITDivisorForFrequency(pcSpeakerNoteFrequency(note, bend))
		if divisor == 0 || divisor <= 16 || divisor >= 32768 {
			continue
		}
		candidates = append(candidates, pcSpeakerCandidate{
			velocity: v.velocity,
			order:    order,
			note:     note,
			divisor:  divisor,
		})
	}
	slices.SortFunc(candidates, func(a, b pcSpeakerCandidate) int {
		if a.order != b.order {
			if a.order > b.order {
				return -1
			}
			return 1
		}
		if a.note != b.note {
			if a.note > b.note {
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
		if a.divisor < b.divisor {
			return -1
		}
		if a.divisor > b.divisor {
			return 1
		}
		return 0
	})
	return candidates
}

func (r *pcSpeakerRenderer) topCandidate() (pcSpeakerCandidate, bool) {
	candidates := r.activeCandidates()
	if len(candidates) == 0 {
		return pcSpeakerCandidate{}, false
	}
	return candidates[0], true
}

func pcSpeakerNoteFrequency(note uint8, bend int16) float64 {
	semitones := float64(note) + float64(bend)/32.0
	return 440.0 * math.Pow(2, (semitones-69.0)/12.0)
}

func normalizePCSpeakerNote(note uint8) uint8 {
	for note < pcSpeakerMinNote {
		note += 12
	}
	for note > pcSpeakerMaxNote {
		note -= 12
	}
	return note
}
