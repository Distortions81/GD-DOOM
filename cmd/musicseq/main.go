package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gddoom/internal/music"
	"gddoom/internal/wad"
)

type manifestEntry struct {
	Lump  string `json:"lump"`
	File  string `json:"file"`
	Count int    `json:"event_count"`
}

type manifest struct {
	Source  string          `json:"source"`
	Format  string          `json:"format"`
	TicRate int             `json:"tic_rate"`
	Songs   []manifestEntry `json:"songs"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("musicseq", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	wadPath := fs.String("wad", "doom.wad", "path to IWAD/PWAD containing GENMIDI and D_* MUS lumps")
	outDir := fs.String("out", "out/music-seq", "output directory")
	songFilter := fs.String("song", "", "exact music lump to export (default: all parseable D_* lumps)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	wf, err := wad.Open(strings.TrimSpace(*wadPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open wad: %v\n", err)
		return 1
	}
	bank, err := loadGENMIDIBank(wf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	songs, err := parseableMusicLumps(wf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if filter := strings.ToUpper(strings.TrimSpace(*songFilter)); filter != "" {
		filtered := songs[:0]
		for _, song := range songs {
			if song == filter {
				filtered = append(filtered, song)
			}
		}
		songs = filtered
		if len(songs) == 0 {
			fmt.Fprintf(os.Stderr, "missing parseable lump %s\n", filter)
			return 1
		}
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		return 1
	}

	m := manifest{
		Source: filepath.Base(strings.TrimSpace(*wadPath)),
		Format: "reg_le16,value_u8,delay_le16",
		Songs:  make([]manifestEntry, 0, len(songs)),
	}
	for _, song := range songs {
		lump, ok := wf.LumpByName(song)
		if !ok {
			fmt.Fprintf(os.Stderr, "missing lump after enumeration: %s\n", song)
			return 1
		}
		musData, err := wf.LumpData(lump)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", song, err)
			return 1
		}
		events, ticRate, err := music.RenderMUSToOPLSeq(musData, bank)
		if err != nil {
			fmt.Fprintf(os.Stderr, "render %s: %v\n", song, err)
			return 1
		}
		if m.TicRate == 0 {
			m.TicRate = ticRate
		}
		base := strings.ToLower(song) + ".seq"
		path := filepath.Join(*outDir, base)
		if err := writeSeqFile(path, events); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			return 1
		}
		m.Songs = append(m.Songs, manifestEntry{
			Lump:  song,
			File:  base,
			Count: len(events),
		})
		fmt.Printf("wrote lump=%s events=%d out=%s\n", song, len(events), path)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal manifest: %v\n", err)
		return 1
	}
	data = append(data, '\n')
	manifestPath := filepath.Join(*outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write manifest: %v\n", err)
		return 1
	}
	return 0
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

func writeSeqFile(path string, events []music.OPLSeqEvent) error {
	buf := make([]byte, 0, len(events)*5)
	for _, ev := range events {
		buf = binary.LittleEndian.AppendUint16(buf, ev.Reg)
		buf = append(buf, ev.Value)
		buf = binary.LittleEndian.AppendUint16(buf, ev.Delay)
	}
	return os.WriteFile(path, buf, 0o644)
}
