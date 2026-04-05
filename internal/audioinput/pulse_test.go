package audioinput

import "testing"

func TestPulseConfigNormalizedDefaults(t *testing.T) {
	got := (PulseConfig{}).normalized()
	if got.SampleRate != defaultPulseSampleRate {
		t.Fatalf("SampleRate=%d want=%d", got.SampleRate, defaultPulseSampleRate)
	}
	if got.Channels != defaultPulseChannels {
		t.Fatalf("Channels=%d want=%d", got.Channels, defaultPulseChannels)
	}
	if got.Format != defaultPulseFormat {
		t.Fatalf("Format=%q want=%q", got.Format, defaultPulseFormat)
	}
	if got.LatencyMillis != defaultPulseLatencyMilli {
		t.Fatalf("LatencyMillis=%d want=%d", got.LatencyMillis, defaultPulseLatencyMilli)
	}
}

func TestPulseConfigValidateRejectsUnsupportedFormat(t *testing.T) {
	err := (PulseConfig{SampleRate: 48000, Channels: 1, Format: "f32le", LatencyMillis: 20}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want unsupported format")
	}
}

func TestPulseConfigValidateRejectsBadSampleRate(t *testing.T) {
	err := (PulseConfig{SampleRate: 0, Channels: 1, Format: "s16le", LatencyMillis: 20}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want bad sample rate")
	}
}

func TestPulseConfigValidateRejectsNonEven20MSFrameRate(t *testing.T) {
	err := (PulseConfig{SampleRate: 44117, Channels: 1, Format: "s16le", LatencyMillis: 20}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want uneven 20 ms frame rate")
	}
}

func TestPulseConfigChunkSizingAtDefaults(t *testing.T) {
	got := (PulseConfig{}).normalized()
	if got.samplesPerFrame() != 960 {
		t.Fatalf("samplesPerFrame=%d want=960", got.samplesPerFrame())
	}
	if got.bytesPerFrame() != 1920 {
		t.Fatalf("bytesPerFrame=%d want=1920", got.bytesPerFrame())
	}
}
