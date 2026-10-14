package doomruntime

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

const gpuAtlasSize = 2048
const gpuModeStride = 16384
const gpuPaletteLookupOffset = gpuModeStride * 2

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

type gpuCutoutPass struct {
	color, logical gpuCommands
	fuzz           bool
}

// gpuRenderer retains all CPU visibility decisions, but records raster work as
// rectangles for the fragment shader. No rendered image is read back by this path.
type gpuRenderer struct {
	shader                                                                   *ebiten.Shader
	lowDetailShader                                                          *ebiten.Shader
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
	fuzzPaletteProbes                                                        int
	frame, cutouts, snapshot                                                 *ebiten.Image
	fuzzLayer                                                                *ebiten.Image
	skyLookup                                                                *ebiten.Image
	skyLookupPixels                                                          []byte
	width, height                                                            int
	spriteSpans                                                              []solidSpan
	clipChanges                                                              []bool
	baseCommands, skyCommands, cutoutCommands, fuzzCommands, overlayCommands gpuCommands
	fuzzLogicalCommands                                                      gpuCommands
	orderedCutouts                                                           bool
	cutoutPasses                                                             []gpuCutoutPass
	cutoutPassesUsed                                                         int
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
	if !g.opts.GPURenderer {
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
		lowDetail, err := ebiten.NewShader(lowDetailShaderSrc)
		if err != nil {
			fmt.Printf("GPU renderer unavailable, using CPU: %v\n", err)
			r.failed = true
			return
		}
		r.lowDetailShader = lowDetail
		r.textures = make(map[gpuTextureKey]gpuTexture)
		r.converted = make(map[gpuTextureKey]gpuTexture)
		r.metadataPixels = make([]byte, 256*256*4)
		r.metadata = newUnmanagedImage(256, 256)
	}
	if r.width != g.viewW || r.height != g.viewH {
		for _, img := range []*ebiten.Image{r.frame, r.cutouts, r.snapshot, r.fuzzLayer} {
			if img != nil {
				img.Deallocate()
			}
		}
		r.width, r.height = g.viewW, g.viewH
		r.frame = newUnmanagedImage(r.width, r.height)
		r.cutouts = newUnmanagedImage(r.width, r.height)
		r.snapshot = newUnmanagedImage(r.width, r.height)
		r.fuzzLayer = newUnmanagedImage(min(r.width, doomLogicalW), min(r.height, doomLogicalH))
	}
	r.baseCommands.reset()
	r.skyCommands.reset()
	r.cutoutCommands.reset()
	r.fuzzCommands.reset()
	r.fuzzLogicalCommands.reset()
	r.orderedCutouts = false
	r.cutoutPassesUsed = 0
	r.overlayCommands.reset()
	r.updateLights(g.opts.DoomPaletteRGBA, g.opts.DoomColorMap)
	g.gpuFrame = r
}

func (c *gpuCommands) reset() {
	c.used = 0
	for i := range c.batches {
		c.batches[i].vertices = c.batches[i].vertices[:0]
		c.batches[i].indices = c.batches[i].indices[:0]
	}
}

func (r *gpuRenderer) updateLights(palette, colormap []byte) {
	rows := doomColormapRowCount()
	if r.lights != nil && r.lightGamma == activeGammaLevel && r.lightRows == rows {
		return
	}
	// Keep the original unshaded row, then append the indexed COLORMAP so
	// fuzz feedback can apply repeated remaps without recovering RGB again.
	h := 258 + max(rows, 1) + rows
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
		copy(r.lightPixels[(257+max(rows, 1))*256*4:], palette[:256*4])
	}
	// Two RGB texels per texture leave the lower half of the metadata image
	// for the RGB 5-bit palette lookup used by the spectre's background snapshot.
	for i, index := range doomPalIndexLUT32 {
		r.metadataPixels[(gpuPaletteLookupOffset+i)*4+2] = index
		r.metadataPixels[(gpuPaletteLookupOffset+i)*4+3] = 255
	}
	// The lookup region's R/G channels hold a small exact active-palette hash.
	// This matters at higher gamma: quantizing RGB can select a different WAD
	// index and change the spectre's classic row-six COLORMAP shading.
	r.fuzzPaletteProbes = putGPUFuzzPalette(r.metadataPixels, wallShadePackedLUT[256][:])
	for row := 0; row < rows; row++ {
		for index := 0; index < 256; index++ {
			i := row*256 + index
			mapped := index
			if i < len(colormap) {
				mapped = int(colormap[i])
			}
			p := ((258+max(rows, 1)+row)*256 + index) * 4
			r.lightPixels[p], r.lightPixels[p+3] = byte(mapped), 255
		}
	}
	r.metadataDirty = true
	r.lights.WritePixels(r.lightPixels)
	r.lightGamma, r.lightRows = activeGammaLevel, rows
}

const gpuFuzzPaletteBuckets = 1024

func fuzzPaletteHash(p uint32) int {
	return (int(byte(p>>pixelRShift))*3 + int(byte(p>>pixelGShift))*5 + int(byte(p>>pixelBShift))*7) % gpuFuzzPaletteBuckets
}

func putGPUFuzzPalette(pixels []byte, palette []uint32) int {
	for bucket := 0; bucket < gpuFuzzPaletteBuckets; bucket++ {
		i := (gpuPaletteLookupOffset + bucket) * 4
		pixels[i], pixels[i+1], pixels[i+3] = 0, 0, 255
	}
	probes := 1
	for index, color := range palette {
		bucket := fuzzPaletteHash(color)
		for probe := 1; probe <= gpuFuzzPaletteBuckets; probe++ {
			i := (gpuPaletteLookupOffset + bucket) * 4
			if pixels[i+1] == 0 || palette[int(pixels[i])] == color {
				pixels[i], pixels[i+1] = byte(index), 255
				probes = max(probes, probe)
				break
			}
			bucket = (bucket + 1) % gpuFuzzPaletteBuckets
		}
	}
	return probes
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
	putGPUTextureMetadata(r.metadataPixels, tex)
	r.metadataDirty = true
	r.textures[key] = tex
	return tex, true
}

func putGPUTextureMetadata(pixels []byte, tex gpuTexture) {
	// Atlas origins and dimensions minus one each fit in 11 bits. Store one
	// origin/dimension pair per RGB texel, with opaque alpha as in the atlas.
	for n, packed := range [2]int{tex.x | (tex.width-1)<<11, tex.y | (tex.height-1)<<11} {
		i := (tex.id*2 + n) * 4
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(packed), byte(packed>>8), byte(packed>>16), 255
	}
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
	vertex := ebiten.Vertex{SrcX: float32(x0), SrcY: float32(y0), ColorR: float32(light), ColorG: float32(tex.id + mode*gpuModeStride), ColorB: float32(other.id), ColorA: float32(alpha) / 255, Custom0: u, Custom1: v, Custom2: du, Custom3: dv}
	if alpha == 0 && mode != 6 && mode != 7 && mode != 11 && mode != 12 {
		// An unblended texture needs no second ID or blend fraction. Carry its
		// two exact 22-bit metadata pairs in those existing vertex channels,
		// avoiding metadata image reads in the fragment shader. Negative alpha
		// identifies this encoding; special passes retain their original form.
		vertex.ColorG = float32(tex.x | (tex.width-1)<<11)
		vertex.ColorB = float32(tex.y | (tex.height-1)<<11)
		vertex.ColorA = -float32(mode + 1)
	}
	// Wall sampling depends only on Y within a column. Adjacent columns with
	// the same texel, light and Y mapping can share a rectangle. Solid colors
	// and sky copies likewise need only the outer bounds of an adjacent run.
	positionIndependent := mode == 6 || mode == 7 || mode == 10 || mode == 11 || mode == 12
	if b != nil && b.page == tex.page && b.otherPage == other.page && len(b.vertices) >= 4 && (mode == 1 || positionIndependent) {
		last := b.vertices[len(b.vertices)-4:]
		previous := last[0]
		previous.DstX, previous.DstY = 0, 0
		previous.SrcX = vertex.SrcX
		if positionIndependent {
			previous.SrcY = vertex.SrcY
		}
		if previous == vertex {
			if last[1].DstX == float32(x0) && last[0].DstY == float32(y0) && last[2].DstY == float32(y1+1) {
				last[1].DstX, last[3].DstX = float32(x1+1), float32(x1+1)
				return
			}
			if positionIndependent && last[2].DstY == float32(y0) && last[0].DstX == float32(x0) && last[1].DstX == float32(x1+1) {
				last[2].DstY, last[3].DstY = float32(y1+1), float32(y1+1)
				return
			}
		}
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
			r.rect(r.opaqueCutoutCommands(), x, span.L, x, span.R, tex, other, 1, light, alpha, float32(tx), gpuWrapFixed(v, tex.height), 0, gpuWrapFixed(step, tex.height))
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
	if it.shadow && !it.debugOverlay {
		g.gpuSpectreFuzz(it, commands)
		return
	}
	tex, ok := r.wallTexture(it.tex)
	if !ok {
		return
	}
	scaleY := it.scale
	if it.scaleY > 0 {
		scaleY = it.scaleY
	}
	light := min(int(it.shadeMul), 256)
	if doomColormapEnabled {
		light = gpuLightRow(light, doomColormapRowForShade(it.shadeMul))
	}
	if it.shadow {
		light = gpuLightRow(doomShadeMulFromRow(6), 6)
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		light = 257 + row
	}
	if unshaded {
		light = 257 + max(r.lightRows, 1)
		if !g.opts.SourcePortMode {
			light = 256
		}
	}
	mode := 2
	if it.flip {
		mode = 3
	}
	if it.debugOverlay {
		mode = 8
		if it.flip {
			mode = 9
		}
	}
	du, dv := float32(1/it.scale), float32(1/scaleY)
	runY := it.y0
	runV := float32((float64(runY) + 0.5 - it.dstY) / scaleY)
	previous := r.spriteSpans[:0]
	clipChanges := r.spriteClipChanges(g, it.x0, it.x1, it.y0, it.y1, it.depthQ)
	var spans []solidSpan
	emit := func(y1 int) {
		for _, span := range previous {
			u := float32((float64(span.L) + 0.5 - it.dstX) / it.scale)
			r.rect(commands, span.L, runY, span.R, y1, tex, tex, mode, light, 0, u, runV, du, dv)
		}
	}
	for y := it.y0; y <= it.y1; y++ {
		if clipChanges[y] {
			spans = g.spriteRowVisibleSpansDepthQ(y, it.x0, it.x1, it.depthQ, it.clipSpans, g.solidClipScratch[:0])
			g.solidClipScratch = spans
		}
		v := float32((float64(y) + 0.5 - it.dstY) / scaleY)
		// Keep the row-by-row path's rounded texel selection. Most affine runs
		// fit in one quad; split at visibility changes or rounding boundaries.
		if y == it.y0 || !slices.Equal(spans, previous) || gpuSpriteTexel(runV+float32(y-runY)*dv, tex.height) != gpuSpriteTexel(v, tex.height) {
			if y != it.y0 {
				emit(y - 1)
			}
			runY, runV = y, v
			previous = append(previous[:0], spans...)
		}
	}
	emit(it.y1)
	r.spriteSpans = previous
}

func (g *game) gpuSpectreFuzz(it cutoutItem, commands *gpuCommands) {
	r := g.gpuFrame
	light := gpuLightRow(doomShadeMulFromRow(6), 6)
	if doomLightingEnabled && g.opts.DoomColorMapRows > 6 {
		light = 257 + 6
	}
	if row, ok := g.playerFixedColormapRow(); ok {
		light = 257 + row
	}
	g.walkSpectreFuzzSpans(it, func(span spectreFuzzSpan) {
		logicalH := min(g.viewH, doomLogicalH)
		r.rect(r.logicalFuzzCommands(), span.cx, span.cy, span.cx, span.y1*logicalH/g.viewH,
			gpuTexture{}, gpuTexture{}, 11, light, 0, float32(span.cx), float32(span.cy), float32(span.phase), 0)
		g.clipSpectreFuzzSpan(it, span, func(x, y0, y1 int) {
			// Presentation keeps native clipping and merges adjacent columns;
			// expensive sampling and palette work happen only on the small layer.
			r.rect(commands, x, y0, x, y1, gpuTexture{}, gpuTexture{}, 12, 0, 0, 0, 0, 0, 0)
		})
	})
}

// Horizontal visibility spans can change only at a nearer wall's top/bottom
// or a portal's opening bounds. Scan columns once, then reuse each row's spans
// until one of those boundaries is crossed. Occlusion buffers are immutable
// while drawing a sprite or building its plane occluders.
func (r *gpuRenderer) spriteClipChanges(g *game, x0, x1, y0, y1 int, depthQ uint16) []bool {
	if len(r.clipChanges) != g.viewH {
		r.clipChanges = make([]bool, g.viewH)
	}
	changes := r.clipChanges
	clear(changes[y0 : y1+1])
	changes[y0] = true
	if !g.billboardClippingEnabled() {
		return changes
	}
	width := len(g.wallDepthQCol)
	if len(g.wallDepthTopCol) != width || len(g.wallDepthBottomCol) != width || len(g.wallDepthClosedCol) != width || len(g.maskedClipCols) != width || len(g.maskedClipFirstDepthQ) != width {
		// Retain the generic row path's behavior for incomplete buffers.
		for y := y0 + 1; y <= y1; y++ {
			changes[y] = true
		}
		return changes
	}
	mark := func(y int) {
		if y > y0 && y <= y1 {
			changes[y] = true
		}
	}
	for x := max(x0, 0); x <= min(x1, width-1); x++ {
		if depthQ > g.wallDepthQCol[x] && !g.wallDepthClosedCol[x] {
			mark(g.wallDepthTopCol[x])
			mark(g.wallDepthBottomCol[x] + 1)
		}
		first := g.maskedClipFirstDepthQ[x]
		if first == 0 || depthQ <= first {
			continue
		}
		for _, span := range g.maskedClipCols[x] {
			if depthQ <= span.DepthQ {
				break
			}
			if span.Closed {
				continue
			}
			if span.HasOpen {
				mark(int(span.OpenY0))
				mark(int(span.OpenY1) + 1)
			} else {
				mark(int(span.Y0))
				mark(int(span.Y1) + 1)
			}
		}
	}
	return changes
}

func gpuSpriteTexel(v float32, size int) int {
	return min(max(int(math.Floor(float64(v))), 0), size-1)
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
	r.drawCommandsSized(dst, commands, blend, background, r.width, r.height)
}

func (r *gpuRenderer) drawCommandsSized(dst *ebiten.Image, commands *gpuCommands, blend ebiten.Blend, background *ebiten.Image, width, height int) {
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
		options := ebiten.DrawTrianglesShaderOptions{Blend: blend, Images: [4]*ebiten.Image{atlas, r.lights, r.metadata, other}, Uniforms: map[string]any{"ViewSize": []float32{float32(width), float32(height)}, "FuzzSourceSize": []float32{float32(r.width), float32(r.height)}, "FuzzShade": fuzzShade, "FuzzOffsets": gpuFuzzOffsets[:], "FuzzPaletteProbes": float32(r.fuzzPaletteProbes), "FuzzColormapOffset": float32(258 + max(r.lightRows, 1))}}
		dst.DrawTrianglesShader(b.vertices, b.indices, r.shader, &options)
	}
}

func (g *game) finishGPUFrame(dst *ebiten.Image, camAng, focal float64) {
	r := g.gpuFrame
	var skyBackground *ebiten.Image
	if !g.opts.SourcePortMode {
		r.updateSkyLookup(g.frameSkyColU, g.frameSkyRowV)
		skyBackground = r.skyLookup
	}
	if r.metadataDirty {
		r.metadata.WritePixels(r.metadataPixels)
		r.metadataDirty = false
	}
	background := color.RGBA{A: 255}
	if !g.opts.SourcePortMode && wallShadePackedOK {
		p := wallShadePackedLUT[256][0]
		background.R, background.G, background.B = byte(p>>pixelRShift), byte(p>>pixelGShift), byte(p>>pixelBShift)
	}
	r.frame.Fill(background)
	r.snapshot.Clear()
	if r.skyCommands.used > 0 {
		if key, tex, ok := g.runtimeSkyTextureEntryForMap(g.m.Name); ok {
			g.initSkyLayerShader()
			if g.enableSkyLayerFrame(camAng, focal, key, tex, effectiveSkyTexHeight(tex)) {
				g.drawSkyLayerFrame(r.snapshot)
			}
		}
	}
	if g.opts.SourcePortMode {
		skyBackground = r.snapshot
	}
	r.drawCommands(r.frame, &r.skyCommands, ebiten.BlendSourceOver, skyBackground, g.worldTic)
	r.drawCommands(r.frame, &r.baseCommands, ebiten.BlendSourceOver, nil, g.worldTic)
	r.drawSceneCutoutPasses()
	r.drawCommands(r.frame, &r.overlayCommands, ebiten.BlendSourceOver, nil, g.worldTic)
	if g.lowDetailMode() {
		dst.DrawRectShader(r.width, r.height, r.lowDetailShader, &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{r.frame}})
	} else {
		dst.DrawImage(r.frame, nil)
	}
}

func (r *gpuRenderer) drawSpectreFuzz(dst *ebiten.Image) {
	r.drawFuzzCommands(dst, &r.fuzzLogicalCommands, &r.fuzzCommands)
}

func (r *gpuRenderer) drawFuzzCommands(dst *ebiten.Image, logical, color *gpuCommands) {
	r.fuzzLayer.Clear()
	r.drawCommandsSized(r.fuzzLayer, logical, ebiten.BlendSourceOver, r.snapshot,
		r.fuzzLayer.Bounds().Dx(), r.fuzzLayer.Bounds().Dy())
	r.drawCommands(dst, color, ebiten.BlendSourceOver, r.fuzzLayer, 0)
}

func (r *gpuRenderer) nextCutoutPass(fuzz bool) *gpuCutoutPass {
	if r.cutoutPassesUsed == len(r.cutoutPasses) {
		r.cutoutPasses = append(r.cutoutPasses, gpuCutoutPass{})
	}
	p := &r.cutoutPasses[r.cutoutPassesUsed]
	r.cutoutPassesUsed++
	p.color.reset()
	p.logical.reset()
	p.fuzz = fuzz
	return p
}

func (r *gpuRenderer) opaqueCutoutCommands() *gpuCommands {
	if !r.orderedCutouts {
		return &r.cutoutCommands
	}
	if r.cutoutPassesUsed > 0 {
		p := &r.cutoutPasses[r.cutoutPassesUsed-1]
		if !p.fuzz {
			return &p.color
		}
	}
	return &r.nextCutoutPass(false).color
}

func (r *gpuRenderer) startFuzzCutout() *gpuCommands {
	if !r.orderedCutouts {
		return &r.fuzzCommands
	}
	// Each spectre has its own snapshot, including any farther spectres.
	return &r.nextCutoutPass(true).color
}

func (r *gpuRenderer) logicalFuzzCommands() *gpuCommands {
	if r.orderedCutouts && r.cutoutPassesUsed > 0 {
		return &r.cutoutPasses[r.cutoutPassesUsed-1].logical
	}
	return &r.fuzzLogicalCommands
}

func (r *gpuRenderer) drawSceneCutoutPasses() {
	if r.orderedCutouts {
		for i := 0; i < r.cutoutPassesUsed; i++ {
			p := &r.cutoutPasses[i]
			if !p.fuzz {
				r.drawCommands(r.frame, &p.color, ebiten.BlendSourceOver, nil, 0)
				continue
			}
			if p.color.used == 0 {
				continue
			}
			r.snapshot.Clear()
			r.snapshot.DrawImage(r.frame, nil)
			r.drawFuzzCommands(r.frame, &p.logical, &p.color)
		}
		return
	}
	r.cutouts.Clear()
	r.drawCommands(r.cutouts, &r.cutoutCommands, ebiten.BlendDestinationOver, nil, 0)
	r.frame.DrawImage(r.cutouts, nil)
	if r.fuzzCommands.used > 0 {
		r.snapshot.Clear()
		r.snapshot.DrawImage(r.frame, nil)
		r.drawSpectreFuzz(r.frame)
	}
}

// Keep Faithful sky sampling identical to the CPU's column/row lookups.
// Only the lookup coordinates are uploaded; the sky texture stays in its atlas.
func (r *gpuRenderer) updateSkyLookup(columns, rows []int) {
	w := max(len(columns), len(rows))
	if w == 0 {
		return
	}
	if r.skyLookup == nil || r.skyLookup.Bounds().Dx() != w {
		if r.skyLookup != nil {
			r.skyLookup.Deallocate()
		}
		r.skyLookup = newUnmanagedImage(w, 2)
		r.skyLookupPixels = make([]byte, w*2*4)
	}
	for y, coords := range [][]int{columns, rows} {
		for x, coord := range coords {
			i := (y*w + x) * 4
			r.skyLookupPixels[i], r.skyLookupPixels[i+1], r.skyLookupPixels[i+3] = byte(coord), byte(coord>>8), 255
		}
	}
	r.skyLookup.WritePixels(r.skyLookupPixels)
}
