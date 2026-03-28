//go:build js && wasm

package music

import (
	_ "embed"
	"path/filepath"
	"strings"
)

const embeddedSC55SoundFontPath = "soundfonts/sc55.sf2"
const embeddedWindowsGMSoundFontPath = "soundfonts/windows-gm.sf2"

//go:embed sc55.sf2
var embeddedSC55SoundFont []byte

//go:embed windows-gm.sf2
var embeddedWindowsGMSoundFont []byte

func embeddedSoundFontDataForPath(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	base := strings.ToLower(filepath.Base(path))
	if base == "sc55.sf2" && len(embeddedSC55SoundFont) > 0 {
		return embeddedSC55SoundFont, true
	}
	if base == "windows-gm.sf2" && len(embeddedWindowsGMSoundFont) > 0 {
		return embeddedWindowsGMSoundFont, true
	}
	return nil, false
}

func EmbeddedSoundFontChoices() []string {
	out := make([]string, 0, 2)
	if len(embeddedSC55SoundFont) > 0 {
		out = append(out, embeddedSC55SoundFontPath)
	}
	if len(embeddedWindowsGMSoundFont) > 0 {
		out = append(out, embeddedWindowsGMSoundFontPath)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func DefaultEmbeddedSoundFontPath() string {
	if len(embeddedSC55SoundFont) > 0 {
		return embeddedSC55SoundFontPath
	}
	if len(embeddedWindowsGMSoundFont) == 0 {
		return ""
	}
	return embeddedWindowsGMSoundFontPath
}
