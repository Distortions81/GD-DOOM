package raymesh

import (
	"fmt"

	"gddoom/internal/render/levelmesh"
)

type TextureFilter string

const (
	Nearest     TextureFilter = "nearest"
	Trilinear   TextureFilter = "trilinear"
	Anisotropic TextureFilter = "anisotropic" // 8x, layered on trilinear sampling.
)

type TextureOptions struct {
	Scale  int
	Filter TextureFilter
}

func (o TextureOptions) Validate() error {
	if o.Scale != 1 && o.Scale != 2 {
		return fmt.Errorf("texture scale must be 1 or 2")
	}
	if o.Filter != Nearest && o.Filter != Trilinear && o.Filter != Anisotropic {
		return fmt.Errorf("texture filter must be nearest, trilinear or anisotropic")
	}
	return nil
}

// prepareTexture changes only the upload image. UVs stay normalized against
// the original texture size, preserving Doom's repeats and pegging.
func prepareTexture(tex levelmesh.Texture, scale int, masked bool) levelmesh.Texture {
	pixels := tex.RGBA
	if masked {
		pixels = bleedTransparentRGB(tex)
	}
	if scale == 1 {
		return levelmesh.Texture{RGBA: pixels, Width: tex.Width, Height: tex.Height}
	}
	w, h := tex.Width*scale, tex.Height*scale
	out := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			src, dst := ((y/scale)*tex.Width+x/scale)*4, (y*w+x)*4
			copy(out[dst:dst+4], pixels[src:src+4])
		}
	}
	return levelmesh.Texture{RGBA: out, Width: w, Height: h}
}

// padPatchTexture gives an isolated patch one transparent texel on every edge.
// A multisampled quad can cover samples whose pixel center is outside its UV
// range. The gutter makes those samples transparent instead of repeating an
// opposite edge. Preserve adjacent RGB beneath zero alpha for filtered use.
func padPatchTexture(tex levelmesh.Texture) levelmesh.Texture {
	w, h := tex.Width+2, tex.Height+2
	out := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			sx, sy := max(0, min(tex.Width-1, x-1)), max(0, min(tex.Height-1, y-1))
			src, dst := (sy*tex.Width+sx)*4, (y*w+x)*4
			copy(out[dst:dst+4], tex.RGBA[src:src+4])
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				out[dst+3] = 0
			}
		}
	}
	return levelmesh.Texture{RGBA: out, Width: w, Height: h}
}

// Filtered alpha cutouts need edge colors under transparent texels, otherwise
// black RGB bleeds into visible edges. Flood nearest opaque colors across the
// repeating image without changing alpha or the source WAD texture.
func bleedTransparentRGB(tex levelmesh.Texture) []byte {
	count := tex.Width * tex.Height
	visited := make([]bool, count)
	queue := make([]int, 0, count)
	for i := range count {
		if tex.RGBA[i*4+3] != 0 {
			visited[i] = true
			queue = append(queue, i)
		}
	}
	if len(queue) == 0 || len(queue) == count {
		return tex.RGBA
	}
	pixels := append([]byte(nil), tex.RGBA...)
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		x, y := i%tex.Width, i/tex.Width
		neighbors := [4]int{
			y*tex.Width + (x+tex.Width-1)%tex.Width,
			y*tex.Width + (x+1)%tex.Width,
			((y+tex.Height-1)%tex.Height)*tex.Width + x,
			((y+1)%tex.Height)*tex.Width + x,
		}
		for _, j := range neighbors {
			if !visited[j] {
				copy(pixels[j*4:j*4+3], pixels[i*4:i*4+3])
				visited[j] = true
				queue = append(queue, j)
			}
		}
	}
	return pixels
}
