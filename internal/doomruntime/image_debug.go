package doomruntime

import (
	"fmt"
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

func debugEbitenNewImage(tag string, w, h int, unmanaged bool) {
	if strings.TrimSpace(runtimeDebugEnv("GD_DEBUG_EBITEN_NEWIMAGE")) == "" {
		return
	}
	fmt.Printf("ebiten-newimage tag=%s size=%dx%d unmanaged=%t\n", tag, w, h, unmanaged)
}

func newDebugImage(tag string, w, h int) *ebiten.Image {
	debugEbitenNewImage(tag, w, h, false)
	return ebiten.NewImage(w, h)
}

func newDebugImageWithOptions(tag string, bounds image.Rectangle, options *ebiten.NewImageOptions) *ebiten.Image {
	debugEbitenNewImage(tag, bounds.Dx(), bounds.Dy(), options != nil && options.Unmanaged)
	return ebiten.NewImageWithOptions(bounds, options)
}
