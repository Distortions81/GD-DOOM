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

func TestPulseConfigValidateRejectsNonEven35HzChunkRate(t *testing.T) {
	err := (PulseConfig{SampleRate: 48000, Channels: 1, Format: "s16le", LatencyMillis: 20}).validate()
	if err == nil {
		t.Fatal("validate() error = nil want uneven 35 Hz chunk rate")
	}
}

func TestPulseConfigChunkSizingAtDefaults(t *testing.T) {
	got := (PulseConfig{}).normalized()
	if got.samplesPerChunk() != 1260 {
		t.Fatalf("samplesPerChunk=%d want=1260", got.samplesPerChunk())
	}
	if got.bytesPerChunk() != 2520 {
		t.Fatalf("bytesPerChunk=%d want=2520", got.bytesPerChunk())
	}
}
