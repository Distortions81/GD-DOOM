//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
)

func TestNativeCRTPreferenceAndExplicitOverrides(t *testing.T) {
	for _, tc := range []struct {
		config string
		args   []string
		want   string
	}{
		{want: "false"}, {config: "crt_effect=true\n", want: "true"},
		{config: "crt_effect=true\n", args: []string{"-crt-effect=false"}, want: "false"},
		{config: "crt_effect=false\n", args: []string{"-crt-effect=true"}, want: "true"},
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
		if got := fs.Lookup("crt-effect").Value.String(); got != tc.want {
			t.Fatalf("CRT %s want %s", got, tc.want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, enabled := range []bool{true, false} {
		s := nativeSettings{crtEffect: enabled, noAspectCorrection: enabled}
		if err := saveNativePreferences(path, s, s.bindings, 640, 400, 2, true, "textured"); err != nil {
			t.Fatal(err)
		}
		cfg, err := configfile.Read(path)
		if err != nil || cfg.CRTEffect == nil || *cfg.CRTEffect != enabled {
			t.Fatalf("CRT preference did not persist: %v", err)
		}
		if cfg.NoAspectCorrection == nil || *cfg.NoAspectCorrection != enabled {
			t.Fatal("aspect preference did not persist")
		}
	}
}
