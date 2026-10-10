package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gddoom/internal/freegames"
	"gddoom/internal/wad"
)

func TestFreeGameChoicesKeepLocalWADsAndDoNotDuplicateInstalledGames(t *testing.T) {
	local := []iwadChoice{{Path: "DOOM1.WAD", Label: "Doom Shareware"}, {Path: "/games/FREEDOOM1.WAD", Label: "My Freedoom"}}
	choices := appendFreeGameChoices(local)
	if len(choices) != 4 || choices[0] != local[0] || choices[1] != local[1] {
		t.Fatalf("local choices changed or duplicate offered: %+v", choices)
	}
	for _, choice := range choices[2:] {
		if _, ok := freeGameChoice(choice.Path); !ok || !strings.HasPrefix(pickerChoiceDetail(choice), "FREE / ") {
			t.Fatalf("missing curated standalone download: %+v", choice)
		}
	}
	if _, ok := freeGameChoice("free-game/not-in-the-catalog"); ok {
		t.Fatal("unknown game accepted")
	}
	// Explicitly chosen browser content must still bypass other base choices.
	if got := wasmPickerChoices(choices, "local/custom.wad"); len(got) != 1 || got[0].Path != "local/custom.wad" {
		t.Fatalf("custom base was mixed with catalog games: %+v", got)
	}
}

func freeGameFixture(t *testing.T, kind string) (freegames.Game, []byte) {
	t.Helper()
	data := buildAppTestWAD(kind, []appTestLump{{name: "TEST", data: []byte("test game")}})
	return freegames.Game{ID: "test", Name: "Test game", Filename: "free.wad", Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, data
}

func TestFreeGameDownloadPinsBytesAndRequiresStandaloneIWAD(t *testing.T) {
	game, data := freeGameFixture(t, "IWAD")
	served := data
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/content/"+game.SHA256 {
			t.Errorf("unexpected download path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(served)
	}))
	defer s.Close()
	if got, err := downloadFreeGame(context.Background(), s.URL, game); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("verified game download: %v", err)
	}
	served = append([]byte(nil), data...)
	served[len(served)-1] ^= 1
	if _, err := downloadFreeGame(context.Background(), s.URL, game); err == nil {
		t.Fatal("accepted changed bytes")
	}
	game, served = freeGameFixture(t, "PWAD")
	if _, err := downloadFreeGame(context.Background(), s.URL, game); err == nil {
		t.Fatal("accepted an add-on as a standalone game")
	}
}

func TestPickerFreeGameDownloadCancellationAndOwnedMemory(t *testing.T) {
	game, data := freeGameFixture(t, "IWAD")
	started := make(chan struct{}, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	}))
	g := &iwadPickerGame{freeGameServer: s.URL}
	g.beginFreeGameDownload(game)
	job := g.freeGameDownload
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	g.cancelFreeGameDownload()
	select {
	case result := <-job.reply:
		if result.err == nil || len(result.data) != 0 {
			t.Fatal("canceled download returned game bytes")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not stop download")
	}
	s.Close()
	if g.freeGameDownload != nil || g.launchQueued || g.freeGamePath != "" {
		t.Fatal("canceled download changed live selection")
	}

	ctx, cancel := context.WithCancel(context.Background())
	g.freeGameDownload = &pickerFreeGameDownload{game: game, ctx: ctx, cancel: cancel, reply: make(chan freeGameDownloadResult, 1)}
	g.freeGameDownload.reply <- freeGameDownloadResult{data: data}
	g.pollFreeGameDownload()
	path := g.freeGamePath
	if !g.launchQueued || g.freeGameID != game.ID || path == "" {
		t.Fatal("verified download did not queue normal game setup")
	}
	if got, ok := wad.EmbeddedDataForPath(path); !ok || !bytes.Equal(got, data) {
		t.Fatal("download registration lost its bytes")
	}
	g.Close()
	if _, ok := wad.EmbeddedDataForPath(path); ok {
		t.Fatal("picker close leaked downloaded game registration")
	}
}
