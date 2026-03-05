//go:build cgo

package sound

import "testing"

func TestNewOPL3UsesNukedWhenCGOEnabled(t *testing.T) {
	if _, ok := NewOPL3(49716).(*NukedOPL3); !ok {
		t.Fatal("NewOPL3 should select Nuked backend when cgo is enabled")
	}
}

func TestNukedOPL3KeyOffHasReleaseTail(t *testing.T) {
	opl := NewNukedOPL3(49716)
	opl.Reset()

	// Simple two-op patch on channel 0.
	opl.WriteReg(0x20, 0x21)
	opl.WriteReg(0x23, 0x21)
	opl.WriteReg(0x40, 0x3f)
	opl.WriteReg(0x43, 0x00)
	opl.WriteReg(0x60, 0xf0)
	opl.WriteReg(0x63, 0xf0)
	opl.WriteReg(0x80, 0x77)
	opl.WriteReg(0x83, 0x77)
	opl.WriteReg(0xc0, 0x31)
	opl.WriteReg(0xa0, 0x98)
	opl.WriteReg(0xb0, 0x31) // key on
	_ = opl.GenerateStereoS16(128)

	opl.WriteReg(0xb0, 0x11) // key off
	pcm := opl.GenerateStereoS16(64)
	if len(pcm) != 128 {
		t.Fatalf("samples=%d want=128", len(pcm))
	}
	var tail int64
	for i := 0; i < len(pcm); i += 2 {
		v := int64(pcm[i])
		if v < 0 {
			v = -v
		}
		tail += v
	}
	if tail == 0 {
		t.Fatal("expected non-zero release tail after key-off")
	}
}
