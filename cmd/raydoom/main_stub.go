//go:build !raylib || !cgo || js

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "Raylib experiment requires a native cgo build: go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3")
	os.Exit(1)
}
