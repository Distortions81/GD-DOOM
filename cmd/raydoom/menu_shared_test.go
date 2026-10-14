//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"gddoom/internal/render/raymesh"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/sessionflow"
	"math"
	"strings"
	"testing"
)

func TestNativeMenusUseEpisodeSkillFlowAndFirstLevel(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{SkillLevel: 3}, []mapdata.MapName{"E1M3", "E1M1", "E2M1", "E2M2"}, "E1M3")
	m.frontend = true
	s := nativeSettings{}
	press := func(in menuInput) menuCommand { in.mouseRow = -1; return m.update(in, &s) }
	m.open(menuMain)
	press(menuInput{confirm: true})
	if m.page != menuNewGame || len(m.rows(s)) != 2 {
		t.Fatal("registered Doom did not offer available episodes")
	}
	press(menuInput{down: true})
	press(menuInput{confirm: true})
	if m.page != menuSkill || m.row != 2 {
		t.Fatal("episode selection did not retain the default Hurt Me Plenty skill")
	}
	press(menuInput{down: true})
	press(menuInput{down: true})
	if press(menuInput{confirm: true}) != menuStart || m.skill != 5 || m.maps[m.mapIndex] != "E2M1" {
		t.Fatal("episode/skill selection did not start the chosen campaign at M1")
	}
	m.open(menuMain)
	press(menuInput{back: true})
	if m.page != menuClosed {
		t.Fatal("Escape did not resume gameplay")
	}
}

func TestNativeMenuHitAreasFollowSharedTransform(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{SkillLevel: 3}, []mapdata.MapName{"E1M1", "E2M1"}, "E1M1")
	for i := 0; i < 18; i++ {
		m.saveSlots = append(m.saveSlots, doomruntime.NativeSaveSlot{Slot: i})
	}
	for _, size := range [][2]int{{320, 200}, {640, 480}, {1280, 800}, {1600, 900}, {800, 1280}} {
		for _, page := range []menuPage{menuMain, menuOptions, menuSound, menuNewGame, menuSkill, menuLoad, menuBindings, menuMusicPlayer, menuControls} {
			m.open(page)
			total := len(m.rows(nativeSettings{}))
			for selected := 0; selected < total; selected++ {
				m.row = selected
				first, step, start, count := m.rowLayout(total)
				if selected < start || selected >= start+count {
					t.Fatal("selected row is not in the visible window")
				}
				scale, ox, oy := raymesh.MenuTransform(size[0], size[1])
				x, y := ox+100*scale, oy+(first+float64(selected-start)*step+3)*scale
				if row := m.rowAt(x, y, size[0], size[1]); row != selected {
					t.Fatalf("page=%d size=%v selected=%d mouse=%d", page, size, selected, row)
				}
			}
		}
	}
}

func TestNativeQuitRequiresYAndUsesMainPrompt(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{}
	m.open(menuMain)
	m.row = 5
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuQuit || len(m.quitPrompt.Lines) < 2 {
		t.Fatal("quit did not use the shared Doom prompt")
	}
	if got := m.update(menuInput{confirm: true, mouseRow: -1}, &s); got == menuExit {
		t.Fatal("Enter unexpectedly accepted a Y/N quit prompt")
	}
	m.update(menuInput{quitNo: true, mouseRow: -1}, &s)
	if m.page != menuMain || m.row != 5 {
		t.Fatal("N did not return to the quit row")
	}
	m.open(menuQuit)
	if got := m.update(menuInput{quitYes: true, mouseRow: -1}, &s); got != menuConfirmQuit {
		t.Fatal("Y did not confirm quit")
	}
}

func TestNativeSharedOptionsAffectPresentation(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{messages: true, showFPS: true, screenBlocks: 1, hudScale: 3, mouseSensitivity: .5}
	m.open(menuOptions)
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	m.row = 1
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	m.row = 2
	m.update(menuInput{right: true, mouseRow: -1}, &s)
	m.row = 3
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if s.messages || s.showFPS || s.screenBlocks != 2 || s.hudScale != 4 {
		t.Fatal("shared options did not apply message, HUD and FPS changes")
	}
	m.row = 5
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuSound || len(m.rows(s)) != 5 {
		t.Fatal("sound options did not use the main five-row menu")
	}
	if cmd := m.update(menuInput{back: true, mouseRow: -1}, &s); cmd != menuNoCommand || m.page != menuOptions {
		t.Fatal("sound menu back did not return to options")
	}
}

func TestNativeOptionsRemoveVoiceAndKeepActionsAndMouseAligned(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{}
	m.open(menuOptions)
	rows := m.rows(s)
	if len(rows) != 8 {
		t.Fatalf("options rows=%d", len(rows))
	}
	for _, row := range rows {
		if strings.Contains(row.label, "VOICE") {
			t.Fatal("voice row is still visible")
		}
	}
	// Navigation from sound lands directly on bindings, with no dead entry.
	m.row = 5
	m.update(menuInput{down: true, mouseRow: -1}, &s)
	if m.row != 6 || rows[m.row].label != "KEY BINDINGS" {
		t.Fatal("navigation did not skip excluded voice")
	}
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuBindings {
		t.Fatal("bindings row opened the wrong action")
	}
	m.update(menuInput{back: true, mouseRow: -1}, &s)
	if m.page != menuOptions || m.row != 6 {
		t.Fatal("binding back did not restore the key bindings row")
	}
	m.row = 7
	// The graphics row now occupies the eighth row, including hit testing.
	scale, ox, oy := raymesh.MenuTransform(1280, 800)
	row := m.rowAt(ox+100*scale, oy+(37+7*16+3)*scale, 1280, 800)
	if row != 7 {
		t.Fatalf("graphics mouse row=%d", row)
	}
	m.update(menuInput{confirm: true, mouseRow: row}, &s)
	if m.page != menuGraphics {
		t.Fatal("graphics row opened the wrong menu")
	}
	m.update(menuInput{back: true, mouseRow: -1}, &s)
	if m.page != menuOptions || m.row != 7 {
		t.Fatal("graphics back did not restore the Raylib options row")
	}
}

func TestNativeBindingsReturnToTheirOpeningMenu(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{}
	m.open(menuControls)
	m.row = 0
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuBindings {
		t.Fatal("controls did not open bindings")
	}
	m.update(menuInput{back: true, mouseRow: -1}, &s)
	if m.page != menuControls || m.row != 0 {
		t.Fatal("bindings did not return to controls")
	}
}

func TestNativeMenuQuitDelayCyclesAndRestoresPreviousPage(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{}
	m.open(menuSound)
	m.row = 3
	m.setStatus("SOUNDFONT DOWNLOAD FAILED", 70)
	m.open(menuQuit)
	first := strings.Join(m.quitPrompt.Lines, "\n")
	m.update(menuInput{quitNo: true, mouseRow: -1}, &s)
	if m.page != menuSound || m.row != 3 || m.status != "SOUNDFONT DOWNLOAD FAILED" {
		t.Fatal("quit cancellation lost the previous menu")
	}
	m.open(menuQuit)
	if first == strings.Join(m.quitPrompt.Lines, "\n") {
		t.Fatal("quit messages did not cycle")
	}
	if got := m.update(menuInput{quitYes: true, mouseRow: -1}, &s); got != menuConfirmQuit {
		t.Fatal("confirmation did not request the quit sound")
	}
	for i := 0; i < sessionflow.QuitPromptExitDelayTics; i++ {
		// Render/input updates must not advance or restart the Doom-tic countdown.
		for range 4 {
			m.update(menuInput{quitYes: true, quitNo: true, back: true, mouseRow: -1}, &s)
		}
		got := m.advanceFrame()
		want := menuNoCommand
		if i == sessionflow.QuitPromptExitDelayTics-1 {
			want = menuExit
		}
		if got != want {
			t.Fatalf("quit tic %d: got %d want %d", i+1, got, want)
		}
	}
}

func TestNativeMenuFeedbackExpiresOnDoomTics(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{messages: true}
	m.open(menuOptions)
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.status != "MESSAGES OFF" {
		t.Fatal("message toggle feedback missing")
	}
	for range 34 {
		m.advanceFrame()
	}
	if m.status == "" {
		t.Fatal("feedback expired too early")
	}
	m.advanceFrame()
	if m.status != "" {
		t.Fatal("feedback stayed on screen")
	}
	m.setStatus("DOWNLOADING BANK", 0)
	for range 100 {
		m.advanceFrame()
	}
	if m.status != "DOWNLOADING BANK" {
		t.Fatal("download status expired while pending")
	}
}

func TestNativeMenuMouseCycleVisitsMaximumBeforeMinimum(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{mouseSensitivity: .5}
	m.open(menuOptions)
	m.row = 4
	visitedMax, visitedMin := false, false
	for range 20 {
		m.update(menuInput{confirm: true, mouseRow: -1}, &s)
		if s.mouseSensitivity == sessionflow.MouseSensitivitySpeedForDot(18) {
			visitedMax = true
		}
		if s.mouseSensitivity == sessionflow.MouseSensitivitySpeedForDot(0) {
			if !visitedMax {
				t.Fatal("Enter skipped maximum sensitivity")
			}
			visitedMin = true
			break
		}
	}
	if !visitedMin {
		t.Fatal("Enter did not wrap sensitivity after maximum")
	}
	s.mouseSensitivity = sessionflow.MouseSensitivitySpeedForDot(18)
	m.update(menuInput{right: true, mouseRow: -1}, &s)
	if s.mouseSensitivity != sessionflow.MouseSensitivitySpeedForDot(18) {
		t.Fatal("right arrow wrapped maximum sensitivity")
	}
}

func TestNativeSingleWADMusicMenuPreservesTrackOnNoChange(t *testing.T) {
	opts := doomruntime.Options{MusicPlayerTrackLoader: func(string, string) ([]byte, error) { return []byte{1}, nil }, MusicPlayerCatalog: []runtimecfg.MusicPlayerWAD{{Episodes: []runtimecfg.MusicPlayerEpisode{{Tracks: []runtimecfg.MusicPlayerTrack{{LumpName: "ONE"}, {LumpName: "TWO"}}}}}}}
	m := newNativeMenu(opts, nil, "")
	m.open(menuMusicPlayer)
	m.musicTrack = 1
	m.update(menuInput{right: true, mouseRow: -1}, &nativeSettings{})
	if m.musicTrack != 1 {
		t.Fatal("moving a singleton WAD selector reset the current track")
	}
	m.row = 1
	m.update(menuInput{right: true, mouseRow: -1}, &nativeSettings{})
	if m.musicTrack != 1 {
		t.Fatal("moving a singleton group selector reset the current track")
	}
	m.update(menuInput{back: true, mouseRow: -1}, &nativeSettings{})
	if m.page != menuSound || m.row != 4 {
		t.Fatal("Escape did not restore the Sound menu's Player row")
	}
}

func TestNativeSoundMenuPreservesFractionalVolume(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{sfxVolume: .35, musicVolume: .45}
	m.open(menuSound)
	m.update(menuInput{right: true, mouseRow: -1}, &s)
	if math.Abs(s.sfxVolume-.45) > 1e-12 {
		t.Fatal("sound menu rounded away the configured volume")
	}
	m.row = 1
	m.update(menuInput{left: true, mouseRow: -1}, &s)
	if math.Abs(s.musicVolume-.35) > 1e-12 {
		t.Fatal("music menu rounded away the configured volume")
	}
}
