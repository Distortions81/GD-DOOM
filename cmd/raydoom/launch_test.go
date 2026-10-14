//go:build raylib && cgo && !js

package main

import (
	"gddoom/internal/doomruntime"
	"gddoom/internal/mapdata"
	"testing"
)

func TestNativeLaunchDefaultsAndOverrides(t *testing.T) {
	maps := []mapdata.MapName{"E1M3", "E1M1", "E1M2"}
	cases := []struct {
		name, requested        string
		menu, menuSet          bool
		wantMap                mapdata.MapName
		wantMenu, wantFrontend bool
	}{
		{"ordinary startup", "", true, false, "E1M1", false, true},
		{"explicit startup menu", "", true, true, "E1M1", true, true},
		{"direct map", "e1m3", true, false, "E1M3", false, false},
		{"explicit inspection menu", "E1M3", true, true, "E1M3", true, false},
		{"skip frontend", "", false, true, "E1M1", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := chooseNativeLaunch(tc.requested, tc.menu, tc.menuSet, maps)
			if err != nil || got.mapName != tc.wantMap || got.showMenu != tc.wantMenu || got.frontend != tc.wantFrontend {
				t.Fatalf("launch=%+v err=%v", got, err)
			}
		})
	}
	got, err := chooseNativeLaunch("", true, false, []mapdata.MapName{"MAP03", "MAP01"})
	if err != nil || got.mapName != "MAP01" || !got.frontend {
		t.Fatal("Doom II did not start at its menu and first map")
	}
	if _, err := chooseNativeLaunch("", true, false, nil); err == nil {
		t.Fatal("empty WAD map list accepted")
	}
}

func TestNativeFrontendCannotResumeUntilNewGame(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{SkillLevel: 3}, []mapdata.MapName{"E1M1", "E1M2"}, "E1M1")
	m.frontend = true
	m.open(menuMain)
	s := nativeSettings{}
	for _, row := range m.rows(s) {
		if row.label == "RESUME" {
			t.Fatal("startup menu exposed a hidden active game")
		}
	}
	m.back()
	if m.page != menuClosed || !m.frontend {
		t.Fatal("Escape did not return to the title loop")
	}
	m.open(menuMain)
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuSkill || m.row != 2 {
		t.Fatal("New Game did not open the default Doom difficulty")
	}
	if command := m.update(menuInput{confirm: true, mouseRow: -1}, &s); command != menuStart || m.frontend || m.page != menuClosed || m.maps[m.mapIndex] != "E1M1" {
		t.Fatal("new game did not enter the selected first level")
	}
}
