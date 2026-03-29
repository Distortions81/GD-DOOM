package app

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"gddoom/internal/media"
	"gddoom/internal/music"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"

	"github.com/remeh/sizedwaitgroup"
)

type dumpMusicTarget struct {
	label       string
	displayName string
	path        string
	pwadPaths   []string
}

type dumpMusicRenderer struct {
	label       string
	displayName string
	backend     music.Backend
	fontPath    string
	soundFont   *music.SoundFontBank
}

type dumpMusicTrack struct {
	lumpName string
	fileBase string
	label    string
	level    string
	music    string
	version  string
	synth    string
}

type dumpMusicJob struct {
	target   dumpMusicTarget
	renderer dumpMusicRenderer
	track    dumpMusicTrack
	musData  []byte
	outPath  string
}

const dumpMusicNormalizePadDB = 3.0

func dumpMusicOutputExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, nil
	}
	return info.Size() > 0, nil
}

func dumpMusicWAVs(outDir string, resolvedWADPath string, wadExplicit bool, pwadPaths []string, explicitSoundFont string, stdout io.Writer, stderr io.Writer) error {
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		outDir = "out/music-dump"
	}
	targets, err := detectDumpMusicTargets(resolvedWADPath, wadExplicit, pwadPaths)
	if err != nil {
		return err
	}
	renderers, err := detectDumpMusicRenderers(strings.TrimSpace(explicitSoundFont), stderr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	workerLimit := runtime.NumCPU()
	if workerLimit < 1 {
		workerLimit = 1
	}
	var outputMu sync.Mutex
	var meltySynthMu sync.Mutex
	for _, target := range targets {
		wf, _, err := openWADStack(target.path, target.pwadPaths)
		if err != nil {
			return fmt.Errorf("open %s: %w", target.path, err)
		}
		tracks, err := dumpMusicTracksForWAD(wf)
		if err != nil {
			return fmt.Errorf("enumerate tracks for %s: %w", target.path, err)
		}
		if len(tracks) == 0 {
			if stderr != nil {
				fmt.Fprintf(stderr, "dump music: skip wad=%s no parseable music tracks\n", target.path)
			}
			continue
		}
		wadOut := filepath.Join(outDir, target.label)
		if err := os.MkdirAll(wadOut, 0o755); err != nil {
			return fmt.Errorf("create wad output directory: %w", err)
		}
		if splash, ok := resolveDumpBootSplashTexture(wf); ok {
			splashPath := filepath.Join(wadOut, "splash.png")
			if err := writeBootSplashPNG(splashPath, splash); err != nil {
				return fmt.Errorf("write splash %s: %w", splashPath, err)
			}
		}
		if err := writeDumpMusicTracksManifest(filepath.Join(wadOut, "tracks.txt"), tracks); err != nil {
			return fmt.Errorf("write tracks manifest for %s: %w", target.path, err)
		}
		var (
			splashTex media.WallTexture
			fontBank  map[rune]media.WallTexture
		)
		if texSet, err := doomtex.LoadFromWAD(wf); err == nil {
			splashTex = buildBootSplashTexture(texSet)
			fontBank = buildMessageFontBank(texSet)
		}
		if splashTex.Width == 0 || splashTex.Height == 0 || len(splashTex.RGBA) != splashTex.Width*splashTex.Height*4 {
			if splash, ok := resolveDumpBootSplashTexture(wf); ok {
				splashTex = splash
			}
		}
		patchBank, err := resolveMusicPatchBank(wf, "", nil)
		if err != nil {
			return fmt.Errorf("resolve music patch bank for %s: %w", target.path, err)
		}
		jobs := make([]dumpMusicJob, 0, len(renderers)*len(tracks))
		for _, renderer := range renderers {
			renderOut := filepath.Join(wadOut, renderer.label)
			if err := os.MkdirAll(renderOut, 0o755); err != nil {
				return fmt.Errorf("create renderer output directory: %w", err)
			}
			for _, track := range tracks {
				outPath := filepath.Join(renderOut, dumpMusicOutputBase(renderer, track)+".wav")
				exists, err := dumpMusicOutputExists(outPath)
				if err != nil {
					return fmt.Errorf("stat %s: %w", outPath, err)
				}
				if exists {
					continue
				}
				lump, ok := wf.LumpByName(track.lumpName)
				if !ok {
					return fmt.Errorf("missing lump %s in %s", track.lumpName, target.path)
				}
				musData, err := wf.LumpDataView(lump)
				if err != nil {
					return fmt.Errorf("read %s from %s: %w", track.lumpName, target.path, err)
				}
				jobs = append(jobs, dumpMusicJob{
					target:   target,
					renderer: renderer,
					track: dumpMusicTrack{
						lumpName: track.lumpName,
						fileBase: track.fileBase,
						label:    track.label,
						level:    track.level,
						music:    track.music,
						version:  target.displayName,
						synth:    renderer.displayName,
					},
					musData: musData,
					outPath: outPath,
				})
			}
		}
		swg := sizedwaitgroup.New(workerLimit)
		errCh := make(chan error, len(jobs))
		for _, job := range jobs {
			job := job
			swg.Add()
			go func() {
				defer swg.Done()
				defer func() {
					if r := recover(); r != nil {
						errCh <- fmt.Errorf("panic while rendering %s with %s for %s: %v\n%s", job.track.lumpName, job.renderer.label, job.target.path, r, debug.Stack())
					}
				}()
				pcm, err := dumpMusicPCMConcurrentSafe(patchBank, job.renderer, job.musData, &meltySynthMu)
				if err != nil {
					errCh <- fmt.Errorf("render %s with %s for %s: %w", job.track.lumpName, job.renderer.label, job.target.path, err)
					return
				}
				pcm = normalizeDumpMusicPCM(pcm, dumpMusicNormalizePadDB)
				if err := writePCM16StereoWAV(job.outPath, music.OutputSampleRate, pcm); err != nil {
					errCh <- fmt.Errorf("write %s: %w", job.outPath, err)
					return
				}
				coverPath := strings.TrimSuffix(job.outPath, filepath.Ext(job.outPath)) + ".png"
				if err := writeDumpTrackCoverPNG(coverPath, splashTex, fontBank, job.track); err != nil {
					errCh <- fmt.Errorf("write %s: %w", coverPath, err)
					return
				}
				if stdout != nil {
					outputMu.Lock()
					fmt.Fprintf(stdout, "dump-music wad=%s renderer=%s track=%s out=%s\n", job.target.label, job.renderer.label, job.track.lumpName, job.outPath)
					outputMu.Unlock()
				}
			}()
		}
		swg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func dumpMusicPCMConcurrentSafe(bank music.PatchBank, renderer dumpMusicRenderer, musData []byte, meltySynthMu *sync.Mutex) ([]int16, error) {
	if music.ResolveBackend(renderer.backend) == music.BackendMeltySynth && meltySynthMu != nil {
		meltySynthMu.Lock()
		defer meltySynthMu.Unlock()
	}
	return dumpMusicPCM(bank, renderer, musData)
}

func detectDumpMusicTargets(resolvedWADPath string, wadExplicit bool, pwadPaths []string) ([]dumpMusicTarget, error) {
	if wadExplicit {
		if strings.TrimSpace(resolvedWADPath) == "" {
			return nil, fmt.Errorf("-wad is required when exporting a specific WAD")
		}
		return []dumpMusicTarget{{
			label:       dumpMusicWADLabel(resolvedWADPath),
			displayName: dumpMusicWADDisplayName(resolvedWADPath),
			path:        resolvedWADPath,
			pwadPaths:   append([]string(nil), pwadPaths...),
		}}, nil
	}
	choices := detectAvailableIWADChoices(".")
	if len(choices) == 0 {
		if strings.TrimSpace(resolvedWADPath) == "" {
			return nil, fmt.Errorf("no IWADs found")
		}
		return []dumpMusicTarget{{
			label:       dumpMusicWADLabel(resolvedWADPath),
			displayName: dumpMusicWADDisplayName(resolvedWADPath),
			path:        resolvedWADPath,
		}}, nil
	}
	out := make([]dumpMusicTarget, 0, len(choices))
	seen := make(map[string]struct{}, len(choices))
	for _, choice := range choices {
		if dumpMusicShouldSkipIWADChoice(choice) {
			continue
		}
		path := strings.TrimSpace(resolveIWADAliasPath(choice.Path))
		if path == "" {
			continue
		}
		key := strings.ToUpper(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, dumpMusicTarget{
			label:       dumpMusicWADLabel(path),
			displayName: strings.TrimSpace(choice.Label),
			path:        path,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no IWADs found")
	}
	return out, nil
}

func dumpMusicShouldSkipIWADChoice(choice iwadChoice) bool {
	return strings.EqualFold(strings.TrimSpace(choice.Label), "DOOM Shareware")
}

func detectDumpMusicRenderers(explicitSoundFont string, stderr io.Writer) ([]dumpMusicRenderer, error) {
	renderers := []dumpMusicRenderer{{
		label:       "OPL",
		displayName: "OPL",
		backend:     music.BackendImpSynth,
	}}
	paths := detectAvailableSoundFonts("soundfonts")
	if explicitSoundFont != "" {
		paths = append(paths, explicitSoundFont)
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return strings.ToUpper(paths[i]) < strings.ToUpper(paths[j])
	})
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := strings.ToUpper(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		sf, err := music.ParseSoundFontFile(path)
		if err != nil {
			if explicitSoundFont != "" && strings.EqualFold(path, explicitSoundFont) {
				return nil, err
			}
			if stderr != nil {
				fmt.Fprintf(stderr, "dump music: skip soundfont=%s error=%v\n", path, err)
			}
			continue
		}
		renderers = append(renderers, dumpMusicRenderer{
			label:       dumpMusicRendererLabel(path),
			displayName: dumpMusicRendererDisplayName(path),
			backend:     music.BackendMeltySynth,
			fontPath:    path,
			soundFont:   sf,
		})
	}
	return renderers, nil
}

func dumpMusicTracksForWAD(wf *wad.File) ([]dumpMusicTrack, error) {
	episodes := musicPlayerEpisodesForWAD(wf)
	if len(episodes) == 0 {
		return nil, nil
	}
	out := make([]dumpMusicTrack, 0, 64)
	seenNames := make(map[string]int, 64)
	appendTrack := func(track runtimecfg.MusicPlayerTrack) error {
		lump := strings.ToUpper(strings.TrimSpace(track.LumpName))
		if lump == "" {
			return nil
		}
		l, ok := wf.LumpByName(lump)
		if !ok {
			return nil
		}
		data, err := wf.LumpDataView(l)
		if err != nil {
			return err
		}
		if _, err := music.ParseMUS(data); err != nil {
			return nil
		}
		base := dumpMusicTrackBase(track)
		if base == "" {
			base = strings.ToLower(strings.TrimPrefix(lump, "D_"))
		}
		if n := seenNames[base]; n > 0 {
			base = fmt.Sprintf("%s-%02d", base, n+1)
		}
		seenNames[base]++
		out = append(out, dumpMusicTrack{
			lumpName: lump,
			fileBase: base,
			label:    dumpMusicTrackLabel(track),
			level:    dumpMusicTrackLevel(track),
			music:    dumpMusicTrackMusic(track),
		})
		return nil
	}
	for _, episode := range episodes {
		for _, track := range episode.Tracks {
			if err := appendTrack(track); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func dumpMusicTrackBase(track runtimecfg.MusicPlayerTrack) string {
	parts := make([]string, 0, 2)
	if mapName := strings.ToUpper(strings.TrimSpace(string(track.MapName))); mapName != "" {
		parts = append(parts, mapName)
	}
	if musicName := musicDumpFilenamePart(track.MusicName); musicName != "" {
		parts = append(parts, musicName)
	}
	if len(parts) == 0 {
		if label := musicDumpFilenamePart(track.Label); label != "" && !strings.EqualFold(label, "Other") {
			parts = append(parts, label)
		}
	}
	if len(parts) == 0 {
		lump := strings.ToUpper(strings.TrimSpace(track.LumpName))
		lump = strings.TrimPrefix(lump, "D_")
		if lump != "" {
			parts = append(parts, lump)
		}
	}
	return strings.Join(parts, "-")
}

func dumpMusicOutputBase(renderer dumpMusicRenderer, track dumpMusicTrack) string {
	parts := make([]string, 0, 2)
	if rendererLabel := strings.TrimSpace(renderer.label); rendererLabel != "" {
		parts = append(parts, rendererLabel)
	}
	if trackBase := strings.TrimSpace(track.fileBase); trackBase != "" {
		parts = append(parts, trackBase)
	}
	if len(parts) == 0 {
		return "track"
	}
	return strings.Join(parts, "-")
}

func dumpMusicTrackLabel(track runtimecfg.MusicPlayerTrack) string {
	label := dumpMusicTrackLevel(track)
	musicName := dumpMusicTrackMusic(track)
	lump := strings.ToUpper(strings.TrimSpace(track.LumpName))
	if label != "" && musicName != "" && strings.EqualFold(label, musicName) {
		musicName = ""
	}
	if label != "" && musicName != "" {
		return fmt.Sprintf("%s | %s | %s", label, musicName, lump)
	}
	if label != "" && lump != "" {
		return fmt.Sprintf("%s | %s", label, lump)
	}
	if musicName != "" && lump != "" {
		return fmt.Sprintf("%s | %s", musicName, lump)
	}
	if label != "" {
		return label
	}
	if musicName != "" {
		return musicName
	}
	return lump
}

func dumpMusicTrackLevel(track runtimecfg.MusicPlayerTrack) string {
	label := strings.TrimSpace(track.Label)
	musicName := strings.TrimSpace(track.MusicName)
	if label != "" && musicName != "" && strings.EqualFold(label, musicName) {
		return "Other"
	}
	if label != "" {
		return label
	}
	lump := strings.ToUpper(strings.TrimSpace(track.LumpName))
	return lump
}

func dumpMusicTrackMusic(track runtimecfg.MusicPlayerTrack) string {
	musicName := strings.TrimSpace(track.MusicName)
	if musicName != "" {
		return musicName
	}
	lump := strings.ToUpper(strings.TrimSpace(track.LumpName))
	return lump
}

func writeDumpMusicTracksManifest(path string, tracks []dumpMusicTrack) error {
	var b strings.Builder
	for _, track := range tracks {
		line := strings.TrimSpace(track.label)
		if line == "" {
			line = strings.ToUpper(strings.TrimSpace(track.lumpName))
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeDumpTrackCoverPNG(path string, splash media.WallTexture, fontBank map[rune]media.WallTexture, track dumpMusicTrack) error {
	img, err := renderDumpTrackCover(splash, fontBank, track)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func renderDumpTrackCover(splash media.WallTexture, fontBank map[rune]media.WallTexture, track dumpMusicTrack) (*image.RGBA, error) {
	img, err := buildBootSplashImage(splash)
	if err != nil {
		return nil, err
	}
	if len(fontBank) == 0 {
		return img, nil
	}
	lines := dumpTrackCoverLines(track)
	if len(lines) == 0 {
		return img, nil
	}
	scale := 4
	spaceW := 8 * scale
	boxPadX := 48
	boxPadY := 28
	lineGap := 18
	textW := 0
	textH := 0
	lineHeights := make([]int, 0, len(lines))
	for i, line := range lines {
		w := measureDumpFontLine(fontBank, line, scale, spaceW)
		if w > textW {
			textW = w
		}
		h := dumpFontLineHeight(fontBank, line, scale)
		lineHeights = append(lineHeights, h)
		textH += h
		if i > 0 {
			textH += lineGap
		}
	}
	boxW := textW + boxPadX*2
	boxH := textH + boxPadY*2
	x0 := (img.Bounds().Dx() - boxW) / 2
	y0 := img.Bounds().Dy() - boxH - 72
	if x0 < 24 {
		x0 = 24
	}
	if y0 < 24 {
		y0 = 24
	}
	drawFilledRectRGBA(img, image.Rect(x0, y0, x0+boxW, y0+boxH), color.RGBA{0, 0, 0, 192})
	y := y0 + boxPadY
	for i, line := range lines {
		drawDumpFontLineCentered(img, fontBank, line, y, scale)
		y += lineHeights[i] + lineGap
	}
	return img, nil
}

func dumpTrackCoverLines(track dumpMusicTrack) []string {
	return appendNonEmptyUpper(make([]string, 0, 4),
		track.version,
		track.level,
		track.music,
		track.synth,
	)
}

func appendNonEmptyUpper(dst []string, values ...string) []string {
	for _, v := range values {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v == "" {
			continue
		}
		dst = append(dst, v)
	}
	return dst
}

func dumpMusicPCM(bank music.PatchBank, renderer dumpMusicRenderer, musData []byte) ([]int16, error) {
	switch music.ResolveBackend(renderer.backend) {
	case music.BackendImpSynth:
		driver, err := music.NewOutputDriverWithBackend(bank, music.BackendImpSynth)
		if err != nil {
			return nil, err
		}
		driver.Reset()
		return driver.RenderMUS(musData)
	case music.BackendMeltySynth:
		driver, err := music.NewMeltySynthDriver(music.OutputSampleRate, renderer.soundFont)
		if err != nil {
			return nil, err
		}
		stream, err := music.NewMUSStreamRenderer(driver, musData)
		if err != nil {
			return nil, err
		}
		var pcm []int16
		for {
			chunk, done, err := stream.NextChunkS16LE(music.DefaultStreamChunkFrames())
			if err != nil {
				return nil, err
			}
			if len(chunk) > 0 {
				pcm = append(pcm, pcmBytesToInt16LE(chunk)...)
			}
			if done {
				return pcm, nil
			}
		}
	default:
		return nil, fmt.Errorf("unsupported backend %q", renderer.backend)
	}
}

func pcmBytesToInt16LE(chunk []byte) []int16 {
	if len(chunk) < 2 {
		return nil
	}
	out := make([]int16, len(chunk)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(chunk[i*2:]))
	}
	return out
}

func normalizeDumpMusicPCM(pcm []int16, padDB float64) []int16 {
	if len(pcm) == 0 {
		return pcm
	}
	peak := dumpMusicPeakAbsSample(pcm)
	if peak == 0 {
		return pcm
	}
	targetPeak := int(math.Round(32767.0 * math.Pow(10, -padDB/20.0)))
	if targetPeak < 1 {
		targetPeak = 1
	}
	scale := float64(targetPeak) / float64(peak)
	if math.Abs(scale-1.0) < 1e-9 {
		return pcm
	}
	out := make([]int16, len(pcm))
	for i, sample := range pcm {
		scaled := math.Round(float64(sample) * scale)
		switch {
		case scaled > 32767:
			out[i] = 32767
		case scaled < -32768:
			out[i] = -32768
		default:
			out[i] = int16(scaled)
		}
	}
	return out
}

func dumpMusicPeakAbsSample(pcm []int16) int {
	peak := 0
	for _, sample := range pcm {
		v := int(sample)
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	return peak
}

func dumpMusicWADLabel(path string) string {
	base := strings.TrimSpace(filepath.Base(path))
	if base == "" {
		return "WAD"
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return "WAD"
	}
	return base
}

func dumpMusicWADDisplayName(path string) string {
	if choice, ok := knownIWADChoiceForPath(path); ok {
		return strings.TrimSpace(choice.Label)
	}
	return dumpMusicWADLabel(path)
}

func dumpMusicRendererLabel(soundFontPath string) string {
	base := strings.TrimSpace(filepath.Base(soundFontPath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		base = "SOUNDFONT"
	}
	return "MIDI-" + base
}

func dumpMusicRendererDisplayName(soundFontPath string) string {
	base := strings.TrimSpace(filepath.Base(soundFontPath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSpace(base)
	if base == "" {
		base = "SoundFont"
	}
	return base
}

func musicDumpFilenamePart(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case r == '\'' || r == '’':
			continue
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastSep = false
		case r == ' ' || r == '-' || r == '_' || r == '(' || r == ')':
			if lastSep {
				continue
			}
			b.WriteByte(' ')
			lastSep = true
		default:
			if lastSep {
				continue
			}
			b.WriteByte(' ')
			lastSep = true
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func writePCM16StereoWAV(path string, sampleRate int, pcm []int16) error {
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate %d", sampleRate)
	}
	if len(pcm)%2 != 0 {
		return fmt.Errorf("pcm sample count must be even, got %d", len(pcm))
	}

	const (
		numChannels   = 2
		bitsPerSample = 16
	)
	blockAlign := numChannels * (bitsPerSample / 8)
	byteRate := sampleRate * blockAlign
	dataSize := len(pcm) * 2

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	defer w.Flush()

	if _, err := w.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("WAVE")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}
	for _, sample := range pcm {
		if err := binary.Write(w, binary.LittleEndian, sample); err != nil {
			return err
		}
	}
	return nil
}

func writeBootSplashPNG(path string, splash media.WallTexture) error {
	dst, err := buildBootSplashImage(splash)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, dst)
}

func buildBootSplashImage(splash media.WallTexture) (*image.RGBA, error) {
	if splash.Width <= 0 || splash.Height <= 0 || len(splash.RGBA) != splash.Width*splash.Height*4 {
		return nil, fmt.Errorf("invalid splash texture")
	}
	const (
		outW = 1920
		outH = 1080
	)
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	fillRGBA(dst, color.RGBA{0, 0, 0, 255})
	src := image.NewRGBA(image.Rect(0, 0, splash.Width, splash.Height))
	copy(src.Pix, splash.RGBA)

	targetW := (outH * 16) / 10
	if targetW > outW {
		targetW = outW
	}
	offsetX := (outW - targetW) / 2
	for y := 0; y < outH; y++ {
		sy := y * splash.Height / outH
		for x := 0; x < targetW; x++ {
			sx := x * splash.Width / targetW
			dst.SetRGBA(offsetX+x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst, nil
}

func fillRGBA(img *image.RGBA, c color.RGBA) {
	if img == nil {
		return
	}
	for y := 0; y < img.Rect.Dy(); y++ {
		row := y * img.Stride
		for x := 0; x < img.Rect.Dx(); x++ {
			i := row + x*4
			img.Pix[i] = c.R
			img.Pix[i+1] = c.G
			img.Pix[i+2] = c.B
			img.Pix[i+3] = c.A
		}
	}
}

func measureDumpFontLine(fontBank map[rune]media.WallTexture, line string, scale int, spaceW int) int {
	if scale <= 0 {
		scale = 1
	}
	w := 0
	for _, ch := range line {
		if ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		}
		if ch == ' ' {
			w += 4 * scale
			continue
		}
		glyph, ok := fontBank[ch]
		if !ok || glyph.Width <= 0 || glyph.Height <= 0 {
			w += 4 * scale
			continue
		}
		w += glyph.Width * scale
	}
	return w
}

func dumpFontLineHeight(fontBank map[rune]media.WallTexture, line string, scale int) int {
	if scale <= 0 {
		scale = 1
	}
	h := 0
	for _, ch := range line {
		glyph, ok := fontBank[ch]
		if !ok || glyph.Width <= 0 || glyph.Height <= 0 {
			continue
		}
		gh := glyph.Height * scale
		if gh > h {
			h = gh
		}
	}
	if h == 0 {
		h = 12 * scale
	}
	return h
}

func drawDumpFontLineCentered(dst *image.RGBA, fontBank map[rune]media.WallTexture, line string, y int, scale int) {
	if dst == nil {
		return
	}
	spaceW := 8 * scale
	lineW := measureDumpFontLine(fontBank, line, scale, spaceW)
	x := (dst.Bounds().Dx() - lineW) / 2
	drawDumpFontLine(dst, fontBank, line, x, y, scale, spaceW)
}

func drawDumpFontLine(dst *image.RGBA, fontBank map[rune]media.WallTexture, line string, x int, y int, scale int, spaceW int) {
	for _, ch := range line {
		if ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		}
		if ch == ' ' {
			x += 4 * scale
			continue
		}
		glyph, ok := fontBank[ch]
		if !ok || glyph.Width <= 0 || glyph.Height <= 0 || len(glyph.RGBA) != glyph.Width*glyph.Height*4 {
			x += 4 * scale
			continue
		}
		drawGlyphRGBA(dst, glyph, x, y, scale)
		x += glyph.Width * scale
	}
}

func drawGlyphRGBA(dst *image.RGBA, glyph media.WallTexture, dx int, dy int, scale int) {
	if dst == nil || scale <= 0 {
		return
	}
	for sy := 0; sy < glyph.Height; sy++ {
		for sx := 0; sx < glyph.Width; sx++ {
			srcIdx := (sy*glyph.Width + sx) * 4
			if srcIdx+3 >= len(glyph.RGBA) || glyph.RGBA[srcIdx+3] == 0 {
				continue
			}
			for oy := 0; oy < scale; oy++ {
				py := dy - glyph.OffsetY*scale + sy*scale + oy
				if py < 0 || py >= dst.Bounds().Dy() {
					continue
				}
				for ox := 0; ox < scale; ox++ {
					px := dx - glyph.OffsetX*scale + sx*scale + ox
					if px < 0 || px >= dst.Bounds().Dx() {
						continue
					}
					dstIdx := py*dst.Stride + px*4
					copy(dst.Pix[dstIdx:dstIdx+4], glyph.RGBA[srcIdx:srcIdx+4])
				}
			}
		}
	}
}

func drawFilledRectRGBA(dst *image.RGBA, rect image.Rectangle, c color.RGBA) {
	if dst == nil {
		return
	}
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		row := y * dst.Stride
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := row + x*4
			dst.Pix[i] = blendChannel(dst.Pix[i], c.R, c.A)
			dst.Pix[i+1] = blendChannel(dst.Pix[i+1], c.G, c.A)
			dst.Pix[i+2] = blendChannel(dst.Pix[i+2], c.B, c.A)
			dst.Pix[i+3] = 255
		}
	}
}

func blendChannel(dst uint8, src uint8, alpha uint8) uint8 {
	a := int(alpha)
	return uint8((int(dst)*(255-a) + int(src)*a) / 255)
}

func resolveDumpBootSplashTexture(wf *wad.File) (media.WallTexture, bool) {
	if wf == nil {
		return media.WallTexture{}, false
	}
	if texSet, err := doomtex.LoadFromWAD(wf); err == nil {
		if splash := buildBootSplashTexture(texSet); splash.Width > 0 && splash.Height > 0 && len(splash.RGBA) == splash.Width*splash.Height*4 {
			return splash, true
		}
	}
	playpal, ok := wf.LumpByName("PLAYPAL")
	if !ok {
		return media.WallTexture{}, false
	}
	playpalData, err := wf.LumpDataView(playpal)
	if err != nil || len(playpalData) < 256*3 {
		return media.WallTexture{}, false
	}
	titlepic, ok := wf.LumpByName("TITLEPIC")
	if !ok {
		return media.WallTexture{}, false
	}
	titleData, err := wf.LumpDataView(titlepic)
	if err != nil || len(titleData) != 320*200 {
		return media.WallTexture{}, false
	}
	rgba := make([]byte, len(titleData)*4)
	for i, idx := range titleData {
		p := int(idx) * 3
		o := i * 4
		rgba[o] = playpalData[p]
		rgba[o+1] = playpalData[p+1]
		rgba[o+2] = playpalData[p+2]
		rgba[o+3] = 0xFF
	}
	return media.WallTexture{
		RGBA:   rgba,
		Width:  320,
		Height: 200,
	}, true
}
