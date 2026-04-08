package sound

import (
	"encoding/binary"
	"fmt"
	"strings"

	"gddoom/internal/wad"
)

const (
	// pcSpeakerTicRate is the DMX timer rate: 4 ticks per game tic at 35 Hz = 140 Hz.
	// Each tone byte lasts 1/140th of a second, not 1/35th.
	pcSpeakerTicRate = 140
	// pcSpeakerPITHz is the 8253/8254 PIT oscillator frequency used to derive
	// the output frequency from a tone byte: freq = pcSpeakerPITHz / (tone * 60).
	pcSpeakerPITHz = 1193180
)

// PCSpeakerTone holds a single DMX-timer tick of PC speaker data.
// Active is true when the PIT is driving the speaker; ToneValue is the
// original lump byte (0 = silence, non-zero gives the PIT divisor).
type PCSpeakerTone struct {
	Active    bool
	ToneValue byte
}

// ToneFrequency returns the PIT output frequency for this tone, or 0 if silent.
func (t PCSpeakerTone) ToneFrequency() float64 {
	if !t.Active || t.ToneValue == 0 {
		return 0
	}
	return float64(pcSpeakerPITHz) / float64(int(t.ToneValue)*60)
}

// BuildToneSequence converts the raw lump tone bytes into a []PCSpeakerTone
// slice (one entry per DMX tick at 140 Hz).  This is the compact intermediate
// representation — no per-sample data, just 140 entries per second of audio.
func BuildToneSequence(s PCSpeakerSound) []PCSpeakerTone {
	out := make([]PCSpeakerTone, len(s.Tones))
	for i, b := range s.Tones {
		out[i] = PCSpeakerTone{Active: b != 0, ToneValue: b}
	}
	return out
}

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
