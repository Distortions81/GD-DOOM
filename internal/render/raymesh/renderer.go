//go:build raylib && cgo && !js

package raymesh

import (
	"fmt"
	"math"
	"slices"
	"unsafe"

	"gddoom/internal/render/levelmesh"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const vertexShader = `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec2 vertexTexCoord2;
in vec4 vertexColor;
uniform mat4 mvp;
// Keep UVs inside covered samples at seams with multisampled edge coverage.
centroid out vec2 fragTexCoord;
out vec4 fragColor;
flat out float fragLightTag;
void main() {
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    fragLightTag = vertexTexCoord2.x;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

const fragmentShader = `#version 330
centroid in vec2 fragTexCoord;
in vec4 fragColor;
flat in float fragLightTag;
uniform sampler2D texture0;
uniform float alphaCutoff;
uniform float viewMode;
uniform float lightingMode;
uniform float shadeRamp[32];
out vec4 finalColor;
float doomShade() {
    // Reciprocal clip W is view-forward depth in the resident mesh's
    // 1/64 world units. This is independent of viewport size and FOV.
    float depth = max(2.0, 64.0 / gl_FragCoord.w);
    bool plane = fragLightTag > 2.0;
    float bias = plane ? 0.0 : fragLightTag;
    float lightNum = clamp(fragColor.a * 255.0 / 16.0 + bias, 0.0, 15.0);
    float startMap = (15.0 - lightNum) * 4.0;
    float distanceTerm;
    if (plane) {
        float lightZ = clamp(depth / 16.0, 0.0, 127.0);
        distanceTerm = 80.0 / (lightZ + 1.0);
    } else {
        distanceTerm = clamp(2560.0 / depth, 0.0, 47.0) / 2.0;
    }
    float row = clamp(startMap - distanceTerm, 0.0, 31.0);
    int lo = int(floor(row));
    return mix(shadeRamp[lo], shadeRamp[min(lo + 1, 31)], fract(row));
}
void main() {
    vec4 texel = texture(texture0, fragTexCoord);
    if (texel.a < alphaCutoff) discard;
    if (viewMode != 0.0) finalColor = vec4(fragColor.rgb, 1.0);
    else if (lightingMode == 1.0) finalColor = vec4(texel.rgb * doomShade(), 1.0);
    else if (lightingMode == 2.0) finalColor = vec4(texel.rgb, 1.0);
    else finalColor = vec4(texel.rgb * fragColor.rgb, 1.0);
}`

type residentBatch struct {
	mesh           rl.Mesh
	positions, uvs []float32 // Retain Go-managed mesh arrays until UnloadMesh.
	lightingUVs    []float32
	colors         []byte
}

type Stats struct {
	Triangles, DrawCalls, ResidentMeshes int
	MeshUploads, BufferUpdates           int // Most recent Sync, not per-draw uploads.
}

type Renderer struct {
	builder                        Builder
	active                         []*Batch
	meshes                         map[BatchKey]*residentBatch
	textures                       map[BatchKey]rl.Texture2D
	textureOptions                 TextureOptions
	shader                         rl.Shader
	material                       rl.Material // One shared material; meshes/textures have one owner.
	defaultShader                  rl.Shader
	defaultTexture                 rl.Texture2D
	alphaLocation, modeLocation    int32
	lightingLocation, rampLocation int32
	lightingMode                   LightingMode
	lightRamp                      []float32 // Separate backing array: safe to pass to cgo.
	fullbright                     bool
	stats                          Stats
}

// NewRenderer requires a Raylib window on the current locked OS thread.
func NewRenderer() (*Renderer, error) {
	return NewRendererWithTextureOptions(TextureOptions{Scale: 1, Filter: Nearest})
}

func NewRendererWithTextureOptions(options TextureOptions) (*Renderer, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	shader := rl.LoadShaderFromMemory(vertexShader, fragmentShader)
	if shader.ID == rl.GetShaderIdDefault() || !rl.IsShaderValid(shader) {
		return nil, fmt.Errorf("Raylib mesh shader could not compile (requires OpenGL 3.3)")
	}
	r := &Renderer{shader: shader, meshes: make(map[BatchKey]*residentBatch), textures: make(map[BatchKey]rl.Texture2D), textureOptions: options, lightingMode: SectorLighting, lightRamp: make([]float32, 32)}
	r.SetLightRamp(linearLightRamp())
	r.alphaLocation = rl.GetShaderLocation(shader, "alphaCutoff")
	r.modeLocation = rl.GetShaderLocation(shader, "viewMode")
	r.lightingLocation = rl.GetShaderLocation(shader, "lightingMode")
	r.rampLocation = rl.GetShaderLocation(shader, "shadeRamp[0]")
	if r.alphaLocation < 0 || r.modeLocation < 0 || r.lightingLocation < 0 || r.rampLocation < 0 {
		rl.UnloadShader(shader)
		return nil, fmt.Errorf("Raylib mesh shader is missing required uniforms")
	}
	r.material = rl.LoadMaterialDefault()
	r.defaultShader = r.material.Shader
	r.defaultTexture = r.material.GetMap(rl.MapAlbedo).Texture
	r.material.Shader = shader
	rl.SetClipPlanes(2*float64(WorldScale), 2048)
	return r, nil
}

// Sync updates only changed vertex/UV buffers. Static batches and animated
// textures remain resident even when a door closes or a frame becomes inactive.
func (r *Renderer) Sync(tris []levelmesh.Triangle, texture func(levelmesh.Triangle) levelmesh.Texture, light func(int) float64, mode levelmesh.Mode) {
	r.active = r.builder.Build(tris, texture, light, mode)
	r.stats = Stats{DrawCalls: len(r.active)}
	for _, batch := range r.active {
		r.stats.Triangles += len(batch.Positions) / 9
		if _, ok := r.textures[batch.Key]; !ok {
			tex := prepareTexture(batch.Texture, r.textureOptions.Scale, batch.Key.Masked)
			// NewImage references Go memory; LoadTextureFromImage copies it to
			// VRAM synchronously. Do not UnloadImage on this borrowed data.
			img := rl.NewImage(tex.RGBA, int32(tex.Width), int32(tex.Height), 1, rl.UncompressedR8g8b8a8)
			gpuTex := rl.LoadTextureFromImage(img)
			rl.SetTextureWrap(gpuTex, rl.WrapRepeat)
			r.applyTextureFilter(&gpuTex, r.textureOptions.Filter)
			r.textures[batch.Key] = gpuTex
		}
		resident := r.meshes[batch.Key]
		if resident == nil || len(resident.positions) != len(batch.Positions) {
			if resident != nil {
				rl.UnloadMesh(&resident.mesh)
			}
			resident = &residentBatch{positions: append([]float32(nil), batch.Positions...), uvs: append([]float32(nil), batch.UVs...), colors: append([]byte(nil), batch.Colors...), lightingUVs: append([]float32(nil), batch.LightingUVs...)}
			resident.mesh = rl.Mesh{VertexCount: int32(len(resident.positions) / 3), TriangleCount: int32(len(resident.positions) / 9), Vertices: &resident.positions[0], Texcoords: &resident.uvs[0], Texcoords2: &resident.lightingUVs[0], Colors: &resident.colors[0]}
			// Raylib-Go's cgo UploadMesh/UnloadMesh wrappers track Go-managed
			// arrays and free only their C-owned VAO/VBO allocations.
			rl.UploadMesh(&resident.mesh, true)
			r.meshes[batch.Key] = resident
			r.stats.MeshUploads++
			continue
		}
		if !slices.Equal(resident.positions, batch.Positions) {
			copy(resident.positions, batch.Positions)
			rl.UpdateMeshBuffer(resident.mesh, 0, floatBytes(resident.positions), 0)
			r.stats.BufferUpdates++
		}
		if !slices.Equal(resident.uvs, batch.UVs) {
			copy(resident.uvs, batch.UVs)
			rl.UpdateMeshBuffer(resident.mesh, 1, floatBytes(resident.uvs), 0)
			r.stats.BufferUpdates++
		}
		if !slices.Equal(resident.colors, batch.Colors) {
			copy(resident.colors, batch.Colors)
			rl.UpdateMeshBuffer(resident.mesh, 3, resident.colors, 0)
			r.stats.BufferUpdates++
		}
		if !slices.Equal(resident.lightingUVs, batch.LightingUVs) {
			copy(resident.lightingUVs, batch.LightingUVs)
			rl.UpdateMeshBuffer(resident.mesh, 5, floatBytes(resident.lightingUVs), 0)
			r.stats.BufferUpdates++
		}
	}
	r.stats.ResidentMeshes = len(r.meshes)
}

func (r *Renderer) SetLightingMode(mode LightingMode) error {
	if err := mode.Validate(); err != nil {
		return err
	}
	r.lightingMode = mode
	return nil
}

// SetLightRamp supplies the WAD's COLORMAP-derived RGB row brightness.
// Changing lighting mode, ramp or a powerup override updates uniforms only.
func (r *Renderer) SetLightRamp(ramp [32]float32) { copy(r.lightRamp, ramp[:]) }
func (r *Renderer) SetFullbright(enabled bool)    { r.fullbright = enabled }

// SetTextureFilter reuses resident textures and geometry. Mipmaps are generated
// once, on the first filtered use. Nearest always samples the original base
// level, even if the image already has a mip chain from an earlier comparison.
func (r *Renderer) SetTextureFilter(filter TextureFilter) error {
	options := r.textureOptions
	options.Filter = filter
	if err := options.Validate(); err != nil {
		return err
	}
	if filter == r.textureOptions.Filter {
		return nil
	}
	for key, tex := range r.textures {
		if r.textureOptions.Filter == Anisotropic {
			rl.TextureParameters(tex.ID, rl.TextureFilterAnisotropic, 1)
		}
		r.applyTextureFilter(&tex, filter)
		r.textures[key] = tex
	}
	r.textureOptions = options
	return nil
}

func (r *Renderer) applyTextureFilter(tex *rl.Texture2D, filter TextureFilter) {
	if filter == Nearest {
		base := *tex
		base.Mipmaps = 1 // Ask Raylib for unfiltered base-level minification.
		rl.SetTextureFilter(base, rl.FilterPoint)
		return
	}
	if tex.Mipmaps == 1 && (tex.Width > 1 || tex.Height > 1) {
		rl.GenTextureMipmaps(tex)
	}
	if tex.Mipmaps > 1 {
		rl.SetTextureFilter(*tex, rl.FilterTrilinear)
	} else {
		rl.SetTextureFilter(*tex, rl.FilterBilinear) // A 1x1 image has no lower mip.
	}
	if filter == Anisotropic {
		// Raylib's anisotropic setting only changes anisotropy, so enable
		// trilinear first. Unsupported hardware retains that base filter.
		rl.SetTextureFilter(*tex, rl.FilterAnisotropic8x)
	}
}

func floatBytes(v []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*4)
}

// Camera uses Doom's 90-degree horizontal FOV and 1.2 pixel aspect at any size.
func Camera(c levelmesh.Camera, width, height int) rl.Camera3D {
	fovy := 2 * math.Atan(float64(height)/(float64(width)*1.2)) * 180 / math.Pi
	p := rl.NewVector3(float32(c.X)*WorldScale, float32(c.Z)*WorldScale, -float32(c.Y)*WorldScale)
	return rl.Camera3D{Position: p, Target: rl.NewVector3(p.X+float32(math.Cos(c.Yaw)), p.Y, p.Z-float32(math.Sin(c.Yaw))), Up: rl.NewVector3(0, 1, 0), Fovy: float32(fovy), Projection: rl.CameraPerspective}
}

func (r *Renderer) Draw(c levelmesh.Camera, width, height int, mode levelmesh.Mode) {
	rl.BeginMode3D(Camera(c, width, height))
	defer rl.EndMode3D()
	viewMode := float32(0)
	if mode != levelmesh.Textured {
		viewMode = 1
	}
	rl.SetShaderValue(r.shader, r.modeLocation, []float32{viewMode}, rl.ShaderUniformFloat)
	lightingMode := float32(0)
	if r.lightingMode == DoomLighting {
		lightingMode = 1
	}
	if r.lightingMode == FullbrightLighting || r.fullbright {
		lightingMode = 2
	}
	rl.SetShaderValue(r.shader, r.lightingLocation, []float32{lightingMode}, rl.ShaderUniformFloat)
	rl.SetShaderValueV(r.shader, r.rampLocation, r.lightRamp[:], rl.ShaderUniformFloat, 32)
	if mode == levelmesh.Wireframe {
		rl.EnableWireMode()
		defer rl.DisableWireMode()
	}
	identity := rl.MatrixIdentity()
	for _, b := range r.active {
		r.material.GetMap(rl.MapAlbedo).Texture = r.textures[b.Key]
		cutoff := float32(0)
		if b.Key.Masked {
			cutoff = 0.5
		}
		rl.SetShaderValue(r.shader, r.alphaLocation, []float32{cutoff}, rl.ShaderUniformFloat)
		rl.DrawMesh(r.meshes[b.Key].mesh, r.material, identity)
	}
}

func (r *Renderer) Stats() Stats { return r.stats }

func (r *Renderer) Close() {
	for _, b := range r.meshes {
		rl.UnloadMesh(&b.mesh)
	}
	for _, tex := range r.textures {
		rl.UnloadTexture(tex)
	}
	// UnloadMaterial normally unloads its shader/textures too. Restore its
	// defaults to avoid double-freeing the separately owned shared resources.
	r.material.Shader = r.defaultShader
	r.material.GetMap(rl.MapAlbedo).Texture = r.defaultTexture
	rl.UnloadMaterial(r.material)
	rl.UnloadShader(r.shader)
}
