//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	"math"
	"sort"

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
	patches                                     map[presentationTextureKey]rl.Texture2D
	skyShader                                   rl.Shader
	yawLocation, viewportLocation, sizeLocation int32
	originLocation                              int32
	fuzz                                        *spectreFuzz
	ordered                                     []int
	instances                                   map[int]*Batch
	sceneWidth, sceneHeight                     int
}

// SetSceneSize supplies the snapshot extent for a reduced top-left viewport.
// Sprite depth and coverage still use the window's multisampled framebuffer.
func (p *Presentation) SetSceneSize(width, height int) { p.sceneWidth, p.sceneHeight = width, height }

type presentationTextureKey struct {
	TextureKey
	Padded bool // HUD/weapon patches have a gutter; panoramic skies repeat.
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
	return &Presentation{Sprites: sprites, patches: make(map[presentationTextureKey]rl.Texture2D), skyShader: shader,
		yawLocation: rl.GetShaderLocation(shader, "yaw"), viewportLocation: rl.GetShaderLocation(shader, "viewport"), sizeLocation: rl.GetShaderLocation(shader, "skySize"), originLocation: rl.GetShaderLocation(shader, "originY")}, nil
}

func (p *Presentation) texture(tex levelmesh.Texture, padded bool) (rl.Texture2D, bool) {
	if tex.Width <= 0 || tex.Height <= 0 || len(tex.RGBA) != tex.Width*tex.Height*4 {
		return rl.Texture2D{}, false
	}
	key := presentationTextureKey{TextureKey: TextureKey{Pixels: &tex.RGBA[0], Width: tex.Width, Height: tex.Height}, Padded: padded}
	if cached, ok := p.patches[key]; ok {
		return cached, true
	}
	if padded {
		tex = padPatchTexture(tex)
	}
	img := rl.NewImage(tex.RGBA, int32(tex.Width), int32(tex.Height), 1, rl.UncompressedR8g8b8a8)
	texture := rl.LoadTextureFromImage(img)
	rl.SetTextureFilter(texture, rl.FilterPoint)
	if padded {
		rl.SetTextureWrap(texture, rl.WrapClamp)
	}
	p.patches[key] = texture
	return texture, true
}

func (p *Presentation) DrawSky(tex levelmesh.Texture, c levelmesh.Camera, width, height int) {
	t, ok := p.texture(tex, false)
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
	p.SyncSpritesViewport(sprites, c, rl.GetScreenWidth(), rl.GetScreenHeight())
}

func (p *Presentation) SyncSpritesViewport(sprites []levelmesh.Sprite, c levelmesh.Camera, width, height int) {
	p.triangles = SpriteTriangles(p.triangles, sprites, c.Yaw)
	p.ordered = p.ordered[:0]
	// Visibility of a shadow triggers ordered rendering. Normal scenes retain
	// their shared texture batches; invisible spectres add no snapshot work.
	shadow := false
	for _, s := range sprites {
		if s.Shadow {
			_, _, _, _, _, visible := levelmesh.ProjectSprite(s, c, width, height)
			shadow = shadow || visible
		}
	}
	if shadow {
		for i, s := range sprites {
			_, _, _, _, _, visible := levelmesh.ProjectSprite(s, c, width, height)
			if visible {
				p.ordered = append(p.ordered, i)
			}
		}
		sort.SliceStable(p.ordered, func(i, j int) bool {
			a, b := sprites[p.ordered[i]], sprites[p.ordered[j]]
			ca, sa := math.Cos(c.Yaw), math.Sin(c.Yaw)
			return (a.X-c.X)*ca+(a.Y-c.Y)*sa > (b.X-c.X)*ca+(b.Y-c.Y)*sa
		})
		for i := range p.triangles {
			p.triangles[i].Instance = p.triangles[i].Sector + 1
			if p.triangles[i].Kind == levelmesh.ShadowBillboard {
				snapFuzzBounds(&p.triangles[i], sprites[p.triangles[i].Sector], c, width, height)
			}
		}
	}
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
	if len(p.ordered) > 0 {
		if p.instances == nil {
			p.instances = make(map[int]*Batch)
		}
		clear(p.instances)
		for _, b := range p.Sprites.active {
			p.instances[b.Key.Instance-1] = b
		}
	}
}

// DrawSprites preserves back-to-front cutout ordering when a spectre needs a
// snapshot of the already drawn world. The final cards still use hardware depth.
func (p *Presentation) DrawSprites(c levelmesh.Camera, width, height int, fuzz func(int, int, int) ([]levelmesh.FuzzSpan, levelmesh.FuzzColors)) error {
	if len(p.ordered) == 0 || fuzz == nil {
		p.Sprites.fuzzTexture = rl.Texture2D{}
		p.Sprites.Draw(c, width, height, levelmesh.Textured)
		return nil
	}
	for _, index := range p.ordered {
		b := p.instances[index]
		if b == nil {
			continue
		}
		p.Sprites.fuzzTexture = rl.Texture2D{}
		if len(b.LightingUVs) > 0 && b.LightingUVs[0] == 7 {
			spans, colors := fuzz(index, width, height)
			if len(spans) == 0 {
				continue
			}
			if p.fuzz == nil {
				var err error
				p.fuzz, err = newSpectreFuzz()
				if err != nil {
					return err
				}
			}
			if err := p.fuzz.draw(p, spans, colors, width, height); err != nil {
				return err
			}
			p.Sprites.fuzzTexture = p.fuzz.layer.Texture
		}
		rl.DrawRenderBatchActive()
		rl.Viewport(0, int32(rl.GetRenderHeight()-height), int32(width), int32(height))
		one := [1]*Batch{b}
		p.Sprites.drawBatches(c, width, height, levelmesh.Textured, one[:])
	}
	p.Sprites.fuzzTexture = rl.Texture2D{}
	return nil
}

func (p *Presentation) drawPatches(patches []levelmesh.Patch, x, y, sx, sy float64) {
	for _, patch := range patches {
		t, ok := p.texture(patch.Texture, true)
		if !ok {
			continue
		}
		// Draw only the original artwork; the transparent upload gutter does
		// not change patch offsets, animation alignment, or on-screen size.
		tint := rl.White
		if patch.Alpha > 0 && patch.Alpha < 1 {
			tint = rl.Fade(tint, float32(patch.Alpha))
		}
		rl.DrawTexturePro(t, rl.NewRectangle(1, 1, float32(patch.Texture.Width), float32(patch.Texture.Height)), rl.NewRectangle(float32(x+patch.X*sx), float32(y+patch.Y*sy), float32(patch.W*sx), float32(patch.H*sy)), rl.Vector2{}, 0, tint)
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
	p.DrawHUDLayout(patches, width, height, 0, 1)
}

// DrawHUDLayout follows the shared status bar's overlay size and visibility.
func (p *Presentation) DrawHUDLayout(patches []levelmesh.Patch, width, height, mode int, hudScale float64) {
	if mode == 2 {
		return
	}
	scale := math.Min(float64(width)/320, float64(height)/240)
	if mode == 1 {
		scale = math.Min(float64(width)/320, hudScale)
	}
	sx, sy := scale, scale*1.2
	y := float64(height) - 32*sy
	if mode == 0 {
		rl.DrawRectangle(0, int32(math.Floor(y)), int32(width), int32(math.Ceil(32*sy)), rl.Black)
	}
	p.drawPatches(patches, (float64(width)-320*sx)/2, y, sx, sy)
}

// DrawUI uses the main frontend's centered 320x200 layout. Menus use square
// screen pixels; the 1.2 world/weapon aspect correction does not apply here.
func (p *Presentation) DrawUI(patches []levelmesh.Patch, width, height int) {
	scale, x, y := MenuTransform(width, height)
	rl.BeginScissorMode(int32(math.Round(x)), int32(math.Round(y)), int32(math.Round(320*scale)), int32(math.Round(200*scale)))
	p.drawPatches(patches, x, y, scale, scale)
	rl.EndScissorMode()
}

func MenuTransform(width, height int) (scale, x, y float64) {
	scale = math.Min(float64(width)/320, float64(height)/200)
	x, y = (float64(width)-320*scale)/2, (float64(height)-200*scale)/2
	return
}

// Title artwork fills the source-port window, as in the main host.
func (p *Presentation) DrawTitle(tex levelmesh.Texture, width, height int) {
	p.drawPatches([]levelmesh.Patch{{Texture: tex, W: 320, H: 200}}, 0, 0, float64(width)/320, float64(height)/200)
}

// DrawScreenPatches draws map sprites in physical viewport coordinates.
func (p *Presentation) DrawScreenPatches(patches []levelmesh.Patch) {
	p.drawPatches(patches, 0, 0, 1, 1)
}

func (p *Presentation) Close() {
	if p.fuzz != nil {
		p.fuzz.close()
	}
	p.Sprites.Close()
	for _, tex := range p.patches {
		rl.UnloadTexture(tex)
	}
	rl.UnloadShader(p.skyShader)
}
