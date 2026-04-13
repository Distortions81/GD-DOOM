package app

import (
	"testing"

	"gddoom/internal/music"
	"gddoom/internal/platformcfg"
	"gddoom/internal/sound"
)

func TestPadDoomSoundSamples_PadsTo512With128(t *testing.T) {
	src := []byte{1, 2, 3}
	got := padDoomSoundSamples(src)
	if len(got) != 512 {
		t.Fatalf("len=%d want=512", len(got))
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("prefix=%v want original prefix", got[:3])
	}
	for i := 3; i < len(got); i++ {
		if got[i] != 128 {
			t.Fatalf("tail byte at %d = %d want 128", i, got[i])
		}
	}
}

func TestPadDoomSoundSamples_AlreadyAlignedReturnsOriginalSlice(t *testing.T) {
	src := make([]byte, 512)
	for i := range src {
		src[i] = byte(i)
	}
	got := padDoomSoundSamples(src)
	if len(got) != 512 {
		t.Fatalf("len=%d want=512", len(got))
	}
	if &got[0] != &src[0] {
		t.Fatal("expected original slice, got copy")
	}
	for i := range src {
		if got[i] != src[i] {
			t.Fatalf("byte %d = %d want %d", i, got[i], src[i])
		}
	}
}

func TestBuildAutomapSoundBank_FaithfulUsesFixed11025MixerRate(t *testing.T) {
	report := sound.DigitalImportReport{
		Sounds: []sound.DigitalSound{{
			Name:       "DSPISTOL",
			SampleRate: 22050,
			Samples:    []byte{1, 2, 3, 4},
		}},
	}
	bank := buildAutomapSoundBank(report, false)
	if bank.ShootPistol.SampleRate != 11025 {
		t.Fatalf("faithful sample rate=%d want=11025", bank.ShootPistol.SampleRate)
	}
}

func TestBuildAutomapSoundBank_SourcePortPreservesLumpRate(t *testing.T) {
	report := sound.DigitalImportReport{
		Sounds: []sound.DigitalSound{{
			Name:       "DSPISTOL",
			SampleRate: 22050,
			Samples:    []byte{1, 2, 3, 4},
		}},
	}
	bank := buildAutomapSoundBank(report, true)
	if bank.ShootPistol.SampleRate != 11025 {
		t.Fatalf("source-port sample rate=%d want=11025", bank.ShootPistol.SampleRate)
	}
}

func TestBuildAutomapSoundBank_WASMSourcePortAlsoPreparesSourcePortAudio(t *testing.T) {
	prev := platformcfg.ForcedWASMMode()
	platformcfg.SetForcedWASMMode(true)
	defer platformcfg.SetForcedWASMMode(prev)

	report := sound.DigitalImportReport{
		Sounds: []sound.DigitalSound{{
			Name:       "DSPISTOL",
			SampleRate: 22050,
			Samples:    []byte{1, 2, 3, 4},
		}},
	}
	bank := buildAutomapSoundBank(report, true)
	if bank.ShootPistol.FaithfulPreparedRate != music.OutputSampleRate {
		t.Fatalf("faithful prepared rate=%d want=%d", bank.ShootPistol.FaithfulPreparedRate, music.OutputSampleRate)
	}
	if bank.ShootPistol.PreparedRate != music.OutputSampleRate {
		t.Fatalf("source-port prepared rate=%d want=%d", bank.ShootPistol.PreparedRate, music.OutputSampleRate)
	}
	if len(bank.ShootPistol.Data) != 512 {
		t.Fatalf("sample len=%d want=512", len(bank.ShootPistol.Data))
	}
}
