//go:build raylib && cgo && !js

package main

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/configfile"
	"gddoom/internal/runtimecfg"
)

func TestNativeDetailPreferencesAndExplicitOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "detail.toml")
	if err := os.WriteFile(path, []byte("detail_level_sourceport=2\nauto_detail=true\ndetail_level_faithful=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		fs := nativePreferenceFlags(path)
		if override {
			if err := fs.Parse([]string{"-detail-level=0", "-auto-detail=false"}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := applyNativeConfig(fs); err != nil {
			t.Fatal(err)
		}
		wantLevel, wantAuto := "2", "true"
		if override {
			wantLevel, wantAuto = "0", "false"
		}
		if fs.Lookup("detail-level").Value.String() != wantLevel || fs.Lookup("auto-detail").Value.String() != wantAuto {
			t.Fatal("detail config/override mismatch")
		}
	}
	if err := saveNativePreferences(path, nativeSettings{detailLevel: 3, autoDetail: true}, runtimecfg.DefaultInputBindings(), 640, 400, 2, true, "textured"); err != nil {
		t.Fatal(err)
	}
	cfg, err := configfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DetailLevelSourcePort == nil || *cfg.DetailLevelSourcePort != 3 || cfg.AutoDetail == nil || !*cfg.AutoDetail || *cfg.DetailLevelFaithful != 1 {
		t.Fatal("detail preferences did not round trip")
	}
}
