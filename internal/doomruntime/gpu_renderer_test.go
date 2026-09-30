package doomruntime

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGPUWorldShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader(worldIndexedShaderSrc); err != nil {
		t.Fatal(err)
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
