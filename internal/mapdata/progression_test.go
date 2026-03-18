package mapdata

import (
	"testing"

	"gddoom/internal/wad"
)

func TestNextMapNameEpisodeNormalProgression(t *testing.T) {
	f := testWADWithMaps("E1M1", "E1M2")
	got, err := NextMapName(f, "E1M1", false)
	if err != nil {
		t.Fatalf("NextMapName() error = %v", err)
	}
	if got != "E1M2" {
		t.Fatalf("NextMapName() = %q, want E1M2", got)
	}
}

func TestNextMapNameEpisodeSecretExit(t *testing.T) {
	f := testWADWithMaps("E1M3", "E1M9")
	got, err := NextMapName(f, "E1M3", true)
	if err != nil {
		t.Fatalf("NextMapName() error = %v", err)
	}
	if got != "E1M9" {
		t.Fatalf("NextMapName() = %q, want E1M9", got)
	}
}

func TestNextMapNameSecretLevelReturnsToMainPath(t *testing.T) {
	f := testWADWithMaps("E1M9", "E1M4")
	got, err := NextMapName(f, "E1M9", false)
	if err != nil {
		t.Fatalf("NextMapName() error = %v", err)
	}
	if got != "E1M4" {
		t.Fatalf("NextMapName() = %q, want E1M4", got)
	}
}

func TestNextMapNameFallsBackToLumpOrder(t *testing.T) {
	f := testWADWithMaps("MAP15", "MAP16")
	got, err := NextMapName(f, "MAP15", true)
	if err != nil {
		t.Fatalf("NextMapName() error = %v", err)
	}
	if got != "MAP16" {
		t.Fatalf("NextMapName() = %q, want MAP16", got)
	}
}

func testWADWithMaps(names ...string) *wad.File {
	lumps := make([]wad.Lump, 0, len(names)*(1+len(requiredLumps)))
	for _, name := range names {
		lumps = append(lumps, wad.Lump{Name: name})
		for _, req := range requiredLumps {
			lumps = append(lumps, wad.Lump{Name: req})
		}
	}
	return &wad.File{Lumps: lumps}
}
