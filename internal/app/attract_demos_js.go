//go:build js && wasm

package app

import (
	"gddoom/internal/demo"
	"gddoom/internal/wad"
)

func builtInAttractDemos(_ *wad.File) []*demo.Script {
	return nil
}
