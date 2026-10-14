//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
)

func TestNativeVsyncPreferencesRespectChosenFrameLimit(t *testing.T) {
	for _, tc := range []struct {
		name, config       string
		args               []string
		vsyncDisabled, fps string
	}{
		{"default", "", nil, "false", "144"},
		{"uncapped CLI", "", []string{"-no-vsync"}, "true", "0"},
		{"uncapped saved", "no_vsync=true\n", nil, "true", "0"},
		{"explicit cap", "", []string{"-no-vsync", "-fps=120"}, "true", "120"},
		{"saved cap", "no_vsync=true\n[raylib]\nfps=60\n", nil, "true", "60"},
		{"explicit enable", "no_vsync=true\n", []string{"-no-vsync=false"}, "false", "144"},
		{"explicit uncap", "no_vsync=false\n[raylib]\nfps=60\n", []string{"-no-vsync", "-fps=0"}, "true", "0"},
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
			if got := fs.Lookup("no-vsync").Value.String(); got != tc.vsyncDisabled {
				t.Fatalf("no-vsync=%s want %s", got, tc.vsyncDisabled)
			}
			if got := fs.Lookup("fps").Value.String(); got != tc.fps {
				t.Fatalf("fps=%s want %s", got, tc.fps)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, enabled := range []bool{true, false} {
		s := nativeSettings{noVsync: enabled, fps: 120}
		if err := saveNativePreferences(path, s, s.bindings, 640, 400, 2, true, "textured"); err != nil {
			t.Fatal(err)
		}
		cfg, err := configfile.Read(path)
		if err != nil || cfg.NoVsync == nil || *cfg.NoVsync != enabled {
			t.Fatalf("VSync preference lost: %v", err)
		}
	}
}
