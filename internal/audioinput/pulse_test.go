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
	err := (PulseConfig{SampleRate: 48000, Channels: 1, Format: "f32le", LatencyMillis: 30}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want unsupported format")
	}
}

func TestPulseConfigValidateRejectsBadSampleRate(t *testing.T) {
	err := (PulseConfig{SampleRate: 0, Channels: 1, Format: "s16le", LatencyMillis: 30}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want bad sample rate")
	}
}

func TestPulseConfigValidateRejectsNonEven30MSFrameRate(t *testing.T) {
	err := (PulseConfig{SampleRate: 44117, Channels: 1, Format: "s16le", LatencyMillis: 30}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want uneven 30 ms frame rate")
	}
}

func TestPulseConfigChunkSizingAtDefaults(t *testing.T) {
	got := (PulseConfig{}).normalized()
	if got.samplesPerFrame() != 1440 {
		t.Fatalf("samplesPerFrame=%d want=1440", got.samplesPerFrame())
	}
	if got.bytesPerFrame() != 2880 {
		t.Fatalf("bytesPerFrame=%d want=2880", got.bytesPerFrame())
	}
}
