package sound

import (
	"encoding/binary"
	"fmt"
	"strings"

	"gddoom/internal/wad"
)

type PCSpeakerSound struct {
	Name  string
	Tones []byte
}

type PCSpeakerImportReport struct {
	Found   int
	Decoded int
	Failed  int
	Sounds  []PCSpeakerSound
}

func ImportPCSpeakerSounds(f *wad.File) PCSpeakerImportReport {
	report := PCSpeakerImportReport{}
	for _, l := range f.Lumps {
		if !strings.HasPrefix(l.Name, "DP") {
			continue
		}
		report.Found++
		data, err := f.LumpDataView(l)
		if err != nil {
			report.Failed++
			continue
		}
		s, err := ParsePCSpeakerLump(l.Name, data)
		if err != nil {
			report.Failed++
			continue
		}
		report.Decoded++
		report.Sounds = append(report.Sounds, s)
	}
	return report
}

func ParsePCSpeakerLump(name string, data []byte) (PCSpeakerSound, error) {
	if len(data) < 4 {
		return PCSpeakerSound{}, fmt.Errorf("pcspeaker %s: too small (%d)", name, len(data))
	}
	zero := binary.LittleEndian.Uint16(data[0:2])
	if zero != 0 {
		return PCSpeakerSound{}, fmt.Errorf("pcspeaker %s: expected leading 0, got %d", name, zero)
	}
	count := int(binary.LittleEndian.Uint16(data[2:4]))
	if count < 0 {
		return PCSpeakerSound{}, fmt.Errorf("pcspeaker %s: invalid count %d", name, count)
	}
	if len(data) != 4+count {
		return PCSpeakerSound{}, fmt.Errorf("pcspeaker %s: size mismatch got=%d want=%d", name, len(data), 4+count)
	}
	return PCSpeakerSound{Name: name, Tones: data[4:]}, nil
}
