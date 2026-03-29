package app

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gddoom/internal/media"
	"gddoom/internal/music"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/runtimecfg"
	"gddoom/internal/wad"
)

type dumpMusicTarget struct {
	label     string
	path      string
	pwadPaths []string
}

type dumpMusicRenderer struct {
	label     string
	backend   music.Backend
	fontPath  string
	soundFont *music.SoundFontBank
}

type dumpMusicTrack struct {
	lumpName string
	fileBase string
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
		patchBank, err := resolveMusicPatchBank(wf, "", nil)
		if err != nil {
			return fmt.Errorf("resolve music patch bank for %s: %w", target.path, err)
		}
		for _, renderer := range renderers {
			renderOut := filepath.Join(wadOut, renderer.label)
			if err := os.MkdirAll(renderOut, 0o755); err != nil {
				return fmt.Errorf("create renderer output directory: %w", err)
			}
			for _, track := range tracks {
				lump, ok := wf.LumpByName(track.lumpName)
				if !ok {
					return fmt.Errorf("missing lump %s in %s", track.lumpName, target.path)
				}
				musData, err := wf.LumpDataView(lump)
				if err != nil {
					return fmt.Errorf("read %s from %s: %w", track.lumpName, target.path, err)
				}
				pcm, err := dumpMusicPCM(patchBank, renderer, musData)
				if err != nil {
					return fmt.Errorf("render %s with %s for %s: %w", track.lumpName, renderer.label, target.path, err)
				}
				outPath := filepath.Join(renderOut, track.fileBase+".wav")
				if err := writePCM16StereoWAV(outPath, music.OutputSampleRate, pcm); err != nil {
					return fmt.Errorf("write %s: %w", outPath, err)
				}
				if stdout != nil {
					fmt.Fprintf(stdout, "dump-music wad=%s renderer=%s track=%s out=%s\n", target.label, renderer.label, track.lumpName, outPath)
				}
			}
		}
	}
	return nil
}

func detectDumpMusicTargets(resolvedWADPath string, wadExplicit bool, pwadPaths []string) ([]dumpMusicTarget, error) {
	if wadExplicit {
		if strings.TrimSpace(resolvedWADPath) == "" {
			return nil, fmt.Errorf("-wad is required when exporting a specific WAD")
		}
		return []dumpMusicTarget{{
			label:     dumpMusicWADLabel(resolvedWADPath),
			path:      resolvedWADPath,
			pwadPaths: append([]string(nil), pwadPaths...),
		}}, nil
	}
	choices := detectAvailableIWADChoices(".")
	if len(choices) == 0 {
		if strings.TrimSpace(resolvedWADPath) == "" {
			return nil, fmt.Errorf("no IWADs found")
		}
		return []dumpMusicTarget{{
			label: dumpMusicWADLabel(resolvedWADPath),
			path:  resolvedWADPath,
		}}, nil
	}
	out := make([]dumpMusicTarget, 0, len(choices))
	seen := make(map[string]struct{}, len(choices))
	for _, choice := range choices {
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
			label: dumpMusicWADLabel(path),
			path:  path,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no IWADs found")
	}
	return out, nil
}

func detectDumpMusicRenderers(explicitSoundFont string, stderr io.Writer) ([]dumpMusicRenderer, error) {
	renderers := []dumpMusicRenderer{{
		label:   "OPL",
		backend: music.BackendImpSynth,
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
			label:     dumpMusicRendererLabel(path),
			backend:   music.BackendMeltySynth,
			fontPath:  path,
			soundFont: sf,
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
	prefix := strings.ToUpper(strings.TrimSpace(string(track.MapName)))
	if prefix == "" {
		prefix = strings.ToUpper(strings.TrimSpace(track.LumpName))
	}
	suffix := musicDumpSlug(track.MusicName)
	if suffix == "" {
		return prefix
	}
	return prefix + "-" + suffix
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

func dumpMusicRendererLabel(soundFontPath string) string {
	base := strings.TrimSpace(filepath.Base(soundFontPath))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		base = "SOUNDFONT"
	}
	return "MIDI-" + base
}

func musicDumpSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		b.WriteByte('-')
		lastDash = true
	}
	out := strings.Trim(b.String(), "-")
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
	if splash.Width <= 0 || splash.Height <= 0 || len(splash.RGBA) != splash.Width*splash.Height*4 {
		return fmt.Errorf("invalid splash texture")
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

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, dst)
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
