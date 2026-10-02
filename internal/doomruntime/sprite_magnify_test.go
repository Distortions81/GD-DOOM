package doomruntime

import "testing"

func TestMagnifiedSpriteRowsUsePixelCenterSamples(t *testing.T) {
	g := &game{viewW: 16, viewH: 16, opts: Options{DisableBillboardClipping: true}}
	g.ensureWallLayer()
	g.ensure3DFrameBuffers()
	tex := &WallTexture{Width: 4, Height: 4, RGBA32: make([]uint32, 16), OpaqueMask: make([]byte, 16)}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			tex.RGBA32[y*4+x] = packRGBA(byte(20+y*50), byte(20+x*50), 80)
			tex.OpaqueMask[y*4+x] = 1
		}
	}
	it := cutoutItem{boundsOK: true, tex: tex, scale: 2.5, dstX: 2.2, dstY: 1.8, x0: 2, x1: 12, y0: 1, y1: 11, clipBottom: 15, shadeMul: 256}
	g.drawSpriteCutoutItem(it)
	for y := 2; y <= 10; y++ {
		for x := 3; x <= 11; x++ {
			tx := int((float64(x) + 0.5 - it.dstX) / it.scale)
			ty := int((float64(y) + 0.5 - it.dstY) / it.scale)
			want := tex.RGBA32[ty*tex.Width+tx]
			if got := g.wallPix32[y*g.viewW+x]; got != want {
				t.Fatalf("pixel=(%d,%d) color=%08x want=%08x for texel=(%d,%d)", x, y, got, want, tx, ty)
			}
		}
	}
}
