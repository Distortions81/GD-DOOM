package main

import (
	"os"

	"gddoom/internal/app"
)

func main() {
	//debug.SetGCPercent(300)
	os.Exit(app.RunParse(os.Args[1:], os.Stdout, os.Stderr))
}
