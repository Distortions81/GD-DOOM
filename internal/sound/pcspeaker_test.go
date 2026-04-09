package sound

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestParsePCSpeakerLumpValid(t *testing.T) {
	data := make([]byte, 4+3)
	binary.LittleEndian.PutUint16(data[0:2], 0)
	binary.LittleEndian.PutUint16(data[2:4], 3)
	data[4] = 0x20
	data[5] = 0x30
	data[6] = 0x40

	s, err := ParsePCSpeakerLump("DPTONE", data)
	if err != nil {
		t.Fatalf("ParsePCSpeakerLump() error=%v", err)
	}
	if got, want := len(s.Tones), 3; got != want {
		t.Fatalf("tones len=%d want=%d", got, want)
	}
	if s.Tones[1] != 0x30 {
		t.Fatalf("tone[1]=%d want=48", s.Tones[1])
	}
}

func TestParsePCSpeakerLumpSharesPayloadSlice(t *testing.T) {
	data := make([]byte, 4+2)
	binary.LittleEndian.PutUint16(data[0:2], 0)
	binary.LittleEndian.PutUint16(data[2:4], 2)
	data[4] = 0x20
	data[5] = 0x30

	s, err := ParsePCSpeakerLump("DPTONE", data)
	if err != nil {
		t.Fatalf("ParsePCSpeakerLump() error=%v", err)
	}
	data[4] = 0x55
	if s.Tones[0] != 0x55 {
		t.Fatalf("Tones[0]=%#x want shared payload %#x", s.Tones[0], byte(0x55))
	}
}

func TestParsePCSpeakerLumpInvalidHeader(t *testing.T) {
	data := []byte{1, 0, 1, 0, 0x20}
	if _, err := ParsePCSpeakerLump("DPTONE", data); err == nil {
		t.Fatal("expected header error")
	}
}

func TestParsePCSpeakerLumpInvalidSize(t *testing.T) {
	data := []byte{0, 0, 2, 0, 0x20}
	if _, err := ParsePCSpeakerLump("DPTONE", data); err == nil {
		t.Fatal("expected size mismatch error")
	}
}

func TestImportPCSpeakerSounds(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sound.wad")
	data := buildWADForSoundTests(t, []lumpSpec{
		{name: "DPGOOD", data: []byte{0, 0, 1, 0, 0x33}},
		{name: "DPBAD", data: []byte{1, 0, 0, 0}},
		{name: "NOTSND", data: []byte{0xff}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	r := ImportPCSpeakerSounds(f)
	if r.Found != 2 || r.Decoded != 1 || r.Failed != 1 {
		t.Fatalf("report=%+v", r)
	}
}

func TestBuildToneSequenceSilence(t *testing.T) {
	s := PCSpeakerSound{Name: "DPTEST", Tones: []byte{0, 0, 0}}
	seq := BuildToneSequence(s)
	if len(seq) != 3 {
		t.Fatalf("len=%d want=3", len(seq))
	}
	for i, tone := range seq {
		if tone.Active {
			t.Fatalf("seq[%d].Active=true, want false for silence", i)
		}
	}
}

func TestBuildToneSequenceLength(t *testing.T) {
	tones := []byte{10, 20, 30}
	s := PCSpeakerSound{Name: "DPTEST", Tones: tones}
	seq := BuildToneSequence(s)
	if len(seq) != len(tones) {
		t.Fatalf("len=%d want=%d", len(seq), len(tones))
	}
}

func TestBuildToneSequenceActive(t *testing.T) {
	s := PCSpeakerSound{Name: "DPTEST", Tones: []byte{0, 60, 0}}
	seq := BuildToneSequence(s)
	if seq[0].Active {
		t.Fatal("seq[0] should be silent")
	}
	if !seq[1].Active {
		t.Fatal("seq[1] should be active")
	}
	if seq[1].ToneValue != 60 {
		t.Fatalf("seq[1].ToneValue=%d want=60", seq[1].ToneValue)
	}
	if seq[2].Active {
		t.Fatal("seq[2] should be silent")
	}
}

func TestBuildToneSequenceToneFrequency(t *testing.T) {
	cases := []struct {
		tone byte
		want float64
	}{
		{1, float64(pcSpeakerPITHz) / 6818.0},
		{10, float64(pcSpeakerPITHz) / 5279.0},
		{20, float64(pcSpeakerPITHz) / 3950.0},
		{40, float64(pcSpeakerPITHz) / 2213.0},
		{80, float64(pcSpeakerPITHz) / 697.0},
		{126, float64(pcSpeakerPITHz) / 184.0},
	}
	for _, tc := range cases {
		got := (PCSpeakerTone{Active: true, ToneValue: tc.tone}).ToneFrequency()
		if math.Abs(got-tc.want) > 0.001 {
			t.Fatalf("tone=%d freq=%f want=%f", tc.tone, got, tc.want)
		}
	}
	if got := (PCSpeakerTone{Active: false, ToneValue: 60}).ToneFrequency(); got != 0 {
		t.Fatalf("silent ToneFrequency()=%f want=0", got)
	}
	if got := (PCSpeakerTone{Active: true, ToneValue: 128}).ToneFrequency(); got != 0 {
		t.Fatalf("out-of-range ToneFrequency()=%f want=0", got)
	}
}

func TestImportPCSpeakerSoundsSharesWADPayloadView(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sound.wad")
	data := buildWADForSoundTests(t, []lumpSpec{
		{name: "DPGOOD", data: []byte{0, 0, 2, 0, 0x33, 0x44}},
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write wad: %v", err)
	}
	f, err := wad.Open(path)
	if err != nil {
		t.Fatalf("open wad: %v", err)
	}

	r := ImportPCSpeakerSounds(f)
	if len(r.Sounds) != 1 {
		t.Fatalf("decoded=%d want=1", len(r.Sounds))
	}
	view, err := f.LumpDataView(f.Lumps[0])
	if err != nil {
		t.Fatalf("LumpDataView() error: %v", err)
	}
	view[4] = 0x55
	if r.Sounds[0].Tones[0] != 0x55 {
		t.Fatalf("Tones[0]=%#x want shared payload %#x", r.Sounds[0].Tones[0], byte(0x55))
	}
}
