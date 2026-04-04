//go:build linux

package audioinput

import "testing"

func TestPulseArgsDefaults(t *testing.T) {
	got := pulseArgs((PulseConfig{}).normalized())
	want := []string{
		"--raw",
		"--rate=44100",
		"--channels=1",
		"--format=s16le",
		"--latency-msec=20",
	}
	if len(got) != len(want) {
		t.Fatalf("arg count=%d want=%d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestPulseArgsIncludesDevice(t *testing.T) {
	got := pulseArgs((PulseConfig{
		Device:        "alsa_input.usb-test",
		SampleRate:    16000,
		Channels:      2,
		Format:        "s16le",
		LatencyMillis: 40,
	}).normalized())
	wantLast := "--device=alsa_input.usb-test"
	if got[len(got)-1] != wantLast {
		t.Fatalf("last arg=%q want=%q", got[len(got)-1], wantLast)
	}
}
