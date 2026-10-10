package freegames

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"testing"
)

func TestCatalogDownloadsArePinnedAndStandalone(t *testing.T) {
	var manifest struct {
		SchemaVersion int `json:"schema_version"`
		Archives      []struct {
			ID, URL, SHA256 string
			Size            int64
		} `json:"archives"`
		Games []struct {
			Game
			Archive, Member, License string
		} `json:"games"`
	}
	data, err := content.ReadFile("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || len(manifest.Games) != 3 || len(manifest.Archives) != 2 {
		t.Fatal("unexpected release catalog structure")
	}
	archives := make(map[string]bool)
	for _, archive := range manifest.Archives {
		if archives[archive.ID] || !strings.HasPrefix(archive.URL, "https://github.com/freedoom/freedoom/releases/download/v0.13.0/") || archive.Size <= 0 {
			t.Fatalf("invalid release archive: %+v", archive)
		}
		checkSHA256(t, archive.SHA256)
		archives[archive.ID] = true
	}
	seen := make(map[string]bool)
	for _, game := range manifest.Games {
		if seen[game.ID] || game.ID == "" || game.Name == "" || game.Description == "" || game.Size <= 0 {
			t.Fatalf("invalid game: %+v", game)
		}
		if game.Version != "0.13.0" || game.License != "BSD-3-Clause" || game.ProjectURL != "https://freedoom.github.io/" {
			t.Fatalf("unexpected release provenance: %+v", game)
		}
		if !archives[game.Archive] || game.Member != game.Archive+"/"+game.Filename || path.Base(game.Filename) != game.Filename || !strings.HasSuffix(game.Filename, ".wad") {
			t.Fatalf("unsafe or missing standalone game member: %+v", game)
		}
		checkSHA256(t, game.SHA256)
		seen[game.ID] = true
	}
}

func TestGamesReturnsIndependentList(t *testing.T) {
	first := Games()
	want := first[0]
	first[0].Name = "changed by caller"
	got, ok := Find(want.ID)
	if !ok || got != want || Games()[0] != want {
		t.Fatal("caller changed the catalog")
	}
	if _, ok := Find("doom-shareware"); ok {
		t.Fatal("bundled shareware must not appear as a free-game download")
	}
	if _, ok := Find("unknown"); ok {
		t.Fatal("unknown game was found")
	}
}

func TestCompleteReleaseNoticesRetained(t *testing.T) {
	var manifest struct {
		Archives []struct {
			Notices []struct {
				Filename, SHA256 string
				Size             int
			}
		}
	}
	data, err := content.ReadFile("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, archive := range manifest.Archives {
		if len(archive.Notices) != 3 {
			t.Fatal("license, contributor credits, and music credits are all required")
		}
		for _, notice := range archive.Notices {
			document, err := content.ReadFile("licenses/freedoom-0.13.0/" + notice.Filename)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(document)
			if len(document) != notice.Size || hex.EncodeToString(digest[:]) != notice.SHA256 {
				t.Fatalf("release notice %s was changed or truncated", notice.Filename)
			}
			if !strings.Contains(Notices(), string(document)) {
				t.Fatalf("public notices omit complete %s", notice.Filename)
			}
		}
	}
}

func checkSHA256(t *testing.T, value string) {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil || len(data) != sha256.Size || strings.ToLower(value) != value {
		t.Fatalf("invalid SHA-256: %q", value)
	}
}
