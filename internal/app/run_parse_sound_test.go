package app

import "testing"

func TestPadDoomSoundSamples_PadsTo512With128(t *testing.T) {
	src := []byte{1, 2, 3}
	got := padDoomSoundSamples(src)
	if len(got) != 512 {
		t.Fatalf("len=%d want=512", len(got))
	}
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("prefix=%v want original prefix", got[:3])
	}
	for i := 3; i < len(got); i++ {
		if got[i] != 128 {
			t.Fatalf("tail byte at %d = %d want 128", i, got[i])
		}
	}
}

func TestPadDoomSoundSamples_AlreadyAlignedStillCopies(t *testing.T) {
	src := make([]byte, 512)
	for i := range src {
		src[i] = byte(i)
	}
	got := padDoomSoundSamples(src)
	if len(got) != 512 {
		t.Fatalf("len=%d want=512", len(got))
	}
	if &got[0] == &src[0] {
		t.Fatal("expected copy, got aliased slice")
	}
	for i := range src {
		if got[i] != src[i] {
			t.Fatalf("byte %d = %d want %d", i, got[i], src[i])
		}
	}
}
