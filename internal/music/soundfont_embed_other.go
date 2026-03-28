//go:build !js || !wasm

package music

func embeddedSoundFontDataForPath(path string) ([]byte, bool) {
	return nil, false
}

func EmbeddedSoundFontChoices() []string {
	return nil
}

func DefaultEmbeddedSoundFontPath() string {
	return ""
}
