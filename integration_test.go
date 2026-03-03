package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"gddoom/internal/app"
	"gddoom/internal/mapdata"
	"gddoom/internal/wad"
)

func TestDOOM1E1M1Parses(t *testing.T) {
	wadPath := filepath.Join("DOOM1.WAD")
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open DOOM1.WAD: %v", err)
	}

	m, err := mapdata.LoadMap(wf, mapdata.MapName("E1M1"))
	if err != nil {
		t.Fatalf("load E1M1: %v", err)
	}
	if len(m.Linedefs) == 0 || len(m.Vertexes) == 0 {
		t.Fatalf("unexpected empty map: linedefs=%d vertexes=%d", len(m.Linedefs), len(m.Vertexes))
	}
}

func TestCLIDefaultMapSelectsFirstLevel(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer
	code := app.RunParse([]string{"-wad", "DOOM1.WAD"}, &out, &err)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, err.String())
	}
	line := out.String()
	if !strings.Contains(line, "map=E1M1 ") {
		t.Fatalf("stdout %q does not contain map=E1M1", line)
	}
}

func TestCLIDetailsIncludesDoorAndSpatialData(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer
	code := app.RunParse([]string{"-wad", "DOOM1.WAD", "-map", "E1M1", "-details"}, &out, &err)
	if code != 0 {
		t.Fatalf("RunParse() code=%d stderr=%q", code, err.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, "doors total=") {
		t.Fatalf("details output missing doors line: %q", stdout)
	}
	if !strings.Contains(stdout, "blockmap origin=") {
		t.Fatalf("details output missing blockmap line: %q", stdout)
	}
	if !strings.Contains(stdout, "reject sectors=") {
		t.Fatalf("details output missing reject line: %q", stdout)
	}
}
