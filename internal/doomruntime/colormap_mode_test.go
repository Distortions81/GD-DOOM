package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func TestSourcePortModeCanToggleSectorLightingWithoutColormapDecimation(t *testing.T) {
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

	// Sourceport mode keeps Doom light-row math, but sector contribution is optional.
	_ = newGame(&mapdata.Map{}, Options{
		SourcePortMode:           true,
		SourcePortSectorLighting: false,
		DoomPaletteRGBA:          palette,
		DoomColorMap:             colormap,
		DoomColorMapRows:         1,
	})
	if !doomLightingEnabled {
		t.Fatal("doom lighting math should be enabled in sourceport mode with valid colormap rows")
	}
	if doomSectorLighting {
		t.Fatal("sector lighting should be disabled in sourceport mode when option is off")
	}
	if doomColormapEnabled {
		t.Fatal("doom colormap should be disabled in sourceport mode")
	}
	if doomColormapRowCount() != 1 {
		t.Fatalf("sourceport should still load colormap rows for fixed effects, got=%d want=1", doomColormapRowCount())
	}
	if got := sectorLightMul(32); got != 256 {
		t.Fatalf("sectorLightMul=%d want=256 when sourceport sector lighting is off", got)
	}

	_ = newGame(&mapdata.Map{}, Options{
		SourcePortMode:           true,
		SourcePortSectorLighting: true,
		DoomPaletteRGBA:          palette,
		DoomColorMap:             colormap,
		DoomColorMapRows:         1,
	})
	if !doomSectorLighting {
		t.Fatal("sector lighting should be enabled in sourceport mode when option is on")
	}

	// Faithful mode should allow Doom colormap shading path.
	_ = newGame(&mapdata.Map{}, Options{
		SourcePortMode:   false,
		DoomPaletteRGBA:  palette,
		DoomColorMap:     colormap,
		DoomColorMapRows: 1,
	})
	if !doomLightingEnabled {
		t.Fatal("doom lighting math should be enabled in faithful mode with valid data")
	}
	if !doomSectorLighting {
		t.Fatal("sector lighting should remain enabled in faithful mode")
	}
	if !doomColormapEnabled {
		t.Fatal("doom colormap should be enabled in faithful mode with valid data")
	}
}

func TestDisableDoomLightingOptionForcesLightingOff(t *testing.T) {
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

	_ = newGame(&mapdata.Map{}, Options{
		DisableDoomLighting: true,
		DoomPaletteRGBA:     palette,
		DoomColorMap:        colormap,
		DoomColorMapRows:    1,
	})
	if doomLightingEnabled {
		t.Fatal("doom lighting should be disabled when DisableDoomLighting=true")
	}
	if doomSectorLighting {
		t.Fatal("sector lighting should be disabled when DisableDoomLighting=true")
	}
	if doomColormapEnabled {
		t.Fatal("doom colormap should be disabled when DisableDoomLighting=true")
	}
	if got := sectorLightMul(64); got != 256 {
		t.Fatalf("sectorLightMul=%d want=256 with DisableDoomLighting=true", got)
	}
}
