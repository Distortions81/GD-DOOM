package main

import (
	"os"

	"gddoom/internal/app"
)

func main() {
	os.Exit(app.RunParse(os.Args[1:], os.Stdout, os.Stderr))
}
