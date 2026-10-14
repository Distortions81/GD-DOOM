//go:build raylib && cgo && !js

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gddoom/internal/demo"
	"gddoom/internal/launchcatalog"
	"gddoom/internal/wad"
)

func TestNativeDemoFlags(t *testing.T) {
	for _, tc := range []struct {
		play, record, trace, pose string
		stop                      int
		invalid                   bool
	}{
		{}, {play: "DEMO1"}, {record: "record.lmp"},
		{play: "DEMO1", trace: "trace.jsonl", stop: 10},
		{play: "DEMO1", record: "record.lmp", invalid: true},
		{trace: "trace.jsonl", invalid: true},
		{stop: -1, invalid: true},
		{play: "DEMO1", pose: "0,0,41,0", invalid: true},
	} {
		if err := validateNativeDemoFlags(tc.play, tc.record, tc.trace, tc.stop, tc.pose); (err != nil) != tc.invalid {
			t.Fatalf("flags %+v: %v", tc, err)
		}
	}
}

func TestNativeDemoLoadsFileOrWADAndResolvesHeaderMap(t *testing.T) {
	wf, err := wad.Open("../../DOOM1.WAD")
	if err != nil {
		t.Fatal(err)
	}
	if script, err := loadNativeDemo(wf, ""); err != nil || script != nil {
		t.Fatalf("empty demo: %v %v", script, err)
	}
	script, err := loadNativeDemo(wf, " demo1 ")
	if err != nil {
		t.Fatal(err)
	}
	if script.Path != "DEMO1" || len(script.Tics) == 0 {
		t.Fatal("built-in demo missing commands")
	}
	name, err := launchcatalog.ResolveDemoStartMap(wf, script, "E1M3")
	if err != nil || name != "E1M5" {
		t.Fatalf("header map=%s err=%v", name, err)
	}
	path := filepath.Join(t.TempDir(), "external.lmp")
	if err := demo.Save(path, script); err != nil {
		t.Fatal(err)
	}
	external, err := loadNativeDemo(wf, path)
	if err != nil || !reflect.DeepEqual(script.Header, external.Header) || !reflect.DeepEqual(script.Tics, external.Tics) {
		t.Fatalf("external demo: %v", err)
	}
	if _, err := loadNativeDemo(wf, filepath.Join(t.TempDir(), "missing.lmp")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	// An existing, malformed file must not silently fall back to the same WAD lump.
	t.Chdir(t.TempDir())
	if err := os.WriteFile("DEMO1", []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNativeDemo(wf, "DEMO1"); err == nil {
		t.Fatal("malformed file fell back to WAD")
	}
}

func TestNativeDemoConfigAndExplicitOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("demo=\"DEMO1\"\nrecord_demo=\"record.lmp\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fs := nativePreferenceFlags(path)
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	if fs.Lookup("demo").Value.String() != "DEMO1" || fs.Lookup("record-demo").Value.String() != "record.lmp" {
		t.Fatal("saved demo preferences not loaded")
	}
	fs = nativePreferenceFlags(path)
	if err := fs.Parse([]string{"-demo=DEMO2", "-record-demo="}); err != nil {
		t.Fatal(err)
	}
	if _, err := applyNativeConfig(fs); err != nil {
		t.Fatal(err)
	}
	if fs.Lookup("demo").Value.String() != "DEMO2" || fs.Lookup("record-demo").Value.String() != "" {
		t.Fatal("explicit demo flags did not win")
	}
}
