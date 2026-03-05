package music

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestRenderE1M1MusicPeakWithinLimit(t *testing.T) {
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}
	musLump, ok := wf.LumpByName("D_E1M1")
	if !ok {
		t.Fatal("missing D_E1M1 lump")
	}
	musData, err := wf.LumpData(musLump)
	if err != nil {
		t.Fatalf("read D_E1M1: %v", err)
	}

	var bank PatchBank
	if genmidiLump, ok := wf.LumpByName("GENMIDI"); ok {
		if genmidiData, err := wf.LumpData(genmidiLump); err == nil {
			if parsed, err := ParseGENMIDIOP2PatchBank(genmidiData); err == nil {
				bank = parsed
			}
		}
	}

	d := NewOutputDriver(bank)
	d.Reset()
	pcm, err := d.RenderMUS(musData)
	if err != nil {
		t.Fatalf("RenderMUS(D_E1M1) error: %v", err)
	}
	if len(pcm) == 0 {
		t.Fatal("expected non-empty PCM for D_E1M1")
	}

	maxAbs := 0
	saturated := 0
	for _, s := range pcm {
		if s == 32767 || s == -32768 {
			saturated++
		}
		v := int(s)
		if v < 0 {
			v = -v
		}
		if v > maxAbs {
			maxAbs = v
		}
	}
	if maxAbs < 256 {
		t.Fatalf("unexpectedly low peak amplitude: %d", maxAbs)
	}
	if saturated > 0 {
		t.Fatalf("detected clipped/saturated samples in E1M1 render: %d (peak=%d)", saturated, maxAbs)
	}
}

func findDOOM1WADForMusicTests(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range []string{"DOOM1.WAD", "DOOM.WAD"} {
			cand := filepath.Join(dir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Fatalf("DOOM1.WAD/DOOM.WAD not found from %s", wd)
	return ""
}
