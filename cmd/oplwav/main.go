package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gddoom/internal/music"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

const wavSampleRate = music.OutputSampleRate

type wadTarget struct {
	label string
	path  string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("oplwav", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outDir := fs.String("out", "out/opl-compare", "output directory")
	doom1Path := fs.String("doom1", "doom.wad", "path to Doom 1 IWAD")
	doom2Path := fs.String("doom2", "doom2.wad", "path to Doom 2 IWAD")
	if err := fs.Parse(args); err != nil {
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
		if err := exportWAD(target, *outDir); err != nil {
			fmt.Fprintf(os.Stderr, "export %s: %v\n", target.path, err)
			return 1
		}
	}

	return 0
}

func exportWAD(target wadTarget, rootOut string) error {
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
		pcm, err := renderComparePCM(bank, musData)
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

func renderComparePCM(bank music.PatchBank, musData []byte) ([]int16, error) {
	impDriver, err := music.NewOutputDriverWithBackend(bank, sound.BackendImpSynth)
	if err != nil {
		return nil, fmt.Errorf("new impsynth driver: %w", err)
	}
	nukedDriver, err := music.NewOutputDriverWithBackend(bank, sound.BackendNuked)
	if err != nil {
		return nil, fmt.Errorf("new nuked driver: %w", err)
	}

	impDriver.Reset()
	nukedDriver.Reset()

	impPCM, err := impDriver.RenderMUS(musData)
	if err != nil {
		return nil, fmt.Errorf("render impsynth: %w", err)
	}
	nukedPCM, err := nukedDriver.RenderMUS(musData)
	if err != nil {
		return nil, fmt.Errorf("render nuked: %w", err)
	}
	return interleaveCompare(impPCM, nukedPCM), nil
}

func interleaveCompare(leftStereo []int16, rightStereo []int16) []int16 {
	leftFrames := len(leftStereo) / 2
	rightFrames := len(rightStereo) / 2
	frames := leftFrames
	if rightFrames > frames {
		frames = rightFrames
	}
	if frames == 0 {
		return nil
	}
	out := make([]int16, frames*2)
	for i := 0; i < frames; i++ {
		out[i*2] = monoFrame(leftStereo, i)
		out[i*2+1] = monoFrame(rightStereo, i)
	}
	return out
}

func monoFrame(stereo []int16, frame int) int16 {
	base := frame * 2
	if base+1 >= len(stereo) {
		return 0
	}
	l := int32(stereo[base])
	r := int32(stereo[base+1])
	return int16((l + r) / 2)
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

	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		return err
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		return err
	}
	if _, err := f.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}
	for _, s := range pcm {
		if err := binary.Write(f, binary.LittleEndian, s); err != nil {
			return err
		}
	}
	return nil
}
