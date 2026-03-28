package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"

	"gddoom/internal/music"
	"gddoom/internal/wad"
)

const wavSampleRate = music.OutputSampleRate

type wadTarget struct {
	label string
	path  string
}

type exportMode string

const (
	exportModeImpSynth exportMode = "impsynth"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("musicwav", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outDir := fs.String("out", "out/music-compare", "output directory")
	doom1Path := fs.String("doom1", "doom.wad", "path to Doom 1 IWAD")
	doom2Path := fs.String("doom2", "doom2.wad", "path to Doom 2 IWAD")
	songFilter := fs.String("song", "", "exact music lump to export (default: all parseable D_* lumps)")
	modeFlag := fs.String("mode", string(exportModeImpSynth), "export mode (impsynth)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	mode, err := parseExportMode(*modeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	targets := []wadTarget{
		{label: "doom1", path: strings.TrimSpace(*doom1Path)},
		{label: "doom2", path: strings.TrimSpace(*doom2Path)},
	}

	found := 0
	for _, target := range targets {
		if target.path == "" {
			continue
		}
		if _, err := os.Stat(target.path); err == nil {
			found++
		}
	}
	if found == 0 {
		fmt.Fprintln(os.Stderr, "no IWADs found; checked doom.wad and doom2.wad by default")
		return 1
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		return 1
	}

	for _, target := range targets {
		if target.path == "" {
			continue
		}
		if _, err := os.Stat(target.path); err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", target.path, err)
			continue
		}
		if err := exportWAD(target, *outDir, strings.ToUpper(strings.TrimSpace(*songFilter)), mode); err != nil {
			fmt.Fprintf(os.Stderr, "export %s: %v\n", target.path, err)
			return 1
		}
	}

	return 0
}

func parseExportMode(raw string) (exportMode, error) {
	switch exportMode(strings.ToLower(strings.TrimSpace(raw))) {
	case exportModeImpSynth:
		return exportModeImpSynth, nil
	default:
		return "", fmt.Errorf("invalid -mode %q (want impsynth)", raw)
	}
}

func exportWAD(target wadTarget, rootOut string, songFilter string, mode exportMode) error {
	wf, err := wad.Open(target.path)
	if err != nil {
		return fmt.Errorf("open wad: %w", err)
	}
	bank, err := loadGENMIDIBank(wf)
	if err != nil {
		return err
	}
	songs, err := parseableMusicLumps(wf)
	if err != nil {
		return err
	}
	if len(songs) == 0 {
		return fmt.Errorf("no parseable D_* lumps found")
	}
	if songFilter != "" {
		filtered := songs[:0]
		for _, song := range songs {
			if song == songFilter {
				filtered = append(filtered, song)
			}
		}
		songs = filtered
		if len(songs) == 0 {
			return fmt.Errorf("missing parseable lump %s", songFilter)
		}
	}

	wadOut := filepath.Join(rootOut, target.label)
	if err := os.MkdirAll(wadOut, 0o755); err != nil {
		return fmt.Errorf("create wad output dir: %w", err)
	}

	for _, song := range songs {
		lump, ok := wf.LumpByName(song)
		if !ok {
			return fmt.Errorf("missing lump after enumeration: %s", song)
		}
		musData, err := wf.LumpData(lump)
		if err != nil {
			return fmt.Errorf("read %s: %w", song, err)
		}
		pcm, err := renderPCM(bank, musData, mode)
		if err != nil {
			return fmt.Errorf("render %s: %w", song, err)
		}
		outPath := filepath.Join(wadOut, song+".wav")
		if err := writePCM16StereoWAV(outPath, wavSampleRate, pcm); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Printf("wrote wad=%s lump=%s frames=%d out=%s\n", target.label, song, len(pcm)/2, outPath)
	}
	return nil
}

func loadGENMIDIBank(wf *wad.File) (music.PatchBank, error) {
	lump, ok := wf.LumpByName("GENMIDI")
	if !ok {
		return nil, fmt.Errorf("%s missing GENMIDI lump", wf.Path)
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		return nil, fmt.Errorf("read GENMIDI from %s: %w", wf.Path, err)
	}
	bank, err := music.ParseGENMIDIOP2PatchBank(data)
	if err != nil {
		return nil, fmt.Errorf("parse GENMIDI from %s: %w", wf.Path, err)
	}
	return bank, nil
}

func parseableMusicLumps(wf *wad.File) ([]string, error) {
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
			return nil, fmt.Errorf("read %s from %s: %w", lump.Name, wf.Path, err)
		}
		if _, err := music.ParseMUS(data); err != nil {
			continue
		}
		seen[lump.Name] = struct{}{}
		songs = append(songs, lump.Name)
	}
	sort.Strings(songs)
	return songs, nil
}

func renderPCM(bank music.PatchBank, musData []byte, mode exportMode) ([]int16, error) {
	switch mode {
	case exportModeImpSynth:
		return renderBackendPCM(bank, musData, music.BackendImpSynth, "impsynth")
	default:
		return nil, fmt.Errorf("unsupported export mode %q", mode)
	}
}

func renderBackendPCM(bank music.PatchBank, musData []byte, backend music.Backend, label string) ([]int16, error) {
	driver, err := music.NewOutputDriverWithBackend(bank, backend)
	if err != nil {
		return nil, fmt.Errorf("new %s driver: %w", label, err)
	}
	driver.Reset()
	pcm, err := driver.RenderMUS(musData)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", label, err)
	}
	return pcm, nil
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
	if len(pcm) == 0 {
		return nil
	}
	if nativeLittleEndian() {
		data := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(pcm))), dataSize)
		if _, err := w.Write(data); err != nil {
			return err
		}
		return nil
	}
	buf := make([]byte, 0, 1<<20)
	for _, s := range pcm {
		buf = binary.LittleEndian.AppendUint16(buf, uint16(s))
		if len(buf) >= 1<<20 {
			if _, err := w.Write(buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func nativeLittleEndian() bool {
	var v uint16 = 0x0102
	return *(*byte)(unsafe.Pointer(&v)) == 0x02
}
