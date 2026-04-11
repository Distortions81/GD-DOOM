package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unsafe"

	"gddoom/internal/audiofx"
	"gddoom/internal/music"
	"gddoom/internal/sound"
	"gddoom/internal/wad"
)

const usageText = `usage:
  pcspeaker capture-mus -wad <path> -song <lump> -out <file.gdpc>
  pcspeaker capture-sfx -wad <path> -sound <DPxxxxxx> -out <file.gdpc> [-repeat N] [-gap ticks]
  pcspeaker interleave -in <file.gdpc> -mix <file.gdpc> -out <file.gdpc>
  pcspeaker dump-csv -in <file.gdpc> [-out <file.csv>]
  pcspeaker play -in <file.gdpc> [-variant passthrough|paper-speaker|small-buzzer]
  pcspeaker wav -in <file.gdpc> [-mix <file.gdpc>] -out <file.wav> [-variant passthrough|paper-speaker|small-buzzer]`

var (
	stdOut io.Writer = os.Stdout
	stdErr io.Writer = os.Stderr
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stdErr, usageText)
		return 2
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "capture-mus":
		return runCaptureMUS(args[1:])
	case "capture-sfx":
		return runCaptureSFX(args[1:])
	case "play":
		return runPlay(args[1:])
	case "interleave":
		return runInterleave(args[1:])
	case "dump-csv":
		return runDumpCSV(args[1:])
	case "wav":
		return runWAV(args[1:])
	case "-h", "--help", "help":
		fmt.Fprintln(stdErr, usageText)
		return 0
	default:
		fmt.Fprintf(stdErr, "unknown subcommand %q\n%s\n", args[0], usageText)
		return 2
	}
}

func runCaptureMUS(args []string) int {
	fs := flag.NewFlagSet("capture-mus", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	wadPath := fs.String("wad", "", "path to IWAD/PWAD containing GENMIDI and the target MUS lump")
	song := fs.String("song", "", "exact music lump name, e.g. D_E1M1")
	outPath := fs.String("out", "", "output capture path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*wadPath) == "" || strings.TrimSpace(*song) == "" || strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stdErr, "capture-mus requires -wad, -song, and -out")
		return 2
	}
	capture, err := captureMUS(strings.TrimSpace(*wadPath), strings.ToUpper(strings.TrimSpace(*song)))
	if err != nil {
		fmt.Fprintf(stdErr, "capture-mus: %v\n", err)
		return 1
	}
	if err := writeCaptureFile(strings.TrimSpace(*outPath), capture); err != nil {
		fmt.Fprintf(stdErr, "write capture: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdOut, "wrote type=music ticks=%d tick_rate=%d out=%s\n", len(capture.Tones), capture.TickRate, *outPath)
	return 0
}

func runCaptureSFX(args []string) int {
	fs := flag.NewFlagSet("capture-sfx", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	wadPath := fs.String("wad", "", "path to WAD containing the target DP* lump")
	soundName := fs.String("sound", "", "exact pc speaker lump name, e.g. DPPISTOL")
	outPath := fs.String("out", "", "output capture path")
	repeatCount := fs.Int("repeat", 1, "repeat the captured SFX this many times")
	gapTicks := fs.Int("gap", 0, "insert this many silent ticks between repeats")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*wadPath) == "" || strings.TrimSpace(*soundName) == "" || strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stdErr, "capture-sfx requires -wad, -sound, and -out")
		return 2
	}
	if *repeatCount <= 0 {
		fmt.Fprintln(stdErr, "capture-sfx requires -repeat >= 1")
		return 2
	}
	if *gapTicks < 0 {
		fmt.Fprintln(stdErr, "capture-sfx requires -gap >= 0")
		return 2
	}
	capture, err := captureSFX(strings.TrimSpace(*wadPath), strings.ToUpper(strings.TrimSpace(*soundName)))
	if err != nil {
		fmt.Fprintf(stdErr, "capture-sfx: %v\n", err)
		return 1
	}
	capture = repeatCapture(capture, *repeatCount, *gapTicks)
	if err := writeCaptureFile(strings.TrimSpace(*outPath), capture); err != nil {
		fmt.Fprintf(stdErr, "write capture: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdOut, "wrote type=sfx ticks=%d tick_rate=%d out=%s\n", len(capture.Tones), capture.TickRate, *outPath)
	return 0
}

func runPlay(args []string) int {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	inPath := fs.String("in", "", "input capture path")
	variantName := fs.String("variant", "paper-speaker", "speaker model (passthrough|paper-speaker|small-buzzer)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*inPath) == "" {
		fmt.Fprintln(stdErr, "play requires -in")
		return 2
	}
	capture, err := readCaptureFile(strings.TrimSpace(*inPath))
	if err != nil {
		fmt.Fprintf(stdErr, "read capture: %v\n", err)
		return 1
	}
	player := audiofx.NewPCSpeakerPlayer(1, audiofx.ParsePCSpeakerVariant(*variantName))
	if player == nil {
		fmt.Fprintln(stdErr, "play capture: audio context unavailable")
		return 1
	}
	player.SetMusic(capture.Tones, capture.TickRate, false)
	duration := captureDuration(capture)
	if duration > 0 {
		time.Sleep(duration + 150*time.Millisecond)
	}
	return 0
}

func runInterleave(args []string) int {
	fs := flag.NewFlagSet("interleave", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	inPath := fs.String("in", "", "primary capture path")
	mixPath := fs.String("mix", "", "secondary capture path")
	outPath := fs.String("out", "", "output capture path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*inPath) == "" || strings.TrimSpace(*mixPath) == "" || strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stdErr, "interleave requires -in, -mix, and -out")
		return 2
	}
	left, err := readCaptureFile(strings.TrimSpace(*inPath))
	if err != nil {
		fmt.Fprintf(stdErr, "read capture: %v\n", err)
		return 1
	}
	right, err := readCaptureFile(strings.TrimSpace(*mixPath))
	if err != nil {
		fmt.Fprintf(stdErr, "read mix capture: %v\n", err)
		return 1
	}
	tones, tickRate := audiofx.InterleavePCSpeakerSequences(left.Tones, left.TickRate, right.Tones, right.TickRate)
	if err := writeCaptureFile(strings.TrimSpace(*outPath), sound.PCSpeakerCapture{
		TickRate: tickRate,
		Tones:    tones,
	}); err != nil {
		fmt.Fprintf(stdErr, "write interleaved capture: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdOut, "wrote type=interleaved ticks=%d tick_rate=%d out=%s\n", len(tones), tickRate, *outPath)
	return 0
}

func runDumpCSV(args []string) int {
	fs := flag.NewFlagSet("dump-csv", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	inPath := fs.String("in", "", "input capture path")
	outPath := fs.String("out", "", "output csv path (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*inPath) == "" {
		fmt.Fprintln(stdErr, "dump-csv requires -in")
		return 2
	}
	capture, err := readCaptureFile(strings.TrimSpace(*inPath))
	if err != nil {
		fmt.Fprintf(stdErr, "read capture: %v\n", err)
		return 1
	}
	var w io.Writer = stdOut
	var f *os.File
	if strings.TrimSpace(*outPath) != "" {
		f, err = os.Create(strings.TrimSpace(*outPath))
		if err != nil {
			fmt.Fprintf(stdErr, "create csv: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}
	if err := writeCaptureCSV(w, capture); err != nil {
		fmt.Fprintf(stdErr, "write csv: %v\n", err)
		return 1
	}
	return 0
}

func runWAV(args []string) int {
	fs := flag.NewFlagSet("wav", flag.ContinueOnError)
	fs.SetOutput(stdErr)
	inPath := fs.String("in", "", "input capture path")
	mixPath := fs.String("mix", "", "optional second capture path to interleave with the input")
	outPath := fs.String("out", "", "output wav path")
	variantName := fs.String("variant", "paper-speaker", "speaker model (passthrough|paper-speaker|small-buzzer)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*inPath) == "" || strings.TrimSpace(*outPath) == "" {
		fmt.Fprintln(stdErr, "wav requires -in and -out")
		return 2
	}
	capture, err := readCaptureFile(strings.TrimSpace(*inPath))
	if err != nil {
		fmt.Fprintf(stdErr, "read capture: %v\n", err)
		return 1
	}
	var pcm []int16
	if strings.TrimSpace(*mixPath) == "" {
		pcm, err = audiofx.RenderPCSpeakerSequenceToPCM(capture.Tones, capture.TickRate, audiofx.ParsePCSpeakerVariant(*variantName))
	} else {
		mixCapture, mixErr := readCaptureFile(strings.TrimSpace(*mixPath))
		if mixErr != nil {
			fmt.Fprintf(stdErr, "read mix capture: %v\n", mixErr)
			return 1
		}
		pcm, err = audiofx.RenderMixedPCSpeakerSequencesToPCM(capture.Tones, capture.TickRate, mixCapture.Tones, mixCapture.TickRate, audiofx.ParsePCSpeakerVariant(*variantName))
	}
	if err != nil {
		fmt.Fprintf(stdErr, "render wav: %v\n", err)
		return 1
	}
	if err := writePCM16StereoWAV(strings.TrimSpace(*outPath), music.OutputSampleRate, pcm); err != nil {
		fmt.Fprintf(stdErr, "write wav: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdOut, "wrote type=wav frames=%d out=%s\n", len(pcm)/2, *outPath)
	return 0
}

func captureMUS(path string, lumpName string) (sound.PCSpeakerCapture, error) {
	wf, err := wad.Open(path)
	if err != nil {
		return sound.PCSpeakerCapture{}, fmt.Errorf("open wad: %w", err)
	}
	bank, err := loadGENMIDIBank(wf)
	if err != nil {
		return sound.PCSpeakerCapture{}, err
	}
	lump, ok := wf.LumpByName(lumpName)
	if !ok {
		return sound.PCSpeakerCapture{}, fmt.Errorf("missing lump %s", lumpName)
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		return sound.PCSpeakerCapture{}, fmt.Errorf("read %s: %w", lumpName, err)
	}
	seq, tickRate, err := music.RenderMUSToPCSpeaker(bank, data)
	if err != nil {
		return sound.PCSpeakerCapture{}, fmt.Errorf("render %s: %w", lumpName, err)
	}
	return sound.PCSpeakerCapture{TickRate: tickRate, Tones: seq}, nil
}

func captureSFX(path string, lumpName string) (sound.PCSpeakerCapture, error) {
	wf, err := wad.Open(path)
	if err != nil {
		return sound.PCSpeakerCapture{}, fmt.Errorf("open wad: %w", err)
	}
	lump, ok := wf.LumpByName(lumpName)
	if !ok {
		return sound.PCSpeakerCapture{}, fmt.Errorf("missing lump %s", lumpName)
	}
	data, err := wf.LumpData(lump)
	if err != nil {
		return sound.PCSpeakerCapture{}, fmt.Errorf("read %s: %w", lumpName, err)
	}
	pc, err := sound.ParsePCSpeakerLump(lumpName, data)
	if err != nil {
		return sound.PCSpeakerCapture{}, err
	}
	return sound.PCSpeakerCapture{
		TickRate: 140,
		Tones:    sound.BuildToneSequence(pc),
	}, nil
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

func writeCaptureFile(path string, capture sound.PCSpeakerCapture) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return sound.WritePCSpeakerCapture(f, capture)
}

func readCaptureFile(path string) (sound.PCSpeakerCapture, error) {
	f, err := os.Open(path)
	if err != nil {
		return sound.PCSpeakerCapture{}, err
	}
	defer f.Close()
	return sound.ReadPCSpeakerCapture(f)
}

func captureDuration(capture sound.PCSpeakerCapture) time.Duration {
	if capture.TickRate <= 0 || len(capture.Tones) == 0 {
		return 0
	}
	seconds := float64(len(capture.Tones)) / float64(capture.TickRate)
	return time.Duration(seconds * float64(time.Second))
}

func repeatCapture(capture sound.PCSpeakerCapture, repeat int, gapTicks int) sound.PCSpeakerCapture {
	if repeat <= 1 || len(capture.Tones) == 0 {
		return capture
	}
	if gapTicks < 0 {
		gapTicks = 0
	}
	total := repeat * len(capture.Tones)
	if repeat > 1 && gapTicks > 0 {
		total += (repeat - 1) * gapTicks
	}
	out := make([]sound.PCSpeakerTone, 0, total)
	silence := make([]sound.PCSpeakerTone, gapTicks)
	for i := 0; i < repeat; i++ {
		out = append(out, capture.Tones...)
		if i+1 < repeat && gapTicks > 0 {
			out = append(out, silence...)
		}
	}
	capture.Tones = out
	return capture
}

func writeCaptureCSV(w io.Writer, capture sound.PCSpeakerCapture) error {
	if w == nil {
		return fmt.Errorf("nil writer")
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	for i, tone := range capture.Tones {
		active := 0
		if tone.Active {
			active = 1
		}
		if _, err := fmt.Fprintf(bw, "%d,%d,%d,%d\n", i, active, tone.ToneValue, tone.ToneDivisor()); err != nil {
			return err
		}
	}
	return nil
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
