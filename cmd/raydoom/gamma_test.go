//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
)

func TestNativeGammaConfigAndExplicitOverrides(t *testing.T) {
	for _, tc := range []struct {
		config string
		args   []string
		want   string
	}{
		{want: "-1"}, {config: "gamma_level=4\n", want: "4"},
		{config: "gamma_level=0\n", want: "0"},
		{config: "gamma_level=4\n", args: []string{"-gamma-level=0"}, want: "0"},
		{config: "gamma_level=0\n", args: []string{"-gamma-level=3"}, want: "3"},
	} {
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
		if got := fs.Lookup("gamma-level").Value.String(); got != tc.want {
			t.Fatalf("gamma=%s want=%s", got, tc.want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, level := range []int{0, 4} {
		s := nativeSettings{gammaLevel: level}
		if err := saveNativePreferences(path, s, s.bindings, 640, 400, 2, true, "textured"); err != nil {
			t.Fatal(err)
		}
		cfg, err := configfile.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.GammaLevel == nil || *cfg.GammaLevel != level {
			t.Fatal("gamma did not persist")
		}
	}
}
