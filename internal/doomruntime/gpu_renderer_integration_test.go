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
		if path := os.Getenv("GD_GPU_WAD"); path != "" {
			gpuCompareMaps(t, path)
		}
	}
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(driver); err != nil {
		t.Fatal(err)
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
	r.fuzzCommands.reset()
	r.rect(&r.fuzzCommands, 0, 0, 63, 63, front, front, 4, 192, 0, 0, 0, 1, 1)
	r.snapshot.Fill(color.Black)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.fuzzCommands, ebiten.BlendSourceOver, r.snapshot, 0)
	r.cutouts.ReadPixels(out)
	if out[3] != 0 || out[7] != 255 || out[4] != 0 || out[5] != 0 || out[6] != 0 {
		t.Error("fuzz failed to preserve sprite mask or sample background")
	}
	g.viewW, g.viewH = 400, 240
	g.beginGPUFrame()
	if got := g.gpu.frame.Bounds().Size(); got.X != 400 || got.Y != 240 {
		t.Errorf("GPU resize=%v", got)
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
		g := newGame(m, Options{Width: 640, Height: 400, SourcePortMode: true, SourcePortSectorLighting: true, PlayerSlot: 1, SkillLevel: 4, DoomPaletteRGBA: palette, DoomColorMap: colormap, DoomColorMapRows: len(colormap) / 256, WallTexBank: walls, FlatBank: flats, FlatBankIndexed: flatIndexed, SpritePatchBank: sprites})
		g.syncRenderState()
		g.prepareRenderState()
		g.viewW = 640
		g.viewH = 400
		cpu := ebiten.NewImage(640, 400)
		gpu := ebiten.NewImage(640, 400)
		for angle := 0; angle < 4; angle++ {
			g.renderAngle = g.p.angle + uint32(angle)*0x40000000
			g.setGammaLevel([]int{2, 1, doomGammaLevels - 1, 2}[angle])
			g.inventory.InvulnTics = 0
			if angle == 2 {
				g.inventory.InvulnTics = 1000
			}
			// Each orientation starts with a fresh CPU framebuffer. Unwritten
			// portal gaps must not contain pixels retained from the prior angle.
			g.ensureWallLayer()
			clear(g.wallPix)
			cpu.Fill(color.Black)
			gpu.Fill(color.Black)
			g.opts.GPURenderer = false
			g.drawDoomBasic3D(cpu)
			cpuPixels := make([]byte, 640*400*4)
			cpu.ReadPixels(cpuPixels)
			g.opts.GPURenderer = true
			g.drawDoomBasic3D(gpu)
			if g.gpu == nil || g.gpu.failed {
				t.Fatal("GPU backend unexpectedly fell back")
			}
			gpuPixels := make([]byte, len(cpuPixels))
			gpu.ReadPixels(gpuPixels)
			differences := gpuPixelDifferences(cpuPixels, gpuPixels, 2)
			t.Logf("%s angle=%d: %.3f%% of pixels differ by more than 2 levels", name, angle, float64(differences)*100/(640*400))
			// Perspective texel boundaries and sky transcendental rounding can differ.
			// Reject structural errors, missing surfaces and clipping regressions.
			if differences > len(cpuPixels)/4/100 {
				t.Errorf("%s angle=%d exceeds 1%% pixel difference budget", name, angle)
			}
			gpuCapture(t, fmt.Sprintf("%s-%d", name, angle), cpuPixels, gpuPixels, 640, 400)
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
