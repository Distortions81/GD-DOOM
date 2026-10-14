package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gddoom/internal/freegames"
	"gddoom/internal/lobby"
	"gddoom/internal/wad"
)

const freeGamePathPrefix = "free-game/"
const freeGameStandaloneHint = "STANDALONE - NO OTHER WAD REQUIRED"

func freeGameChoice(path string) (freegames.Game, bool) {
	if !strings.HasPrefix(path, freeGamePathPrefix) {
		return freegames.Game{}, false
	}
	return freegames.Find(strings.TrimPrefix(path, freeGamePathPrefix))
}

func appendFreeGameChoices(choices []iwadChoice) []iwadChoice {
	result := append([]iwadChoice(nil), choices...)
	for _, game := range freegames.Games() {
		found := false
		for _, choice := range choices {
			if strings.EqualFold(filepath.Base(choice.Path), game.Filename) {
				found = true
				break
			}
		}
		if !found {
			label := game.Name
			if game.ID == "freedm" {
				label = "FreeDM Deathmatch"
			}
			result = append(result, iwadChoice{Path: freeGamePathPrefix + game.ID, Label: label})
		}
	}
	return result
}

func pickerChoiceDetail(choice iwadChoice) string {
	if game, ok := freeGameChoice(choice.Path); ok {
		return "FREE / " + humanDownloadSize(game.Size)
	}
	return strings.ToUpper(filepath.Base(choice.Path))
}

type freeGameDownloadResult struct {
	data []byte
	err  error
}

type pickerFreeGameDownload struct {
	game   freegames.Game
	ctx    context.Context
	cancel context.CancelFunc
	reply  chan freeGameDownloadResult
}

// The catalog pins the bytes, while the selected content host supplies them.
// Downloading never grants permission to join a multiplayer session.
func downloadFreeGame(ctx context.Context, address string, game freegames.Game) ([]byte, error) {
	if strings.TrimSpace(address) == "" {
		address = freegames.DefaultContentServer
	}
	data, err := lobby.Download(ctx, address, lobby.PackFile{
		Name: game.Filename, Size: game.Size, SHA256: game.SHA256,
		Downloadable: true, License: "BSD-3-Clause", Source: game.ProjectURL,
	})
	if err != nil {
		return nil, err
	}
	file, err := wad.OpenData(game.Filename, data)
	if err != nil {
		return nil, fmt.Errorf("invalid downloaded game: %w", err)
	}
	if file.Header.Identification != "IWAD" {
		return nil, fmt.Errorf("downloaded game is not a standalone IWAD")
	}
	return data, nil
}

func (g *iwadPickerGame) beginFreeGameDownload(game freegames.Game) {
	g.cancelFreeGameDownload()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	job := &pickerFreeGameDownload{game: game, ctx: ctx, cancel: cancel, reply: make(chan freeGameDownloadResult, 1)}
	g.freeGameDownload = job
	g.launchQueued, g.launchDrawn = false, false
	g.status = "DOWNLOADING " + game.Name + "\n" + humanDownloadSize(game.Size) + " - ESC TO CANCEL"
	address := g.freeGameServer
	go func() {
		data, err := downloadFreeGame(ctx, address, game)
		job.reply <- freeGameDownloadResult{data: data, err: err}
	}()
}

func (g *iwadPickerGame) cancelFreeGameDownload() {
	if job := g.freeGameDownload; job != nil {
		job.cancel()
		g.freeGameDownload = nil
	}
}

func (g *iwadPickerGame) pollFreeGameDownload() {
	job := g.freeGameDownload
	if job == nil {
		return
	}
	select {
	case result := <-job.reply:
		if result.err == nil {
			result.err = job.ctx.Err()
		}
		job.cancel()
		g.freeGameDownload = nil
		if result.err != nil {
			g.status = "GAME DOWNLOAD FAILED\nCHECK CONNECTION\nENTER TO RETRY - ESC TO GO BACK"
			return
		}
		paths, cleanup, err := wad.RegisterMemoryFiles([]string{job.game.Filename}, [][]byte{result.data})
		if err != nil {
			g.status = "COULD NOT LOAD DOWNLOADED GAME"
			return
		}
		if g.freeGameCleanup != nil {
			g.freeGameCleanup()
		}
		g.freeGameID, g.freeGamePath, g.freeGameCleanup = job.game.ID, paths[0], cleanup
		g.queuePickerLaunch()
	default:
	}
}

func (g *iwadPickerGame) pickerLoading() bool {
	return strings.TrimSpace(g.loadingPath) != "" || g.launchQueued || g.freeGameDownload != nil
}
