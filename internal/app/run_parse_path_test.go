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

func TestDetectAvailableIWADChoicesSkipsSharewareUntilLast(t *testing.T) {
	td := t.TempDir()
	for _, name := range []string{"doom1.wad", "doom.wad", "doom2.wad", "tnt.wad", "plutonia.wad"} {
		if err := os.WriteFile(filepath.Join(td, name), []byte("wad"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	choices := detectAvailableIWADChoices(td)
	want := []string{"doom.wad", "doom2.wad", "tnt.wad", "plutonia.wad", "doom1.wad"}
	if len(choices) != len(want) {
		t.Fatalf("choices len=%d want=%d", len(choices), len(want))
	}
	for i, choice := range choices {
		if got := filepath.Base(choice.Path); got != want[i] {
			t.Fatalf("choice %d base=%q want=%q", i, got, want[i])
		}
	}
}

func TestDetectAvailableIWADChoicesKeepsSharewareWhenOnlyOption(t *testing.T) {
	td := t.TempDir()
	if err := os.WriteFile(filepath.Join(td, "doom1.wad"), []byte("wad"), 0o644); err != nil {
		t.Fatalf("write doom1.wad: %v", err)
	}

	choices := detectAvailableIWADChoices(td)
	if len(choices) != 1 {
		t.Fatalf("choices len=%d want=1", len(choices))
	}
	if got := filepath.Base(choices[0].Path); got != "doom1.wad" {
		t.Fatalf("choice base=%q want=doom1.wad", got)
	}
}

func TestPickerDefaultsPreferSourcePortAndFirstNonSharewareIWAD(t *testing.T) {
	game, err := newIWADPickerGame([]iwadChoice{
		{Path: "/tmp/doom.wad", Label: "The Ultimate DOOM"},
		{Path: "/tmp/doom2.wad", Label: "DOOM II: Hell on Earth"},
	}, nil)
	if err != nil {
		t.Fatalf("newIWADPickerGame() error: %v", err)
	}

	if game.profile != pickerProfileSourcePort {
		t.Fatalf("default profile=%v want sourceport", game.profile)
	}
	if game.selected != 0 {
		t.Fatalf("default selected=%d want=0", game.selected)
	}
}
