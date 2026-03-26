package app

import (
	"gddoom/internal/demo"
	"gddoom/internal/wad"
)

func builtInAttractDemos(wf *wad.File) []*demo.Script {
	return loadBuiltInDemos(wf)
}
