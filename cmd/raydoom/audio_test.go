//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/sound"
	"testing"
)

func TestNativeQuitSoundSourcesUseMainFallbacks(t *testing.T) {
	var report sound.DigitalImportReport
	for i, name := range []string{"DSPLPAIN", "DSPOPAIN", "DSSWTCHN", "DSPOSIT2", "DSPISTOL", "DSBRSSIT", "DSDMACT", "DSITEMUP", "DSFIRSHT", "DSOOF"} {
		report.Sounds = append(report.Sounds, sound.DigitalSound{Name: name, SampleRate: 11025, Samples: []byte{byte(i + 1)}})
	}
	got := nativeQuitSoundSources(report)
	want := [2][8]string{
		{"DSPLPAIN", "DSPOPAIN", "DSPOPAIN", "DSOOF", "DSSWTCHN", "DSPOSIT2", "", "DSPISTOL"},
		{"", "DSITEMUP", "DSBRSSIT", "DSOOF", "DSFIRSHT", "", "DSDMACT", "DSPISTOL"},
	}
	if got != want {
		t.Fatalf("quit aliases=%v want=%v", got, want)
	}
}
