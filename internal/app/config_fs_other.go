//go:build !js || !wasm

package app

func configFileAccessSupported() bool {
	return true
}
