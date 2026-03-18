//go:build !js || !wasm

package runtimecfg

func isWASMBuild() bool {
	return false
}
