package doomruntime

import (
	"gddoom/internal/render/levelmesh"
	"reflect"
	"testing"
)

func TestNativeSpectrePostsUseSharedMaskAndKeepViewport(t *testing.T) {
	for _, size := range [][2]int{{320, 200}, {640, 400}, {1920, 1080}} {
		g := &game{viewW: 17, viewH: 19, spectreFuzzPos: 48}
		n := &NativeMeshGame{g: g}
		mask := []byte{1, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1}
		rgba := make([]byte, len(mask)*4)
		for i, v := range mask {
			if v != 0 {
				rgba[i*4+3] = 255
			}
		}
		s := levelmesh.Sprite{Texture: levelmesh.Texture{RGBA: rgba, Width: 2, Height: 6}, X: 32, Y: 24, Z: 8, Shadow: true}
		camera := levelmesh.Camera{}
		got := append([]levelmesh.FuzzSpan(nil), n.SpectreFuzz(s, camera, size[0], size[1])...)
		if g.viewW != 17 || g.viewH != 19 {
			t.Fatal("fuzz changed runtime viewport")
		}
		if len(got) == 0 {
			t.Fatal("visible spectre has no posts")
		}
		x, y, sx, sy, _, _ := levelmesh.ProjectSprite(s, camera, size[0], size[1])
		// Use the main mask walker at the independently derived integer bounds.
		ref := &game{viewW: size[0], viewH: size[1], spectreFuzzPos: 48}
		tex := WallTexture{Width: 2, Height: 6, RGBA: rgba, OpaqueMask: mask}
		it := cutoutItem{tex: &tex, scale: sx, scaleY: sy, dstX: x, dstY: y, x0: int(x), x1: int(x+2*sx) - 1, y0: int(y), y1: int(y+6*sy) - 1}
		var want []levelmesh.FuzzSpan
		ref.walkSpectreFuzzSpans(it, func(span spectreFuzzSpan) {
			want = append(want, levelmesh.FuzzSpan{X: span.cx, Y0: span.cy, Y1: span.y1 * min(size[1], 200) / size[1], Phase: span.phase})
		})
		if !reflect.DeepEqual(got, want) || g.spectreFuzzPos != ref.spectreFuzzPos {
			t.Fatalf("native fuzz posts diverged at %v", size)
		}
	}
}
