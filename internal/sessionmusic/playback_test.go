package sessionmusic

import (
	"testing"

	"gddoom/internal/music"
	"gddoom/internal/sound"
)

type playbackSpeakerProbe struct{ loops []bool }

func (*playbackSpeakerProbe) Play([]sound.PCSpeakerTone) {}
func (p *playbackSpeakerProbe) SetMusic(_ []sound.PCSpeakerTone, _ int, loop bool) {
	p.loops = append(p.loops, loop)
}
func (*playbackSpeakerProbe) ClearMusic()       {}
func (*playbackSpeakerProbe) Stop()             {}
func (*playbackSpeakerProbe) SetVolume(float64) {}
func (*playbackSpeakerProbe) Close() error      { return nil }

func TestPlaybackTitlePlaysOnceAndGameplayMusicStillLoops(t *testing.T) {
	parsed, err := music.ParseMUSData(buildMUSTestLump([]byte{0x90, 0xbc, 100, 8, 0x60}))
	if err != nil {
		t.Fatal(err)
	}
	output := &playbackSpeakerProbe{}
	p := &Playback{
		ctl:                &Controller{pcSpeaker: output},
		titleLoader:        func() (*music.ParsedMUS, error) { return parsed, nil },
		mapLoader:          func(string) (*music.ParsedMUS, error) { return parsed, nil },
		intermissionLoader: func(bool) (*music.ParsedMUS, error) { return parsed, nil },
	}
	p.PlayTitle(1)
	if len(output.loops) != 1 || output.loops[0] {
		t.Fatalf("title playback automatically repeats: loop requests %v", output.loops)
	}
	p.PlayMap("E1M1", 1)
	p.PlayIntermission(false, 1)
	if len(output.loops) != 3 || !output.loops[1] || !output.loops[2] {
		t.Fatalf("gameplay/intermission music no longer loops: %v", output.loops)
	}
	// A genuine return to the title starts a fresh intro once.
	p.PlayTitle(1)
	if len(output.loops) != 4 || output.loops[3] {
		t.Fatalf("returning to title did not start a fresh one-shot: %v", output.loops)
	}
}
