package sound

import (
	"math"
	"testing"
)

func TestImpSynthGenerateNonZeroWhenKeyOn(t *testing.T) {
	opl := NewImpSynth(49716)
	// Channel 0 tone: FNUM + BLOCK + KEYON.
	opl.WriteReg(0x20, 0x01)
	opl.WriteReg(0x23, 0x01)
	opl.WriteReg(0x60, 0xF3)
	opl.WriteReg(0x63, 0xF3)
	opl.WriteReg(0x80, 0x24)
	opl.WriteReg(0x83, 0x24)
	opl.WriteReg(0xA0, 0x98)
	opl.WriteReg(0xB0, 0x31) // block=4, key-on
	opl.WriteReg(0x43, 0x00) // loud carrier
	opl.WriteReg(0xC0, 0x30) // L+R

	pcm := opl.GenerateStereoS16(256)
	if len(pcm) != 512 {
		t.Fatalf("stereo samples=%d want=%d", len(pcm), 512)
	}
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-zero PCM with key-on tone")
	}
}

func TestImpSynthKeyOffHasReleaseTail(t *testing.T) {
	opl := NewImpSynth(49716)
	opl.WriteReg(0x20, 0x21)
	opl.WriteReg(0x23, 0x21)
	opl.WriteReg(0x40, 0x3f)
	opl.WriteReg(0xA0, 0x60)
	opl.WriteReg(0x43, 0x00)
	opl.WriteReg(0x60, 0xf0)
	opl.WriteReg(0x63, 0xf0)
	opl.WriteReg(0x80, 0x77)
	opl.WriteReg(0x83, 0x77)
	opl.WriteReg(0xC0, 0x30)
	opl.WriteReg(0xB0, 0x2C) // key-on
	_ = opl.GenerateStereoS16(128)
	opl.WriteReg(0xB0, 0x0C) // key-off

	pcm := opl.GenerateStereoS16(64)
	var tail int64
	for i := 0; i < len(pcm); i += 2 {
		v := int64(pcm[i])
		if v < 0 {
			v = -v
		}
		tail += v
	}
	if tail == 0 {
		t.Fatal("expected non-zero release tail after key-off")
	}
}

func TestImpSynthGenerateMonoU8(t *testing.T) {
	opl := NewImpSynth(49716)
	opl.WriteReg(0x20, 0x01)
	opl.WriteReg(0x23, 0x01)
	opl.WriteReg(0xA0, 0x80)
	opl.WriteReg(0xB0, 0x31)
	opl.WriteReg(0x43, 0x00)
	mono := opl.GenerateMonoU8(128)
	if len(mono) != 128 {
		t.Fatalf("mono samples=%d want=128", len(mono))
	}
}

func TestImpSynthPatchSettingsChangeWaveform(t *testing.T) {
	render := func(setup func(*ImpSynth)) []int16 {
		opl := NewImpSynth(49716)
		opl.WriteReg(0x01, 0x20) // enable waveform select
		opl.WriteReg(0x20, 0x21)
		opl.WriteReg(0x23, 0x21)
		opl.WriteReg(0x40, 0x10)
		opl.WriteReg(0x43, 0x00)
		opl.WriteReg(0x60, 0xF1)
		opl.WriteReg(0x63, 0xF1)
		opl.WriteReg(0x80, 0x22)
		opl.WriteReg(0x83, 0x22)
		opl.WriteReg(0xA0, 0x98)
		opl.WriteReg(0xB0, 0x31)
		opl.WriteReg(0xC0, 0x30)
		setup(opl)
		return opl.GenerateStereoS16(128)
	}

	pcmA := render(func(opl *ImpSynth) {
		opl.WriteReg(0xE0, 0x00)
		opl.WriteReg(0xE3, 0x00)
		opl.WriteReg(0xC0, 0x30)
	})
	pcmB := render(func(opl *ImpSynth) {
		opl.WriteReg(0xE0, 0x07)
		opl.WriteReg(0xE3, 0x05)
		opl.WriteReg(0xC0, 0x31)
	})

	if len(pcmA) != len(pcmB) {
		t.Fatalf("pcm lengths differ: %d vs %d", len(pcmA), len(pcmB))
	}
	same := true
	for i := range pcmA {
		if pcmA[i] != pcmB[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("expected different PCM when waveform/algorithm settings change")
	}
}

func TestNewSynthWithBackendUsesImpSynth(t *testing.T) {
	opl, err := NewSynthWithBackend(49716, BackendImpSynth)
	if err != nil {
		t.Fatalf("NewSynthWithBackend() error: %v", err)
	}
	if _, ok := opl.(*ImpSynth); !ok {
		t.Fatalf("backend type=%T want *ImpSynth", opl)
	}
}

func TestImpSynthGenerateStereoS16ReusesBuffer(t *testing.T) {
	opl := NewImpSynth(49716)
	opl.WriteReg(0x20, 0x01)
	opl.WriteReg(0x23, 0x01)
	opl.WriteReg(0xA0, 0x98)
	opl.WriteReg(0xB0, 0x31)
	opl.WriteReg(0x43, 0x00)
	_ = opl.GenerateStereoS16(256) // warm buffer

	allocs := testing.AllocsPerRun(100, func() {
		_ = opl.GenerateStereoS16(256)
	})
	if allocs != 0 {
		t.Fatalf("GenerateStereoS16 allocs=%v want 0", allocs)
	}
}

func TestImpSynthGenerateMonoU8ReusesBuffer(t *testing.T) {
	opl := NewImpSynth(49716)
	opl.WriteReg(0x20, 0x01)
	opl.WriteReg(0x23, 0x01)
	opl.WriteReg(0xA0, 0x98)
	opl.WriteReg(0xB0, 0x31)
	opl.WriteReg(0x43, 0x00)
	_ = opl.GenerateMonoU8(256) // warm buffer

	allocs := testing.AllocsPerRun(100, func() {
		_ = opl.GenerateMonoU8(256)
	})
	if allocs != 0 {
		t.Fatalf("GenerateMonoU8 allocs=%v want 0", allocs)
	}
}

func TestImpSynthPitchStaysStableAcrossOutputRates(t *testing.T) {
	render := func(rate int) []int16 {
		opl := NewImpSynth(rate)
		opl.WriteReg(0x20, 0x01)
		opl.WriteReg(0x23, 0x01)
		opl.WriteReg(0x40, 0x3f)
		opl.WriteReg(0x43, 0x00)
		opl.WriteReg(0x60, 0xF3)
		opl.WriteReg(0x63, 0xF3)
		opl.WriteReg(0x80, 0x24)
		opl.WriteReg(0x83, 0x24)
		opl.WriteReg(0xA0, 0x98)
		opl.WriteReg(0xB0, 0x31)
		opl.WriteReg(0xC0, 0x30)
		return opl.GenerateStereoS16(8192)
	}

	refHz := estimateDominantHz(render(49716), 49716)
	testHz := estimateDominantHz(render(44100), 44100)
	if refHz == 0 || testHz == 0 {
		t.Fatalf("dominant frequency estimate failed: ref=%.2f test=%.2f", refHz, testHz)
	}
	ratio := testHz / refHz
	if math.Abs(1-ratio) > 0.02 {
		t.Fatalf("pitch ratio=%.4f want ~= 1.0 (ref=%.2fHz test=%.2fHz)", ratio, refHz, testHz)
	}
}

func estimateDominantHz(stereo []int16, sampleRate int) float64 {
	if len(stereo) < 6 || sampleRate <= 0 {
		return 0
	}
	prev := 0.0
	lastCross := -1
	var periods []int
	for i := 0; i < len(stereo)/2; i++ {
		v := float64(stereo[i*2])
		if i > 256 && prev < 0 && v >= 0 {
			if lastCross >= 0 {
				periods = append(periods, i-lastCross)
			}
			lastCross = i
			if len(periods) >= 32 {
				break
			}
		}
		prev = v
	}
	if len(periods) == 0 {
		return 0
	}
	total := 0
	for _, p := range periods {
		total += p
	}
	avgPeriod := float64(total) / float64(len(periods))
	if avgPeriod == 0 {
		return 0
	}
	return float64(sampleRate) / avgPeriod
}
