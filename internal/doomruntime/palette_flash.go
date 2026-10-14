package doomruntime

import (
	"gddoom/internal/render/hud"
	"github.com/hajimehoshi/ebiten/v2"
)

type paletteFlashState struct {
	tables map[int][]byte
	images map[int]*ebiten.Image
	shader *ebiten.Shader
	source *ebiten.Image
	pixels []byte
}

func (g *game) flashPaletteTable() []byte {
	index := hud.FlashPaletteIndex(g.statusDamageCount, g.statusBonusCount, g.inventory.StrengthCount, g.inventory.RadSuitTics)
	if index == 0 || index >= len(g.opts.DoomPalettesRGBA) {
		return nil
	}
	if g.paletteFlash.tables == nil {
		g.paletteFlash.tables = make(map[int][]byte)
	}
	if table, ok := g.paletteFlash.tables[index]; ok {
		return table
	}
	table := hud.PaletteChannels(g.opts.DoomPaletteRGBA, g.opts.DoomPalettesRGBA[index])
	g.paletteFlash.tables[index] = table
	return table
}

const paletteFlashShader = `//kage:unit pixels
package main
func Fragment(dst vec4, pos vec2, color vec4) vec4 {
    c:=imageSrc0At(pos)
    rgb:=floor(c.rgb*255+0.5)
    o:=imageSrc0Origin()+vec2(0.5,0.5)
    return vec4(imageSrc1UnsafeAt(o+vec2(rgb.r,0)).r,imageSrc1UnsafeAt(o+vec2(rgb.g,0)).g,imageSrc1UnsafeAt(o+vec2(rgb.b,0)).b,c.a)
}`

func (g *game) drawPaletteFlash(screen *ebiten.Image) bool {
	table := g.flashPaletteTable()
	if len(table) == 0 {
		return false
	}
	state := &g.paletteFlash
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if !g.opts.KageShader {
		if len(state.pixels) != w*h*4 {
			state.pixels = make([]byte, w*h*4)
		}
		screen.ReadPixels(state.pixels)
		for i := 0; i < len(state.pixels); i += 4 {
			for c := 0; c < 3; c++ {
				state.pixels[i+c] = table[int(state.pixels[i+c])*4+c]
			}
		}
		screen.WritePixels(state.pixels)
		return true
	}
	if state.shader == nil {
		var err error
		state.shader, err = ebiten.NewShader([]byte(paletteFlashShader))
		if err != nil {
			panic(err)
		}
	}
	if state.source == nil || state.source.Bounds().Dx() != w || state.source.Bounds().Dy() != h {
		if state.source != nil {
			state.source.Dispose()
		}
		state.source = ebiten.NewImage(w, h)
	}
	index := hud.FlashPaletteIndex(g.statusDamageCount, g.statusBonusCount, g.inventory.StrengthCount, g.inventory.RadSuitTics)
	if state.images == nil {
		state.images = make(map[int]*ebiten.Image)
	}
	lut := state.images[index]
	if lut == nil {
		lut = ebiten.NewImage(256, 1)
		lut.WritePixels(table)
		state.images[index] = lut
	}
	state.source.Clear()
	state.source.DrawImage(screen, nil)
	vertices := []ebiten.Vertex{{DstX: 0, DstY: 0, SrcX: 0, SrcY: 0}, {DstX: float32(w), DstY: 0, SrcX: float32(w), SrcY: 0}, {DstX: 0, DstY: float32(h), SrcX: 0, SrcY: float32(h)}, {DstX: float32(w), DstY: float32(h), SrcX: float32(w), SrcY: float32(h)}}
	screen.DrawTrianglesShader(vertices, []uint16{0, 1, 2, 1, 3, 2}, state.shader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{state.source, lut}, Blend: ebiten.BlendCopy})
	return true
}
