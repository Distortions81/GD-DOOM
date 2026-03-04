package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunParseLoadsConfigDefaults(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
	}
}

func TestRunParseCLIOverridesConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath, "-map", "E1M1"}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M1 ") {
		t.Fatalf("stdout %q does not contain map=E1M1", out.String())
	}
}

func TestRunParseRejectsDemoAndRecordDemoTogether(t *testing.T) {
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{
		"-wad", wadPath,
		"-render=false",
		"-demo", "bench.demo",
		"-record-demo", "out.demo",
	}, &out, &errb)
	if code != 2 {
		t.Fatalf("RunParse() code=%d want=2 stderr=%q", code, errb.String())
	}
	if !strings.Contains(errb.String(), "mutually exclusive") {
		t.Fatalf("stderr %q does not mention mutual exclusion", errb.String())
	}
}

func TestRunParseLoadsNoVsyncFromConfig(t *testing.T) {
	td := t.TempDir()
	cfgPath := filepath.Join(td, "cfg.toml")
	cfg := []byte("map = \"E1M2\"\nrender = false\nno_vsync = true\n")
	if err := os.WriteFile(cfgPath, cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out bytes.Buffer
	var errb bytes.Buffer
	wadPath := filepath.Join("..", "..", "DOOM1.WAD")
	code := RunParse([]string{"-wad", wadPath, "-config", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, errb.String())
	}
	if !strings.Contains(out.String(), "map=E1M2 ") {
		t.Fatalf("stdout %q does not contain map=E1M2", out.String())
	}
}
