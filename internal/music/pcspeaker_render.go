package music

import (
	"math"
	"slices"

	"gddoom/internal/sound"
)

const pcSpeakerMusicSubstepsPerTick = 8
const pcSpeakerMusicTickRate = defaultTicRate * pcSpeakerMusicSubstepsPerTick
const pcSpeakerMinNote = 48
const pcSpeakerMaxNote = 84
const pcSpeakerInterleaveTargetHz = 65.41 // Note C2
const pcSpeakerInterleaveMinCycles = 1.0
const pcSpeakerInterleaveMaxHoldSubsteps = 1000

type nullSynth struct{}

func (nullSynth) Reset()                        {}
func (nullSynth) WriteReg(uint16, uint8)        {}
func (nullSynth) GenerateStereoS16(int) []int16 { return nil }
func (nullSynth) GenerateMonoU8(int) []byte     { return nil }

type pcSpeakerRenderer struct {
	driver            *Driver
	percussionPattern []uint16
	percussionStep    int
	activeNotes       map[uint16]pcSpeakerNoteState
	forceFront        int
	rotatePhase       int
	renderStep        uint64
}

type pcSpeakerNoteState struct {
	channel  uint8
	note     uint8
	velocity uint8
	program  uint8
	playNote uint8
	fineTune int16
	patch    Patch
	start    uint64
}

type pcSpeakerCandidate struct {
	start    uint64
	age      uint64
	audible  bool
	priority int
	velocity uint8
	order    int
	ch       uint8
	instr    uint8
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
	return &pcSpeakerRenderer{
		driver:      d,
		activeNotes: make(map[uint16]pcSpeakerNoteState, defaultVoices),
	}
}

func (r *pcSpeakerRenderer) applyEvent(ev Event) {
	if r == nil || r.driver == nil {
		return
	}
	if ev.Type == EventNoteOn && (ev.Channel&0x0f) == 9 && ev.B > 0 {
		r.percussionPattern = pcSpeakerPercussionPattern(ev.A, ev.B)
		r.percussionStep = 0
	}
	r.driver.applyEvent(ev)
	if ev.Type == EventNoteOn && ev.B > 0 && (ev.Channel&0x0f) != 9 {
		r.recordNoteOn(ev)
		r.forceFront = 1
		r.rotatePhase = 1
	} else if ev.Type == EventNoteOff && (ev.Channel&0x0f) != 9 {
		delete(r.activeNotes, pcSpeakerNoteKey(ev.Channel, ev.A))
	} else if ev.Type == EventControlChange && (ev.A == controllerAllSoundsOff || ev.A == controllerAllNotesOff) {
		r.clearChannelNotes(ev.Channel)
	}
}

func (r *pcSpeakerRenderer) toneForSubTick(subTick int) sound.PCSpeakerTone {
	defer func() {
		if r != nil {
			r.renderStep++
		}
	}()
	candidates := r.activeCandidates()
	candidate, ok := r.selectCandidate(candidates, subTick)
	if div, active := r.nextPercussionTone(); active {
		return sound.PCSpeakerTone{Active: true, Divisor: div}
	}
	if !ok {
		return sound.PCSpeakerTone{}
	}
	if candidate.divisor == 0 {
		return sound.PCSpeakerTone{}
	}
	return sound.PCSpeakerTone{Active: true, Divisor: candidate.divisor}
}

func (r *pcSpeakerRenderer) nextPercussionTone() (uint16, bool) {
	if r == nil || r.percussionStep >= len(r.percussionPattern) {
		return 0, false
	}
	div := r.percussionPattern[r.percussionStep]
	r.percussionStep++
	if r.percussionStep >= len(r.percussionPattern) {
		r.percussionPattern = nil
		r.percussionStep = 0
	}
	if div == 0 {
		return 0, false
	}
	return div, true
}

func (r *pcSpeakerRenderer) activeCandidates() []pcSpeakerCandidate {
	if r == nil || r.driver == nil || len(r.activeNotes) == 0 {
		return nil
	}
	candidates := make([]pcSpeakerCandidate, 0, len(r.activeNotes))
	for _, n := range r.activeNotes {
		start := n.start
		age := uint64(0)
		if r.renderStep > start {
			age = r.renderStep - start
		}
		audible := pcSpeakerNoteAudible(n, age)
		bend := r.driver.ch[n.channel&0x0f].pitchBend + n.fineTune
		freqWord := dmxFrequencyWord(int(n.playNote), bend)
		divisor := sound.PITDivisorForFrequency(dmxFrequencyWordToHz(freqWord))
		if divisor == 0 || divisor <= 16 || divisor >= 32768 {
			continue
		}
		candidates = append(candidates, pcSpeakerCandidate{
			start:    start,
			age:      age,
			audible:  audible,
			priority: pcSpeakerCandidatePriority(n, age, audible),
			velocity: n.velocity,
			order:    0,
			ch:       n.channel & 0x0f,
			instr:    0,
			note:     normalizePCSpeakerNote(n.playNote),
			divisor:  divisor,
		})
	}
	slices.SortFunc(candidates, func(a, b pcSpeakerCandidate) int {
		if a.audible != b.audible {
			if a.audible {
				return -1
			}
			return 1
		}
		if a.priority != b.priority {
			if a.priority > b.priority {
				return -1
			}
			return 1
		}
		if a.start != b.start {
			if a.start > b.start {
				return -1
			}
			return 1
		}
		if a.instr != b.instr {
			if a.instr < b.instr {
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

func (r *pcSpeakerRenderer) recordNoteOn(ev Event) {
	if r == nil || r.driver == nil || (ev.Channel&0x0f) == 9 || ev.B == 0 {
		return
	}
	prog := r.driver.ch[ev.Channel&0x0f].program
	patches := r.driver.voicePatches(prog, false, ev.A)
	if len(patches) == 0 {
		return
	}
	np := patches[0]
	r.activeNotes[pcSpeakerNoteKey(ev.Channel, ev.A)] = pcSpeakerNoteState{
		channel:  ev.Channel,
		note:     ev.A,
		velocity: ev.B,
		program:  prog,
		playNote: resolveVoiceNote(ev.A, false, np),
		fineTune: np.FineTune,
		patch:    np.Patch,
		start:    r.renderStep,
	}
}

func (r *pcSpeakerRenderer) clearChannelNotes(ch uint8) {
	if r == nil {
		return
	}
	target := ch & 0x0f
	for key, n := range r.activeNotes {
		if (n.channel & 0x0f) == target {
			delete(r.activeNotes, key)
		}
	}
}

func pcSpeakerNoteKey(ch, note uint8) uint16 {
	return uint16(ch&0x0f)<<8 | uint16(note)
}

func (r *pcSpeakerRenderer) topCandidate() (pcSpeakerCandidate, bool) {
	candidates := r.activeCandidates()
	if len(candidates) == 0 {
		return pcSpeakerCandidate{}, false
	}
	return candidates[0], true
}

func (r *pcSpeakerRenderer) selectCandidate(candidates []pcSpeakerCandidate, subTick int) (pcSpeakerCandidate, bool) {
	if len(candidates) == 0 {
		return pcSpeakerCandidate{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	pool := pcSpeakerPriorityPool(candidates)
	if r != nil && r.forceFront > 0 {
		r.forceFront--
		return candidates[0], true
	}
	slot := 0
	pattern := pcSpeakerInterleavePattern(len(pool))
	if r != nil {
		slot = pattern[r.interleavePhase(pool)%len(pattern)]
	} else {
		slot = pattern[subTick%len(pattern)]
	}
	return pool[slot], true
}

func pcSpeakerPriorityPool(candidates []pcSpeakerCandidate) []pcSpeakerCandidate {
	if len(candidates) <= 1 {
		return candidates
	}
	const prioritySlack = 420
	top := candidates[0].priority
	limit := len(candidates)
	if limit > 3 {
		limit = 3
	}
	count := 1
	for count < limit {
		if top-candidates[count].priority > prioritySlack {
			break
		}
		count++
	}
	return candidates[:count]
}

func pcSpeakerInterleavePattern(n int) []int {
	switch n {
	case 0:
		return []int{0}
	case 1:
		return []int{0}
	case 2:
		return []int{0, 1, 0, 1, 0, 1, 0, 1}
	default:
		return []int{0, 1, 0, 2, 0, 1, 0, 2}
	}
}

func (r *pcSpeakerRenderer) interleavePhase(pool []pcSpeakerCandidate) int {
	if r == nil {
		return 0
	}
	hold := pcSpeakerInterleaveHoldSubsteps(pool)
	if hold <= 0 {
		hold = 1
	}
	return int(r.renderStep / uint64(hold))
}

func pcSpeakerInterleaveHoldSubsteps(candidates []pcSpeakerCandidate) int {
	if len(candidates) == 0 {
		return 1
	}
	lowestHz := math.MaxFloat64
	for _, c := range candidates {
		if c.divisor == 0 {
			continue
		}
		hz := float64(sound.PCSpeakerPITHz()) / float64(c.divisor)
		if hz > 0 && hz < lowestHz {
			lowestHz = hz
		}
	}
	holdSeconds := 1.0 / pcSpeakerInterleaveTargetHz
	if lowestHz != math.MaxFloat64 {
		minSeconds := pcSpeakerInterleaveMinCycles / lowestHz
		if minSeconds > holdSeconds {
			holdSeconds = minSeconds
		}
	}
	hold := int(math.Ceil(holdSeconds * pcSpeakerMusicTickRate))
	if hold < 1 {
		hold = 1
	} else if hold > pcSpeakerInterleaveMaxHoldSubsteps {
		hold = pcSpeakerInterleaveMaxHoldSubsteps
	}
	return hold
}

func pcSpeakerNoteFrequency(note uint8, bend int16) float64 {
	semitones := float64(note) + float64(bend)/32.0
	return 440.0 * math.Pow(2, (semitones-69.0)/12.0)
}

func dmxFrequencyWordToHz(freqWord uint16) float64 {
	fnum := float64(freqWord & 0x03FF)
	if !(fnum > 0) {
		return 0
	}
	block := float64((freqWord >> 10) & 0x07)
	return fnum * 49716.0 * math.Exp2(block-20.0)
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

func pcSpeakerPercussionPattern(note, velocity uint8) []uint16 {
	scale := 0
	switch {
	case velocity >= 112:
		scale = -36
	case velocity >= 80:
		scale = -20
	case velocity <= 32:
		scale = 36
	case velocity <= 56:
		scale = 20
	}
	adjust := func(div uint16) uint16 {
		v := int(div) + scale
		if v < 24 {
			v = 24
		}
		return uint16(v)
	}
	emphasize := func(seq []uint16) []uint16 {
		out := make([]uint16, 0, len(seq)*2)
		for _, v := range seq {
			if v == 0 {
				out = append(out, 0)
				continue
			}
			// A single PC speaker voice has no real amplitude control, so make
			// percussion feel stronger by holding each hit for twice as long while
			// keeping the gaps short enough that melody can still reappear.
			out = append(out, v, v)
		}
		return out
	}
	switch {
	case note <= 36:
		return emphasize([]uint16{
			adjust(3600), adjust(3000), adjust(2400), adjust(1900),
			0, adjust(1500), adjust(1200), adjust(960),
			0, adjust(760),
		})
	case note <= 40:
		return emphasize([]uint16{
			adjust(2600), adjust(2100), adjust(1650), 0,
			adjust(1280), adjust(980), 0, adjust(760),
			0, adjust(620),
		})
	case note <= 45:
		return emphasize([]uint16{
			adjust(150), 0, adjust(96), adjust(132),
			0, adjust(84), 0, adjust(118),
			adjust(74), 0, adjust(104), 0,
		})
	case note <= 50:
		return emphasize([]uint16{
			adjust(700), adjust(560), adjust(430), 0,
			adjust(340), 0, adjust(300), adjust(250),
			0, adjust(210),
		})
	case note <= 55:
		return emphasize([]uint16{
			adjust(118), 0, adjust(86), 0,
			adjust(62), 0, adjust(94), 0,
			adjust(70), 0, adjust(54), 0,
		})
	case note <= 63:
		return emphasize([]uint16{
			adjust(92), 0, adjust(64), 0,
			adjust(48), 0, adjust(40), 0,
			adjust(58), 0, adjust(44), 0,
		})
	default:
		return emphasize([]uint16{
			adjust(54), 0, adjust(42), 0,
			adjust(32), 0, adjust(26), 0,
			adjust(36), 0, adjust(28), 0,
			adjust(24), 0,
		})
	}
}

func pcSpeakerNoteAudible(n pcSpeakerNoteState, age uint64) bool {
	maxAge := pcSpeakerAudibleSubsteps(n.patch, n.velocity)
	if maxAge == 0 {
		return false
	}
	return age <= maxAge
}

func pcSpeakerCandidatePriority(n pcSpeakerNoteState, age uint64, audible bool) int {
	score := 0
	if audible {
		score += 4000
	}

	score += pcSpeakerChannelPriority(n.channel)
	score += pcSpeakerInstrumentPriority(n)

	// Higher resolved notes usually carry the melody better on a single speaker.
	score += int(normalizePCSpeakerNote(n.playNote)) * 24
	score += int(n.velocity) * 8

	attack := int((n.patch.Car60 >> 4) & 0x0F)
	decay := int(n.patch.Car60 & 0x0F)
	sustain := int((n.patch.Car80 >> 4) & 0x0F)
	carrierLevel := int(n.patch.Car40 & 0x3F)

	// Favor notes with stronger carriers and usable sustain.
	score += (63 - carrierLevel) * 6
	score += (15 - sustain) * 18
	score += attack * 10
	score += decay * 4

	// Recency matters, but only as a modest interruption hint.
	recent := 24 - int(age)
	if recent > 0 {
		score += recent * 12
	}
	return score
}

func pcSpeakerChannelPriority(ch uint8) int {
	switch ch & 0x0f {
	case 0:
		return 220
	case 1:
		return 180
	case 2:
		return 140
	case 3:
		return 100
	case 4:
		return 60
	default:
		return 0
	}
}

func pcSpeakerInstrumentPriority(n pcSpeakerNoteState) int {
	carrierLevel := int(n.patch.Car40 & 0x3F)
	attack := int((n.patch.Car60 >> 4) & 0x0F)
	decay := int(n.patch.Car60 & 0x0F)
	sustain := int((n.patch.Car80 >> 4) & 0x0F)
	feedback := int(n.patch.C0 & 0x07)

	score := 0
	// Strong, bright carriers with a usable sustain tend to read as the lead.
	score += (63 - carrierLevel) * 4
	score += attack * 18
	score += (15 - sustain) * 14
	score += feedback * 6

	// Low notes are more likely accompaniment/bass; don't let them dominate.
	note := int(normalizePCSpeakerNote(n.playNote))
	if note < 60 {
		score -= (60 - note) * 10
	}
	// Very fast-decay high-sustain shapes often vanish quickly in the speaker reduction.
	if sustain >= 12 && decay >= 12 {
		score -= 120
	}
	// Strong attack with moderate decay is useful for lead/pluck presence.
	if attack >= 10 && sustain <= 8 {
		score += 160
	}
	return score
}

func pcSpeakerAudibleSubsteps(p Patch, velocity uint8) uint64 {
	carrierLevel := p.Car40 & 0x3F
	if carrierLevel >= 0x3F {
		return 0
	}
	attackRate := (p.Car60 >> 4) & 0x0F
	decayRate := p.Car60 & 0x0F
	sustainLevel := (p.Car80 >> 4) & 0x0F

	// Instruments with a meaningful sustain should remain eligible until note-off.
	if sustainLevel <= 7 {
		return ^uint64(0)
	}

	// Estimate the audible post-attack span from the carrier envelope itself.
	// Higher sustain attenuates sooner; faster decay shortens the useful tail.
	audible := 8 +
		int(15-attackRate) +
		int(15-decayRate)*3 +
		int(15-sustainLevel)*6 +
		int(velocity)/6

	switch {
	case sustainLevel >= 15:
		audible -= 18
	case sustainLevel >= 13:
		audible -= 10
	case sustainLevel >= 10:
		audible -= 4
	}
	if decayRate >= 14 {
		audible -= 8
	} else if decayRate >= 12 {
		audible -= 4
	}
	if audible < 6 {
		audible = 6
	}
	return uint64(audible)
}
