package music

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gddoom/internal/wad"
)

var (
	benchmarkMeltySoundFontOnce sync.Once
	benchmarkMeltySoundFont     *SoundFontBank
	benchmarkMeltySoundFontErr  error
)

func benchmarkSoundFont(b *testing.B) *SoundFontBank {
	b.Helper()
	benchmarkMeltySoundFontOnce.Do(func() {
		benchmarkMeltySoundFont, benchmarkMeltySoundFontErr = ParseSoundFontFile(filepath.Join("..", "..", "soundfonts", "SC55.sf2"))
	})
	if benchmarkMeltySoundFontErr != nil {
		b.Fatalf("ParseSoundFontFile() error: %v", benchmarkMeltySoundFontErr)
	}
	return benchmarkMeltySoundFont
}

func newBenchmarkMeltySynthDriver(b *testing.B) *MeltySynthDriver {
	b.Helper()
	d, err := NewMeltySynthDriver(OutputSampleRate, benchmarkSoundFont(b))
	if err != nil {
		b.Fatalf("NewMeltySynthDriver() error: %v", err)
	}
	return d
}

func benchmarkDOOM1E1M1MUS(b *testing.B) []byte {
	b.Helper()
	wadPath := findDOOM1WADForMusicBenchmarks(b)
	wf, err := wad.Open(wadPath)
	if err != nil {
		b.Fatalf("open wad %s: %v", wadPath, err)
	}
	lump, ok := wf.LumpByName("D_E1M1")
	if !ok {
		b.Fatalf("missing D_E1M1 in %s", wadPath)
	}
	mus, err := wf.LumpData(lump)
	if err != nil {
		b.Fatalf("read D_E1M1 from %s: %v", wadPath, err)
	}
	return mus
}

func findDOOM1WADForMusicBenchmarks(tb testing.TB) string {
	tb.Helper()

	wd, err := os.Getwd()
	if err != nil {
		tb.Fatalf("getwd: %v", err)
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
	tb.Skipf("DOOM1.WAD/DOOM.WAD not found from %s", wd)
	return ""
}

func BenchmarkMeltySynthE1M1StreamStartup(b *testing.B) {
	mus := benchmarkDOOM1E1M1MUS(b)
	parsed, err := ParseMUSData(mus)
	if err != nil {
		b.Fatalf("ParseMUSData() error: %v", err)
	}
	for _, frames := range []int{256, 1024, 2048} {
		b.Run("chunk_frames_"+itoa(frames), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				driver := newBenchmarkMeltySynthDriver(b)
				b.StartTimer()
				stream, err := NewParsedMUSStreamRenderer(driver, parsed)
				if err != nil {
					b.Fatalf("NewParsedMUSStreamRenderer() error: %v", err)
				}
				_, _, err = stream.NextChunkS16LE(frames)
				if err != nil {
					b.Fatalf("NextChunkS16LE() error: %v", err)
				}
				b.StopTimer()
			}
		})
	}
}

func BenchmarkMeltySynthE1M1StreamRestart(b *testing.B) {
	mus := benchmarkDOOM1E1M1MUS(b)
	parsed, err := ParseMUSData(mus)
	if err != nil {
		b.Fatalf("ParseMUSData() error: %v", err)
	}
	for _, frames := range []int{256, 1024, 2048} {
		b.Run("chunk_frames_"+itoa(frames), func(b *testing.B) {
			driver := newBenchmarkMeltySynthDriver(b)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StartTimer()
				stream, err := NewParsedMUSStreamRenderer(driver, parsed)
				if err != nil {
					b.Fatalf("NewParsedMUSStreamRenderer() error: %v", err)
				}
				_, _, err = stream.NextChunkS16LE(frames)
				if err != nil {
					b.Fatalf("NextChunkS16LE() error: %v", err)
				}
				b.StopTimer()
			}
		})
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
