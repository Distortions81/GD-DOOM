//go:build !js || !wasm

package wad

func embeddedDataForPath(path string) ([]byte, bool) {
	return nil, false
}
