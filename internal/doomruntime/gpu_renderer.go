package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const gpuAtlasSize = 2048
const gpuModeStride = 16384

type gpuTextureKey struct {
	indexed       *byte
	rgba32        *uint32
	mask          *byte
	width, height int
}
type gpuTexture struct{ id, page, x, y, width, height int }
type gpuAtlasPage struct {
	image           *ebiten.Image
	x, y, rowHeight int
}
type gpuBatch struct {
	page, otherPage int
	vertices        []ebiten.Vertex
	indices         []uint16
}
type gpuCommands struct {
	batches []gpuBatch
	used    int
}

// gpuRenderer retains all CPU visibility decisions, but records raster work as
// rectangles for the fragment shader. No rendered image is read back by this path.
type gpuRenderer struct {
	shader                                                                   *ebiten.Shader
	failed                                                                   bool
	textures                                                                 map[gpuTextureKey]gpuTexture
	converted                                                                map[gpuTextureKey]gpuTexture
	pages                                                                    []gpuAtlasPage
	metadata                                                                 *ebiten.Image
	metadataPixels                                                           []byte
	metadataDirty                                                            bool
	lights                                                                   *ebiten.Image
	lightGamma, lightRows                                                    int
	lightPixels                                                              []byte
	frame, cutouts, snapshot                                                 *ebiten.Image
	width, height                                                            int
	baseCommands, skyCommands, cutoutCommands, fuzzCommands, overlayCommands gpuCommands
}

// Reject unsupported texture banks before recording any draws, so the CPU
// fallback remains a complete frame. Banks are immutable during a game session.
func gpuValidateAssets(opts Options) error {
	if len(opts.WallTexBank)+len(opts.SpritePatchBank)+len(opts.FlatBankIndexed) >= gpuModeStride {
		return fmt.Errorf("texture count exceeds %d", gpuModeStride-1)
	}
	for _, bank := range []map[string]WallTexture{opts.WallTexBank, opts.SpritePatchBank} {
		for name, tex := range bank {
			if tex.Width <= 0 || tex.Height <= 0 || tex.Width > gpuAtlasSize || tex.Height > gpuAtlasSize {
				return fmt.Errorf("texture %s dimensions exceed GPU atlas capacity", name)
			}
			if len(tex.Indexed) != tex.Width*tex.Height && len(tex.RGBA) != tex.Width*tex.Height*4 && len(tex.RGBA32) != tex.Width*tex.Height {
				return fmt.Errorf("texture %s has no indexed pixels", name)
			}
		}
	}
	return nil
}

var gpuFuzzOffsets = func() [50]float32 {
	var offsets [50]float32
	for i, delta := range doomFuzzOffsets {
		offsets[i] = float32(delta)
	}
	return offsets
}()

func (g *game) beginGPUFrame() {
	if !g.opts.GPURenderer || !g.opts.SourcePortMode {
		return
	}
	if g.gpu == nil {
		g.gpu = &gpuRenderer{lightGamma: -1}
	}
	r := g.gpu
	if r.failed {
		return
	}
	if r.shader == nil {
		if err := gpuValidateAssets(g.opts); err != nil {
			fmt.Printf("GPU renderer unavailable, using CPU: %v\n", err)
			r.failed = true
			return
		}
		shader, err := ebiten.NewShader(worldIndexedShaderSrc)
		if err != nil {
			fmt.Printf("GPU renderer unavailable, using CPU: %v\n", err)
			r.failed = true
			return
		}
		r.shader = shader
		r.textures = make(map[gpuTextureKey]gpuTexture)
		r.converted = make(map[gpuTextureKey]gpuTexture)
		r.metadataPixels = make([]byte, 256*256*4)
		r.metadata = newUnmanagedImage(256, 256)
	}
	if r.width != g.viewW || r.height != g.viewH {
		for _, img := range []*ebiten.Image{r.frame, r.cutouts, r.snapshot} {
			if img != nil {
				img.Deallocate()
			}
		}
		r.width, r.height = g.viewW, g.viewH
		r.frame = newUnmanagedImage(r.width, r.height)
		r.cutouts = newUnmanagedImage(r.width, r.height)
		r.snapshot = newUnmanagedImage(r.width, r.height)
	}
	r.baseCommands.reset()
	r.skyCommands.reset()
	r.cutoutCommands.reset()
	r.fuzzCommands.reset()
	r.overlayCommands.reset()
	r.updateLights(g.opts.DoomPaletteRGBA)
	g.gpuFrame = r
}

func (c *gpuCommands) reset() {
	c.used = 0
	for i := range c.batches {
		c.batches[i].vertices = c.batches[i].vertices[:0]
		c.batches[i].indices = c.batches[i].indices[:0]
	}
}

func (r *gpuRenderer) updateLights(palette []byte) {
	rows := doomColormapRowCount()
	if r.lights != nil && r.lightGamma == activeGammaLevel && r.lightRows == rows {
		return
	}
	h := 257 + max(rows, 1) + 1
	if r.lights == nil || r.lightRows != rows {
		if r.lights != nil {
			r.lights.Deallocate()
		}
		r.lights = newUnmanagedImage(256, h)
		r.lightPixels = make([]byte, 256*h*4)
	}
	for shade := 0; shade <= 256; shade++ {
		for idx, p := range wallShadePackedLUT[shade] {
			putGPUPackedPixel(r.lightPixels, (shade*256+idx)*4, p)
		}
	}
	for row := 0; row < rows; row++ {
		for idx, p := range doomColormapPackedRow(row) {
			putGPUPackedPixel(r.lightPixels, ((257+row)*256+idx)*4, p)
		}
	}
	// Teleport overlays use the original unshaded RGBA palette on the CPU.
	if len(palette) >= 256*4 {
		copy(r.lightPixels[(h-1)*256*4:], palette[:256*4])
	}
	// The unused blue metadata channel holds the RGB 5-bit palette lookup
	// for shading the spectre's background snapshot without a GPU readback.
	for i, index := range doomPalIndexLUT32 {
		r.metadataPixels[i*4+2] = index
	}
	r.metadataDirty = true
	r.lights.WritePixels(r.lightPixels)
	r.lightGamma, r.lightRows = activeGammaLevel, rows
}

func putGPUPackedPixel(dst []byte, i int, p uint32) {
	dst[i] = byte(p >> pixelRShift)
	dst[i+1] = byte(p >> pixelGShift)
	dst[i+2] = byte(p >> pixelBShift)
	dst[i+3] = 255
}

func gpuLightRow(shade, row int) int {
	if row >= doomNumColorMaps || doomColormapEnabled {
		return 257 + min(max(row, 0), max(doomColormapRowCount()-1, 0))
	}
	return min(max(shade, 0), 256)
}

func gpuWrapFixed(v int64, size int) float32 {
	period := int64(size) * fracUnit
	if period <= 0 {
		return 0
	}
	v %= period
	if v < 0 {
		v += period
	}
	return float32(v)
}

func (r *gpuRenderer) texture(indexed, mask []byte, width, height int) (gpuTexture, bool) {
	if width <= 0 || height <= 0 || width > gpuAtlasSize || height > gpuAtlasSize || len(indexed) != width*height {
		return gpuTexture{}, false
	}
	key := gpuTextureKey{indexed: &indexed[0], width: width, height: height}
	if len(mask) == len(indexed) {
		key.mask = &mask[0]
	}
	if tex, ok := r.textures[key]; ok {
		return tex, true
	}
	if len(r.textures) >= gpuModeStride {
		panic("GPU texture metadata capacity exceeded")
	}
	page := len(r.pages) - 1
	if page < 0 {
		r.pages = append(r.pages, gpuAtlasPage{image: newUnmanagedImage(gpuAtlasSize, gpuAtlasSize)})
		page = 0
	}
	p := &r.pages[page]
	if p.x+width > gpuAtlasSize {
		p.x = 0
		p.y += p.rowHeight
		p.rowHeight = 0
	}
	if p.y+height > gpuAtlasSize {
		r.pages = append(r.pages, gpuAtlasPage{image: newUnmanagedImage(gpuAtlasSize, gpuAtlasSize)})
		page++
		p = &r.pages[page]
	}
	tex := gpuTexture{id: len(r.textures), page: page, x: p.x, y: p.y, width: width, height: height}
	pixels := make([]byte, width*height*4)
	for i, index := range indexed {
		pixels[i*4] = index
		pixels[i*4+1] = 255
		pixels[i*4+3] = 255
		if key.mask != nil && mask[i] == 0 {
			pixels[i*4+1] = 0
		}
	}
	p.image.SubImage(image.Rect(p.x, p.y, p.x+width, p.y+height)).(*ebiten.Image).WritePixels(pixels)
	p.x += width
	p.rowHeight = max(p.rowHeight, height)
	for n, v := range []int{tex.x, tex.y, width, height} {
		i := (tex.id*4 + n) * 4
		r.metadataPixels[i] = byte(v)
		r.metadataPixels[i+1] = byte(v >> 8)
		r.metadataPixels[i+3] = 255
	}
	r.metadataDirty = true
	r.textures[key] = tex
	return tex, true
}

func (r *gpuRenderer) wallTexture(tex *WallTexture) (gpuTexture, bool) {
	if tex == nil {
		return gpuTexture{}, false
	}
	tex.EnsureOpaqueMask()
	if len(tex.Indexed) == tex.Width*tex.Height {
		return r.texture(tex.Indexed, tex.OpaqueMask, tex.Width, tex.Height)
	}
	// Some imported sprite frames carry only RGBA. Convert them once, using
	// the same palette matching as the CPU sprite path, and retain the atlas entry.
	key := gpuTextureKey{width: tex.Width, height: tex.Height}
	if len(tex.RGBA) > 0 {
		key.indexed = &tex.RGBA[0]
	}
	if len(tex.RGBA32) > 0 {
		key.rgba32 = &tex.RGBA32[0]
	}
	if entry, ok := r.converted[key]; ok {
		return entry, true
	}
	converted, ok := synthesizeIndexedSpriteTexture(*tex)
	if !ok {
		return gpuTexture{}, false
	}
	entry, ok := r.texture(converted.Indexed, converted.OpaqueMask, converted.Width, converted.Height)
	if ok {
		r.converted[key] = entry
	}
	return entry, ok
}

func (r *gpuRenderer) rect(commands *gpuCommands, x0, y0, x1, y1 int, tex, other gpuTexture, mode, light int, alpha uint8, u, v, du, dv float32) {
	if x1 < x0 || y1 < y0 {
		return
	}
	var b *gpuBatch
	if commands.used > 0 {
		b = &commands.batches[commands.used-1]
	}
	if b == nil || b.page != tex.page || b.otherPage != other.page || len(b.vertices)+4 > 65532 {
		if commands.used == len(commands.batches) {
			commands.batches = append(commands.batches, gpuBatch{})
		}
		b = &commands.batches[commands.used]
		commands.used++
		b.page = tex.page
		b.otherPage = other.page
	}
	offset := uint16(len(b.vertices))
	vertex := ebiten.Vertex{SrcX: float32(x0), SrcY: float32(y0), ColorR: float32(light), ColorG: float32(tex.id + mode*gpuModeStride), ColorB: float32(other.id), ColorA: float32(alpha) / 255, Custom0: u, Custom1: v, Custom2: du, Custom3: dv}
	for _, p := range [4][2]int{{x0, y0}, {x1 + 1, y0}, {x0, y1 + 1}, {x1 + 1, y1 + 1}} {
		vertex.DstX = float32(p[0])
		vertex.DstY = float32(p[1])
		b.vertices = append(b.vertices, vertex)
	}
	b.indices = append(b.indices, offset, offset+1, offset+2, offset+1, offset+2, offset+3)
}

func (r *gpuRenderer) solid(commands *gpuCommands, x0, y0, x1, y1 int, p uint32) {
	r.rect(commands, x0, y0, x1, y1, gpuTexture{}, gpuTexture{}, 6, 0, 0, float32(byte(p>>pixelRShift))/255, float32(byte(p>>pixelGShift))/255, float32(byte(p>>pixelBShift))/255, 0)
}

func (g *game) gpuWallColumn(x, y0, y1 int, depth, texU, texMid, focal float64, sample wallTextureBlendSample, shade, row int, masked bool) {
	r := g.gpuFrame
	tex, ok := r.wallTexture(sample.from)
	if !ok {
		return
	}
	other := tex
	alpha := uint8(0)
	if masked && sample.to != nil && sample.alpha != 0 && sample.to.Width == sample.from.Width && sample.to.Height == sample.from.Height {
		if next, valid := r.wallTexture(sample.to); valid {
			other = next
			alpha = sample.alpha
		}
	}
	tx := wrapIndex(int(floorFixed(texU)>>fracBits), tex.width)
	step := floorFixed(depth / focal)
	start := floorFixed(texMid - (float64(g.viewH)*0.5-float64(y0)-0.5)*depth/focal)
	light := gpuLightRow(shade, row)
	if masked {
		light = -light - 1
		for _, span := range g.maskedColumnVisibleSpans(x, y0, y1, encodeDepthQ(depth)) {
			v := start + int64(span.L-y0)*step
			r.rect(&r.cutoutCommands, x, span.L, x, span.R, tex, other, 1, light, alpha, float32(tx), gpuWrapFixed(v, tex.height), 0, gpuWrapFixed(step, tex.height))
		}
	} else {
		r.rect(&r.baseCommands, x, y0, x, y1, tex, other, 1, light, 0, float32(tx), gpuWrapFixed(start, tex.height), 0, gpuWrapFixed(step, tex.height))
	}
}

func (g *game) gpuPlaneSpan(rowPix, x1, x2 int, sample flatTextureBlendSample, state planeRowRenderState) {
	r := g.gpuFrame
	y := rowPix / g.viewW
	if !doomColormapEnabled && state.defaultShade == 0 {
		r.solid(&r.baseCommands, x1, y, x2, y, pixelOpaqueA)
		return
	}
	tex, ok := r.texture(sample.fromIndexed, nil, 64, 64)
	if !ok {
		return
	}
	// Match the CPU's current plane path, which samples only the active frame.
	u := state.rowBaseWXFixed + int64(x1)*state.stepWXFixed
	v := state.rowBaseWYFixed + int64(x1)*state.stepWYFixed
	r.rect(&r.baseCommands, x1, y, x2, y, tex, tex, 0, gpuLightRow(int(state.defaultShade), state.defaultRow), 0, gpuWrapFixed(u, 64), gpuWrapFixed(v, 64), gpuWrapFixed(state.stepWXFixed, 64), gpuWrapFixed(state.stepWYFixed, 64))
}

func (g *game) gpuSprite(it cutoutItem, commands *gpuCommands, unshaded bool) {
	if !it.boundsOK || it.tex == nil || it.scale <= 0 || it.x1 < it.x0 || it.y1 < it.y0 {
		return
	}
	r := g.gpuFrame
	tex, ok := r.wallTexture(it.tex)
	if !ok {
		return
	}
	scaleY := it.scale
	if it.scaleY > 0 {
		scaleY = it.scaleY
	}
	light := min(int(it.shadeMul), 256)
	if it.shadow {
		light = gpuLightRow(doomShadeMulFromRow(6), 6)
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		light = 257 + row
	}
	if unshaded {
		light = 257 + max(r.lightRows, 1)
	}
	mode := 2
	if it.flip {
		mode = 3
	}
	if it.shadow {
		mode += 2
	}
	if it.debugOverlay {
		mode = 8
		if it.flip {
			mode = 9
		}
	}
	for y := it.y0; y <= it.y1; y++ {
		spans := g.spriteRowVisibleSpansDepthQ(y, it.x0, it.x1, it.depthQ, it.clipSpans, g.solidClipScratch[:0])
		g.solidClipScratch = spans
		for _, span := range spans {
			u := float32((float64(span.L) + 0.5 - it.dstX) / it.scale)
			v := float32((float64(y) + 0.5 - it.dstY) / scaleY)
			r.rect(commands, span.L, y, span.R, y, tex, tex, mode, light, 0, u, v, float32(1/it.scale), float32(1/scaleY))
		}
	}
}

func (g *game) gpuTeleportPuff(it projectedPuffItem, focal, focalV float64) {
	tex := it.spriteTex
	if tex == nil || it.dist <= 0 {
		return
	}
	scale, scaleY := focal/it.dist, focalV/it.dist
	dstX := it.sx - float64(tex.OffsetX)*scale
	dstY := it.sy - float64(tex.OffsetY)*scaleY
	x0 := max(0, int(math.Floor(dstX)))
	x1 := min(g.viewW-1, int(math.Ceil(dstX+float64(tex.Width)*scale))-1)
	y0 := max(it.clipTop, max(0, int(math.Floor(dstY))))
	y1 := min(it.clipBottom, min(g.viewH-1, int(math.Ceil(dstY+float64(tex.Height)*scaleY))-1))
	g.gpuSprite(cutoutItem{boundsOK: true, tex: tex, scale: scale, scaleY: scaleY, dstX: dstX, dstY: dstY, x0: x0, x1: x1, y0: y0, y1: y1, depthQ: encodeDepthQ(it.dist), shadeMul: 256}, &g.gpuFrame.overlayCommands, true)
}

func (r *gpuRenderer) drawCommands(dst *ebiten.Image, commands *gpuCommands, blend ebiten.Blend, background *ebiten.Image, tic int) {
	fuzzShade := float32(1)
	if doomLightingEnabled {
		fuzzShade = float32(doomShadeMulFromRow(6)) / 256
	}
	for i := 0; i < commands.used; i++ {
		b := &commands.batches[i]
		var atlas, other *ebiten.Image
		if len(r.pages) > 0 {
			atlas = r.pages[b.page].image
			other = r.pages[b.otherPage].image
		}
		if background != nil {
			other = background
		}
		options := ebiten.DrawTrianglesShaderOptions{Blend: blend, Images: [4]*ebiten.Image{atlas, r.lights, r.metadata, other}, Uniforms: map[string]any{"ViewSize": []float32{float32(r.width), float32(r.height)}, "FuzzPhase": float32(tic), "FuzzShade": fuzzShade, "FuzzOffsets": gpuFuzzOffsets[:]}}
		dst.DrawTrianglesShader(b.vertices, b.indices, r.shader, &options)
	}
}

func (g *game) finishGPUFrame(dst *ebiten.Image, camAng, focal float64) {
	r := g.gpuFrame
	if r.metadataDirty {
		r.metadata.WritePixels(r.metadataPixels)
		r.metadataDirty = false
	}
	r.frame.Fill(color.Black)
	r.snapshot.Clear()
	if key, tex, ok := g.runtimeSkyTextureEntryForMap(g.m.Name); ok {
		g.initSkyLayerShader()
		if g.enableSkyLayerFrame(camAng, focal, key, tex, effectiveSkyTexHeight(tex)) {
			g.drawSkyLayerFrame(r.snapshot)
		}
	}
	r.drawCommands(r.frame, &r.skyCommands, ebiten.BlendSourceOver, r.snapshot, g.worldTic)
	r.drawCommands(r.frame, &r.baseCommands, ebiten.BlendSourceOver, nil, g.worldTic)
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.cutoutCommands, ebiten.BlendDestinationOver, nil, g.worldTic)
	r.frame.DrawImage(r.cutouts, nil)
	if r.fuzzCommands.used > 0 {
		r.snapshot.Clear()
		r.snapshot.DrawImage(r.frame, nil)
		r.drawCommands(r.frame, &r.fuzzCommands, ebiten.BlendSourceOver, r.snapshot, g.worldTic)
	}
	r.drawCommands(r.frame, &r.overlayCommands, ebiten.BlendSourceOver, nil, g.worldTic)
	dst.DrawImage(r.frame, nil)
}
