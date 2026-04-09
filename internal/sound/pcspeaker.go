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
	// pcSpeakerPITHz is the 8253/8254 PIT oscillator frequency.
	pcSpeakerPITHz = 1193181
)

// pcSpeakerDivisors maps raw DP* tone bytes to PIT divisors, matching the
// vanilla PC speaker lookup table used by Chocolate Doom.
var pcSpeakerDivisors = [...]uint16{
	0,
	6818, 6628, 6449, 6279, 6087, 5906, 5736, 5575,
	5423, 5279, 5120, 4971, 4830, 4697, 4554, 4435,
	4307, 4186, 4058, 3950, 3836, 3728, 3615, 3519,
	3418, 3323, 3224, 3131, 3043, 2960, 2875, 2794,
	2711, 2633, 2560, 2485, 2415, 2348, 2281, 2213,
	2153, 2089, 2032, 1975, 1918, 1864, 1810, 1757,
	1709, 1659, 1612, 1565, 1521, 1478, 1435, 1395,
	1355, 1316, 1280, 1242, 1207, 1173, 1140, 1107,
	1075, 1045, 1015, 986, 959, 931, 905, 879,
	854, 829, 806, 783, 760, 739, 718, 697,
	677, 658, 640, 621, 604, 586, 570, 553,
	538, 522, 507, 493, 479, 465, 452, 439,
	427, 415, 403, 391, 380, 369, 359, 348,
	339, 329, 319, 310, 302, 293, 285, 276,
	269, 261, 253, 246, 239, 232, 226, 219,
	213, 207, 201, 195, 190, 184, 179,
}

func PCSpeakerPITHz() int {
	return pcSpeakerPITHz
}

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
	divisor := t.ToneDivisor()
	if divisor == 0 {
		return 0
	}
	return float64(pcSpeakerPITHz) / float64(divisor)
}

// ToneDivisor returns the vanilla PIT divisor for this tone, or 0 if silent
// or out of range.
func (t PCSpeakerTone) ToneDivisor() uint16 {
	if !t.Active || t.ToneValue == 0 {
		return 0
	}
	if int(t.ToneValue) >= len(pcSpeakerDivisors) {
		return 0
	}
	return pcSpeakerDivisors[t.ToneValue]
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
