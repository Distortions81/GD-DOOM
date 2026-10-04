//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	"math"

	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const skyFragmentShader = `#version 330
uniform sampler2D texture0;
uniform float yaw;
uniform vec2 viewport;
uniform vec2 skySize;
uniform float originY;
out vec4 finalColor;
void main() {
    vec2 p = gl_FragCoord.xy;
    p.y -= originY;
    float angle = yaw + atan((viewport.x*0.5-p.x)/(viewport.x*0.5));
    float u = angle*4.0/6.28318530718;
    float v = (100.0+(viewport.y*0.5-p.y)*320.0/viewport.x)/skySize.y;
    finalColor = vec4(texture(texture0,vec2(u,v)).rgb,1.0);
}`

type Presentation struct {
	Sprites                                     *Renderer
	triangles                                   []levelmesh.Triangle
	patches                                     map[TextureKey]rl.Texture2D
	skyShader                                   rl.Shader
	yawLocation, viewportLocation, sizeLocation int32
	originLocation                              int32
}

func NewPresentation(options TextureOptions) (*Presentation, error) {
	sprites, err := NewRendererWithTextureOptions(options)
	if err != nil {
		return nil, err
	}
	shader := rl.LoadShaderFromMemory("", skyFragmentShader)
	if !rl.IsShaderValid(shader) || shader.ID == rl.GetShaderIdDefault() {
		sprites.Close()
		return nil, fmt.Errorf("Raylib sky shader could not compile")
	}
	return &Presentation{Sprites: sprites, patches: make(map[TextureKey]rl.Texture2D), skyShader: shader,
		yawLocation: rl.GetShaderLocation(shader, "yaw"), viewportLocation: rl.GetShaderLocation(shader, "viewport"), sizeLocation: rl.GetShaderLocation(shader, "skySize"), originLocation: rl.GetShaderLocation(shader, "originY")}, nil
}

func (p *Presentation) texture(tex levelmesh.Texture) (rl.Texture2D, bool) {
	if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return rl.Texture2D{}, false
	}
	key := TextureKey{Pixels: &tex.RGBA[0], Width: tex.Width, Height: tex.Height}
	if cached, ok := p.patches[key]; ok {
		return cached, true
	}
	img := rl.NewImage(tex.RGBA, int32(tex.Width), int32(tex.Height), 1, rl.UncompressedR8g8b8a8)
	texture := rl.LoadTextureFromImage(img)
	rl.SetTextureFilter(texture, rl.FilterPoint)
	p.patches[key] = texture
	return texture, true
}

func (p *Presentation) DrawSky(tex levelmesh.Texture, c levelmesh.Camera, width, height int) {
	t, ok := p.texture(tex)
	if !ok {
		return
	}
	rl.SetTextureWrap(t, rl.WrapRepeat)
	rl.SetTextureFilter(t, rl.FilterBilinear)
	rl.SetShaderValue(p.skyShader, p.yawLocation, []float32{float32(c.Yaw)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(p.skyShader, p.viewportLocation, []float32{float32(width), float32(height)}, rl.ShaderUniformVec2)
	rl.SetShaderValue(p.skyShader, p.sizeLocation, []float32{float32(tex.Width), float32(tex.Height)}, rl.ShaderUniformVec2)
	rl.SetShaderValue(p.skyShader, p.originLocation, []float32{float32(rl.GetScreenHeight() - height)}, rl.ShaderUniformFloat)
	rl.BeginShaderMode(p.skyShader)
	rl.DrawTexturePro(t, rl.NewRectangle(0, 0, float32(t.Width), float32(t.Height)), rl.NewRectangle(0, 0, float32(width), float32(height)), rl.Vector2{}, 0, rl.White)
	rl.EndShaderMode()
}

func (p *Presentation) SyncSprites(sprites []levelmesh.Sprite, c levelmesh.Camera) {
	p.triangles = SpriteTriangles(p.triangles, sprites, c.Yaw)
	p.Sprites.Sync(p.triangles, func(t levelmesh.Triangle) levelmesh.Texture { return sprites[t.Sector].Texture }, func(sector int) float64 {
		s := sprites[sector]
		if s.Shadow {
			return 0.3
		}
		if s.Fullbright {
			return 1
		}
		return s.Light
	}, levelmesh.Textured)
}

func (p *Presentation) drawPatches(patches []levelmesh.Patch, x, y, sx, sy float64) {
	for _, patch := range patches {
		t, ok := p.texture(patch.Texture)
		if !ok {
			continue
		}
		rl.DrawTexturePro(t, rl.NewRectangle(0, 0, float32(t.Width), float32(t.Height)), rl.NewRectangle(float32(x+patch.X*sx), float32(y+patch.Y*sy), float32(patch.W*sx), float32(patch.H*sy)), rl.Vector2{}, 0, rl.White)
	}
}

// HUDHeight fits Doom's 320x32 bar without stretching it on widescreen windows.
func HUDHeight(width, height int) int {
	scale := math.Min(float64(width)/320, float64(height)/240)
	return int(math.Round(32 * scale * 1.2))
}

func (p *Presentation) DrawWeapon(patches []levelmesh.Patch, width, viewHeight int) {
	scale := math.Min(float64(width)/320, float64(viewHeight)/240)
	sx, sy := scale, scale*1.2
	x := (float64(width) - 320*sx) / 2
	y := float64(viewHeight) - 200*sy
	rl.BeginScissorMode(0, 0, int32(width), int32(viewHeight))
	p.drawPatches(patches, x, y, sx, sy)
	rl.EndScissorMode()
}

func (p *Presentation) DrawHUD(patches []levelmesh.Patch, width, height int) {
	scale := math.Min(float64(width)/320, float64(height)/240)
	sx, sy := scale, scale*1.2
	y := float64(height) - 32*sy
	rl.DrawRectangle(0, int32(math.Floor(y)), int32(width), int32(math.Ceil(32*sy)), rl.Black)
	p.drawPatches(patches, (float64(width)-320*sx)/2, y, sx, sy)
}

func (p *Presentation) Close() {
	p.Sprites.Close()
	for _, tex := range p.patches {
		rl.UnloadTexture(tex)
	}
	rl.UnloadShader(p.skyShader)
}
