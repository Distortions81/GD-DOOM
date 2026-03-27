package main

import (
	"os"
	"runtime/debug"

	"gddoom/internal/app"
)

func main() {
	debug.SetGCPercent(300)
	os.Exit(app.RunParse(os.Args[1:], os.Stdout, os.Stderr))
}
