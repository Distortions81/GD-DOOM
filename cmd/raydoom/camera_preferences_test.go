//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
	"gddoom/internal/doomruntime"
)

func TestNativeCameraPreferenceConfigAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		args         []string
		want         string
	}{
		{name: "default smoothing", want: "true"},
		{name: "saved disabled", config: "smooth_camera_yaw=false\n", want: "false"},
		{name: "explicit enabled", config: "smooth_camera_yaw=false\n", args: []string{"-smooth-camera-yaw=true"}, want: "true"},
		{name: "explicit disabled", config: "smooth_camera_yaw=true\n", args: []string{"-smooth-camera-yaw=false"}, want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := ""
			if tc.config != "" {
				path = filepath.Join(t.TempDir(), "config.toml")
				if err := os.WriteFile(path, []byte(tc.config), 0600); err != nil {
					t.Fatal(err)
				}
			}
			fs := nativePreferenceFlags(path)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if _, err := applyNativeConfig(fs); err != nil {
				t.Fatal(err)
			}
			if got := fs.Lookup("smooth-camera-yaw").Value.String(); got != tc.want {
				t.Fatalf("smoothing=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestNativeControlsCameraToggleAndPersistence(t *testing.T) {
	m := newNativeMenu(doomruntime.Options{}, nil, "")
	s := nativeSettings{smoothCameraYaw: true}
	m.open(menuControls)
	found := false
	for i, row := range m.rows(s) {
		if row.label == "SMOOTH CAMERA" {
			found = true
			m.row = i
			m.update(menuInput{confirm: true, mouseRow: -1}, &s)
			if s.smoothCameraYaw {
				t.Fatal("camera toggle did not disable smoothing")
			}
			m.update(menuInput{right: true, mouseRow: -1}, &s)
			if !s.smoothCameraYaw {
				t.Fatal("camera toggle did not enable smoothing")
			}
		}
	}
	if !found {
		t.Fatal("controls omitted camera smoothing")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, enabled := range []bool{true, false} {
		s.smoothCameraYaw = enabled
		if err := saveNativePreferences(path, s, s.bindings, 1280, 800, 2, true, "textured"); err != nil {
			t.Fatal(err)
		}
		cfg, err := configfile.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SmoothCameraYaw == nil || *cfg.SmoothCameraYaw != enabled {
			t.Fatal("camera preference did not round trip")
		}
	}
	rows := m.rows(s)
	m.row = len(rows) - 1
	if rows[m.row].label != "BACK" {
		t.Fatal("controls lost back row")
	}
	m.update(menuInput{confirm: true, mouseRow: -1}, &s)
	if m.page != menuGraphics {
		t.Fatal("back did not return to renderer options")
	}
}
