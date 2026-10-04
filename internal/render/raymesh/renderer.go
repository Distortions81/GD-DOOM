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
in vec4 vertexColor;
uniform mat4 mvp;
out vec2 fragTexCoord;
out vec4 fragColor;
void main() {
    fragTexCoord = vertexTexCoord;
    fragColor = vertexColor;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

const fragmentShader = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
uniform float alphaCutoff;
uniform float viewMode;
out vec4 finalColor;
void main() {
    vec4 texel = texture(texture0, fragTexCoord);
    if (texel.a < alphaCutoff) discard;
    if (viewMode != 0.0) finalColor = fragColor;
    else finalColor = vec4(texel.rgb * fragColor.rgb, 1.0);
}`

type residentBatch struct {
	mesh           rl.Mesh
	positions, uvs []float32 // Retain Go-managed mesh arrays until UnloadMesh.
	colors         []byte
}

type Stats struct {
	Triangles, DrawCalls, ResidentMeshes int
	MeshUploads, BufferUpdates           int // Most recent Sync, not per-draw uploads.
}

type Renderer struct {
	builder                     Builder
	active                      []*Batch
	meshes                      map[BatchKey]*residentBatch
	textures                    map[TextureKey]rl.Texture2D
	shader                      rl.Shader
	material                    rl.Material // One shared material; meshes/textures have one owner.
	defaultShader               rl.Shader
	defaultTexture              rl.Texture2D
	alphaLocation, modeLocation int32
	stats                       Stats
}

// NewRenderer requires a Raylib window on the current locked OS thread.
func NewRenderer() (*Renderer, error) {
	shader := rl.LoadShaderFromMemory(vertexShader, fragmentShader)
	if shader.ID == rl.GetShaderIdDefault() || !rl.IsShaderValid(shader) {
		return nil, fmt.Errorf("Raylib mesh shader could not compile (requires OpenGL 3.3)")
	}
	r := &Renderer{shader: shader, meshes: make(map[BatchKey]*residentBatch), textures: make(map[TextureKey]rl.Texture2D)}
	r.alphaLocation = rl.GetShaderLocation(shader, "alphaCutoff")
	r.modeLocation = rl.GetShaderLocation(shader, "viewMode")
	if r.alphaLocation < 0 || r.modeLocation < 0 {
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
		if _, ok := r.textures[batch.Key.Texture]; !ok {
			tex := batch.Texture
			// NewImage references Go memory; LoadTextureFromImage copies it to
			// VRAM synchronously. Do not UnloadImage on this borrowed data.
			img := rl.NewImage(tex.RGBA, int32(tex.Width), int32(tex.Height), 1, rl.UncompressedR8g8b8a8)
			gpuTex := rl.LoadTextureFromImage(img)
			rl.SetTextureFilter(gpuTex, rl.FilterPoint)
			rl.SetTextureWrap(gpuTex, rl.WrapRepeat)
			r.textures[batch.Key.Texture] = gpuTex
		}
		resident := r.meshes[batch.Key]
		if resident == nil || len(resident.positions) != len(batch.Positions) {
			if resident != nil {
				rl.UnloadMesh(&resident.mesh)
			}
			resident = &residentBatch{positions: append([]float32(nil), batch.Positions...), uvs: append([]float32(nil), batch.UVs...), colors: append([]byte(nil), batch.Colors...)}
			resident.mesh = rl.Mesh{VertexCount: int32(len(resident.positions) / 3), TriangleCount: int32(len(resident.positions) / 9), Vertices: &resident.positions[0], Texcoords: &resident.uvs[0], Colors: &resident.colors[0]}
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
	}
	r.stats.ResidentMeshes = len(r.meshes)
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
	if mode == levelmesh.Wireframe {
		rl.EnableWireMode()
		defer rl.DisableWireMode()
	}
	identity := rl.MatrixIdentity()
	for _, b := range r.active {
		r.material.GetMap(rl.MapAlbedo).Texture = r.textures[b.Key.Texture]
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
