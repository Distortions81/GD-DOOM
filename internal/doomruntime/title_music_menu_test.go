package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/music"
	"gddoom/internal/sessionmusic"
	"gddoom/internal/sound"

	"github.com/hajimehoshi/ebiten/v2"
)

type titleMenuMusicProbe struct{ loops []bool }

func (*titleMenuMusicProbe) Play([]sound.PCSpeakerTone) {}
func (p *titleMenuMusicProbe) SetMusic(_ []sound.PCSpeakerTone, _ int, loop bool) {
	p.loops = append(p.loops, loop)
}
func (*titleMenuMusicProbe) ClearMusic()       {}
func (*titleMenuMusicProbe) Stop()             {}
func (*titleMenuMusicProbe) SetVolume(float64) {}
func (*titleMenuMusicProbe) Close() error      { return nil }

func TestTitleIntroMenuNavigationDoesNotRepeatMusic(t *testing.T) {
	// One brief note is enough to exercise the real playback/controller path
	// without requiring an audio device or waiting for the full title track.
	parsed, err := music.ParseMUSData([]byte{'M', 'U', 'S', 0x1a, 5, 0, 16, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0x90, 0xbc, 100, 8, 0x60})
	if err != nil {
		t.Fatal(err)
	}
	output := &titleMenuMusicProbe{}
	titleLoads, mapLoads := 0, 0
	playback, err := sessionmusic.NewPlayback(1, 1, 1, false, music.BackendPCSpeaker, nil, nil, output,
		func(string) (*music.ParsedMUS, error) { mapLoads++; return parsed, nil },
		func() (*music.ParsedMUS, error) { titleLoads++; return parsed, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(playback.Close)
	m := predictionTestMap()
	m.Name = "E1M1"
	opts := Options{Headless: true, SkillLevel: 3, MusicBackend: music.BackendPCSpeaker, PCSpeakerVolume: 1, SharedPCSpeaker: output}
	opts.AttractDemos = []*DemoScript{{Path: "DEMO1", Header: DemoHeader{Version: demoVersion110, Skill: 2, Episode: 1, Map: 1, PlayerInGame: [4]bool{true}}, Tics: []DemoTic{{}, {}}}}
	opts.DemoMapLoader = func(*DemoScript) (*mapdata.Map, error) { return cloneMapForRestart(m), nil }
	g := newGame(cloneMapForRestart(m), opts)
	sg := &sessionGame{g: g, rt: g, bootMap: m, opts: opts, current: m.Name, musicCtl: playback}
	sg.startFrontend()
	if titleLoads != 1 || len(output.loops) != 1 || output.loops[0] {
		t.Fatalf("title did not start once: loads=%d loop flags=%v", titleLoads, output.loops)
	}
	initialTimer := sg.frontend.AttractPageTic
	steps := 0
	press := func(key ebiten.Key) {
		t.Helper()
		sg.input = sessionInputSnapshot{justPressedKeys: map[ebiten.Key]int{key: 1}}
		if err := sg.tickFrontend(); err != nil {
			t.Fatal(err)
		}
		steps++
	}
	press(ebiten.KeyEnter) // Open menu over the already playing title.
	press(ebiten.KeyArrowDown)
	press(ebiten.KeyEnter) // Options.
	if sg.frontend.Mode != frontendModeOptions {
		t.Fatal("fixture did not navigate into Options")
	}
	for range 100 {
		press(ebiten.KeyArrowDown)
	}
	press(ebiten.KeyEscape)
	press(ebiten.KeyEscape)
	if titleLoads != 1 || mapLoads != 0 || len(output.loops) != 1 {
		t.Fatalf("menu navigation restarted music: title=%d map=%d requests=%v", titleLoads, mapLoads, output.loops)
	}
	if sg.frontend.AttractPageTic != initialTimer-steps || sg.frontend.MenuActive {
		t.Fatal("menu interaction changed normal attract countdown or failed to close")
	}
	// Genuine sequence transitions still change music; returning to TITLEPIC
	// is allowed to start the intro anew, but never as an automatic audio loop.
	sg.frontend.AttractPageTic = 1
	sg.input = sessionInputSnapshot{}
	if err := sg.tickFrontend(); err != nil {
		t.Fatal(err)
	}
	if sg.g.opts.DemoScript == nil || mapLoads != 1 || len(output.loops) != 2 || !output.loops[1] {
		t.Fatalf("title expiry did not start looping demo music: map=%d requests=%v", mapLoads, output.loops)
	}
	sg.startFrontend()
	if titleLoads != 2 || len(output.loops) != 3 || output.loops[2] {
		t.Fatalf("fresh title entry did not play intro once: title=%d requests=%v", titleLoads, output.loops)
	}
}
