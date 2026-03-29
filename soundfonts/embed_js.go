//go:build js && wasm

package soundfonts

import _ "embed"

const (
	EmbeddedGeneralMIDIPath = "soundfonts/general-midi.sf2"
)

//go:embed general-midi.sf2
var embeddedGeneralMIDI []byte

func EmbeddedDataForPath(path string) ([]byte, bool) {
	switch path {
	case EmbeddedGeneralMIDIPath:
		if len(embeddedGeneralMIDI) > 0 {
			return embeddedGeneralMIDI, true
		}
	}
	return nil, false
}

func EmbeddedChoices() []string {
	out := make([]string, 0, 1)
	if len(embeddedGeneralMIDI) > 0 {
		out = append(out, EmbeddedGeneralMIDIPath)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
