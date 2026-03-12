//go:build cgo

package music

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

func TestPureGoMaintainsFullSongActivityAgainstNuked(t *testing.T) {
	requireOPLTuningSuite(t)
	wadPath := findDOOM1WADForMusicTests(t)
	wf, err := wad.Open(wadPath)
	if err != nil {
		t.Fatalf("open wad %s: %v", wadPath, err)
	}

	var bank PatchBank
	if lump, ok := wf.LumpByName("GENMIDI"); ok {
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read GENMIDI: %v", err)
		}
		bank, err = ParseGENMIDIOP2PatchBank(data)
		if err != nil {
			t.Fatalf("parse GENMIDI: %v", err)
		}
	}

	songs := []string{"D_E1M1", "D_E1M5", "D_E1M8", "D_E3M5"}
	ratios := []struct {
		name           string
		minRMSRatio    float64
		minActiveRatio float64
	}{
		{name: "default", minRMSRatio: 0.20, minActiveRatio: 0.50},
	}

	_ = ratios
	for _, song := range songs {
		lump, ok := wf.LumpByName(song)
		if !ok {
			t.Logf("skip %s: missing lump in %s", song, wadPath)
			continue
		}
		musData, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read %s: %v", song, err)
		}

		nukedStats := renderSongStats(t, bank, sound.BackendNuked, musData)
		pureStats := renderSongStats(t, bank, sound.BackendPureGo, musData)
		if nukedStats.activeWindows == 0 {
			t.Fatalf("%s nuked active windows=0", song)
		}

		rmsRatio := pureStats.rms / nukedStats.rms
		activeRatio := float64(pureStats.activeWindows) / float64(nukedStats.activeWindows)
		dropoutRatio := activeWindowDropoutRatio(pureStats.windowRMS, nukedStats.windowRMS, 200)
		pureLongestSilent, pureLongestStart := longestInactiveRun(pureStats.windowRMS, 200)
		nukedLongestSilent, nukedLongestStart := longestInactiveRun(nukedStats.windowRMS, 200)
		t.Logf("%s purego rms=%.1f active=%d longestSilent=%d@%.1fs | nuked rms=%.1f active=%d longestSilent=%d@%.1fs | ratios rms=%.3f active=%.3f dropout=%.3f",
			song,
			pureStats.rms, pureStats.activeWindows, pureLongestSilent, windowIndexSeconds(pureLongestStart, 2048),
			nukedStats.rms, nukedStats.activeWindows, nukedLongestSilent, windowIndexSeconds(nukedLongestStart, 2048),
			rmsRatio, activeRatio, dropoutRatio)
		if rmsRatio < 0.20 {
			t.Fatalf("%s purego rms ratio=%.3f want >= 0.20", song, rmsRatio)
		}
		if activeRatio < 0.50 {
			t.Fatalf("%s purego active ratio=%.3f want >= 0.50", song, activeRatio)
		}
		if dropoutRatio > 0.35 {
			t.Fatalf("%s purego dropout ratio=%.3f want <= 0.35", song, dropoutRatio)
		}
		if pureLongestSilent > nukedLongestSilent+12 {
			t.Fatalf("%s purego longest silent run=%d want <= nuked+12 (%d)", song, pureLongestSilent, nukedLongestSilent+12)
		}
	}
}

func TestPureGoMaintainsWideDoomCorpusActivityAgainstNuked(t *testing.T) {
	requireOPLTuningSuite(t)

	type corpusWAD struct {
		label string
		path  string
	}
	wads := []corpusWAD{
		{label: "doom.wad", path: findNamedWADForMusicTests(t, "DOOM.WAD", "doom.wad")},
		{label: "doom2.wad", path: findNamedWADForMusicTests(t, "DOOM2.WAD", "doom2.wad")},
	}

	type corpusResult struct {
		wad           string
		song          string
		rmsRatio      float64
		activeRatio   float64
		dropoutRatio  float64
		pureSilent    int
		nukedSilent   int
		severityScore float64
	}
	var results []corpusResult
	totalSongs := 0
	const maxSongTics = 140 * 45

	for _, wadInfo := range wads {
		wf, err := wad.Open(wadInfo.path)
		if err != nil {
			t.Fatalf("open wad %s: %v", wadInfo.path, err)
		}
		bank := loadGENMIDIBankForMusicTests(t, wf)
		songs := parseableMusicLumpsForTests(t, wf)
		if len(songs) == 0 {
			t.Fatalf("%s has no parseable music lumps", wadInfo.path)
		}
		t.Logf("%s: testing %d music lumps", wadInfo.label, len(songs))

		for _, song := range songs {
			totalSongs++
			lump, ok := wf.LumpByName(song)
			if !ok {
				t.Fatalf("%s missing lump %s after discovery", wadInfo.path, song)
			}
			musData, err := wf.LumpData(lump)
			if err != nil {
				t.Fatalf("read %s/%s: %v", wadInfo.label, song, err)
			}
			events, err := ParseMUS(musData)
			if err != nil {
				t.Fatalf("parse %s/%s: %v", wadInfo.label, song, err)
			}
			trimmed := trimEventsToTics(events, maxSongTics)
			nukedStats := renderEventStats(t, bank, sound.BackendNuked, trimmed)
			pureStats := renderEventStats(t, bank, sound.BackendPureGo, trimmed)
			if nukedStats.activeWindows == 0 {
				t.Fatalf("%s/%s nuked active windows=0", wadInfo.label, song)
			}

			rmsRatio := pureStats.rms / nukedStats.rms
			activeRatio := float64(pureStats.activeWindows) / float64(nukedStats.activeWindows)
			dropoutRatio := activeWindowDropoutRatio(pureStats.windowRMS, nukedStats.windowRMS, 200)
			pureLongestSilent, _ := longestInactiveRun(pureStats.windowRMS, 200)
			nukedLongestSilent, _ := longestInactiveRun(nukedStats.windowRMS, 200)
			result := corpusResult{
				wad:          wadInfo.label,
				song:         song,
				rmsRatio:     rmsRatio,
				activeRatio:  activeRatio,
				dropoutRatio: dropoutRatio,
				pureSilent:   pureLongestSilent,
				nukedSilent:  nukedLongestSilent,
			}
			result.severityScore =
				math.Max(0, 0.20-rmsRatio)*4 +
					math.Max(0, 0.50-activeRatio)*4 +
					math.Max(0, dropoutRatio-0.35)*6 +
					math.Max(0, float64(pureLongestSilent-(nukedLongestSilent+12)))/24.0
			results = append(results, result)

			if rmsRatio < 0.20 {
				t.Errorf("%s/%s purego rms ratio=%.3f want >= 0.20", wadInfo.label, song, rmsRatio)
			}
			if activeRatio < 0.50 {
				t.Errorf("%s/%s purego active ratio=%.3f want >= 0.50", wadInfo.label, song, activeRatio)
			}
			if dropoutRatio > 0.35 {
				t.Errorf("%s/%s purego dropout ratio=%.3f want <= 0.35", wadInfo.label, song, dropoutRatio)
			}
			if pureLongestSilent > nukedLongestSilent+12 {
				t.Errorf("%s/%s purego longest silent run=%d want <= nuked+12 (%d)", wadInfo.label, song, pureLongestSilent, nukedLongestSilent+12)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].severityScore == results[j].severityScore {
			if results[i].dropoutRatio == results[j].dropoutRatio {
				if results[i].activeRatio == results[j].activeRatio {
					return results[i].song < results[j].song
				}
				return results[i].activeRatio < results[j].activeRatio
			}
			return results[i].dropoutRatio > results[j].dropoutRatio
		}
		return results[i].severityScore > results[j].severityScore
	})

	top := len(results)
	if top > 8 {
		top = 8
	}
	for i := 0; i < top; i++ {
		r := results[i]
		t.Logf("worst[%d] %s/%s rms=%.3f active=%.3f dropout=%.3f longestSilent=%d vs %d score=%.2f",
			i+1, r.wad, r.song, r.rmsRatio, r.activeRatio, r.dropoutRatio, r.pureSilent, r.nukedSilent, r.severityScore)
	}
	t.Logf("checked %d songs across %d WADs (trimmed to %d tics each)", totalSongs, len(wads), maxSongTics)
}

type songRenderStats struct {
	rms           float64
	activeWindows int
	windowRMS     []float64
}

func renderSongStats(t *testing.T, bank PatchBank, backend sound.Backend, musData []byte) songRenderStats {
	t.Helper()
	driver, err := NewOutputDriverWithBackend(bank, backend)
	if err != nil {
		t.Fatalf("new driver %s: %v", backend, err)
	}
	driver.Reset()
	pcm, err := driver.RenderMUS(musData)
	if err != nil {
		t.Fatalf("render mus %s: %v", backend, err)
	}
	windowRMS := rmsWindows(pcm, 2048)
	return songRenderStats{
		rms:           rmsInt16(pcm),
		activeWindows: countActiveWindows(windowRMS, 200),
		windowRMS:     windowRMS,
	}
}

func renderEventStats(t *testing.T, bank PatchBank, backend sound.Backend, events []Event) songRenderStats {
	t.Helper()
	driver, err := NewOutputDriverWithBackend(bank, backend)
	if err != nil {
		t.Fatalf("new driver %s: %v", backend, err)
	}
	driver.Reset()
	pcm := driver.Render(events)
	windowRMS := rmsWindows(pcm, 2048)
	return songRenderStats{
		rms:           rmsInt16(pcm),
		activeWindows: countActiveWindows(windowRMS, 200),
		windowRMS:     windowRMS,
	}
}

func rmsInt16(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		v := float64(s)
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func rmsWindows(samples []int16, window int) []float64 {
	if len(samples) < window*2 || window <= 0 {
		return nil
	}
	frames := len(samples) / 2
	out := make([]float64, 0, frames/window)
	for base := 0; base+window <= frames; base += window {
		var leftSum, rightSum float64
		for i := 0; i < window; i++ {
			l := float64(samples[(base+i)*2])
			r := float64(samples[(base+i)*2+1])
			leftSum += l * l
			rightSum += r * r
		}
		leftRMS := math.Sqrt(leftSum / float64(window))
		rightRMS := math.Sqrt(rightSum / float64(window))
		if rightRMS > leftRMS {
			out = append(out, rightRMS)
		} else {
			out = append(out, leftRMS)
		}
	}
	return out
}

func countActiveWindows(windows []float64, threshold float64) int {
	count := 0
	for _, w := range windows {
		if w >= threshold {
			count++
		}
	}
	return count
}

func activeWindowDropoutRatio(pure []float64, ref []float64, threshold float64) float64 {
	n := minSongTestInt(len(pure), len(ref))
	if n == 0 {
		return 0
	}
	refActive := 0
	dropouts := 0
	for i := 0; i < n; i++ {
		if ref[i] < threshold {
			continue
		}
		refActive++
		if pure[i] < threshold {
			dropouts++
		}
	}
	if refActive == 0 {
		return 0
	}
	return float64(dropouts) / float64(refActive)
}

func longestInactiveRun(windows []float64, threshold float64) (int, int) {
	run := 0
	longest := 0
	start := -1
	longestStart := -1
	for i, w := range windows {
		if w < threshold {
			if run == 0 {
				start = i
			}
			run++
			if run > longest {
				longest = run
				longestStart = start
			}
			continue
		}
		run = 0
		start = -1
	}
	return longest, longestStart
}

func minSongTestInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func windowIndexSeconds(index int, windowFrames int) float64 {
	if index < 0 || windowFrames <= 0 {
		return -1
	}
	return float64(index*windowFrames) / float64(OutputSampleRate)
}

func findNamedWADForMusicTests(t *testing.T, names ...string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		for _, name := range names {
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
	t.Fatalf("could not find any of %v from %s", names, wd)
	return ""
}

func loadGENMIDIBankForMusicTests(t *testing.T, wf *wad.File) PatchBank {
	t.Helper()

	lump, ok := wf.LumpByName("GENMIDI")
	if !ok {
		t.Fatalf("%s missing GENMIDI lump", wf.Path)
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		t.Fatalf("read GENMIDI from %s: %v", wf.Path, err)
	}
	bank, err := ParseGENMIDIOP2PatchBank(data)
	if err != nil {
		t.Fatalf("parse GENMIDI from %s: %v", wf.Path, err)
	}
	return bank
}

func parseableMusicLumpsForTests(t *testing.T, wf *wad.File) []string {
	t.Helper()

	seen := map[string]struct{}{}
	var songs []string
	for _, lump := range wf.Lumps {
		if !strings.HasPrefix(lump.Name, "D_") {
			continue
		}
		if _, ok := seen[lump.Name]; ok {
			continue
		}
		data, err := wf.LumpData(lump)
		if err != nil {
			t.Fatalf("read %s from %s: %v", lump.Name, wf.Path, err)
		}
		if _, err := ParseMUS(data); err != nil {
			continue
		}
		seen[lump.Name] = struct{}{}
		songs = append(songs, lump.Name)
	}
	sort.Strings(songs)
	return songs
}
