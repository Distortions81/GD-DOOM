//go:build js && wasm

package sessiontransition

func isWASMBuild() bool {
	return true
}
