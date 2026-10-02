//go:build integration

package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gddoom/internal/mapdata"
	"gddoom/internal/render/doomtex"
	"gddoom/internal/render/scene"
	"gddoom/internal/wad"
	"github.com/hajimehoshi/ebiten/v2"
)

type gpuComparisonDriver struct {
	t    *testing.T
	done bool
	run  func()
}

func (d *gpuComparisonDriver) Update() error {
	if d.done {
		return ebiten.Termination
	}
	return nil
}
func (d *gpuComparisonDriver) Layout(int, int) (int, int) { return 320, 200 }
func (d *gpuComparisonDriver) Draw(screen *ebiten.Image) {
	if !d.done {
		d.run()
		d.done = true
	}
}

// Runs real shader draws and readbacks only in this opt-in test, never during
// gameplay. GD_GPU_WAD adds map comparisons; GD_GPU_CAPTURE_DIR writes PNG pairs.
func TestGPUFramebufferComparison(t *testing.T) {
	if os.Getenv("GD_GPU_INTEGRATION") == "" {
		t.Skip("set GD_GPU_INTEGRATION=1 to compare rendered GPU pixels")
	}
	driver := &gpuComparisonDriver{t: t}
	driver.run = func() {
		gpuCompareSynthetic(t)
		gpuCompareFaithfulPalette(t)
		gpuCompareSpectreFuzz(t)
		gpuCompareFuzzDrawOrder(t)
		if path := os.Getenv("GD_GPU_WAD"); path != "" {
			gpuCompareMaps(t, path)
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
	}
}

// Use a deliberately non-linear colormap: multiplying RGB or choosing a
// nearby palette color cannot accidentally match the required indexed lookup.
func gpuCompareFaithfulPalette(t *testing.T) {
	palette := make([]byte, 256*4)
	indexed := make([]byte, 256)
	mask := make([]byte, 256)
	colormap := make([]byte, 33*256)
	for i := range indexed {
		indexed[i], mask[i] = byte(i), 1
		palette[i*4], palette[i*4+1], palette[i*4+2], palette[i*4+3] = byte(i), byte(i*53), byte(255-i), 255
	}
	for row := 0; row < 33; row++ {
		for i := 0; i < 256; i++ {
			colormap[row*256+i] = byte(i*73 + row*19)
		}
	}
	g := newGame(&mapdata.Map{Name: "E1M1"}, Options{Width: 256, Height: 40, GPURenderer: true, DisableBillboardClipping: true, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: 33})
	g.ensureWallLayer()
	g.ensure3DFrameBuffers()
	tex := &WallTexture{Width: 256, Height: 1, Indexed: indexed, OpaqueMask: mask, RGBA: append([]byte(nil), palette...)}
	actual := make([]byte, 256*40*4)
	for _, gamma := range []int{0, 2, doomGammaLevels - 1} {
		g.setGammaLevel(gamma)
		g.clearCutoutCoverage()
		g.beginGPUFrame()
		if g.gpuFrame == nil {
			t.Fatal("Faithful mode did not initialize GPU rendering")
		}
		r := g.gpuFrame
		atlas, _ := r.wallTexture(tex)
		for row := 0; row < 33; row++ {
			r.rect(&r.baseCommands, 0, row, 255, row, atlas, atlas, 0, gpuLightRow(192, row), 0, 0, 0, fracUnit, 0)
		}
		shades := []uint32{64, 192, 256}
		for i, shade := range shades {
			y := 33 + i
			it := cutoutItem{boundsOK: true, tex: tex, scale: 1, dstY: float64(y), x0: 0, x1: 255, y0: y, y1: y, shadeMul: shade}
			g.gpuSprite(it, &r.cutoutCommands, false)
			g.gpuFrame = nil
			g.drawSpriteCutoutItem(it)
			g.gpuFrame = r
		}
		g.frameSkyColU = make([]int, 256)
		g.frameSkyRowV = make([]int, 40)
		for x := range g.frameSkyColU {
			g.frameSkyColU[x] = 255 - x
		}
		r.rect(&r.skyCommands, 0, 36, 255, 36, atlas, atlas, 10, 256, 0, 0, 0, 0, 0)
		puff := projectedPuffItem{dist: 128, sx: 0, sy: 37, hasSprite: true, spriteTex: tex, clipBottom: 39}
		g.gpuTeleportPuff(puff, 128, 128)
		g.gpuFrame = nil
		g.drawProjectedPuffItem(puff, 128, 128, 256, 40)
		g.gpuFrame = r
		g.finishGPUFrame(ebiten.NewImage(256, 40), 0, 128)
		r.frame.ReadPixels(actual)
		gpuCheckPalette(t, actual)
		for y := 0; y <= 37; y++ {
			for x := 0; x < 256; x++ {
				var want uint32
				if y < 33 {
					pi := int(colormap[y*256+x]) * 4
					want = packRGBA(doomGammaTables[gamma][palette[pi]], doomGammaTables[gamma][palette[pi+1]], doomGammaTables[gamma][palette[pi+2]])
				} else if y < 36 {
					row := ((256 - int(shades[y-33])) * 31) / 256
					want = doomColormapPackedRow(row)[x]
					if g.wallPix32[y*256+x] != want {
						t.Fatalf("CPU Faithful sprite gamma=%d shade=%d index=%d bypassed COLORMAP", gamma, shades[y-33], x)
					}
				} else if y == 36 {
					want = wallShadePackedLUT[256][255-x]
				} else {
					want = wallShadePackedLUT[256][x]
					if g.wallPix32[y*256+x] != want {
						t.Fatalf("CPU Faithful teleport gamma=%d index=%d bypassed the active palette", gamma, x)
					}
				}
				i := (y*256 + x) * 4
				if actual[i] != byte(want>>pixelRShift) || actual[i+1] != byte(want>>pixelGShift) || actual[i+2] != byte(want>>pixelBShift) || actual[i+3] != 255 {
					t.Fatalf("Faithful gamma=%d row=%d index=%d differs from indexed palette lookup", gamma, y, x)
				}
			}
		}
	}
}

func gpuCompareSynthetic(t *testing.T) {
	palette := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		palette[i*4] = byte(i)
		palette[i*4+1] = byte(255 - i)
		palette[i*4+2] = byte(i * 53)
		palette[i*4+3] = 255
	}
	initWallShadePackedLUT(palette)
	doomColormapEnabled = false
	g := &game{opts: Options{GPURenderer: true, SourcePortMode: true, DoomPaletteRGBA: palette}, viewW: 320, viewH: 200, m: &mapdata.Map{Name: "E1M1"}}
	g.ensureWallLayer()
	g.ensure3DFrameBuffers()
	g.beginGPUFrame()
	if g.gpuFrame == nil {
		t.Fatal("GPU renderer failed to initialize")
	}
	for i := range g.wallPix32 {
		g.wallPix32[i] = pixelOpaqueA
	}
	indexed := planeTestIndexedTexture()
	rgba := make([]byte, 64*64*4)
	for i, index := range indexed {
		copy(rgba[i*4:i*4+4], palette[int(index)*4:int(index)*4+4])
	}
	tex := &WallTexture{Width: 64, Height: 64, Indexed: indexed, RGBA: rgba}
	for x, step := range []int64{fracUnit/8 - 1, fracUnit / 8, fracUnit / 3, fracUnit - 1, fracUnit, fracUnit * 3 / 2} {
		column := x*3 + 5
		g.gpuWallColumn(column, 3, 190, float64(step)/fracUnit*160, 17.2, 22.7, 160, wallTextureBlendSample{from: tex}, 192, 0, false)
		active := g.gpuFrame
		g.gpuFrame = nil
		g.drawBasicWallColumnTextured(column, 3, 190, float64(step)/fracUnit*160, 17.2, 22.7, 160, wallTextureBlendSample{from: tex}, 192, 0)
		g.gpuFrame = active
	}
	sample := flatTextureBlendSample{fromIndexed: indexed}
	for y := 0; y < 5; y++ {
		state := planeRowRenderState{rowBaseWXFixed: -fracUnit / 3, rowBaseWYFixed: fracUnit*63 + fracUnit/5, stepWXFixed: fracUnit / 3, stepWYFixed: -fracUnit / 7, defaultShade: 160}
		g.gpuPlaneSpan((195+y)*320, 0, 319, sample, state)
		active := g.gpuFrame
		g.gpuFrame = nil
		g.drawPlaneTexturedSpanAtDepth(g.wallPix32, (195+y)*320, 0, 319, plane3DKey{}, sample, state)
		g.gpuFrame = active
	}
	g.finishGPUFrame(ebiten.NewImage(320, 200), 0, 160)
	actual := make([]byte, 320*200*4)
	g.gpu.frame.ReadPixels(actual)
	if mismatches := gpuPixelDifferences(g.wallPix, actual, 0); mismatches != 0 {
		t.Errorf("opaque walls/planes: %d differing pixels", mismatches)
	}
	gpuCapture(t, "synthetic", g.wallPix, actual, 320, 200)
	// An opaque palette index zero must survive, while alpha holes and
	// front-to-back overlap must reveal the correct rear texel.
	r := g.gpuFrame
	r.cutoutCommands.reset()
	mask := make([]byte, len(indexed))
	for i := range mask {
		mask[i] = 1
	}
	mask[0] = 0
	front, _ := r.texture(indexed, mask, 64, 64)
	backPixels := make([]byte, 1)
	backPixels[0] = 200
	back, _ := r.texture(backPixels, nil, 1, 1)
	r.rect(&r.cutoutCommands, 0, 0, 63, 63, front, front, 2, 256, 0, 0, 0, 1, 1)
	r.rect(&r.cutoutCommands, 0, 0, 63, 63, back, back, 2, 256, 0, 0, 0, 0, 0)
	r.metadata.WritePixels(r.metadataPixels)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.cutoutCommands, ebiten.BlendDestinationOver, nil, 0)
	out := make([]byte, 320*200*4)
	r.cutouts.ReadPixels(out)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			index := indexed[y*64+x]
			if x == 0 && y == 0 {
				index = 200
			}
			want := wallShadePackedLUT[256][index]
			i := (y*320 + x) * 4
			if out[i] != byte(want>>pixelRShift) || out[i+1] != byte(want>>pixelGShift) || out[i+2] != byte(want>>pixelBShift) || out[i+3] != 255 {
				t.Errorf("masked overlap (%d,%d) differs", x, y)
				return
			}
		}
	}
	// Check flip, masked animation rounding, debug color, and a GPU-only
	// background snapshot. These draws exercise shader modes absent from many maps.
	r.overlayCommands.reset()
	animationPixels := make([]byte, 64*64)
	for i := range animationPixels {
		animationPixels[i] = 200
	}
	animation, _ := r.texture(animationPixels, nil, 64, 64)
	r.metadata.WritePixels(r.metadataPixels)
	r.rect(&r.overlayCommands, 0, 0, 63, 0, front, front, 3, 192, 0, 0, 1, 1, 0)
	r.rect(&r.overlayCommands, 0, 1, 63, 1, front, animation, 1, -193, 127, 0, fracUnit, 0, 0)
	r.rect(&r.overlayCommands, 0, 2, 63, 2, front, front, 8, 192, 0, 0, 1, 1, 0)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, nil, 0)
	r.cutouts.ReadPixels(out)
	for x := 0; x < 64; x++ {
		for y := 0; y < 3; y++ {
			want := wallShadePackedLUT[192][indexed[64+63-x]]
			if y == 1 {
				want = blendPackedRGBA(wallShadePackedLUT[192][indexed[64]], wallShadePackedLUT[192][200], 127)
			}
			if y == 2 {
				want = packRGBA(255, 0, 0)
			}
			i := (y*320 + x) * 4
			if out[i] != byte(want>>pixelRShift) || out[i+1] != byte(want>>pixelGShift) || out[i+2] != byte(want>>pixelBShift) || out[i+3] != 255 {
				t.Errorf("shader mode comparison (%d,%d) differs", x, y)
				return
			}
		}
	}
	// Teleport overlays retain their raw palette even at a different gamma.
	r.overlayCommands.reset()
	r.rect(&r.overlayCommands, 0, 0, 0, 0, front, front, 2, 257+max(r.lightRows, 1), 0, 0, 1, 0, 0)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, nil, 0)
	r.cutouts.ReadPixels(out)
	paletteOffset := int(indexed[64]) * 4
	if out[0] != palette[paletteOffset] || out[1] != palette[paletteOffset+1] || out[2] != palette[paletteOffset+2] || out[3] != 255 {
		t.Error("unshaded overlay palette changed")
	}
	// Imported sprite banks can contain RGBA-only frames. Ensure conversion
	// preserves a transparent hole, palette color, and reuses its atlas entry.
	rgbaOnly := &WallTexture{Width: 2, Height: 1, RGBA: append([]byte{0, 0, 0, 0}, palette[200*4:201*4]...)}
	converted, ok := r.wallTexture(rgbaOnly)
	if !ok {
		t.Fatal("RGBA-only sprite was not converted")
	}
	again, ok := r.wallTexture(rgbaOnly)
	if !ok || again != converted {
		t.Fatal("RGBA-only sprite atlas entry was not reused")
	}
	r.metadata.WritePixels(r.metadataPixels)
	r.overlayCommands.reset()
	r.rect(&r.overlayCommands, 0, 0, 1, 0, converted, converted, 2, 256, 0, 0, 0, 1, 0)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, nil, 0)
	r.cutouts.ReadPixels(out)
	wantRGBA := wallShadePackedLUT[256][200]
	if out[3] != 0 || out[7] != 255 || out[4] != byte(wantRGBA>>pixelRShift) || out[5] != byte(wantRGBA>>pixelGShift) || out[6] != byte(wantRGBA>>pixelBShift) {
		t.Error("RGBA-only sprite color or mask changed")
	}
	gpuCompareSpriteRuns(t, g, &WallTexture{Width: 64, Height: 64, Indexed: indexed, OpaqueMask: mask})
	// Check metadata row boundaries and the final texture ID on the GPU.
	// Reuse an uploaded texel at a known atlas origin for each aliased ID.
	r.overlayCommands.reset()
	for x, id := range []int{127, 128, gpuModeStride - 1} {
		alias := back
		alias.id = id
		putGPUTextureMetadata(r.metadataPixels, alias)
		r.rect(&r.overlayCommands, x, 0, x, 0, alias, alias, 2, 256, 0, 0, 0, 0, 0)
	}
	r.metadata.WritePixels(r.metadataPixels)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, nil, 0)
	r.cutouts.ReadPixels(out)
	for x := 0; x < 3; x++ {
		i := x * 4
		want := wallShadePackedLUT[256][200]
		if out[i] != byte(want>>pixelRShift) || out[i+1] != byte(want>>pixelGShift) || out[i+2] != byte(want>>pixelBShift) || out[i+3] != 255 {
			t.Errorf("metadata boundary pixel %d differs", x)
		}
	}
	// Dimensions of 2048 and atlas coordinates of 2047 use every packed bit.
	widePixels := make([]byte, gpuAtlasSize)
	widePixels[gpuAtlasSize-1] = 200
	wide, _ := r.texture(widePixels, nil, gpuAtlasSize, 1)
	tall, _ := r.texture(widePixels, nil, 1, gpuAtlasSize)
	r.texture(make([]byte, gpuAtlasSize-2), nil, gpuAtlasSize-2, 1)
	edge, _ := r.texture([]byte{200}, nil, 1, 1)
	if edge.x != gpuAtlasSize-1 {
		t.Fatalf("edge texture x=%d", edge.x)
	}
	r.overlayCommands.reset()
	r.rect(&r.overlayCommands, 0, 0, 0, 0, wide, wide, 2, 256, 0, gpuAtlasSize-1, 0, 0, 0)
	r.rect(&r.overlayCommands, 1, 0, 1, 0, tall, tall, 2, 256, 0, 0, gpuAtlasSize-1, 0, 0)
	r.rect(&r.overlayCommands, 2, 0, 2, 0, edge, edge, 2, 256, 0, 0, 0, 0, 0)
	r.metadata.WritePixels(r.metadataPixels)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, nil, 0)
	r.cutouts.ReadPixels(out)
	for x := 0; x < 3; x++ {
		i := x * 4
		want := wallShadePackedLUT[256][200]
		if out[i] != byte(want>>pixelRShift) || out[i+1] != byte(want>>pixelGShift) || out[i+2] != byte(want>>pixelBShift) || out[i+3] != 255 {
			t.Errorf("metadata atlas limit pixel %d differs", x)
		}
	}
	g.viewW, g.viewH = 400, 240
	g.beginGPUFrame()
	if got := g.gpu.frame.Bounds().Size(); got.X != 400 || got.Y != 240 {
		t.Errorf("GPU resize=%v", got)
	}

}

func gpuCompareSpriteRuns(t *testing.T, g *game, tex *WallTexture) {
	r := g.gpuFrame
	atlas, _ := r.wallTexture(tex)
	r.metadata.WritePixels(r.metadataPixels)
	g.wallDepthQCol = make([]uint16, g.viewW)
	g.wallDepthTopCol = make([]int, g.viewW)
	g.wallDepthBottomCol = make([]int, g.viewW)
	g.wallDepthClosedCol = make([]bool, g.viewW)
	g.maskedClipCols = make([][]scene.MaskedClipSpan, g.viewW)
	g.maskedClipFirstDepthQ = make([]uint16, g.viewW)
	for x := 30; x <= 60; x++ {
		g.wallDepthTopCol[x], g.wallDepthBottomCol[x] = 40, 130
	}
	for x := 100; x <= 110; x++ {
		g.maskedClipFirstDepthQ[x] = 20
		g.maskedClipCols[x] = []scene.MaskedClipSpan{{HasOpen: true, OpenY0: 40, OpenY1: 140, DepthQ: 20}}
	}
	actual, expected := make([]byte, g.viewW*g.viewH*4), make([]byte, g.viewW*g.viewH*4)
	for _, scale := range []float64{0.7, 1, 1.5, 2, 2.1, 3, 8} {
		for _, mode := range []int{2, 3, 8, 9} {
			it := cutoutItem{boundsOK: true, tex: tex, scale: scale, dstX: 0.2, dstY: -0.25, x0: 5, x1: 150, y0: 3, y1: 183, depthQ: 100, shadeMul: 192, flip: mode%2 != 0, shadow: mode == 4 || mode == 5, debugOverlay: mode >= 8}
			r.overlayCommands.reset()
			g.gpuSprite(it, &r.overlayCommands, false)
			rowCommands := &gpuCommands{}
			light := 192
			if it.shadow {
				light = gpuLightRow(doomShadeMulFromRow(6), 6)
			}
			for y := it.y0; y <= it.y1; y++ {
				for _, span := range g.spriteRowVisibleSpansDepthQ(y, it.x0, it.x1, it.depthQ, nil, nil) {
					u := float32((float64(span.L) + 0.5 - it.dstX) / scale)
					v := float32((float64(y) + 0.5 - it.dstY) / scale)
					r.rect(rowCommands, span.L, y, span.R, y, atlas, atlas, mode, light, 0, u, v, float32(1/scale), float32(1/scale))
				}
			}
			r.snapshot.Fill(color.RGBA{R: 83, G: 171, B: 249, A: 255})
			r.cutouts.Clear()
			r.drawCommands(r.cutouts, rowCommands, ebiten.BlendSourceOver, r.snapshot, 0)
			r.cutouts.ReadPixels(expected)
			r.cutouts.Clear()
			r.drawCommands(r.cutouts, &r.overlayCommands, ebiten.BlendSourceOver, r.snapshot, 0)
			r.cutouts.ReadPixels(actual)
			if differences := gpuPixelDifferences(actual, expected, 0); differences != 0 {
				t.Errorf("sprite run scale=%v mode=%d: %d pixels differ from row commands", scale, mode, differences)
			}
		}
	}
}

func gpuCompareMaps(t *testing.T, path string) {
	wf, err := wad.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := doomtex.LoadFromWAD(wf)
	if err != nil {
		t.Fatal(err)
	}
	palette, err := ts.PaletteRGBA(0)
	if err != nil {
		t.Fatal(err)
	}
	walls := map[string]WallTexture{}
	for _, name := range ts.TextureNames() {
		indexed, w, h, err := ts.BuildTextureIndexed(name)
		if err != nil {
			continue
		}
		rgba, _, _, err := ts.BuildTextureRGBA(name, 0)
		if err != nil {
			continue
		}
		column := make([]byte, len(indexed))
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				column[x*h+y] = indexed[y*w+x]
			}
		}
		walls[name] = WallTexture{Width: w, Height: h, Indexed: indexed, IndexedColMajor: column, RGBA: rgba}
	}
	sprites := map[string]WallTexture{}
	inSprites := false
	for _, lump := range wf.Lumps {
		if lump.Name == "S_START" || lump.Name == "SS_START" {
			inSprites = true
			continue
		}
		if lump.Name == "S_END" || lump.Name == "SS_END" {
			inSprites = false
			continue
		}
		if !inSprites {
			continue
		}
		indexed, opaque, w, h, ox, oy, err := ts.BuildPatchIndexedView(lump.Name)
		if err != nil {
			continue
		}
		rgba, _, _, _, _, err := ts.BuildPatchRGBA(lump.Name, 0)
		if err != nil {
			continue
		}
		mask := make([]byte, len(opaque))
		for i, on := range opaque {
			if on {
				mask[i] = 1
			}
		}
		sprites[lump.Name] = WallTexture{Width: w, Height: h, OffsetX: ox, OffsetY: oy, RGBA: rgba, Indexed: indexed, OpaqueMask: mask}
	}
	flats, err := doomtex.LoadFlatsRGBA(wf, 0)
	if err != nil {
		t.Fatal(err)
	}
	flatIndexed, err := doomtex.LoadFlatsIndexed(wf)
	if err != nil {
		t.Fatal(err)
	}
	colormap := []byte(nil)
	if lump, ok := wf.LumpByName("COLORMAP"); ok {
		colormap, _ = wf.LumpData(lump)
	}
	for _, name := range []mapdata.MapName{"E1M1", "E1M5", "MAP11", "MAP26"} {
		m, err := mapdata.LoadMap(wf, name)
		if err != nil {
			continue
		}
		for _, mode := range []struct {
			name                  string
			sourcePort            bool
			width, height, detail int
		}{{"source-port", true, 640, 400, 0}, {"faithful", false, 320, 200, 0}, {"faithful-low", false, 320, 200, 1}} {
			w, h := mode.width, mode.height
			g := newGame(m, Options{Width: w, Height: h, SourcePortMode: mode.sourcePort, InitialDetailLevel: mode.detail, SourcePortSectorLighting: true, PlayerSlot: 1, SkillLevel: 4, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: len(colormap) / 256, WallTexBank: walls, FlatBank: flats, FlatBankIndexed: flatIndexed, SpritePatchBank: sprites})
			g.syncRenderState()
			g.prepareRenderState()
			g.viewW, g.viewH = w, h
			cpu := ebiten.NewImage(w, h)
			gpu := ebiten.NewImage(w, h)
			type comparisonView struct {
				x, y, eyeZ float64
				angle      uint32
				label      string
			}
			views := make([]comparisonView, 0, 8)
			for angle := 0; angle < 4; angle++ {
				views = append(views, comparisonView{g.renderPX, g.renderPY, g.playerEyeZ(), g.p.angle + uint32(angle)*0x40000000, fmt.Sprintf("angle-%d", angle)})
			}
			seen := map[string]bool{}
			for i, th := range m.Things {
				kind := ""
				if th.Type == 58 && g.thingHP[i] > 0 {
					kind = "spectre"
				} else if isMonster(th.Type) && g.thingHP[i] > 0 {
					kind = "monster"
				} else if isBarrelThingType(th.Type) {
					kind = "barrel"
				}
				if kind == "" || seen[kind] {
					continue
				}
				seen[kind] = true
				x, y := g.thingPosFixed(i, th)
				z, _, _ := g.thingSupportState(i, th)
				for _, distance := range []float64{32, 48} {
					views = append(views, comparisonView{float64(x)/fracUnit - distance, float64(y) / fracUnit, float64(z)/fracUnit + 41, 0, fmt.Sprintf("melee-%s-%g", kind, distance)})
				}
			}
			for index, view := range views {
				g.renderPX, g.renderPY, g.renderAngle = view.x, view.y, view.angle
				g.playerViewZ = int64(view.eyeZ * fracUnit)
				g.setGammaLevel([]int{2, 1, doomGammaLevels - 1, 2}[index%4])
				g.inventory.InvulnTics = 0
				if index == 2 {
					g.inventory.InvulnTics = 1000
				}
				// Each orientation starts with a fresh CPU framebuffer. Unwritten
				// portal gaps must not contain pixels retained from the prior angle.
				g.ensureWallLayer()
				clear(g.wallPix)
				cpu.Fill(color.Black)
				gpu.Fill(color.Black)
				g.opts.GPURenderer = false
				fuzzPhase := g.spectreFuzzPos
				g.drawDoomBasic3D(cpu)
				cpuPixels := make([]byte, w*h*4)
				cpu.ReadPixels(cpuPixels)
				g.opts.GPURenderer = true
				g.spectreFuzzPos = fuzzPhase
				g.drawDoomBasic3D(gpu)
				if g.gpu == nil || g.gpu.failed {
					t.Fatal("GPU backend unexpectedly fell back")
				}
				gpuPixels := make([]byte, len(cpuPixels))
				gpu.ReadPixels(gpuPixels)
				differences := gpuPixelDifferences(cpuPixels, gpuPixels, 2)
				t.Logf("%s %s %s: %.3f%% of pixels differ by more than 2 levels", name, mode.name, view.label, float64(differences)*100/float64(w*h))
				// Perspective texel boundaries and sky transcendental rounding can differ.
				// Reject structural errors, missing surfaces and clipping regressions.
				if differences > len(cpuPixels)/4/100 {
					t.Errorf("%s %s %s exceeds 1%% pixel difference budget", name, mode.name, view.label)
				}
				gpuCapture(t, fmt.Sprintf("%s-%s-%s", name, mode.name, view.label), cpuPixels, gpuPixels, w, h)
				if !mode.sourcePort {
					gpuCheckPalette(t, gpuPixels)
					if mode.detail == 1 {
						for y := 0; y < h; y++ {
							for x := 1; x < w; x += 2 {
								i := (y*w + x) * 4
								if string(gpuPixels[i:i+4]) != string(gpuPixels[i-4:i]) {
									t.Fatalf("%s low-detail column pair differs at (%d,%d)", name, x, y)
								}
							}
						}
					}
				}
			}
		}
	}
}

func gpuCheckPalette(t *testing.T, pixels []byte) {
	t.Helper()
	allowed := make(map[uint32]bool, 256)
	for _, p := range wallShadePackedLUT[256] {
		allowed[p] = true
	}
	for i := 0; i < len(pixels); i += 4 {
		p := packRGBA(pixels[i], pixels[i+1], pixels[i+2])
		if !allowed[p] || pixels[i+3] != 255 {
			t.Fatalf("Faithful pixel %d color=%08x is outside the active 256-color palette", i/4, p)
		}
	}
}

func gpuPixelDifferences(a, b []byte, tolerance int) int {
	count := 0
	for i := 0; i < len(a); i += 4 {
		for c := 0; c < 4; c++ {
			delta := int(a[i+c]) - int(b[i+c])
			if delta < -tolerance || delta > tolerance {
				count++
				break
			}
		}
	}
	return count
}

func gpuCapture(t *testing.T, name string, cpu, gpu []byte, width, height int) {
	dir := os.Getenv("GD_GPU_CAPTURE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for i, pixels := range [][]byte{cpu, gpu} {
		path := filepath.Join(dir, fmt.Sprintf("%s-%s.png", name, []string{"cpu", "gpu"}[i]))
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, &image.RGBA{Pix: pixels, Stride: width * 4, Rect: image.Rect(0, 0, width, height)})
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
