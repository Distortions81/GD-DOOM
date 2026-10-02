package doomruntime

import (
	"math/rand/v2"
	"slices"
	"testing"

	"gddoom/internal/render/scene"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestGPUSpriteClipBandsMatchEveryRow(t *testing.T) {
	random := rand.New(rand.NewPCG(17, 29))
	const width, height = 64, 40
	for trial := 0; trial < 40; trial++ {
		g, _ := gpuSpriteTestGame(width, height)
		g.wallDepthQCol = make([]uint16, width)
		g.wallDepthTopCol = make([]int, width)
		g.wallDepthBottomCol = make([]int, width)
		g.wallDepthClosedCol = make([]bool, width)
		g.maskedClipCols = make([][]scene.MaskedClipSpan, width)
		g.maskedClipFirstDepthQ = make([]uint16, width)
		for x := 0; x < width; x++ {
			g.wallDepthQCol[x] = uint16(random.IntN(600))
			g.wallDepthTopCol[x] = random.IntN(height+10) - 5
			g.wallDepthBottomCol[x] = g.wallDepthTopCol[x] + random.IntN(height)
			g.wallDepthClosedCol[x] = random.IntN(8) == 0
			if random.IntN(3) != 0 {
				continue
			}
			g.maskedClipFirstDepthQ[x] = 200
			for _, depth := range []uint16{200, 400} {
				top := int16(random.IntN(height+10) - 5)
				bottom := top + int16(random.IntN(height))
				g.maskedClipCols[x] = append(g.maskedClipCols[x], scene.MaskedClipSpan{Y0: top, Y1: bottom, OpenY0: top, OpenY1: bottom, DepthQ: depth, Closed: random.IntN(10) == 0, HasOpen: random.IntN(2) == 0})
			}
		}
		// Include disabled clipping and incomplete buffers using the generic path.
		g.opts.DisableBillboardClipping = trial%10 == 0
		if trial%7 == 0 {
			g.wallDepthClosedCol = nil
		}
		for _, depth := range []uint16{0, 200, 201, 400, 401, 600} {
			for _, clip := range [][]solidSpan{nil, {{L: 5, R: 17}, {L: 24, R: 50}}} {
				changes := g.gpuFrame.spriteClipChanges(g, -2, width+2, 2, height-2, depth)
				var cached []solidSpan
				for y := 2; y <= height-2; y++ {
					want := g.spriteRowVisibleSpansDepthQ(y, -2, width+2, depth, clip, nil)
					if changes[y] {
						cached = slices.Clone(want)
					}
					if !slices.Equal(cached, want) {
						t.Fatalf("trial=%d depth=%d y=%d cached=%v want=%v", trial, depth, y, cached, want)
					}
				}
			}
		}
	}
}

func gpuSpriteTestGame(width, height int) (*game, *WallTexture) {
	tex := &WallTexture{Width: 64, Height: 64, Indexed: make([]byte, 64*64), OpaqueMask: make([]byte, 64*64)}
	for i := range tex.OpaqueMask {
		tex.OpaqueMask[i] = 1
	}
	key := gpuTextureKey{indexed: &tex.Indexed[0], mask: &tex.OpaqueMask[0], width: 64, height: 64}
	r := &gpuRenderer{textures: map[gpuTextureKey]gpuTexture{key: {width: 64, height: 64}}}
	return &game{gpuFrame: r, viewW: width, viewH: height}, tex
}

func TestGPUSpriteRunsPreserveRowSamplingAndClipping(t *testing.T) {
	for _, scale := range []float64{0.7, 1, 1.5, 2, 2.1, 3, 8} {
		g, tex := gpuSpriteTestGame(128, 192)
		g.wallDepthQCol = make([]uint16, 128)
		g.wallDepthTopCol = make([]int, 128)
		g.wallDepthBottomCol = make([]int, 128)
		for x := 45; x <= 75; x++ {
			g.wallDepthTopCol[x], g.wallDepthBottomCol[x] = 40, 90
		}
		it := cutoutItem{boundsOK: true, tex: tex, scale: scale, scaleY: scale, dstX: 0.2, dstY: -0.25, x0: 10, x1: 110, y0: 1, y1: 179, depthQ: 100, shadeMul: 192}
		commands := &g.gpuFrame.cutoutCommands
		g.gpuSprite(it, commands, false)
		coverage := make([]int, 128*192)
		for _, batch := range commands.batches[:commands.used] {
			for i := 0; i < len(batch.vertices); i += 4 {
				v := batch.vertices[i]
				end := batch.vertices[i+3]
				for y := int(v.DstY); y < int(end.DstY); y++ {
					wantY := gpuSpriteTexel(float32((float64(y)+0.5-it.dstY)/scale), tex.Height)
					gotY := gpuSpriteTexel(v.Custom1+float32(y-int(v.SrcY))*v.Custom3, tex.Height)
					if gotY != wantY {
						t.Fatalf("scale=%v y=%d texel=%d want=%d", scale, y, gotY, wantY)
					}
					for x := int(v.DstX); x < int(end.DstX); x++ {
						coverage[y*128+x]++
					}
				}
			}
		}
		for y := it.y0; y <= it.y1; y++ {
			for _, span := range g.spriteRowVisibleSpansDepthQ(y, it.x0, it.x1, it.depthQ, nil, nil) {
				for x := span.L; x <= span.R; x++ {
					coverage[y*128+x]--
				}
			}
		}
		for i, count := range coverage {
			if count != 0 {
				t.Fatalf("scale=%v pixel=(%d,%d) coverage difference=%d", scale, i%128, i/128, count)
			}
		}
	}
	g, tex := gpuSpriteTestGame(128, 192)
	g.gpuSprite(cutoutItem{boundsOK: true, tex: tex, scale: 2, x1: 127, y1: 191, shadeMul: 192}, &g.gpuFrame.cutoutCommands, false)
	if got := len(g.gpuFrame.cutoutCommands.batches[0].vertices); got != 4 {
		t.Fatalf("unclipped sprite vertices=%d want=4 (one run)", got)
	}
}

func TestGPUWallRunsSplitAtSamplingAndLightingChanges(t *testing.T) {
	r, commands := &gpuRenderer{}, &gpuCommands{}
	tex := gpuTexture{id: 1}
	for x := 0; x < 64; x++ {
		r.rect(commands, x, 4, x, 127, tex, tex, 1, 192, 0, 7, fracUnit/3, 0, fracUnit/7)
	}
	if got := len(commands.batches[0].vertices); got != 4 || commands.batches[0].vertices[3].DstX != 64 {
		t.Fatalf("repeated wall vertices=%d want=4", got)
	}
	// Each of these changes must end the prior run.
	r.rect(commands, 64, 4, 64, 127, tex, tex, 1, 191, 0, 7, fracUnit/3, 0, fracUnit/7)
	r.rect(commands, 65, 4, 65, 127, tex, tex, 1, 191, 0, 8, fracUnit/3, 0, fracUnit/7)
	r.rect(commands, 66, 5, 66, 127, tex, tex, 1, 191, 0, 8, fracUnit/3, 0, fracUnit/7)
	r.rect(commands, 67, 5, 67, 127, tex, tex, 1, 191, 0, 8, fracUnit/3, 0, fracUnit/7+1)
	if got := len(commands.batches[0].vertices); got != 20 {
		t.Fatalf("wall vertices after parameter changes=%d want=20", got)
	}
}

func TestGPUWorldShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader(worldIndexedShaderSrc); err != nil {
		t.Fatal(err)
	}
}

func TestGPUTextureMetadataPacking(t *testing.T) {
	pixels := make([]byte, 256*256*4)
	// Keep the spectre lookup untouched, including when writing the last ID.
	for i := gpuPaletteLookupOffset * 4; i < len(pixels); i++ {
		pixels[i] = 173
	}
	for _, id := range []int{0, 127, 128, gpuModeStride - 1} {
		for _, origin := range []int{0, 1, 255, 256, 1023, 1024, gpuAtlasSize - 1} {
			for _, size := range []int{1, 2, 64, 255, 256, 1024, gpuAtlasSize} {
				tex := gpuTexture{id: id, x: origin, y: gpuAtlasSize - 1 - origin, width: size, height: gpuAtlasSize + 1 - size}
				putGPUTextureMetadata(pixels, tex)
				for n, want := range [2][2]int{{tex.x, tex.width}, {tex.y, tex.height}} {
					i := (id*2 + n) * 4
					// Decode with float32 arithmetic, as the shader does.
					packed := float32(pixels[i]) + float32(pixels[i+1])*256 + float32(pixels[i+2])*65536
					got := [2]int{int(packed) % gpuAtlasSize, int(packed)/gpuAtlasSize + 1}
					if got != want || pixels[i+3] != 255 {
						t.Fatalf("id=%d axis=%d got=%v alpha=%d want=%v", id, n, got, pixels[i+3], want)
					}
				}
			}
		}
	}
	for i := gpuPaletteLookupOffset * 4; i < len(pixels); i++ {
		if pixels[i] != 173 {
			t.Fatalf("metadata overwrote palette lookup at byte %d", i)
		}
	}
}

func TestGPUWrapFixedPreservesTexturePhase(t *testing.T) {
	for _, size := range []int{1, 64, 128} {
		period := int64(size) * fracUnit
		for _, v := range []int64{-period - 1, -fracUnit, -1, 0, 1, period - 1, period, period + 1} {
			got := int64(gpuWrapFixed(v, size))
			want := v % period
			if want < 0 {
				want += period
			}
			if got != want {
				t.Fatalf("size=%d v=%d got=%d want=%d", size, v, got, want)
			}
		}
	}
}

func TestGPUBatchesPreserveAtlasOrderAndReuseStorage(t *testing.T) {
	r := &gpuRenderer{}
	commands := &gpuCommands{}
	a := gpuTexture{id: 1, page: 0}
	b := gpuTexture{id: 2, page: 1}
	for _, tex := range []gpuTexture{a, a, b, a} {
		r.rect(commands, 0, 0, 1, 1, tex, tex, 2, 256, 0, 0, 0, 1, 1)
	}
	if commands.used != 3 {
		t.Fatalf("batches=%d want 3", commands.used)
	}
	if len(commands.batches[0].vertices) != 8 {
		t.Fatal("adjacent atlas commands were not combined")
	}
	backing := &commands.batches[0].vertices[0]
	commands.reset()
	r.rect(commands, 0, 0, 1, 1, a, a, 2, 256, 0, 0, 0, 1, 1)
	if commands.used != 1 || backing != &commands.batches[0].vertices[0] {
		t.Fatal("batch storage was not reused")
	}
	for i := 0; i < 17000; i++ {
		r.rect(commands, 0, 0, 1, 1, a, a, 2, 256, 0, 0, 0, 1, 1)
	}
	for _, batch := range commands.batches[:commands.used] {
		if len(batch.vertices) > 65532 {
			t.Fatal("batch exceeds uint16 vertex capacity")
		}
		for _, index := range batch.indices {
			if int(index) >= len(batch.vertices) {
				t.Fatal("index outside batch")
			}
		}
	}
}

func TestGPUUnsupportedAssetsFallBackBeforeDrawing(t *testing.T) {
	for _, tex := range []WallTexture{
		{Width: gpuAtlasSize + 1, Height: 1, Indexed: make([]byte, gpuAtlasSize+1)},
		{Width: 8, Height: 8},
	} {
		g := &game{opts: Options{GPURenderer: true, SourcePortMode: true, WallTexBank: map[string]WallTexture{"CUSTOM": tex}}}
		g.beginGPUFrame()
		if g.gpuFrame != nil || g.gpu == nil || !g.gpu.failed {
			t.Fatal("unsupported bank did not select CPU fallback")
		}
	}
}
