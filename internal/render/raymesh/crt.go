//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The main renderer's shaders/crt_post.kage, translated to GLSL. All CRT
// coordinates use a top-left origin, including scanline phase and RGB masking.
const crtFragmentShader = `#version 330
uniform sampler2D texture0;
uniform float time;
uniform float originY;
out vec4 finalColor;
void main() {
    vec2 size = vec2(textureSize(texture0,0));
    vec2 uv = vec2(gl_FragCoord.x,size.y+originY-gl_FragCoord.y)/size;
    vec2 p = uv*2.0-vec2(1.0);
    p *= 1.0+0.04*dot(p,p);
    uv = (p+vec2(1.0))*0.5;
    if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) {
        finalColor = vec4(0.0,0.0,0.0,1.0);
        return;
    }
    vec4 c = texture(texture0,vec2(uv.x,1.0-uv.y));
    float scan = 0.90+0.10*sin((uv.y*size.y+time*2.0)*3.14159265);
    float phase = floor(mod(uv.x*size.x,3.0));
    vec3 mask;
    if (phase < 1.0) mask = vec3(1.00,0.88,0.88);
    else if (phase < 2.0) mask = vec3(0.88,1.00,0.88);
    else mask = vec3(0.88,0.88,1.00);
    vec2 v = uv*(1.0-uv);
    float vig = clamp(pow(v.x*v.y*20.0,0.35),0.0,1.0);
    finalColor = vec4(c.rgb*scan*mask*(0.65+0.35*vig),1.0);
}`

// CRT retains a shader and one GPU scene snapshot. The disabled path creates
// neither; Apply resolves MSAA on the GPU and never reads pixels to the CPU.
type CRT struct {
	shader         rl.Shader
	scene          rl.RenderTexture2D
	timeLocation   int32
	originLocation int32
}

// Apply processes the current window before 3D overlays or menu drawing.
// Timing follows the shared 35-Hz world clock, so pausing freezes scanlines.
func (c *CRT) Apply(worldTic int) error {
	return c.applyRegion(worldTic, int(rl.GetRenderWidth()), int(rl.GetRenderHeight()), int(rl.GetScreenWidth()), int(rl.GetScreenHeight()))
}

func (c *CRT) ApplyRegion(worldTic, width, height int) error {
	return c.applyRegion(worldTic, width, height, width, height)
}

func (c *CRT) applyRegion(worldTic, width, height, destinationWidth, destinationHeight int) error {
	if c.shader.ID == 0 {
		c.shader = rl.LoadShaderFromMemory("", crtFragmentShader)
		c.timeLocation = rl.GetShaderLocation(c.shader, "time")
		c.originLocation = rl.GetShaderLocation(c.shader, "originY")
		if !rl.IsShaderValid(c.shader) || c.timeLocation < 0 || c.originLocation < 0 {
			c.Close()
			return fmt.Errorf("could not initialize CRT shader")
		}
	}
	w, h := int32(width), int32(height)
	if c.scene.ID == 0 || c.scene.Texture.Width != w || c.scene.Texture.Height != h {
		if c.scene.ID != 0 {
			rl.UnloadRenderTexture(c.scene)
			c.scene = rl.RenderTexture2D{}
		}
		c.scene = rl.LoadRenderTexture(w, h)
		if !rl.IsRenderTextureValid(c.scene) {
			return fmt.Errorf("could not allocate CRT snapshot %dx%d", w, h)
		}
		rl.SetTextureFilter(c.scene.Texture, rl.FilterPoint)
		rl.SetTextureWrap(c.scene.Texture, rl.WrapClamp)
	}
	if err := CopyWindowRegionToTexture(c.scene, w, h); err != nil {
		return err
	}
	rl.SetShaderValue(c.shader, c.timeLocation, []float32{float32(worldTic) / 35}, rl.ShaderUniformFloat)
	rl.SetShaderValue(c.shader, c.originLocation, []float32{float32(rl.GetRenderHeight()) - float32(h)}, rl.ShaderUniformFloat)
	rl.BeginShaderMode(c.shader)
	rl.DrawTexturePro(c.scene.Texture, rl.NewRectangle(0, 0, float32(w), -float32(h)),
		rl.NewRectangle(0, 0, float32(destinationWidth), float32(destinationHeight)), rl.Vector2{}, 0, rl.White)
	rl.EndShaderMode()
	return nil
}

func (c *CRT) Close() {
	if c.scene.ID != 0 {
		rl.UnloadRenderTexture(c.scene)
	}
	if c.shader.ID != 0 {
		rl.UnloadShader(c.shader)
	}
	*c = CRT{}
}
