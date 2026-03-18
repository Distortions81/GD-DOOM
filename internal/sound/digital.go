package sound

import (
	"encoding/binary"
	"fmt"
	"strings"

	"gddoom/internal/wad"
)

type DigitalSound struct {
	Name       string
	Format     uint16
	SampleRate uint16
	Samples    []byte
}

type DigitalImportReport struct {
	Found   int
	Decoded int
	Failed  int
	Sounds  []DigitalSound
}

func ImportDigitalSounds(f *wad.File) DigitalImportReport {
	report := DigitalImportReport{}
	for _, l := range f.Lumps {
		if !strings.HasPrefix(l.Name, "DS") {
			continue
		}
		report.Found++
		data, err := f.LumpData(l)
		if err != nil {
			report.Failed++
			continue
		}
		s, err := ParseDigitalLump(l.Name, data)
		if err != nil {
			report.Failed++
			continue
		}
		report.Decoded++
		report.Sounds = append(report.Sounds, s)
	}
	return report
}

func ParseDigitalLump(name string, data []byte) (DigitalSound, error) {
	if len(data) < 8 {
		return DigitalSound{}, fmt.Errorf("digital %s: too small (%d)", name, len(data))
	}
	format := binary.LittleEndian.Uint16(data[0:2])
	if format != 3 {
		return DigitalSound{}, fmt.Errorf("digital %s: unsupported format %d", name, format)
	}
	rate := binary.LittleEndian.Uint16(data[2:4])
	count := int(binary.LittleEndian.Uint32(data[4:8]))
	if count < 0 {
		return DigitalSound{}, fmt.Errorf("digital %s: invalid sample count %d", name, count)
	}
	if len(data) != 8+count {
		return DigitalSound{}, fmt.Errorf("digital %s: size mismatch got=%d want=%d", name, len(data), 8+count)
	}
	samples := make([]byte, count)
	copy(samples, data[8:])
	return DigitalSound{
		Name:       name,
		Format:     format,
		SampleRate: rate,
		Samples:    samples,
	}, nil
}
