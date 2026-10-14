//go:build js && wasm

package app

import "gddoom/internal/platformcfg"

// Set by the WASM release build. These only prefill the in-game browser;
// joining still requires an explicit action in the game.
var wasmMultiplayerServer, wasmMultiplayerLobby string

func multiplayerBuildDefaults() (server, lobby string) {
	return wasmMultiplayerServer, wasmMultiplayerLobby
}

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
