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
uniform sampler2D texture1;
uniform sampler2D texture2;
uniform float textureBlend;
uniform float fuzzEnabled;
uniform vec2 fuzzView;
uniform float fuzzOrigin;
uniform float alphaCutoff;
uniform float viewMode;
uniform float lightingMode;
uniform float shadeRamp[32];
uniform float shadeRows;
uniform float gammaEnabled;
uniform float gammaTable[256];
out vec4 finalColor;
float doomShade() {
	if (fragLightTag == 6.0) return 1.0;
	if (fragLightTag == 7.0) return 0.3;
    // Reciprocal clip W is view-forward depth in the resident mesh's
    // 1/64 world units. This is independent of viewport size and FOV.
    float depth = max(2.0, 64.0 / gl_FragCoord.w);
    float light = floor(fragColor.a * 255.0 + 0.5);
    if (fragLightTag == 8.0) {
        float distanceMul = floor((1.0 - 0.55*clamp((depth-2.0)/1200.0,0.0,1.0))*256.0);
        return floor(distanceMul*light/256.0)/256.0;
    }
    if (shadeRows < 1.0) return light/256.0;
    bool plane = fragLightTag == 4.0;
    bool masked = fragLightTag == 5.0;
    float bias = (plane || masked) ? 0.0 : fragLightTag;
    float lightNum = clamp(light / 16.0 + bias, 0.0, 15.0);
    float startMap = (15.0 - lightNum) * 2.0 * shadeRows / 16.0;
    float distanceTerm;
    if (masked) {
        distanceTerm = 0.0;
    } else if (plane) {
        float lightZ = clamp(depth / 16.0, 0.0, 127.0);
        distanceTerm = 80.0 / (lightZ + 1.0);
    } else {
        distanceTerm = clamp(2560.0 / depth, 0.0, 47.0) / 2.0;
    }
    float row = clamp(startMap - distanceTerm, 0.0, shadeRows-1.0);
    int lo = int(floor(row));
    return mix(shadeRamp[lo], shadeRamp[min(lo + 1, 31)], fract(row));
}
void main() {
    if (fragLightTag == 7.0 && fuzzEnabled != 0.0) {
        vec2 p = floor(vec2(gl_FragCoord.x, fuzzView.y-(gl_FragCoord.y-fuzzOrigin)));
        ivec2 size = textureSize(texture1,0);
        ivec2 cell = ivec2(floor(p*vec2(size)/fuzzView));
        cell = clamp(cell,ivec2(0),size-1);
        vec4 sampleColor = texelFetch(texture1,ivec2(cell.x,size.y-1-cell.y),0);
        if (sampleColor.a < 0.5) discard;
        finalColor = sampleColor;
        return;
    }
    vec4 texel = texture(texture0, fragTexCoord);
    if (textureBlend > 0.0) {
        vec4 other = texture(texture2, fragTexCoord);
        // Doom masked animations keep the union of both frames' cutouts.
        texel.rgb = floor(mix(texel.rgb, other.rgb, textureBlend)*255.0+vec3(0.5))/255.0;
        texel.a = max(texel.a, other.a);
    }
    if (texel.a < alphaCutoff) discard;
    if (viewMode != 0.0) finalColor = vec4(fragColor.rgb, 1.0);
    else if (lightingMode == 1.0) {
		// Main source-port lighting interpolates its row multiplier,
        // rounds it to 0..256, then truncates each shaded byte before gamma.
        float mul = floor(doomShade()*256.0+0.5);
        vec3 rgb = floor(texel.rgb*255.0+vec3(0.5));
        finalColor = vec4(floor(rgb*mul/256.0)/255.0,1.0);
    }
    else if (lightingMode == 2.0) finalColor = vec4(texel.rgb, 1.0);
    else finalColor = vec4(texel.rgb * fragColor.rgb, 1.0);
    if (viewMode == 0.0 && gammaEnabled != 0.0) {
        ivec3 c = ivec3(clamp(floor(finalColor.rgb*255.0+vec3(0.5)),vec3(0.0),vec3(255.0)));
        finalColor.rgb = vec3(gammaTable[c.r],gammaTable[c.g],gammaTable[c.b]);
    }
}`

type residentBatch struct {
	mesh           rl.Mesh
	positions, uvs []float32 // Retain Go-managed mesh arrays until UnloadMesh.
	lightingUVs    []float32
	colors         []byte
}

type fixedTextureKey struct {
	Batch  BatchKey
	Pixels *byte // Distinguish immutable variants without changing mesh identity.
}

type Stats struct {
	Triangles, DrawCalls, ResidentMeshes int
	MeshUploads, BufferUpdates           int // Most recent Sync, not per-draw uploads.
}

type Renderer struct {
	builder                                            Builder
	active                                             []*Batch
	meshes                                             map[BatchKey]*residentBatch
	textures                                           map[BatchKey]rl.Texture2D
	fixedTextures                                      map[fixedTextureKey]rl.Texture2D
	textureOptions                                     TextureOptions
	shader                                             rl.Shader
	material                                           rl.Material // One shared material; meshes/textures have one owner.
	defaultShader                                      rl.Shader
	defaultTexture                                     rl.Texture2D
	alphaLocation, modeLocation                        int32
	blendLocation                                      int32
	lightingLocation, rampLocation                     int32
	rowsLocation                                       int32
	lightRows                                          float32
	fuzzLocation, fuzzViewLocation, fuzzOriginLocation int32
	fuzzTexture                                        rl.Texture2D
	gammaLocation, gammaTableLocation                  int32
	gammaTable                                         []float32
	gammaEnabled, gammaDirty                           bool
	lightingMode                                       LightingMode
	lightRamp                                          []float32 // Separate backing array: safe to pass to cgo.
	fullbright                                         bool
	fixedColormap                                      bool
	stats                                              Stats
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
	r := &Renderer{shader: shader, meshes: make(map[BatchKey]*residentBatch), textures: make(map[BatchKey]rl.Texture2D), fixedTextures: make(map[fixedTextureKey]rl.Texture2D), textureOptions: options, lightingMode: SectorLighting, lightRamp: make([]float32, 32)}
	r.SetLightRamp(linearLightRamp())
	r.SetLightRows(32)
	r.alphaLocation = rl.GetShaderLocation(shader, "alphaCutoff")
	r.blendLocation = rl.GetShaderLocation(shader, "textureBlend")
	r.modeLocation = rl.GetShaderLocation(shader, "viewMode")
	r.lightingLocation = rl.GetShaderLocation(shader, "lightingMode")
	r.rampLocation = rl.GetShaderLocation(shader, "shadeRamp[0]")
	r.rowsLocation = rl.GetShaderLocation(shader, "shadeRows")
	r.fuzzLocation = rl.GetShaderLocation(shader, "fuzzEnabled")
	r.fuzzViewLocation = rl.GetShaderLocation(shader, "fuzzView")
	r.fuzzOriginLocation = rl.GetShaderLocation(shader, "fuzzOrigin")
	r.gammaLocation = rl.GetShaderLocation(shader, "gammaEnabled")
	r.gammaTableLocation = rl.GetShaderLocation(shader, "gammaTable[0]")
	if r.alphaLocation < 0 || r.blendLocation < 0 || r.modeLocation < 0 || r.lightingLocation < 0 || r.rampLocation < 0 || r.rowsLocation < 0 {
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
		r.ensureTexture(batch.Texture, batch.Key.Masked)
		if batch.Texture.HasBlend() {
			r.ensureTexture(batch.Texture.BlendTexture(), batch.Key.Masked)
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

func (r *Renderer) ensureTexture(tex levelmesh.Texture, masked bool) {
	key := BatchKey{Texture: TextureKey{&tex.RGBA[0], tex.Width, tex.Height}, Masked: masked}
	if _, ok := r.textures[key]; !ok {
		r.textures[key] = r.uploadTexture(tex, masked)
	}
	if len(tex.FixedRGBA) == tex.Width*tex.Height*4 {
		fixedKey := fixedTextureKey{Batch: key, Pixels: &tex.FixedRGBA[0]}
		if _, ok := r.fixedTextures[fixedKey]; !ok {
			tex.RGBA = tex.FixedRGBA
			r.fixedTextures[fixedKey] = r.uploadTexture(tex, masked)
		}
	}
}

func (r *Renderer) uploadTexture(tex levelmesh.Texture, masked bool) rl.Texture2D {
	tex = prepareTexture(tex, r.textureOptions.Scale, masked)
	// NewImage borrows Go memory; LoadTextureFromImage copies it synchronously.
	img := rl.NewImage(tex.RGBA, int32(tex.Width), int32(tex.Height), 1, rl.UncompressedR8g8b8a8)
	gpuTex := rl.LoadTextureFromImage(img)
	rl.SetTextureWrap(gpuTex, rl.WrapRepeat)
	r.applyTextureFilter(&gpuTex, r.textureOptions.Filter)
	return gpuTex
}

func (r *Renderer) residentTexture(tex levelmesh.Texture, masked bool) rl.Texture2D {
	key := BatchKey{Texture: TextureKey{&tex.RGBA[0], tex.Width, tex.Height}, Masked: masked}
	if r.fixedColormap && len(tex.FixedRGBA) > 0 {
		if fixed, ok := r.fixedTextures[fixedTextureKey{Batch: key, Pixels: &tex.FixedRGBA[0]}]; ok {
			return fixed
		}
	}
	return r.textures[key]
}

func (r *Renderer) SetLightingMode(mode LightingMode) error {
	if err := mode.Validate(); err != nil {
		return err
	}
	r.lightingMode = mode
	return nil
}

// SetLightRamp supplies the shared source-port RGB row brightness.
// Changing lighting mode, ramp or a powerup override updates uniforms only.
func (r *Renderer) SetLightRamp(ramp [32]float32) { copy(r.lightRamp, ramp[:]) }
func (r *Renderer) SetLightRows(rows int)         { r.lightRows = float32(max(0, min(32, rows))) }
func (r *Renderer) SetFullbright(enabled bool)    { r.fullbright = enabled }
func (r *Renderer) SetFixedColormap(enabled bool) { r.fixedColormap = enabled }

// Gamma changes shader uniforms without rebuilding textures or mesh buffers.
func (r *Renderer) SetGammaTable(table [256]uint8) {
	if r.gammaTable == nil {
		r.gammaTable = make([]float32, 256)
	}
	r.gammaEnabled = false
	for i, value := range table {
		v := float32(value) / 255
		if r.gammaTable[i] != v {
			r.gammaTable[i], r.gammaDirty = v, true
		}
		r.gammaEnabled = r.gammaEnabled || value != uint8(i)
	}
}

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
	for key, tex := range r.fixedTextures {
		if r.textureOptions.Filter == Anisotropic {
			rl.TextureParameters(tex.ID, rl.TextureFilterAnisotropic, 1)
		}
		r.applyTextureFilter(&tex, filter)
		r.fixedTextures[key] = tex
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
	r.drawBatches(c, width, height, mode, r.active)
}

func (r *Renderer) drawBatches(c levelmesh.Camera, width, height int, mode levelmesh.Mode, batches []*Batch) {
	camera := Camera(c, width, height)
	rl.BeginMode3D(camera)
	// The world viewport can exclude the status bar. Raylib's default
	// projection uses the whole window, so set the view's aspect explicitly.
	// Doom uses a 90-degree horizontal FOV and a 1.2 vertical pixel aspect.
	// Fovy carries the vertical correction; the extra aspect factor keeps
	// horizontal focal length at width/2 instead of narrowing it to width*.6.
	rl.SetMatrixProjection(rl.MatrixPerspective(camera.Fovy*math.Pi/180, float32(width)*1.2/float32(height), 2*WorldScale, 2048))
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
	if r.lightingMode == FullbrightLighting || r.fullbright || r.fixedColormap {
		lightingMode = 2
	}
	rl.SetShaderValue(r.shader, r.lightingLocation, []float32{lightingMode}, rl.ShaderUniformFloat)
	rl.SetShaderValueV(r.shader, r.rampLocation, r.lightRamp[:], rl.ShaderUniformFloat, 32)
	rl.SetShaderValue(r.shader, r.rowsLocation, []float32{r.lightRows}, rl.ShaderUniformFloat)
	if r.gammaDirty {
		rl.SetShaderValueV(r.shader, r.gammaTableLocation, r.gammaTable, rl.ShaderUniformFloat, 256)
		r.gammaDirty = false
	}
	gamma := float32(0)
	// Fixed variants and fuzz layers already contain the active gamma colors.
	if r.gammaEnabled && !r.fixedColormap {
		gamma = 1
	}
	rl.SetShaderValue(r.shader, r.gammaLocation, []float32{gamma}, rl.ShaderUniformFloat)
	fuzz := float32(0)
	if r.fuzzTexture.ID != 0 {
		fuzz = 1
	}
	rl.SetShaderValue(r.shader, r.fuzzLocation, []float32{fuzz}, rl.ShaderUniformFloat)
	rl.SetShaderValue(r.shader, r.fuzzViewLocation, []float32{float32(width), float32(height)}, rl.ShaderUniformVec2)
	rl.SetShaderValue(r.shader, r.fuzzOriginLocation, []float32{float32(rl.GetRenderHeight() - height)}, rl.ShaderUniformFloat)
	r.material.GetMap(rl.MapMetalness).Texture = r.fuzzTexture
	if mode == levelmesh.Wireframe {
		rl.EnableWireMode()
		defer rl.DisableWireMode()
	}
	identity := rl.MatrixIdentity()
	for _, b := range batches {
		r.material.GetMap(rl.MapAlbedo).Texture = r.residentTexture(b.Texture, b.Key.Masked)
		blend := float32(0)
		r.material.GetMap(rl.MapNormal).Texture = rl.Texture2D{}
		if b.Texture.HasBlend() {
			blend = float32(b.Texture.BlendAlpha) / 255
			r.material.GetMap(rl.MapNormal).Texture = r.residentTexture(b.Texture.BlendTexture(), b.Key.Masked)
		}
		rl.SetShaderValue(r.shader, r.blendLocation, []float32{blend}, rl.ShaderUniformFloat)
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
	for _, tex := range r.fixedTextures {
		rl.UnloadTexture(tex)
	}
	// UnloadMaterial normally unloads its shader/textures too. Restore its
	// defaults to avoid double-freeing the separately owned shared resources.
	r.material.Shader = r.defaultShader
	r.material.GetMap(rl.MapAlbedo).Texture = r.defaultTexture
	r.material.GetMap(rl.MapMetalness).Texture = rl.Texture2D{}
	r.material.GetMap(rl.MapNormal).Texture = rl.Texture2D{}
	rl.UnloadMaterial(r.material)
	rl.UnloadShader(r.shader)
}
