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

	for _, lumpName := range []string{"D_E1M1"} {
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

func TestAnalyzeE1M5MUSVolumeCompression(t *testing.T) {
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	musLump, ok := wf.LumpByName("D_E1M5")
	if !ok {
		t.Fatal("missing D_E1M5 lump")
	}
	musData, err := wf.LumpData(musLump)
	if err != nil {
		t.Fatalf("read D_E1M5: %v", err)
	}
	parsed, err := ParseMUSData(musData)
	if err != nil {
		t.Fatalf("ParseMUSData(D_E1M5) error: %v", err)
	}

	stats := AnalyzeMUSVolumeCompression(parsed, DefaultMUSVolumeCompression)
	if stats.NoteOnCount <= 0 {
		t.Fatal("expected note-on events in D_E1M5")
	}
	if stats.AvgNoteVelocityAfter <= stats.AvgNoteVelocityBefore {
		t.Fatalf("avg note velocity did not increase: before=%.2f after=%.2f", stats.AvgNoteVelocityBefore, stats.AvgNoteVelocityAfter)
	}
	t.Logf("D_E1M5 ratio=%.2f notes=%d avg_note_velocity %.2f -> %.2f volume_events=%d %.2f -> %.2f expression_events=%d %.2f -> %.2f",
		stats.Ratio,
		stats.NoteOnCount,
		stats.AvgNoteVelocityBefore, stats.AvgNoteVelocityAfter,
		stats.ControllerVolumeCount,
		stats.AvgControllerVolumeBefore, stats.AvgControllerVolumeAfter,
		stats.ControllerExpressionCount,
		stats.AvgControllerExpressionBefore, stats.AvgControllerExpressionAfter,
	)
}

func findDOOM1WADForMusicTests(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range []string{"DOOM.WAD", "doom.wad", "DOOM1.WAD", "doom1.wad"} {
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
