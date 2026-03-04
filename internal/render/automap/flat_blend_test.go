package automap

import "testing"

func TestBlendFlatRGBA64OpaqueMatchesGeneric(t *testing.T) {
	a := make([]byte, flatRGBABytes)
	b := make([]byte, flatRGBABytes)
	for i := 0; i < flatRGBABytes; i += 4 {
		a[i+0] = byte((i / 4) % 251)
		a[i+1] = byte((i / 8) % 241)
		a[i+2] = byte((i / 16) % 239)
		a[i+3] = 0xFF
		b[i+0] = byte((255 - (i / 4)) % 251)
		b[i+1] = byte((200 - (i / 8)) % 241)
		b[i+2] = byte((120 + (i / 16)) % 239)
		b[i+3] = 0xFF
	}
	const alpha = 0.375
	got := blendFlatRGBA64Opaque(a, b, alpha)
	want := blendRGBA(a, b, alpha)
	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("pixel[%d]=%d want=%d", i, got[i], want[i])
		}
	}
}

func TestBlendFlatRGBA64OpaqueSetsOpaqueAlpha(t *testing.T) {
	a := make([]byte, flatRGBABytes)
	b := make([]byte, flatRGBABytes)
	for i := 0; i < flatRGBABytes; i += 4 {
		a[i+3] = 0xFF
		b[i+3] = 0xFF
	}
	got := blendFlatRGBA64Opaque(a, b, 0.5)
	for i := 3; i < len(got); i += 4 {
		if got[i] != 0xFF {
			t.Fatalf("alpha[%d]=%d want=255", i, got[i])
		}
	}
}
