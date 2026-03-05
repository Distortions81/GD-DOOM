package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveIWADAliasPathResolvesRequestedPathCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	actual := filepath.Join(td, "doom1.wad")
	if err := os.WriteFile(actual, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	got := resolveIWADAliasPath(filepath.Join(td, "DOOM1.WAD"))
	if got != actual {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, actual)
	}
}

func TestResolveIWADAliasPathFallsBackToDoomAliasCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	alias := filepath.Join(td, "doom.wad")
	if err := os.WriteFile(alias, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write alias wad: %v", err)
	}
	missing := filepath.Join(td, "DOOM1.WAD")
	got := resolveIWADAliasPath(missing)
	if got != alias {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, alias)
	}
}

func TestResolveIWADAliasPathResolvesNonDoomPathCaseInsensitively(t *testing.T) {
	td := t.TempDir()
	actual := filepath.Join(td, "custom.wad")
	if err := os.WriteFile(actual, []byte("wad"), 0o644); err != nil {
		t.Fatalf("write custom wad: %v", err)
	}
	got := resolveIWADAliasPath(filepath.Join(td, "CUSTOM.WAD"))
	if got != actual {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, actual)
	}
}

func TestResolveIWADAliasPathReturnsOriginalWhenNoCandidateExists(t *testing.T) {
	td := t.TempDir()
	missing := filepath.Join(td, "DOOM1.WAD")
	got := resolveIWADAliasPath(missing)
	if got != missing {
		t.Fatalf("resolveIWADAliasPath() = %q want %q", got, missing)
	}
}
