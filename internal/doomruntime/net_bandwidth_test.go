package doomruntime

import "testing"

type stubBandwidthMeter struct {
	up   float64
	down float64
}

func (m stubBandwidthMeter) BandwidthStats() (float64, float64) {
	return m.up, m.down
}

type stubVoiceSyncMeter struct {
	millis int
	ok     bool
}

func (m stubVoiceSyncMeter) VoiceSyncOffsetMillis() (int, bool) {
	return m.millis, m.ok
}

func TestFormatNetBandwidthLabelUsesDownloadForWatch(t *testing.T) {
	label := formatNetBandwidthLabel(
		stubBandwidthMeter{down: 1536},
		stubBandwidthMeter{down: 4096},
		nil,
		false,
	)
	if label != "game 1.53 kB/s  voice 4.09 kB/s" {
		t.Fatalf("label=%q", label)
	}
}

func TestFormatNetBandwidthLabelUsesUploadForBroadcast(t *testing.T) {
	label := formatNetBandwidthLabel(
		stubBandwidthMeter{up: 2500},
		stubBandwidthMeter{up: 7500},
		nil,
		true,
	)
	if label != "game 2.5 kB/s  voice 7.5 kB/s" {
		t.Fatalf("label=%q", label)
	}
}

func TestFormatNetBandwidthLabelOmitsEmptyMeters(t *testing.T) {
	label := formatNetBandwidthLabel(
		stubBandwidthMeter{down: 1024},
		nil,
		nil,
		false,
	)
	if label != "game 1.02 kB/s" {
		t.Fatalf("label=%q", label)
	}
}

func TestFormatNetBandwidthLabelIncludesVoiceSyncOffset(t *testing.T) {
	t.Setenv("GD_DOOM_VOICE_SYNC_OVERLAY", "1")
	label := formatNetBandwidthLabel(
		nil,
		stubBandwidthMeter{down: 4096},
		stubVoiceSyncMeter{millis: 86, ok: true},
		false,
	)
	if label != "voice 4.09 kB/s  sync +86ms" {
		t.Fatalf("label=%q", label)
	}
}

func TestFormatNetBandwidthLabelOmitsVoiceSyncOffsetWithoutEnv(t *testing.T) {
	label := formatNetBandwidthLabel(
		nil,
		stubBandwidthMeter{down: 4096},
		stubVoiceSyncMeter{millis: 86, ok: true},
		false,
	)
	if label != "voice 4.09 kB/s" {
		t.Fatalf("label=%q", label)
	}
}
