package music

import (
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/wad"
)

func TestRenderEpisode1MusicPeaksWithinLimitAtDefaultGain(t *testing.T) {
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if genmidiLump, ok := wf.LumpByName("GENMIDI"); ok {
		if genmidiData, err := wf.LumpData(genmidiLump); err == nil {
			if parsed, err := ParseGENMIDIOP2PatchBank(genmidiData); err == nil {
				bank = parsed
			}
		}
	}

	for _, lumpName := range []string{"D_E1M1", "D_E1M2", "D_E1M3", "D_E1M4", "D_E1M5"} {
		lumpName := lumpName
		t.Run(lumpName, func(t *testing.T) {
			musLump, ok := wf.LumpByName(lumpName)
			if !ok {
				t.Fatalf("missing %s lump", lumpName)
			}
			musData, err := wf.LumpData(musLump)
			if err != nil {
				t.Fatalf("read %s: %v", lumpName, err)
			}

			d := NewOutputDriver(bank)
			if d.outputGain != DefaultOutputGain {
				t.Fatalf("default output gain=%.2f want %.2f", d.outputGain, DefaultOutputGain)
			}
			d.Reset()
			pcm, err := d.RenderMUS(musData)
			if err != nil {
				t.Fatalf("RenderMUS(%s) error: %v", lumpName, err)
			}
			if len(pcm) == 0 {
				t.Fatalf("expected non-empty PCM for %s", lumpName)
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
				t.Fatalf("unexpectedly low peak amplitude for %s: %d", lumpName, maxAbs)
			}
			if saturated > 0 {
				t.Fatalf("detected clipped/saturated samples in %s render: %d (peak=%d)", lumpName, saturated, maxAbs)
			}
			t.Logf("%s peak=%d (%.2f%%FS) saturated=%d", lumpName, maxAbs, float64(maxAbs)/32767.0*100.0, saturated)
		})
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
