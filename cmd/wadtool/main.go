package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gddoom/internal/wad"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "extract-lump":
		os.Exit(runExtractLump(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  wadtool extract-lump -wad <path> -lump <name> -out <path>")
}

func runExtractLump(args []string) int {
	fs := flag.NewFlagSet("extract-lump", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	wadPath := fs.String("wad", "", "path to WAD")
	lumpName := fs.String("lump", "", "lump name to extract")
	outPath := fs.String("out", "", "output path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*wadPath) == "" || strings.TrimSpace(*lumpName) == "" || strings.TrimSpace(*outPath) == "" {
		usage()
		return 2
	}
	wf, err := wad.Open(*wadPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open wad: %v\n", err)
		return 1
	}
	l, ok := wf.LumpByName(strings.ToUpper(strings.TrimSpace(*lumpName)))
	if !ok {
		fmt.Fprintf(os.Stderr, "lump not found: %s\n", *lumpName)
		return 1
	}
	data, err := wf.LumpData(l)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read lump: %v\n", err)
		return 1
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "extracted lump=%s bytes=%d out=%s\n", strings.ToUpper(strings.TrimSpace(*lumpName)), len(data), *outPath)
	return 0
}
