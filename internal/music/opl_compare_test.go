//go:build cgo

package music

import (
	"math"
	"os"
	"strconv"
	"testing"

	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

type oplTraceKind uint8

const (
	oplTraceWrite oplTraceKind = iota
	oplTraceGenerate
)

type oplTraceOp struct {
	kind   oplTraceKind
	addr   uint16
	value  uint8
	frames int
}

type traceOPL struct {
	ops []oplTraceOp
}

func (o *traceOPL) Reset() {
	o.ops = o.ops[:0]
}

func (o *traceOPL) WriteReg(addr uint16, value uint8) {
	o.ops = append(o.ops, oplTraceOp{kind: oplTraceWrite, addr: addr, value: value})
}

func (o *traceOPL) GenerateStereoS16(frames int) []int16 {
	o.ops = append(o.ops, oplTraceOp{kind: oplTraceGenerate, frames: frames})
	return nil
}

func (o *traceOPL) GenerateMonoU8(frames int) []byte {
	return nil
}

type fixedPatchBank struct {
	patch Patch
}

func (b fixedPatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	return b.patch
}

type fixedVoicePatchBank struct {
	voices []NotePatch
}

func (b fixedVoicePatchBank) Patch(program uint8, percussion bool, note uint8) Patch {
	if len(b.voices) == 0 {
		return DefaultPatchBank{}.Patch(program, percussion, note)
	}
	return b.voices[0].Patch
}

func (b fixedVoicePatchBank) PatchVoices(program uint8, percussion bool, note uint8) []NotePatch {
	return append([]NotePatch(nil), b.voices...)
}

type oplSimilarityThresholds struct {
	maxOnsetFrames int
	minEnvCorr     float64
	maxEnvErr      float64
	minMidCorr     float64
	minTailCorr    float64
	maxBandErr     float64
}

type oplSimilarityMetrics struct {
	onsetFrames int
	envCorr     float64
	envErr      float64
	midCorr     float64
	tailCorr    float64
	bandErr     float64
}

type fftTimbreMetrics struct {
	logMagErr          float64
	spectralCorr       float64
	centroidDeltaHz    float64
	highBandRatioDelta float64
}

type oplLevelMetrics struct {
	rmsRatio  float64
	peakRatio float64
	rmsDelta  float64
	peakDelta float64
}

type notePhrase struct {
	name    string
	channel uint8
	note    uint8
	events  []Event
}

type fftScoredPhrase struct {
	name    string
	metrics fftTimbreMetrics
}

func TestBasicOPL3MatchesNukedForMicroPatches(t *testing.T) {
	cases := []struct {
		name       string
		bank       PatchBank
		events     []Event
		thresholds oplSimilarityThresholds
	}{
		{
			name: "fm_voice",
			bank: fixedPatchBank{patch: Patch{
				Mod20: 0x21, Mod40: 0x08, Mod60: 0xF3, Mod80: 0x24, ModE0: 0x00,
				Car20: 0x01, Car40: 0x00, Car60: 0xF3, Car80: 0x24, CarE0: 0x00,
				C0: 0x00,
			}},
			events: []Event{
				{Type: EventProgramChange, Channel: 0, A: 0},
				{Type: EventNoteOn, Channel: 0, A: 60, B: 120},
				{Type: EventNoteOff, Channel: 0, A: 60, DeltaTics: 42},
				{Type: EventEnd, DeltaTics: 24},
			},
			thresholds: oplSimilarityThresholds{maxOnsetFrames: 64, minEnvCorr: 0.72, maxEnvErr: 0.22, minMidCorr: 0.60, minTailCorr: 0.55, maxBandErr: 0.28},
		},
		{
			name: "additive_double_voice",
			bank: fixedVoicePatchBank{voices: []NotePatch{
				{Patch: Patch{
					Mod20: 0x21, Mod40: 0x16, Mod60: 0xF2, Mod80: 0x24, ModE0: 0x00,
					Car20: 0x01, Car40: 0x00, Car60: 0xF2, Car80: 0x24, CarE0: 0x00,
					C0: 0x01,
				}},
				{Patch: Patch{
					Mod20: 0x21, Mod40: 0x20, Mod60: 0xF2, Mod80: 0x34, ModE0: 0x00,
					Car20: 0x01, Car40: 0x00, Car60: 0xF2, Car80: 0x34, CarE0: 0x00,
					C0: 0x01,
				}, BaseNoteOffset: 12},
			}},
			events: []Event{
				{Type: EventProgramChange, Channel: 0, A: 0},
				{Type: EventNoteOn, Channel: 0, A: 64, B: 110},
				{Type: EventNoteOff, Channel: 0, A: 64, DeltaTics: 36},
				{Type: EventEnd, DeltaTics: 20},
			},
			thresholds: oplSimilarityThresholds{maxOnsetFrames: 96, minEnvCorr: 0.65, maxEnvErr: 0.28, minMidCorr: 0.48, minTailCorr: 0.45, maxBandErr: 0.36},
		},
		{
			name: "bright_feedback_wave",
			bank: fixedPatchBank{patch: Patch{
				Mod20: 0x21, Mod40: 0x04, Mod60: 0xF4, Mod80: 0x22, ModE0: 0x07,
				Car20: 0x21, Car40: 0x00, Car60: 0xF4, Car80: 0x22, CarE0: 0x04,
				C0: 0x0E,
			}},
			events: []Event{
				{Type: EventProgramChange, Channel: 0, A: 0},
				{Type: EventNoteOn, Channel: 0, A: 72, B: 118},
				{Type: EventNoteOff, Channel: 0, A: 72, DeltaTics: 30},
				{Type: EventEnd, DeltaTics: 18},
			},
			thresholds: oplSimilarityThresholds{maxOnsetFrames: 96, minEnvCorr: 0.55, maxEnvErr: 0.36, minMidCorr: 0.35, minTailCorr: 0.25, maxBandErr: 0.62},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			trace := captureTraceForEvents(t, tc.bank, tc.events)
			basicPCM := renderTraceWithBackend(t, trace, sound.NewBasicOPL3(OutputSampleRate))
			nukedPCM := renderTraceWithBackend(t, trace, sound.NewNukedOPL3(OutputSampleRate))
			assertOPLSimilarity(t, tc.name, basicPCM, nukedPCM, tc.thresholds)
		})
	}
}

func TestBasicOPL3MatchesNukedOnReferenceDoomTracks(t *testing.T) {
	requireOPLTuningSuite(t)
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if lump, ok := wf.LumpByName("GENMIDI"); ok {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read GENMIDI: %v", err)
		}
		bank, err = ParseGENMIDIOP2PatchBank(data)
		if err != nil {
			t.Fatalf("parse GENMIDI: %v", err)
		}
	}

	cases := []struct {
		lump       string
		maxTics    uint32
		thresholds oplSimilarityThresholds
	}{
		{lump: "D_E1M1", maxTics: 140 * 5, thresholds: oplSimilarityThresholds{maxOnsetFrames: 96, minEnvCorr: 0.58, maxEnvErr: 0.27, minMidCorr: 0.45, minTailCorr: 0.30, maxBandErr: 0.34}},
		{lump: "D_E1M4", maxTics: 140 * 5, thresholds: oplSimilarityThresholds{maxOnsetFrames: 128, minEnvCorr: 0.44, maxEnvErr: 0.31, minMidCorr: 0.36, minTailCorr: 0.22, maxBandErr: 0.38}},
		{lump: "D_E1M8", maxTics: 140 * 5, thresholds: oplSimilarityThresholds{maxOnsetFrames: 128, minEnvCorr: 0.40, maxEnvErr: 0.35, minMidCorr: 0.24, minTailCorr: 0.16, maxBandErr: 0.42}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.lump, func(t *testing.T) {
			musLump, ok := wf.LumpByName(tc.lump)
			if !ok {
				t.Fatalf("missing %s lump", tc.lump)
			}
			musData, err := wf.LumpData(musLump)
			if err != nil {
				t.Fatalf("read %s: %v", tc.lump, err)
			}
			events, err := ParseMUS(musData)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.lump, err)
			}
			trace := captureTraceForEvents(t, bank, trimEventsToTics(events, tc.maxTics))
			basicPCM := renderTraceWithBackend(t, trace, sound.NewBasicOPL3(OutputSampleRate))
			nukedPCM := renderTraceWithBackend(t, trace, sound.NewNukedOPL3(OutputSampleRate))
			assertOPLSimilarity(t, tc.lump, basicPCM, nukedPCM, tc.thresholds)
		})
	}
}

func TestBasicOPL3MatchesNukedOnE1M1LeadPhrases(t *testing.T) {
	requireOPLTuningSuite(t)
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if lump, ok := wf.LumpByName("GENMIDI"); ok {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read GENMIDI: %v", err)
		}
		bank, err = ParseGENMIDIOP2PatchBank(data)
		if err != nil {
			t.Fatalf("parse GENMIDI: %v", err)
		}
	}

	lump, ok := wf.LumpByName("D_E1M1")
	if !ok {
		t.Fatal("missing D_E1M1 lump")
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		t.Fatalf("read D_E1M1: %v", err)
	}
	events, err := ParseMUS(data)
	if err != nil {
		t.Fatalf("parse D_E1M1: %v", err)
	}

	phrases := extractNotePhrases(events, 5, 48, 18)
	if len(phrases) == 0 {
		t.Fatal("no E1M1 note phrases extracted")
	}

	thresholds := oplSimilarityThresholds{
		maxOnsetFrames: 72,
		minEnvCorr:     0.72,
		maxEnvErr:      0.20,
		minMidCorr:     0.58,
		minTailCorr:    0.45,
		maxBandErr:     0.26,
	}

	for _, phrase := range phrases {
		phrase := phrase
		t.Run(phrase.name, func(t *testing.T) {
			trace := captureTraceForEvents(t, bank, phrase.events)
			basicPCM := renderTraceWithBackend(t, trace, sound.NewBasicOPL3(OutputSampleRate))
			nukedPCM := renderTraceWithBackend(t, trace, sound.NewNukedOPL3(OutputSampleRate))
			assertOPLSimilarity(t, phrase.name, basicPCM, nukedPCM, thresholds)
		})
	}
}

func TestBasicOPL3MatchesNukedOnE1M1LeadPhrasesFFT(t *testing.T) {
	requireOPLTuningSuite(t)
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if lump, ok := wf.LumpByName("GENMIDI"); ok {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read GENMIDI: %v", err)
		}
		bank, err = ParseGENMIDIOP2PatchBank(data)
		if err != nil {
			t.Fatalf("parse GENMIDI: %v", err)
		}
	}

	lump, ok := wf.LumpByName("D_E1M1")
	if !ok {
		t.Fatal("missing D_E1M1 lump")
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		t.Fatalf("read D_E1M1: %v", err)
	}
	events, err := ParseMUS(data)
	if err != nil {
		t.Fatalf("parse D_E1M1: %v", err)
	}

	phrases := extractNotePhrases(events, 6, 48, 18)
	if len(phrases) == 0 {
		t.Fatal("no E1M1 note phrases extracted")
	}

	var scored []fftScoredPhrase
	for _, phrase := range phrases {
		trace := captureTraceForEvents(t, bank, phrase.events)
		basicPCM := renderTraceWithBackend(t, trace, sound.NewBasicOPL3(OutputSampleRate))
		nukedPCM := renderTraceWithBackend(t, trace, sound.NewNukedOPL3(OutputSampleRate))
		metrics := computeFFTTimbreMetrics(basicPCM, nukedPCM, OutputSampleRate)
		scored = append(scored, fftScoredPhrase{name: phrase.name, metrics: metrics})
		t.Logf("%s fft logMagErr=%.3f spectralCorr=%.3f centroidDeltaHz=%.1f highBandDelta=%.3f",
			phrase.name, metrics.logMagErr, metrics.spectralCorr, metrics.centroidDeltaHz, metrics.highBandRatioDelta)
	}

	worst := worstFFTPhrase(scored)
	if worst.name != "" {
		t.Logf("worst fft phrase=%s logMagErr=%.3f spectralCorr=%.3f centroidDeltaHz=%.1f highBandDelta=%.3f",
			worst.name, worst.metrics.logMagErr, worst.metrics.spectralCorr, worst.metrics.centroidDeltaHz, worst.metrics.highBandRatioDelta)
	}

	for _, phrase := range scored {
		t.Run(phrase.name, func(t *testing.T) {
			if phrase.metrics.logMagErr > 0.22 {
				t.Fatalf("fft log magnitude error=%.3f want <= 0.22", phrase.metrics.logMagErr)
			}
			if phrase.metrics.spectralCorr < 0.90 {
				t.Fatalf("fft spectral correlation=%.3f want >= 0.90", phrase.metrics.spectralCorr)
			}
			if phrase.metrics.centroidDeltaHz > 700 {
				t.Fatalf("fft centroid delta=%.1fHz want <= 700Hz", phrase.metrics.centroidDeltaHz)
			}
			if phrase.metrics.highBandRatioDelta > 0.12 {
				t.Fatalf("fft high-band ratio delta=%.3f want <= 0.12", phrase.metrics.highBandRatioDelta)
			}
		})
	}
}

func TestBasicOPL3MatchesNukedOnReferenceNoteLevels(t *testing.T) {
	requireOPLTuningSuite(t)
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if lump, ok := wf.LumpByName("GENMIDI"); ok {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read GENMIDI: %v", err)
		}
		bank, err = ParseGENMIDIOP2PatchBank(data)
		if err != nil {
			t.Fatalf("parse GENMIDI: %v", err)
		}
	}

	for _, song := range []string{"D_E1M1", "D_E1M4", "D_E1M8"} {
		lump, ok := wf.LumpByName(song)
		if !ok {
			t.Fatalf("missing %s lump", song)
		}
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read %s: %v", song, err)
		}
		events, err := ParseMUS(data)
		if err != nil {
			t.Fatalf("parse %s: %v", song, err)
		}

		t.Run(song, func(t *testing.T) {
			phrases := extractNotePhrases(events, 6, 48, 18)
			if len(phrases) == 0 {
				t.Fatalf("no note phrases extracted from %s", song)
			}
			for _, phrase := range phrases {
				phrase := phrase
				t.Run(phrase.name, func(t *testing.T) {
					trace := captureTraceForEvents(t, bank, phrase.events)
					basicPCM := renderTraceWithBackend(t, trace, sound.NewBasicOPL3(OutputSampleRate))
					nukedPCM := renderTraceWithBackend(t, trace, sound.NewNukedOPL3(OutputSampleRate))
					metrics := computeOPLLevelMetrics(basicPCM, nukedPCM)
					t.Logf("%s rmsRatio=%.3f peakRatio=%.3f rmsDelta=%.1f peakDelta=%.1f",
						phrase.name, metrics.rmsRatio, metrics.peakRatio, metrics.rmsDelta, metrics.peakDelta)
					if math.Abs(metrics.rmsRatio-1.0) > 0.18 {
						t.Fatalf("rmsRatio=%.3f want within 0.18 of 1.0", metrics.rmsRatio)
					}
					if math.Abs(metrics.peakRatio-1.0) > 0.18 {
						t.Fatalf("peakRatio=%.3f want within 0.18 of 1.0", metrics.peakRatio)
					}
				})
			}
		})
	}
}

func captureTraceForEvents(t *testing.T, bank PatchBank, events []Event) []oplTraceOp {
	t.Helper()
	d := NewOutputDriver(bank)
	tracer := &traceOPL{}
	d.opl = tracer
	d.Reset()
	_ = d.Render(events)
	return append([]oplTraceOp(nil), tracer.ops...)
}

func computeOPLLevelMetrics(a []int16, b []int16) oplLevelMetrics {
	rmsA := rmsPCM(a)
	rmsB := rmsPCM(b)
	peakA := peakPCM(a)
	peakB := peakPCM(b)
	return oplLevelMetrics{
		rmsRatio:  safeRatio(rmsA, rmsB),
		peakRatio: safeRatio(peakA, peakB),
		rmsDelta:  rmsA - rmsB,
		peakDelta: peakA - peakB,
	}
}

func renderTraceWithBackend(t *testing.T, trace []oplTraceOp, opl sound.OPL3) []int16 {
	t.Helper()
	opl.Reset()
	var pcm []int16
	for _, op := range trace {
		switch op.kind {
		case oplTraceWrite:
			opl.WriteReg(op.addr, op.value)
		case oplTraceGenerate:
			pcm = append(pcm, opl.GenerateStereoS16(op.frames)...)
		default:
			t.Fatalf("unknown trace op kind=%d", op.kind)
		}
	}
	return pcm
}

func trimEventsToTics(events []Event, maxTics uint32) []Event {
	if len(events) == 0 || maxTics == 0 {
		return []Event{{Type: EventEnd}}
	}
	var total uint32
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		if total+ev.DeltaTics > maxTics {
			trimmed := ev
			trimmed.DeltaTics = maxTics - total
			if trimmed.DeltaTics > 0 {
				out = append(out, trimmed)
			}
			break
		}
		out = append(out, ev)
		total += ev.DeltaTics
		if ev.Type == EventEnd || total >= maxTics {
			break
		}
	}
	if len(out) == 0 || out[len(out)-1].Type != EventEnd {
		out = append(out, Event{Type: EventEnd})
	}
	return out
}

func extractNotePhrases(events []Event, count int, maxNoteTics uint32, releaseTailTics uint32) []notePhrase {
	if len(events) == 0 || count <= 0 {
		return nil
	}

	type channelSnapshot struct {
		hasProgram   bool
		program      Event
		controls     map[uint8]Event
		hasPitchBend bool
		pitchBend    Event
	}

	snapshots := [16]channelSnapshot{}
	for i := range snapshots {
		snapshots[i].controls = map[uint8]Event{}
	}
	absTics := make([]uint32, len(events))
	var runningTic uint32
	for i, ev := range events {
		runningTic += ev.DeltaTics
		absTics[i] = runningTic
	}

	var phrases []notePhrase

	for idx, ev := range events {
		absTic := absTics[idx]
		ch := ev.Channel & 0x0F

		switch ev.Type {
		case EventProgramChange:
			cp := ev
			cp.DeltaTics = 0
			s := &snapshots[ch]
			s.hasProgram = true
			s.program = cp
		case EventControlChange:
			cp := ev
			cp.DeltaTics = 0
			snapshots[ch].controls[ev.A] = cp
		case EventPitchBend:
			cp := ev
			cp.DeltaTics = 0
			s := &snapshots[ch]
			s.hasPitchBend = true
			s.pitchBend = cp
		}

		if ev.Type != EventNoteOn || ev.B == 0 || isPercussionChannel(ev.Channel) {
			continue
		}

		phraseEvents := make([]Event, 0, 8)
		if s := snapshots[ch]; s.hasProgram {
			phraseEvents = append(phraseEvents, s.program)
			for _, ctl := range []uint8{controllerVol, controllerExpr, controllerPan} {
				if control, ok := s.controls[ctl]; ok {
					phraseEvents = append(phraseEvents, control)
				}
			}
			if s.hasPitchBend {
				phraseEvents = append(phraseEvents, s.pitchBend)
			}
		}

		startTic := absTic
		noteOn := ev
		noteOn.DeltaTics = 0
		phraseEvents = append(phraseEvents, noteOn)
		lastKeptTic := startTic
		noteEnded := false

		for j := idx + 1; j < len(events); j++ {
			next := events[j]
			absNext := absTics[j]
			if absNext-startTic > maxNoteTics {
				break
			}
			if next.Channel&0x0F != ch {
				continue
			}
			switch next.Type {
			case EventControlChange, EventPitchBend:
				cp := next
				cp.DeltaTics = absNext - lastKeptTic
				phraseEvents = append(phraseEvents, cp)
				lastKeptTic = absNext
			case EventNoteOff:
				if next.A != ev.A {
					continue
				}
				cp := next
				cp.DeltaTics = absNext - lastKeptTic
				phraseEvents = append(phraseEvents, cp)
				lastKeptTic = absNext
				noteEnded = true
			case EventNoteOn:
				if next.B == 0 || next.A != ev.A {
					continue
				}
				cp := Event{Type: EventNoteOff, Channel: next.Channel, A: next.A, DeltaTics: absNext - lastKeptTic}
				phraseEvents = append(phraseEvents, cp)
				lastKeptTic = absNext
				noteEnded = true
			}
			if noteEnded {
				break
			}
		}

		if !noteEnded {
			phraseEvents = append(phraseEvents, Event{Type: EventNoteOff, Channel: ev.Channel, A: ev.A, DeltaTics: maxNoteTics - (lastKeptTic - startTic)})
			lastKeptTic = startTic + maxNoteTics
		}
		phraseEvents = append(phraseEvents, Event{Type: EventEnd, DeltaTics: releaseTailTics})

		phrases = append(phrases, notePhrase{
			name:    phraseName(len(phrases)+1, ch, ev.A),
			channel: ch,
			note:    ev.A,
			events:  phraseEvents,
		})
		if len(phrases) >= count {
			break
		}
	}

	return phrases
}

func phraseName(index int, ch uint8, note uint8) string {
	return "phrase_" + strconv.Itoa(index) + "_ch" + strconv.Itoa(int(ch)) + "_n" + strconv.Itoa(int(note))
}

func requireOPLTuningSuite(t *testing.T) {
	t.Helper()
	if os.Getenv("GD_DOOM_OPL_TUNING") == "" {
		t.Skip("set GD_DOOM_OPL_TUNING=1 to run OPL tuning comparison suites")
	}
}

func assertOPLSimilarity(t *testing.T, name string, basicPCM []int16, nukedPCM []int16, thresholds oplSimilarityThresholds) {
	t.Helper()
	metrics := computeOPLSimilarity(basicPCM, nukedPCM, OutputSampleRate)
	t.Logf("%s onset=%d envCorr=%.3f envErr=%.3f midCorr=%.3f tailCorr=%.3f bandErr=%.3f", name, metrics.onsetFrames, metrics.envCorr, metrics.envErr, metrics.midCorr, metrics.tailCorr, metrics.bandErr)
	if metrics.onsetFrames > thresholds.maxOnsetFrames {
		t.Fatalf("onset delta=%d want <= %d", metrics.onsetFrames, thresholds.maxOnsetFrames)
	}
	if metrics.envCorr < thresholds.minEnvCorr {
		t.Fatalf("envCorr=%.3f want >= %.3f", metrics.envCorr, thresholds.minEnvCorr)
	}
	if metrics.envErr > thresholds.maxEnvErr {
		t.Fatalf("envErr=%.3f want <= %.3f", metrics.envErr, thresholds.maxEnvErr)
	}
	if metrics.midCorr < thresholds.minMidCorr {
		t.Fatalf("midCorr=%.3f want >= %.3f", metrics.midCorr, thresholds.minMidCorr)
	}
	if metrics.tailCorr < thresholds.minTailCorr {
		t.Fatalf("tailCorr=%.3f want >= %.3f", metrics.tailCorr, thresholds.minTailCorr)
	}
	if metrics.bandErr > thresholds.maxBandErr {
		t.Fatalf("bandErr=%.3f want <= %.3f", metrics.bandErr, thresholds.maxBandErr)
	}
}

func computeOPLSimilarity(a []int16, b []int16, sampleRate int) oplSimilarityMetrics {
	monoA := monoFromStereoPCM(a)
	monoB := monoFromStereoPCM(b)
	envA := normalizeSeries(windowedRMS(monoA, 256))
	envB := normalizeSeries(windowedRMS(monoB, 256))
	midA := midSeries(envA)
	midB := midSeries(envB)
	tailA := tailSeries(envA)
	tailB := tailSeries(envB)
	bands := []float64{110, 220, 440, 880, 1760, 3520, 7040}
	return oplSimilarityMetrics{
		onsetFrames: absInt(onsetFrame(monoA) - onsetFrame(monoB)),
		envCorr:     correlation(envA, envB),
		envErr:      meanAbsDiff(envA, envB),
		midCorr:     correlation(midA, midB),
		tailCorr:    correlation(tailA, tailB),
		bandErr:     meanAbsDiff(normalizeSeries(goertzelBandEnergies(monoA, sampleRate, bands)), normalizeSeries(goertzelBandEnergies(monoB, sampleRate, bands))),
	}
}

func computeFFTTimbreMetrics(a []int16, b []int16, sampleRate int) fftTimbreMetrics {
	const fftWindow = 2048
	windowA := fftAnalysisWindow(monoFromStereoPCM(a), fftWindow)
	windowB := fftAnalysisWindow(monoFromStereoPCM(b), fftWindow)
	specA := normalizedLogSpectrum(windowA)
	specB := normalizedLogSpectrum(windowB)
	return fftTimbreMetrics{
		logMagErr:          meanAbsDiff(specA, specB),
		spectralCorr:       correlation(specA, specB),
		centroidDeltaHz:    math.Abs(spectralCentroidHz(windowA, sampleRate) - spectralCentroidHz(windowB, sampleRate)),
		highBandRatioDelta: math.Abs(highBandEnergyRatio(windowA, sampleRate) - highBandEnergyRatio(windowB, sampleRate)),
	}
}

func rmsPCM(p []int16) float64 {
	if len(p) == 0 {
		return 0
	}
	var sum float64
	for _, v := range p {
		x := float64(v)
		sum += x * x
	}
	return math.Sqrt(sum / float64(len(p)))
}

func peakPCM(p []int16) float64 {
	peak := 0.0
	for _, v := range p {
		x := math.Abs(float64(v))
		if x > peak {
			peak = x
		}
	}
	return peak
}

func safeRatio(a float64, b float64) float64 {
	if b == 0 {
		if a == 0 {
			return 1
		}
		return a
	}
	return a / b
}

func fftAnalysisWindow(mono []float64, n int) []float64 {
	if len(mono) == 0 || n <= 0 {
		return nil
	}
	start := onsetFrame(mono) + n/4
	if start < 0 {
		start = 0
	}
	if start+n > len(mono) {
		start = len(mono) - n
	}
	if start < 0 {
		start = 0
	}
	window := make([]float64, n)
	for i := 0; i < n; i++ {
		idx := start + i
		if idx >= len(mono) {
			break
		}
		// Hann window to reduce leakage and make timbre comparison more stable.
		w := 0.5 - 0.5*math.Cos((2*math.Pi*float64(i))/float64(n-1))
		window[i] = mono[idx] * w
	}
	return window
}

func normalizedLogSpectrum(samples []float64) []float64 {
	mags := dftMagnitudes(samples)
	if len(mags) == 0 {
		return nil
	}
	spec := make([]float64, len(mags))
	for i, m := range mags {
		spec[i] = math.Log1p(m)
	}
	return normalizeSeries(spec)
}

func dftMagnitudes(samples []float64) []float64 {
	n := len(samples)
	if n == 0 {
		return nil
	}
	bins := n / 2
	out := make([]float64, bins)
	for k := 0; k < bins; k++ {
		var re, im float64
		for t, s := range samples {
			angle := -2 * math.Pi * float64(k*t) / float64(n)
			re += s * math.Cos(angle)
			im += s * math.Sin(angle)
		}
		out[k] = math.Hypot(re, im)
	}
	return out
}

func spectralCentroidHz(samples []float64, sampleRate int) float64 {
	mags := dftMagnitudes(samples)
	if len(mags) == 0 || sampleRate <= 0 {
		return 0
	}
	var weighted, total float64
	for k, mag := range mags {
		freq := float64(k*sampleRate) / float64(len(samples))
		weighted += freq * mag
		total += mag
	}
	if total == 0 {
		return 0
	}
	return weighted / total
}

func highBandEnergyRatio(samples []float64, sampleRate int) float64 {
	mags := dftMagnitudes(samples)
	if len(mags) == 0 || sampleRate <= 0 {
		return 0
	}
	var high, total float64
	for k, mag := range mags {
		freq := float64(k*sampleRate) / float64(len(samples))
		total += mag
		if freq >= 2000 {
			high += mag
		}
	}
	if total == 0 {
		return 0
	}
	return high / total
}

func worstFFTPhrase(phrases []fftScoredPhrase) fftScoredPhrase {
	var zero fftScoredPhrase
	if len(phrases) == 0 {
		return zero
	}
	worst := phrases[0]
	worstScore := phrases[0].metrics.logMagErr + (1 - phrases[0].metrics.spectralCorr) + phrases[0].metrics.highBandRatioDelta
	for _, phrase := range phrases[1:] {
		score := phrase.metrics.logMagErr + (1 - phrase.metrics.spectralCorr) + phrase.metrics.highBandRatioDelta
		if score > worstScore {
			worst = phrase
			worstScore = score
		}
	}
	return worst
}

func monoFromStereoPCM(pcm []int16) []float64 {
	if len(pcm) == 0 {
		return nil
	}
	out := make([]float64, len(pcm)/2)
	for i := 0; i < len(out); i++ {
		out[i] = float64(int(pcm[i*2])+int(pcm[i*2+1])) / 65534.0
	}
	return out
}

func onsetFrame(mono []float64) int {
	if len(mono) == 0 {
		return 0
	}
	maxAbs := 0.0
	for _, sample := range mono {
		if abs := math.Abs(sample); abs > maxAbs {
			maxAbs = abs
		}
	}
	threshold := maxAbs * 0.08
	for i, sample := range mono {
		if math.Abs(sample) >= threshold {
			return i
		}
	}
	return len(mono)
}

func windowedRMS(mono []float64, window int) []float64 {
	if window <= 0 || len(mono) == 0 {
		return nil
	}
	count := len(mono) / window
	if count == 0 {
		return nil
	}
	out := make([]float64, count)
	for i := 0; i < count; i++ {
		var sum float64
		base := i * window
		for j := 0; j < window; j++ {
			v := mono[base+j]
			sum += v * v
		}
		out[i] = math.Sqrt(sum / float64(window))
	}
	return out
}

func midSeries(in []float64) []float64 {
	if len(in) < 3 {
		return in
	}
	start := len(in) / 3
	end := (len(in) * 2) / 3
	if end <= start {
		return in
	}
	return in[start:end]
}

func tailSeries(in []float64) []float64 {
	if len(in) < 4 {
		return in
	}
	start := (len(in) * 3) / 4
	if start >= len(in) {
		start = len(in) - 1
	}
	return in[start:]
}

func normalizeSeries(in []float64) []float64 {
	if len(in) == 0 {
		return nil
	}
	maxV := 0.0
	for _, v := range in {
		if v > maxV {
			maxV = v
		}
	}
	out := make([]float64, len(in))
	if maxV == 0 {
		return out
	}
	for i, v := range in {
		out[i] = v / maxV
	}
	return out
}

func goertzelBandEnergies(mono []float64, sampleRate int, bands []float64) []float64 {
	if len(mono) == 0 || sampleRate <= 0 || len(bands) == 0 {
		return nil
	}
	start := onsetFrame(mono)
	if start < len(mono) {
		mono = mono[start:]
	}
	if len(mono) > 8192 {
		mono = mono[:8192]
	}
	out := make([]float64, len(bands))
	n := float64(len(mono))
	for i, freq := range bands {
		k := math.Round(0.5 + (n*freq)/float64(sampleRate))
		w := (2 * math.Pi / n) * k
		coeff := 2 * math.Cos(w)
		var s0, s1, s2 float64
		for _, sample := range mono {
			s0 = sample + coeff*s1 - s2
			s2 = s1
			s1 = s0
		}
		out[i] = s1*s1 + s2*s2 - coeff*s1*s2
	}
	return out
}

func correlation(a []float64, b []float64) float64 {
	n := minInt(len(a), len(b))
	if n == 0 {
		return 1
	}
	var dot, magA, magB float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 && magB == 0 {
		return 1
	}
	if magA == 0 || magB == 0 {
		return 1 - meanAbsDiff(a[:n], b[:n])
	}
	cosine := dot / math.Sqrt(magA*magB)

	var meanA, meanB float64
	for i := 0; i < n; i++ {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= float64(n)
	meanB /= float64(n)
	var num, denA, denB float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		num += da * db
		denA += da * da
		denB += db * db
	}
	if denA == 0 && denB == 0 {
		if cosine > 0 {
			return cosine
		}
		return 1
	}
	if denA == 0 || denB == 0 {
		if cosine > 0 {
			return cosine
		}
		return 1 - meanAbsDiff(a[:n], b[:n])
	}
	pearson := num / math.Sqrt(denA*denB)
	if cosine > pearson {
		return cosine
	}
	return pearson
}

func meanAbsDiff(a []float64, b []float64) float64 {
	n := minInt(len(a), len(b))
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += math.Abs(a[i] - b[i])
	}
	return sum / float64(n)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
