//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Match Ebiten's logical fuzz shader, including four-negative-offset feedback
// chains and exact palette recovery before its 5-bit nearest-color fallback.
const fuzzFragmentShader = `#version 330
uniform sampler2D texture0;
uniform sampler2D backgroundTexture;
uniform sampler2D paletteTexture;
uniform sampler2D remapTexture;
uniform sampler2D lookupTexture;
uniform vec2 logicalSize;
uniform vec2 sourceSize;
uniform float viewHeight;
uniform float remapEnabled;
uniform float fuzzShade;
uniform float probes;
uniform float offsets[50];
out vec4 finalColor;
vec4 lookup(int i) { return texelFetch(lookupTexture,ivec2(i%256,i/256),0); }
int paletteIndex(vec3 rgb) {
    ivec3 c=ivec3(floor(rgb*255.0+vec3(0.5)));
    int bucket=(c.r*3+c.g*5+c.b*7)%1024;
    for(int probe=0;probe<256;probe++) {
        if(probe>=int(probes)) break;
        vec4 entry=lookup(32768+bucket);
        if(entry.g<0.5) break;
        int index=int(floor(entry.r*255.0+0.5));
        ivec3 candidate=ivec3(floor(texelFetch(paletteTexture,ivec2(index,0),0).rgb*255.0+vec3(0.5)));
        if(all(equal(candidate,c))) return index;
        bucket=(bucket+1)%1024;
    }
    ivec3 q=c/8;
    return int(floor(lookup(32768+q.r*1024+q.g*32+q.b).b*255.0+0.5));
}
void main() {
    ivec2 cell=ivec2(int(gl_FragCoord.x),int(logicalSize.y)-1-int(gl_FragCoord.y));
    vec4 post=texelFetch(texture0,cell,0);
    if(post.b<0.5) discard;
    int top=int(floor(post.g*255.0+0.5));
    int phase=int(floor(post.r*255.0+0.5));
    int cy=cell.y;
    phase+=cy-top;
    float direction=offsets[phase%50];
    int steps=1;
    for(int n=0;n<4;n++) {
        if(direction>0.0 || cy<=top) break;
        cy--; phase--; steps++;
        direction=offsets[(phase+50)%50];
    }
    cy+=int(direction);
    ivec2 pos=ivec2(floor((vec2(cell.x,cy)+vec2(0.5))*vec2(sourceSize.x,viewHeight)/logicalSize));
    pos.y=int(sourceSize.y)-1-pos.y;
    vec3 rgb=texelFetch(backgroundTexture,pos,0).rgb;
    if(remapEnabled!=0.0) {
        int index=paletteIndex(rgb);
        for(int n=0;n<5;n++) {
            if(n>=steps) break;
            index=int(floor(texelFetch(remapTexture,ivec2(index,0),0).r*255.0+0.5));
        }
        rgb=texelFetch(paletteTexture,ivec2(index,0),0).rgb;
    } else {
        rgb=floor(rgb*255.0+vec3(0.5));
        for(int n=0;n<5;n++) { if(n>=steps) break; rgb=floor(rgb*fuzzShade); }
        rgb/=255.0;
    }
    finalColor=vec4(rgb,1.0);
}`

type spectreFuzz struct {
	shader          rl.Shader
	snapshot, layer rl.RenderTexture2D
	posts           rl.Texture2D
	pixels          []byte
	offsets         []float32 // Keep cgo uniform data separate from Go-pointer-bearing structs.
}

func newSpectreFuzz() (*spectreFuzz, error) {
	shader := rl.LoadShaderFromMemory("", fuzzFragmentShader)
	if !rl.IsShaderValid(shader) || shader.ID == rl.GetShaderIdDefault() {
		return nil, fmt.Errorf("spectre fuzz shader could not compile")
	}
	return &spectreFuzz{shader: shader, offsets: make([]float32, 50)}, nil
}

func (f *spectreFuzz) draw(p *Presentation, spans []levelmesh.FuzzSpan, colors levelmesh.FuzzColors, width, height int) error {
	sw, sh := int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight())
	if p.sceneWidth > 0 && p.sceneHeight > 0 {
		sw, sh = int32(p.sceneWidth), int32(p.sceneHeight)
	}
	cw, ch := int32(min(width, 320)), int32(min(height, 200))
	if f.snapshot.ID == 0 || f.snapshot.Texture.Width != sw || f.snapshot.Texture.Height != sh {
		if f.snapshot.ID != 0 {
			rl.UnloadRenderTexture(f.snapshot)
		}
		f.snapshot = rl.LoadRenderTexture(sw, sh)
		if !rl.IsRenderTextureValid(f.snapshot) {
			return fmt.Errorf("could not allocate spectre snapshot")
		}
	}
	if f.layer.ID == 0 || f.layer.Texture.Width != cw || f.layer.Texture.Height != ch {
		if f.layer.ID != 0 {
			rl.UnloadRenderTexture(f.layer)
			rl.UnloadTexture(f.posts)
		}
		f.layer = rl.LoadRenderTexture(cw, ch)
		if !rl.IsRenderTextureValid(f.layer) {
			return fmt.Errorf("could not allocate logical spectre layer")
		}
		rl.SetTextureFilter(f.layer.Texture, rl.FilterPoint)
		f.pixels = make([]byte, int(cw*ch)*4)
		img := rl.NewImage(f.pixels, cw, ch, 1, rl.UncompressedR8g8b8a8)
		f.posts = rl.LoadTextureFromImage(img)
		rl.SetTextureFilter(f.posts, rl.FilterPoint)
	}
	if err := CopyWindowRegionToTexture(f.snapshot, sw, sh); err != nil {
		return err
	}
	clear(f.pixels)
	for _, s := range spans {
		if s.X < 0 || s.X >= int(cw) {
			continue
		}
		for y := max(0, s.Y0); y <= min(int(ch)-1, s.Y1); y++ {
			i := (y*int(cw) + s.X) * 4
			f.pixels[i], f.pixels[i+1], f.pixels[i+2], f.pixels[i+3] = byte(s.Phase%50), byte(s.Y0), 255, 255
		}
	}
	rl.UpdateTexture(f.posts, f.pixels)
	rl.BeginTextureMode(f.layer)
	rl.ClearBackground(rl.Blank)
	rl.BeginShaderMode(f.shader)
	set := func(name string, v []float32, kind rl.ShaderUniformDataType) {
		rl.SetShaderValue(f.shader, rl.GetShaderLocation(f.shader, name), v, kind)
	}
	set("logicalSize", []float32{float32(cw), float32(ch)}, rl.ShaderUniformVec2)
	set("sourceSize", []float32{float32(sw), float32(sh)}, rl.ShaderUniformVec2)
	set("viewHeight", []float32{float32(height)}, rl.ShaderUniformFloat)
	set("fuzzShade", []float32{colors.Shade}, rl.ShaderUniformFloat)
	set("probes", []float32{float32(colors.Probes)}, rl.ShaderUniformFloat)
	copy(f.offsets, colors.Offsets[:])
	rl.SetShaderValueV(f.shader, rl.GetShaderLocation(f.shader, "offsets[0]"), f.offsets, rl.ShaderUniformFloat, 50)
	rl.SetShaderValueTexture(f.shader, rl.GetShaderLocation(f.shader, "backgroundTexture"), f.snapshot.Texture)
	enabled := float32(0)
	if colors.RemapEnabled {
		for _, input := range []struct {
			name string
			tex  levelmesh.Texture
		}{{"paletteTexture", colors.Palette}, {"remapTexture", colors.Remap}, {"lookupTexture", colors.Lookup}} {
			t, ok := p.texture(input.tex, false)
			if !ok {
				rl.EndShaderMode()
				rl.EndTextureMode()
				return fmt.Errorf("invalid spectre palette image")
			}
			rl.SetShaderValueTexture(f.shader, rl.GetShaderLocation(f.shader, input.name), t)
		}
		enabled = 1
	}
	set("remapEnabled", []float32{enabled}, rl.ShaderUniformFloat)
	rl.DrawTexture(f.posts, 0, 0, rl.White)
	rl.EndShaderMode()
	rl.EndTextureMode()
	return nil
}

func (f *spectreFuzz) close() {
	if f.snapshot.ID != 0 {
		rl.UnloadRenderTexture(f.snapshot)
	}
	if f.layer.ID != 0 {
		rl.UnloadRenderTexture(f.layer)
	}
	if f.posts.ID != 0 {
		rl.UnloadTexture(f.posts)
	}
	rl.UnloadShader(f.shader)
}
