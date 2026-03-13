//go:build js && wasm

package doomruntime

func isWASMBuild() bool {
	return true
}
