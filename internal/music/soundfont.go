package music

import (
	"bytes"
	"fmt"
	"os"

	"github.com/sinshu/go-meltysynth/meltysynth"
)

type SoundFontBank struct {
	font *meltysynth.SoundFont
}

func ParseSoundFontFile(path string) (*SoundFontBank, error) {
	if data, ok := embeddedSoundFontDataForPath(path); ok {
		font, err := meltysynth.NewSoundFont(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("music: parse embedded soundfont %s: %w", path, err)
		}
		return &SoundFontBank{font: font}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("music: open soundfont %s: %w", path, err)
	}
	defer f.Close()
	font, err := meltysynth.NewSoundFont(f)
	if err != nil {
		return nil, fmt.Errorf("music: parse soundfont %s: %w", path, err)
	}
	return &SoundFontBank{font: font}, nil
}
