package doomruntime

import (
	"strings"
	"testing"

	"gddoom/internal/music"
)

func TestFrontendMusicSoundFontDownloadStatusIncludesLabelAndPercent(t *testing.T) {
	msg := frontendMusicSoundFontDownloadStatus("soundfonts/SC55-HQ.sf2")
	if !strings.Contains(msg, "DOWNLOADING SC55-HQ") {
		t.Fatalf("status=%q want soundfont label", msg)
	}
}

func TestQueueFrontendMusicConfigDownloadSetsPendingState(t *testing.T) {
	sg := &sessionGame{}
	sg.queueFrontendMusicConfigDownload(music.BackendMeltySynth, "soundfonts/SGM-HQ.sf2")
	if !sg.frontendMusicConfig.active {
		t.Fatal("expected pending frontend music config")
	}
	if got := sg.frontendMusicConfig.backend; got != music.BackendMeltySynth {
		t.Fatalf("backend=%v want meltysynth", got)
	}
	if got := sg.frontendMusicConfig.soundFontPath; got != "soundfonts/SGM-HQ.sf2" {
		t.Fatalf("soundFontPath=%q want SGM-HQ path", got)
	}
	if !strings.Contains(sg.frontend.Status, "DOWNLOADING SGM-HQ") {
		t.Fatalf("status=%q want download label", sg.frontend.Status)
	}
	if sg.frontend.StatusTic != 0 {
		t.Fatalf("status tic=%d want 0", sg.frontend.StatusTic)
	}
}

func TestTickPendingFrontendMusicConfigNoopWithoutPending(t *testing.T) {
	sg := &sessionGame{}
	handled, err := sg.tickPendingFrontendMusicConfig()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if handled {
		t.Fatal("expected no pending work")
	}
}
