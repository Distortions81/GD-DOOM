//go:build !js || !wasm

package app

import (
	"gddoom/internal/freegames"
	"gddoom/internal/platformcfg"
)

func multiplayerBuildDefaults() (server, lobby string) {
	return defaultAuthorityCoopAddress, freegames.DefaultContentServer
}

func isWASMBuild() bool {
	return platformcfg.IsWASMBuild()
}
