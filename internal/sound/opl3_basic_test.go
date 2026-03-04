package sound

import "testing"

func TestBasicOPL3GenerateNonZeroWhenKeyOn(t *testing.T) {
	opl := NewBasicOPL3(49716)
	// Channel 0 tone: FNUM + BLOCK + KEYON.
	opl.WriteReg(0xA0, 0x98)
	opl.WriteReg(0xB0, 0x31) // block=4, key-on
	opl.WriteReg(0x43, 0x00) // loud carrier
	opl.WriteReg(0xC0, 0x30) // L+R

	pcm := opl.GenerateStereoS16(256)
	if len(pcm) != 512 {
		t.Fatalf("stereo samples=%d want=%d", len(pcm), 512)
	}
	nonZero := false
	for _, s := range pcm {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatal("expected non-zero PCM with key-on tone")
	}
}

func TestBasicOPL3KeyOffSilencesOutput(t *testing.T) {
	opl := NewBasicOPL3(49716)
	opl.WriteReg(0xA0, 0x60)
	opl.WriteReg(0x43, 0x00)
	opl.WriteReg(0xC0, 0x30)
	opl.WriteReg(0xB0, 0x2C) // key-on
	_ = opl.GenerateStereoS16(64)
	opl.WriteReg(0xB0, 0x0C) // key-off

	pcm := opl.GenerateStereoS16(128)
	for i, s := range pcm {
		if s != 0 {
			t.Fatalf("expected silence after key-off, idx=%d sample=%d", i, s)
		}
	}
}

func TestBasicOPL3GenerateMonoU8(t *testing.T) {
	opl := NewBasicOPL3(49716)
	opl.WriteReg(0xA0, 0x80)
	opl.WriteReg(0xB0, 0x31)
	opl.WriteReg(0x43, 0x00)
	mono := opl.GenerateMonoU8(128)
	if len(mono) != 128 {
		t.Fatalf("mono samples=%d want=128", len(mono))
	}
}
