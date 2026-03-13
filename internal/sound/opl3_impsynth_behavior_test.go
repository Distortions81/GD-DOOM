package sound

import (
	"math"
	"testing"
)

func TestImpSynthCarrierAttenuationTracksTLAndKSL(t *testing.T) {
	lowTL := measureCarrierEnvelopeOut(0x00, 0x20, 0x20)
	highTL := measureCarrierEnvelopeOut(0x20, 0x20, 0x20)
	if highTL <= lowTL {
		t.Fatalf("carrier attenuation with TL=%d want > TL=0 attenuation=%d", highTL, lowTL)
	}

	lowKSL := measureCarrierEnvelopeOut(0x40, 0x20, 0x20)
	highKSL := measureCarrierEnvelopeOut(0x40, 0xF0, 0x3B)
	if highKSL <= lowKSL {
		t.Fatalf("carrier attenuation with higher KSL=%d want > lower KSL=%d", highKSL, lowKSL)
	}
}

func TestImpSynthAttackDecayReleaseHaveExpectedShape(t *testing.T) {
	opl := newImpSynthTone(
		0x20, 0x01,
		0x23, 0x01,
		0x40, 0x3F,
		0x43, 0x00,
		0x60, 0xF2,
		0x63, 0xF2,
		0x80, 0x44,
		0x83, 0x44,
		0xC0, 0x30,
		0xA0, 0x98,
	)
	opl.WriteReg(0xB0, 0x31)
	ch := &opl.ch[0]
	op := &ch.ops[1]
	startAtten := op.egOut
	minAttackAtten := startAtten
	for i := 0; i < 2048; i++ {
		opl.advanceEnvelope(ch, op)
		opl.advanceChipState()
		if op.egOut < minAttackAtten {
			minAttackAtten = op.egOut
		}
	}
	if minAttackAtten >= startAtten {
		t.Fatalf("attack attenuation=%d want below start attenuation=%d", minAttackAtten, startAtten)
	}

	postAttackAtten := op.egOut
	for i := 0; i < 4096; i++ {
		opl.advanceEnvelope(ch, op)
		opl.advanceChipState()
	}
	if op.egOut <= postAttackAtten {
		t.Fatalf("decay attenuation=%d want above post-attack attenuation=%d", op.egOut, postAttackAtten)
	}

	opl.WriteReg(0xB0, 0x11)
	releaseStart := op.egOut
	for i := 0; i < 4096; i++ {
		opl.advanceEnvelope(ch, op)
		opl.advanceChipState()
	}
	if op.egOut <= releaseStart {
		t.Fatalf("release attenuation=%d want above release start attenuation=%d", op.egOut, releaseStart)
	}
}

func TestImpSynthAdditiveAndFMModeProduceDifferentSpectra(t *testing.T) {
	fm := renderImpSynthTone(0x20, 0x2D, 0x23, 0x01, 0x40, 0x00, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	add := renderImpSynthTone(0x20, 0x2D, 0x23, 0x01, 0x40, 0x00, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x31, 0xA0, 0x98, 0xB0, 0x31)
	diff := meanAbsPCMDiff(fm, add)
	if diff < 0.01 {
		t.Fatalf("mean PCM difference=%0.4f want >= 0.01", diff)
	}
}

func TestImpSynthFeedbackChangesToneShape(t *testing.T) {
	noFeedback := renderImpSynthTone(0x20, 0x2B, 0x23, 0x21, 0x40, 0x00, 0x43, 0x00, 0x60, 0xF3, 0x63, 0xF3, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	fullFeedback := renderImpSynthTone(0x20, 0x2B, 0x23, 0x21, 0x40, 0x00, 0x43, 0x00, 0x60, 0xF3, 0x63, 0xF3, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x3E, 0xA0, 0x98, 0xB0, 0x31)
	diff := meanAbsPCMDiff(noFeedback, fullFeedback)
	if diff < 0.001 {
		t.Fatalf("feedback mean PCM difference=%0.4f want >= 0.001", diff)
	}
}

func TestImpSynthTremoloChangesWindowAmplitudeVariance(t *testing.T) {
	steady := renderImpSynthTone(0x20, 0x01, 0x23, 0x01, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	trem := renderImpSynthTone(0xBD, 0x80, 0x20, 0x81, 0x23, 0x81, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	steadyVar := variance(windowedRMS(monoFromStereo(steady), 256))
	tremVar := variance(windowedRMS(monoFromStereo(trem), 256))
	if tremVar <= steadyVar*1.5 {
		t.Fatalf("tremolo variance=%0.6f want > steady variance=%0.6f", tremVar, steadyVar)
	}
}

func TestImpSynthVibratoChangesZeroCrossingVariance(t *testing.T) {
	steady := renderImpSynthTone(0x20, 0x01, 0x23, 0x01, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	vibrato := renderImpSynthTone(0xBD, 0x40, 0x20, 0x41, 0x23, 0x41, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31)
	steadyCross := zeroCrossingsPerWindow(monoFromStereo(steady), 512)
	vibratoCross := zeroCrossingsPerWindow(monoFromStereo(vibrato), 512)
	steadyVar := variance(steadyCross)
	vibratoVar := variance(vibratoCross)
	if math.Abs(vibratoVar-steadyVar) < 0.01 && meanAbsDiff(steadyCross, vibratoCross) < 0.5 {
		t.Fatalf("vibrato change too small: steadyVar=%0.6f vibratoVar=%0.6f meanCrossDiff=%0.6f", steadyVar, vibratoVar, meanAbsDiff(steadyCross, vibratoCross))
	}
}

func TestImpSynthPanBitsMatchDriverSemantics(t *testing.T) {
	rightOnly := renderImpSynthTone(0x20, 0x01, 0x23, 0x01, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x10, 0xA0, 0x98, 0xB0, 0x31)
	leftOnly := renderImpSynthTone(0x20, 0x01, 0x23, 0x01, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x20, 0xA0, 0x98, 0xB0, 0x31)

	rightL, rightR := channelRMS(rightOnly)
	leftL, leftR := channelRMS(leftOnly)
	if rightR <= rightL*4 {
		t.Fatalf("right-only pan rms L=%.1f R=%.1f want right-dominant output", rightL, rightR)
	}
	if leftL <= leftR*4 {
		t.Fatalf("left-only pan rms L=%.1f R=%.1f want left-dominant output", leftL, leftR)
	}
}

func TestImpSynthStereoExtensionPanFollowsD0(t *testing.T) {
	leftOnly := renderImpSynthTone(
		0x105, 0x03,
		0x20, 0x01, 0x23, 0x01, 0x43, 0x00,
		0x60, 0xF4, 0x63, 0xF4,
		0x80, 0x24, 0x83, 0x24,
		0xC0, 0x30, 0xD0, 0x00,
		0xA0, 0x98, 0xB0, 0x31,
	)
	center := renderImpSynthTone(
		0x105, 0x03,
		0x20, 0x01, 0x23, 0x01, 0x43, 0x00,
		0x60, 0xF4, 0x63, 0xF4,
		0x80, 0x24, 0x83, 0x24,
		0xC0, 0x30, 0xD0, 0x80,
		0xA0, 0x98, 0xB0, 0x31,
	)
	rightOnly := renderImpSynthTone(
		0x105, 0x03,
		0x20, 0x01, 0x23, 0x01, 0x43, 0x00,
		0x60, 0xF4, 0x63, 0xF4,
		0x80, 0x24, 0x83, 0x24,
		0xC0, 0x30, 0xD0, 0xFF,
		0xA0, 0x98, 0xB0, 0x31,
	)

	leftL, leftR := channelRMS(leftOnly)
	centerL, centerR := channelRMS(center)
	rightL, rightR := channelRMS(rightOnly)

	if leftL <= leftR*20 {
		t.Fatalf("stereoext left pan rms L=%.1f R=%.1f want strongly left-dominant output", leftL, leftR)
	}
	if rightR <= rightL*20 {
		t.Fatalf("stereoext right pan rms L=%.1f R=%.1f want strongly right-dominant output", rightL, rightR)
	}
	if math.Abs(centerL-centerR) > centerL*0.15 {
		t.Fatalf("stereoext center pan rms L=%.1f R=%.1f want roughly balanced output", centerL, centerR)
	}
}

func TestImpSynthMediumAttackPatchEscapesSilence(t *testing.T) {
	opl := newImpSynthTone(
		0x20, 0x60,
		0x23, 0xB1,
		0x40, 0x51,
		0x43, 0x80,
		0x60, 0xC0,
		0x63, 0x55,
		0x80, 0x04,
		0x83, 0x04,
		0xE0, 0x01,
		0xE3, 0x01,
		0xC0, 0x34,
		0xA0, 0x98,
		0xB0, 0x31,
	)

	ch := &opl.ch[0]
	car := &ch.ops[1]
	start := car.egRout
	for i := 0; i < 8192; i++ {
		_, _ = opl.renderChannel(0)
		opl.advanceChipState()
	}
	if car.egRout >= start {
		t.Fatalf("carrier egRout=%d want below initial attenuation=%d", car.egRout, start)
	}

	pcm := opl.GenerateStereoS16(8192)
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected audible PCM for medium-attack GENMIDI-style patch")
	}
}

func BenchmarkImpSynthRenderReferenceCorpus(b *testing.B) {
	cases := []struct {
		name string
		regs []uint16
	}{
		{
			name: "melodic_fm",
			regs: []uint16{0x20, 0x21, 0x23, 0x01, 0x40, 0x08, 0x43, 0x00, 0x60, 0xF2, 0x63, 0xF2, 0x80, 0x24, 0x83, 0x24, 0xC0, 0x30, 0xA0, 0x98, 0xB0, 0x31},
		},
		{
			name: "bright_feedback",
			regs: []uint16{0x20, 0x21, 0x23, 0x21, 0x40, 0x04, 0x43, 0x00, 0x60, 0xF4, 0x63, 0xF4, 0x80, 0x22, 0x83, 0x22, 0xC0, 0x3C, 0xA0, 0xC0, 0xB0, 0x35},
		},
		{
			name: "trem_vib",
			regs: []uint16{0xBD, 0xC0, 0x20, 0xC1, 0x23, 0xC1, 0x40, 0x18, 0x43, 0x00, 0x60, 0xF3, 0x63, 0xF3, 0x80, 0x34, 0x83, 0x34, 0xC0, 0x30, 0xA0, 0x88, 0xB0, 0x33},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			opl := newImpSynthTone(tc.regs...)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = opl.GenerateStereoS16(2048)
			}
		})
	}
}

func measureCarrierEnvelopeOut(car40, a0, b0 uint8) uint16 {
	opl := newImpSynthTone(
		0x20, 0x01,
		0x23, 0x01,
		0x40, 0x3F,
		0x43, uint16(car40),
		0x60, 0xF0,
		0x63, 0xF0,
		0x80, 0x00,
		0x83, 0x00,
		0xC0, 0x31,
		0xA0, uint16(a0),
		0xB0, uint16(b0),
	)
	ch := &opl.ch[0]
	op := &ch.ops[1]
	opl.advanceEnvelope(ch, op)
	return op.egOut
}

func renderImpSynthTone(regs ...uint16) []int16 {
	opl := newImpSynthTone(regs...)
	return opl.GenerateStereoS16(8192)
}

func newImpSynthTone(regs ...uint16) *ImpSynth {
	opl := NewImpSynth(49716)
	opl.WriteReg(0x01, 0x20)
	for i := 0; i+1 < len(regs); i += 2 {
		opl.WriteReg(regs[i], uint8(regs[i+1]))
	}
	return opl
}

func monoFromStereo(pcm []int16) []float64 {
	if len(pcm) == 0 {
		return nil
	}
	mono := make([]float64, len(pcm)/2)
	for i := 0; i < len(mono); i++ {
		mono[i] = float64(int(pcm[i*2])+int(pcm[i*2+1])) / 65534.0
	}
	return mono
}

func normalizedEnvelope(pcm []int16, window int) []float64 {
	return normalizeSeries(windowedRMS(monoFromStereo(pcm), window))
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

func zeroCrossingsPerWindow(mono []float64, window int) []float64 {
	if window <= 0 || len(mono) < 2 {
		return nil
	}
	count := len(mono) / window
	if count == 0 {
		return nil
	}
	out := make([]float64, count)
	for i := 0; i < count; i++ {
		base := i * window
		crossings := 0
		prev := mono[base]
		for j := 1; j < window; j++ {
			cur := mono[base+j]
			if (prev < 0 && cur >= 0) || (prev > 0 && cur <= 0) {
				crossings++
			}
			prev = cur
		}
		out[i] = float64(crossings)
	}
	return out
}

func normalizedBandEnergyDistance(a []int16, b []int16, sampleRate int) float64 {
	bands := []float64{110, 220, 440, 880, 1760, 3520, 7040}
	na := normalizeSeries(goertzelBandEnergies(monoFromStereo(a), sampleRate, bands))
	nb := normalizeSeries(goertzelBandEnergies(monoFromStereo(b), sampleRate, bands))
	return meanAbsDiff(na, nb)
}

func meanAbsPCMDiff(a []int16, b []int16) float64 {
	n := minInt(len(a), len(b))
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += math.Abs(float64(int(a[i])-int(b[i]))) / 32767.0
	}
	return sum / float64(n)
}

func channelRMS(samples []int16) (float64, float64) {
	if len(samples) < 2 {
		return 0, 0
	}
	var leftSum, rightSum float64
	frames := len(samples) / 2
	for i := 0; i < frames; i++ {
		l := float64(samples[i*2])
		r := float64(samples[i*2+1])
		leftSum += l * l
		rightSum += r * r
	}
	return math.Sqrt(leftSum / float64(frames)), math.Sqrt(rightSum / float64(frames))
}

func goertzelBandEnergies(mono []float64, sampleRate int, bands []float64) []float64 {
	if len(mono) == 0 || sampleRate <= 0 || len(bands) == 0 {
		return nil
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

func normalizeSeries(in []float64) []float64 {
	if len(in) == 0 {
		return nil
	}
	maxV := maxFloat64(in)
	out := make([]float64, len(in))
	if maxV == 0 {
		return out
	}
	for i, v := range in {
		out[i] = v / maxV
	}
	return out
}

func maxFloat64(in []float64) float64 {
	maxV := 0.0
	for _, v := range in {
		if v > maxV {
			maxV = v
		}
	}
	return maxV
}

func variance(in []float64) float64 {
	if len(in) == 0 {
		return 0
	}
	var mean float64
	for _, v := range in {
		mean += v
	}
	mean /= float64(len(in))
	var sum float64
	for _, v := range in {
		d := v - mean
		sum += d * d
	}
	return sum / float64(len(in))
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
