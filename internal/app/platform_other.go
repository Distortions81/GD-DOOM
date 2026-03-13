//go:build !js || !wasm

package app

func isWASMBuild() bool {
	return false
}
