//go:build !js || !wasm

package app

import "gddoom/internal/platformcfg"

func multiplayerBuildDefaults() (server, lobby string) {
	return "", ""
}

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
