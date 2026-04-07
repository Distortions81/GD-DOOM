package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/youthlin/silk"
)

type wavPCM struct {
	sampleRate int
	channels   int
	bits       int
	data       []byte
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("silkroundtrip", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	inPath := fs.String("in", "", "input wav path")
	outPath := fs.String("out", "", "output wav path")
	packetMs := fs.Int("packet-ms", 20, "silk packet size in milliseconds")
	bitRate := fs.Int("bitrate", 25000, "silk bitrate")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *inPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: silkroundtrip -in input.wav -out output.wav")
		return 2
	}

	inFile, err := os.Open(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open input: %v\n", err)
		return 1
	}
	defer inFile.Close()

	wav, err := readPCM16WAV(inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read wav: %v\n", err)
		return 1
	}
	if wav.channels != 1 {
		fmt.Fprintf(os.Stderr, "input wav must be mono, got %d channels\n", wav.channels)
		return 1
	}
	if wav.bits != 16 {
		fmt.Fprintf(os.Stderr, "input wav must be 16-bit PCM, got %d-bit\n", wav.bits)
		return 1
	}
	minFrameBytes := wav.sampleRate * 2 * 20 / 1000
	if len(wav.data) < minFrameBytes {
		fmt.Fprintf(os.Stderr, "input wav is too short: got %d bytes pcm, need at least %d bytes for one 20ms frame at %d Hz\n", len(wav.data), minFrameBytes, wav.sampleRate)
		return 1
	}

	encoded, err := silk.Encode(bytes.NewReader(wav.data),
		silk.SampleRate(wav.sampleRate),
		silk.PacketSizeMs(*packetMs),
		silk.BitRate(*bitRate),
		silk.Stx(false),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode silk: %v\n", err)
		return 1
	}

	decoded, err := silk.Decode(bytes.NewReader(encoded), silk.WithSampleRate(wav.sampleRate))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode silk: %v\n", err)
		return 1
	}
	if len(decoded) == 0 {
		fmt.Fprintf(os.Stderr, "decode silk produced no pcm; encoded=%d bytes input_pcm=%d bytes\n", len(encoded), len(wav.data))
		return 1
	}

	outFile, err := os.Create(*outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output: %v\n", err)
		return 1
	}
	defer outFile.Close()

	if err := writePCM16WAV(outFile, wav.sampleRate, 1, decoded); err != nil {
		fmt.Fprintf(os.Stderr, "write wav: %v\n", err)
		return 1
	}

	fmt.Printf("wrote %s encoded=%d decoded_pcm=%d\n", *outPath, len(encoded), len(decoded))
	return 0
}

func readPCM16WAV(r io.Reader) (wavPCM, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return wavPCM{}, err
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return wavPCM{}, fmt.Errorf("not a RIFF/WAVE file")
	}

	var out wavPCM
	for {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(r, chunkHeader[:]); err != nil {
			return wavPCM{}, err
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := int(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		chunkData := make([]byte, chunkSize)
		if _, err := io.ReadFull(r, chunkData); err != nil {
			return wavPCM{}, err
		}
		if chunkSize%2 == 1 {
			var pad [1]byte
			if _, err := io.ReadFull(r, pad[:]); err != nil {
				return wavPCM{}, err
			}
		}

		switch chunkID {
		case "fmt ":
			if len(chunkData) < 16 {
				return wavPCM{}, fmt.Errorf("wav fmt chunk too short")
			}
			audioFormat := binary.LittleEndian.Uint16(chunkData[0:2])
			if audioFormat != 1 {
				return wavPCM{}, fmt.Errorf("wav format=%d want pcm", audioFormat)
			}
			out.channels = int(binary.LittleEndian.Uint16(chunkData[2:4]))
			out.sampleRate = int(binary.LittleEndian.Uint32(chunkData[4:8]))
			out.bits = int(binary.LittleEndian.Uint16(chunkData[14:16]))
		case "data":
			out.data = chunkData
		}

		if out.sampleRate > 0 && len(out.data) > 0 {
			return out, nil
		}
	}
}

func writePCM16WAV(w io.Writer, sampleRate, channels int, pcm []byte) error {
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate %d", sampleRate)
	}
	if channels <= 0 {
		return fmt.Errorf("invalid channel count %d", channels)
	}
	if len(pcm)%2 != 0 {
		return fmt.Errorf("pcm length must be even, got %d", len(pcm))
	}

	const bitsPerSample = 16
	blockAlign := channels * (bitsPerSample / 8)
	byteRate := sampleRate * blockAlign

	if _, err := w.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(36+len(pcm))); err != nil {
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
	if err := binary.Write(w, binary.LittleEndian, uint16(channels)); err != nil {
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
	if err := binary.Write(w, binary.LittleEndian, uint32(len(pcm))); err != nil {
		return err
	}
	_, err := w.Write(pcm)
	return err
}
