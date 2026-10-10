// Package freegames describes the pinned, freely redistributable standalone
// games offered by GD-DOOM. A catalog entry approves one exact release file;
// it does not grant permission to share arbitrary files with the same name.
package freegames

import (
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// DefaultContentServer serves approved game data for the built-in catalog.
const DefaultContentServer = "https://m45sci.xyz:6672"

// Game is a standalone IWAD whose contents and size are pinned for downloads.
type Game struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
	ProjectURL  string `json:"project_url"`
	Size        int64  `json:"size"`
}

//go:embed catalog.json licenses/freedoom-0.13.0/*.txt
var content embed.FS

var games = readGames()
var notices = readNotices()

func readGames() []Game {
	data, err := content.ReadFile("catalog.json")
	if err != nil {
		panic(err)
	}
	var catalog struct {
		Games []Game `json:"games"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		panic(fmt.Sprintf("invalid embedded free-game catalog: %v", err))
	}
	return catalog.Games
}

// Games returns a copy of the curated standalone game list. The existing Doom
// shareware is supplied separately and is not part of this download list.
func Games() []Game { return slices.Clone(games) }

// Find looks up a curated game by its stable ID.
func Find(id string) (Game, bool) {
	for _, game := range games {
		if game.ID == id {
			return game, true
		}
	}
	return Game{}, false
}

// Notices returns the complete license and credit documents distributed with
// the pinned release. The app and website retain these with every download.
func Notices() string { return notices }

func readNotices() string {
	var result strings.Builder
	result.WriteString("Freedoom: Phase 1, Freedoom: Phase 2, and FreeDM 0.13.0\n")
	result.WriteString("Project: https://freedoom.github.io/\n")
	result.WriteString("Release: https://github.com/freedoom/freedoom/releases/tag/v0.13.0\n")
	for _, name := range []string{"COPYING.txt", "CREDITS.txt", "CREDITS-MUSIC.txt"} {
		data, err := content.ReadFile("licenses/freedoom-0.13.0/" + name)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&result, "\n===== %s =====\n\n", name)
		result.Write(data)
		result.WriteByte('\n')
	}
	return result.String()
}
