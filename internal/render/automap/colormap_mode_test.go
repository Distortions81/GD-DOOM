package automap

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSourcePortModeDisablesDoomColormapDecimation(t *testing.T) {
	palette := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		base := i * 4
		palette[base+0] = byte(i)
		palette[base+1] = byte(255 - i)
		palette[base+2] = byte((i * 3) & 0xFF)
		palette[base+3] = 255
	}
	colormap := make([]byte, 256)
	for i := 0; i < 256; i++ {
		colormap[i] = byte(i)
	}

	// Sourceport mode should force full-color shading path.
	_ = newGame(&mapdata.Map{}, Options{
		SourcePortMode:   true,
		DoomPaletteRGBA:  palette,
		DoomColorMap:     colormap,
		DoomColorMapRows: 1,
	})
	if doomColormapEnabled {
		t.Fatal("doom colormap should be disabled in sourceport mode")
	}

	// Faithful mode should allow Doom colormap shading path.
	_ = newGame(&mapdata.Map{}, Options{
		SourcePortMode:   false,
		DoomPaletteRGBA:  palette,
		DoomColorMap:     colormap,
		DoomColorMapRows: 1,
	})
	if !doomColormapEnabled {
		t.Fatal("doom colormap should be enabled in faithful mode with valid data")
	}
}
