package doomruntime

import (
	"bytes"
	"testing"

	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/levelmesh"
	"gddoom/internal/wad"
)

func TestNativeFixedColormapPreservesIndicesAlphaCacheAndBlink(t *testing.T) {
	rows, rgba := doomColormapRows, doomColormapRGBA
	t.Cleanup(func() { doomColormapRows, doomColormapRGBA = rows, rgba })
	doomColormapRows = 33
	doomColormapRGBA = make([]uint32, 33*256)
	doomColormapRGBA[32*256+1] = packRGBA(210, 30, 70)
	doomColormapRGBA[32*256+2] = packRGBA(10, 240, 90)
	n := &NativeMeshGame{g: &game{inventory: playerInventory{InvulnTics: 160}}}
	// Identical input RGB can have distinct palette indices and colormap results.
	tex := levelmesh.Texture{Width: 3, Height: 1, RGBA: []byte{80, 80, 80, 255, 80, 80, 80, 120, 80, 80, 80, 0}, Indexed: []byte{1, 2, 1}}
	source := bytes.Clone(tex.RGBA)
	got := n.fixedWorldTexture(tex)
	want := []byte{210, 30, 70, 255, 10, 240, 90, 120, 80, 80, 80, 0}
	if !bytes.Equal(got.FixedRGBA, want) || !bytes.Equal(tex.RGBA, source) || &got.RGBA[0] != &tex.RGBA[0] {
		t.Fatalf("fixed mapping changed indices, opacity or source texture: %v", got.FixedRGBA)
	}
	for _, tics := range []int{160, 129, 128, 120, 8, 7, 0} {
		n.g.inventory.InvulnTics = tics
		mapped := n.fixedWorldTexture(tex)
		active := tics > 128 || tics&8 != 0
		if active {
			if len(mapped.FixedRGBA) == 0 || &mapped.FixedRGBA[0] != &got.FixedRGBA[0] {
				t.Fatal("blink rebuilt cached palette pixels")
			}
		} else if len(mapped.FixedRGBA) != 0 {
			t.Fatal("inverse palette remained during blink-off")
		}
	}
	n.g.invulnerable = true
	if len(n.fixedWorldTexture(tex).FixedRGBA) != 0 {
		t.Fatal("IDDQD incorrectly activated inverse palette")
	}
}

func TestNativeFixedColormapRemapsBothCrossfadeFrames(t *testing.T) {
	rows, rgba := doomColormapRows, doomColormapRGBA
	t.Cleanup(func() { doomColormapRows, doomColormapRGBA = rows, rgba })
	doomColormapRows, doomColormapRGBA = 33, make([]uint32, 33*256)
	doomColormapRGBA[32*256+1], doomColormapRGBA[32*256+2] = packRGBA(210, 30, 70), packRGBA(10, 240, 90)
	n := &NativeMeshGame{g: &game{inventory: playerInventory{InvulnTics: 160}}}
	tex := levelmesh.Texture{Width: 1, Height: 1, RGBA: []byte{80, 80, 80, 255}, Indexed: []byte{1}, BlendRGBA: []byte{80, 80, 80, 255}, BlendIndexed: []byte{2}, BlendAlpha: 128}
	got := n.fixedWorldTexture(tex)
	if !bytes.Equal(got.FixedRGBA, []byte{210, 30, 70, 255}) || !bytes.Equal(got.BlendFixedRGBA, []byte{10, 240, 90, 255}) {
		t.Fatal("fixed crossfade lost the second frame's original palette indices")
	}
	for _, alpha := range []uint8{1, 64, 128, 254} {
		tex.BlendAlpha = alpha
		mapped := n.fixedWorldTexture(tex)
		if &mapped.FixedRGBA[0] != &got.FixedRGBA[0] || &mapped.BlendFixedRGBA[0] != &got.BlendFixedRGBA[0] || mapped.BlendAlpha != alpha || len(n.fixedTextures) != 2 {
			t.Fatal("changing crossfade weights allocated palette variants")
		}
	}
}

func TestNativeInvulnerabilityMatchesWADColormapAndKeepsOverlayArtwork(t *testing.T) {
	fixture := loadNativeCombatGame(t)
	wf, err := wad.Open(findDOOM1WAD(t))
	if err != nil {
		t.Fatal(err)
	}
	opts := fixture.g.opts
	opts.DoomPaletteRGBA, err = doomtex.LoadPaletteRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	lump, ok := wf.LumpByName("COLORMAP")
	if !ok {
		t.Fatal("IWAD colormap missing")
	}
	opts.DoomColorMap, err = wf.LumpData(lump)
	if err != nil {
		t.Fatal(err)
	}
	opts.DoomColorMapRows = len(opts.DoomColorMap) / 256
	n := NewNativeMeshGame(fixture.g.m, opts)
	before := n.Frame(1)
	if before.FixedColormap {
		t.Fatal("normal world has inverse palette")
	}
	n.g.inventory.InvulnTics = 160
	checksum := n.g.SimChecksum()
	frame := n.Frame(1)
	if !frame.FixedColormap {
		t.Fatal("powerup did not enable the fixed palette")
	}
	seen := 0
	for _, tri := range frame.Triangles {
		tex := n.Texture(tri)
		if len(tex.RGBA) == 0 {
			continue
		}
		if tri.Sky {
			if len(tex.FixedRGBA) != 0 {
				t.Fatal("sky received inverse palette")
			}
			continue
		}
		if len(tex.FixedRGBA) != len(tex.RGBA) {
			t.Fatal("world texture lacks inverse palette")
		}
		for i := 0; i < len(tex.RGBA); i += 4 {
			if tex.RGBA[i+3] == 0 {
				continue
			}
			index, valid := packedColorPaletteIndex(packRGBA(tex.RGBA[i], tex.RGBA[i+1], tex.RGBA[i+2]))
			if !valid {
				t.Fatal("WAD pixel could not be resolved")
			}
			want := shadePaletteIndexDOOMRow(index, 32)
			got := tex.FixedRGBA[i : i+4]
			if got[0] != byte(want>>pixelRShift) || got[1] != byte(want>>pixelGShift) || got[2] != byte(want>>pixelBShift) || got[3] != tex.RGBA[i+3] {
				t.Fatal("native texture differs from the main fixed colormap")
			}
		}
		seen++
	}
	if seen == 0 {
		t.Fatal("fixture has no world textures")
	}
	for _, sprite := range frame.Sprites {
		if len(sprite.Texture.FixedRGBA) != len(sprite.Texture.RGBA) {
			t.Fatal("world sprite lacks fixed palette")
		}
	}
	if len(frame.WeaponPatches) == 0 {
		t.Fatal("fixture lacks weapon artwork")
	}
	if len(frame.Sky.FixedRGBA) != 0 || len(frame.WeaponPatches[0].Texture.FixedRGBA) != 0 || len(frame.HUD[0].Texture.FixedRGBA) != 0 {
		t.Fatal("unshaded sky, weapon or HUD artwork changed")
	}
	if n.g.SimChecksum() != checksum {
		t.Fatal("inverse palette rendering changed simulation state")
	}
}
