//go:build !js || !wasm

package wad

func BrowserLocalWADPaths() []string {
	return nil
}
