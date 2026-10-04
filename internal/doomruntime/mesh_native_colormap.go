package doomruntime

import "gddoom/internal/render/levelmesh"

type nativeFixedTextureKey struct {
	pixels, indexed *byte
	row             *uint32
	width, height   int
}

// Cache the same packed COLORMAP row used by the main wall/plane/sprite paths.
// The original texture identity stays stable, so a powerup blink only switches
// resident GPU textures. Sky, weapon artwork and the HUD keep their normal colors,
// matching the main source-port presentation.
func (n *NativeMeshGame) fixedWorldTexture(tex levelmesh.Texture) levelmesh.Texture {
	if tex.HasBlend() {
		other := n.fixedWorldTexture(tex.BlendTexture())
		tex.BlendFixedRGBA = other.FixedRGBA
	}
	row, active := n.g.playerFixedColormapRow()
	if !active || tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return tex
	}
	colors := doomColormapPackedRow(row)
	if len(colors) < 256 {
		return tex
	}
	key := nativeFixedTextureKey{pixels: &tex.RGBA[0], row: &colors[0], width: tex.Width, height: tex.Height}
	indexed := len(tex.Indexed) == tex.Width*tex.Height
	if indexed {
		key.indexed = &tex.Indexed[0]
	}
	if n.fixedTextures == nil {
		n.fixedTextures = make(map[nativeFixedTextureKey][]byte)
	}
	if cached, ok := n.fixedTextures[key]; ok {
		tex.FixedRGBA = cached
		return tex
	}
	remapped := make([]byte, len(tex.RGBA))
	for i := 0; i < len(tex.RGBA); i += 4 {
		copy(remapped[i:i+4], tex.RGBA[i:i+4])
		if tex.RGBA[i+3] == 0 {
			continue
		}
		var index byte
		if indexed {
			index = tex.Indexed[i/4]
		} else {
			// RGBA composites use the main renderer's exact-color lookup and
			// nearest-palette fallback, rather than inventing a grayscale effect.
			var ok bool
			index, ok = packedColorPaletteIndex(packRGBA(tex.RGBA[i], tex.RGBA[i+1], tex.RGBA[i+2]))
			if !ok {
				continue
			}
		}
		p := colors[int(index)]
		remapped[i], remapped[i+1], remapped[i+2] = byte(p>>pixelRShift), byte(p>>pixelGShift), byte(p>>pixelBShift)
	}
	n.fixedTextures[key] = remapped
	tex.FixedRGBA = remapped
	return tex
}
