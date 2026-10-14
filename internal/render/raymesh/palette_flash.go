//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const paletteFlashFragment = `#version 330
uniform sampler2D texture0;
uniform sampler2D channelTable;
in vec2 fragTexCoord;
out vec4 finalColor;
void main() {
 vec4 c=texture(texture0,fragTexCoord);
 ivec3 rgb=ivec3(floor(c.rgb*255.0+vec3(0.5)));
 finalColor=vec4(texelFetch(channelTable,ivec2(rgb.r,0),0).r,texelFetch(channelTable,ivec2(rgb.g,0),0).g,texelFetch(channelTable,ivec2(rgb.b,0),0).b,c.a);
}`

type paletteFlash struct {
	shader rl.Shader
	source rl.RenderTexture2D
	tables map[*byte]rl.Texture2D
}

func (p *Presentation) DrawPaletteFlash(table []byte, width, height int) error {
	if len(table) != 1024 {
		return nil
	}
	if p.paletteFlash == nil {
		shader := rl.LoadShaderFromMemory("", paletteFlashFragment)
		if !rl.IsShaderValid(shader) || shader.ID == rl.GetShaderIdDefault() {
			return fmt.Errorf("palette flash shader could not compile")
		}
		p.paletteFlash = &paletteFlash{shader: shader, tables: make(map[*byte]rl.Texture2D)}
	}
	f := p.paletteFlash
	if f.source.ID == 0 || int(f.source.Texture.Width) != width || int(f.source.Texture.Height) != height {
		if f.source.ID != 0 {
			rl.UnloadRenderTexture(f.source)
		}
		f.source = rl.LoadRenderTexture(int32(width), int32(height))
	}
	if err := CopyWindowRegionToTexture(f.source, int32(width), int32(height)); err != nil {
		return err
	}
	lut, ok := f.tables[&table[0]]
	if !ok {
		lut = rl.LoadTextureFromImage(rl.NewImage(table, 256, 1, 1, rl.UncompressedR8g8b8a8))
		rl.SetTextureFilter(lut, rl.FilterPoint)
		f.tables[&table[0]] = lut
	}
	rl.BeginShaderMode(f.shader)
	rl.SetShaderValueTexture(f.shader, rl.GetShaderLocation(f.shader, "channelTable"), lut)
	rl.DrawTextureRec(f.source.Texture, rl.NewRectangle(0, 0, float32(width), -float32(height)), rl.Vector2{}, rl.White)
	rl.EndShaderMode()
	return nil
}
func (f *paletteFlash) close() {
	for _, t := range f.tables {
		rl.UnloadTexture(t)
	}
	if f.source.ID != 0 {
		rl.UnloadRenderTexture(f.source)
	}
	rl.UnloadShader(f.shader)
}
