package doomruntime

import (
	"testing"

	"gddoom/internal/mapdata"
)

func channelR(v uint32) uint8 {
	return uint8((v >> pixelRShift) & 0xFF)
}

func TestSetGammaLevelSelectsPackedShadeLUTBank(t *testing.T) {
	palette := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		base := i * 4
		palette[base+0] = byte(i)
		palette[base+1] = byte(i)
		palette[base+2] = byte(i)
		palette[base+3] = 255
	}

	initWallShadePackedLUT(palette)
	g := &game{}
	g.setGammaLevel(1)
	base := channelR(wallShadePackedLUT[256][64])
	g.setGammaLevel(doomGammaLevels - 1)
	boosted := channelR(wallShadePackedLUT[256][64])
	if boosted <= base {
		t.Fatalf("packed gamma bank did not brighten: base=%d boosted=%d", base, boosted)
	}
}

func TestSetGammaLevelSelectsFaithfulColormapBank(t *testing.T) {
	palette := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		base := i * 4
		palette[base+0] = byte(i)
		palette[base+1] = byte(i)
		palette[base+2] = byte(i)
		palette[base+3] = 255
	}
	colormap := make([]byte, 256)
	for i := 0; i < 256; i++ {
		colormap[i] = byte(i)
	}

	initDoomColormapShading(palette, colormap, 1, true)
	g := &game{}
	g.setGammaLevel(1)
	base := channelR(doomColormapRGBA[64])
	g.setGammaLevel(doomGammaLevels - 1)
	boosted := channelR(doomColormapRGBA[64])
	if boosted <= base {
		t.Fatalf("colormap gamma bank did not brighten: base=%d boosted=%d", base, boosted)
	}
}

func TestCycleGammaLevelWrapsAtMax(t *testing.T) {
	g := &game{gammaLevel: doomGammaLevels - 1}
	g.cycleGammaLevel()
	if g.gammaLevel != 0 {
		t.Fatalf("gammaLevel=%d want 0 after wrap", g.gammaLevel)
	}
}

func TestBaseGammaTableIsDarkerThanIdentity(t *testing.T) {
	if got := doomGammaTables[1][64]; got >= 64 {
		t.Fatalf("base gamma table[64]=%d want < 64", got)
	}
}

func TestGammaTableContainsIdentityStep(t *testing.T) {
	if got := doomGammaTables[0][64]; got != 64 {
		t.Fatalf("identity gamma table[64]=%d want 64", got)
	}
}

func TestNewGameDefaultsToGammaTwo(t *testing.T) {
	g := newGame(&mapdata.Map{}, Options{})
	if g.gammaLevel != defaultGammaLevel {
		t.Fatalf("default gammaLevel=%d want %d", g.gammaLevel, defaultGammaLevel)
	}
}
