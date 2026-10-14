//go:build !js || !wasm

package app

import (
	"testing"

	"gddoom/internal/freegames"
	"gddoom/internal/runtimecfg"
)

func TestDesktopDefaultsEnableRoomBrowserWithoutJoining(t *testing.T) {
	server, address := multiplayerBuildDefaults()
	if server != defaultAuthorityCoopAddress || address != freegames.DefaultContentServer {
		t.Fatalf("desktop multiplayer defaults: server=%q lobby=%q", server, address)
	}
	opts := runtimecfg.Options{AuthorityJoinDefaults: runtimecfg.AuthorityJoinRequest{Address: server}}
	if err := configureAuthorityLobby(&opts, []string{"../../DOOM1.WAD"}, address); err != nil {
		t.Fatal(err)
	}
	if opts.AuthorityLobby == nil || opts.AuthorityCreateGame == nil || opts.AuthorityPrepareRoom == nil {
		t.Fatal("desktop release cannot browse, create, and prepare rooms")
	}
	if opts.AuthorityClient != nil {
		t.Fatal("desktop defaults unexpectedly joined a multiplayer game")
	}
}
