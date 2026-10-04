# Raylib GPU mesh experiment

The `experiments` branch has a native Raylib-Go host for the 3D level mesh. It
drives the existing Doom simulation at 35 Hz and draws the repaired directed-side
floor/ceiling triangulation, walls and masked middle textures using OpenGL depth
testing. The window defaults to 1280×800 and renders at the window resolution.

From the repository root on Linux:

```bash
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3

# Build once for subsequent runs.
go build -tags raylib,x11 -o build/raydoom ./cmd/raydoom
./build/raydoom -wad DOOM1.WAD -map E1M3
```

This backend requires cgo, a C compiler, OpenGL 3.3 and platform window-system
development libraries. Raylib-Go v0.60.1 is pinned in `go.mod`. On Linux the
`x11` build tag selects X11; see the upstream [build prerequisites and platform
instructions](https://github.com/gen2brain/raylib-go/tree/v0.60.1/raylib).
The mesh ownership code uses the cgo bindings; `CGO_ENABLED=0` is unsupported.
The ordinary application and WASM entry points continue to use Ebiten.

## Controls and scope

- WASD moves; up/down also move forward/backward.
- Mouse or left/right arrows turn; Shift runs.
- E or Space uses doors and switches. Left mouse or Ctrl fires; 1–7 selects weapons.
- F7 cycles textured, sector colors and wireframe views.
- F8 cycles nearest, trilinear and 8x anisotropic texture filtering.
- F9 cycles Doom lighting, sector-only lighting and fullbright.
- R reloads the level at its normal player start. Escape exits.

This is a single-level geometry experiment with monsters disabled and an
invulnerable player. Movement, collision, sector thinkers, doors, lifts, pickups
and weapon state use the existing simulation. Actors, items, weapon sprites,
panoramic sky, audio, menus, automap, saves and campaign transitions have not
been ported to this host. A simple numerical HUD shows health, armor and ammo.
Sky surfaces reveal the background. Doom-style sector and distance lighting
use RGB shading; faithful palette remapping remains future work.

Use `-width`, `-height`, `-fps` (0 means uncapped) and `-mode` to adjust the view.
The camera retains the CPU experiment's horizontal FOV and pixel aspect.

## Seam antialiasing

The native window requests **4x MSAA** by default. Multisample coverage smooths
geometry silhouettes and boundaries between adjacent materials or sector
lights, while leaving texture interiors sharp. It works with the existing
depth buffer and resident meshes; no blur shader, extra draw pass or mesh
upload is needed. The window draws directly to its multisampled framebuffer.
Texture UVs use centroid interpolation to avoid sampling outside covered
triangles at partially covered pixels. Screenshot captures explicitly resolve
the window samples into a single image before reading them back.

Use `-msaa=false` for a comparison or to reduce framebuffer cost. The setting
is applied before window creation, so changing it requires restarting. Actual
sample availability depends on the graphics driver; this is Raylib's 4x MSAA
hint. Multisampling adds framebuffer storage and sample/depth work, with no
change to geometry or texture memory. Alpha-tested texture cutouts still use
the existing threshold; this pass smooths geometry coverage.

## Doom-style lighting

The default `-lighting doom` adds view-distance shading to the existing animated
sector lights. Walls and planes use their respective Doom light-table curves,
normalized to the original 320-wide basis so resolution changes do not change
the falloff. Horizontal walls have Doom's darker axis bias, vertical walls
its brighter bias, and diagonal walls no bias. Both sides of a wall keep the
same axis classification while retaining their own sector light.

The shader selects a continuous shade row per fragment using view-forward
depth, then interpolates a 32-row RGB brightness ramp derived from the WAD's
PLAYPAL/COLORMAP data. This follows the project's Source Port lighting math
and preserves filtered full-color textures. It approximates colormap brightness;
it does not reproduce each palette color's exact remapping or vanilla's
quantized light buckets. Masked walls retain alpha-tested texture holes.

```bash
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting doom

# Comparisons; F9 also switches between these views while playing.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting sector
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 -lighting fullbright
```

Flicker, strobe, glow and switch-driven changes use the original simulation
and interpolated sector-light cache. Sector light is carried in vertex colors;
static surface metadata uses the second UV attribute. Camera motion and F9
comparisons only change uniforms, without rebuilding GPU meshes or increasing
the texture batches. An active light-amplification pickup overrides distance
shading and retains its original expiration blink. Muzzle-flash extra light,
invulnerability's palette effect and custom fixed colormaps remain future work.

The original references are id Software's [wall lighting and axis
bias](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_segs.c),
[plane lighting](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_plane.c)
and [light-table setup](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_main.c).

## Texture enlargement and filtering

The native launcher defaults to **2x texture dimensions with 8x anisotropic
filtering over a trilinear mip chain**. Enlargement duplicates each source pixel
into a 2x2 block before upload, so original colors and sharp nearby detail are
preserved. It adds no new artwork or detail. GPU UVs still use the original
dimensions: texture repeats, offsets and wall pegging remain the same size in
the world.

```bash
# Current default; anisotropic sampling retains detail at oblique angles.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 2 -texture-filter anisotropic

# Compare mipmapped trilinear sampling.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 2 -texture-filter trilinear

# Original unfiltered textures, for comparison.
go run -tags raylib,x11 ./cmd/raydoom -wad DOOM1.WAD -map E1M3 \
  -texture-scale 1 -texture-filter nearest
```

Scale accepts 1 or 2. F8 changes the filter without reuploading meshes or texture
images; mipmaps are generated once on the first filtered use. Nearest samples
only the base level, even after toggling from a mipmapped filter. Anisotropic
filtering depends on driver support; trilinear remains the base filter if the
requested anisotropy is unavailable. Raylib reports unsupported hardware or
limits in the terminal.

Filtered middle-wall cutouts use edge colors under transparent texels, with
wrap-aware propagation across repeat seams. Their original alpha remains
unchanged, so black transparent RGB cannot darken the visible edges. Masked and
opaque uses have separate GPU images when sharing a source texture. Mipmapped
alpha still uses the existing 0.5 cutoff; very thin cutouts can lose coverage
at a distance. Alpha-coverage-preserving mipmaps would be a further experiment.

Doubling both dimensions costs 4x the base texture storage; a full mip chain
adds roughly another third (about 5.3x the original base image for square
textures). Draw-call and geometry counts are unchanged.

## GPU storage

Triangles are grouped by resolved texture and masked/opaque status. Sector
lighting lives in vertex colors, so sectors sharing a texture share a draw
call. Mesh and texture buffers stay resident after upload. Each frame reuses
CPU staging arrays and compares positions, UVs and colors; only changed buffers
are sent to the GPU. Changing topology recreates the affected mesh. Animated
texture frames retain their own cached batches. Hardware depth and alpha
discard make transparent texture holes independent of draw order.

The inspected E1M3 exit-room view has 4,468 non-sky triangles in 73 batches.
Moving the camera alone requires no geometry upload. Sector-light animation
can update color buffers, and doors/lifts update positions. Mesh construction
and batch comparison still run on the CPU; this is not a measured hardware
speedup claim. Frustum/BSP visibility culling, indexed vertices and retaining
unchanged CPU geometry are useful follow-up optimizations.

Horizontal mesh topology uses the [same directed original linedef-side fill
as the CPU experiment](mesh-renderer-experiment.md). Its documented limitations
for open boundaries and rendering tricks also apply here.

## Validation and captures

```bash
go test ./internal/render/raymesh
go test ./internal/doomruntime -run '^TestNativeMesh'

# Actual OpenGL depth, alpha discard and resident-buffer update regression.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibResidentMeshDepthAndMask$' -v

# GPU lighting versus the existing independent wall/plane lighting functions.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/doomruntime -run '^TestNativeMeshLightingMatchesDoom$' -v

# Multisampled shared seams, silhouettes and unchanged material interiors.
GD_RAYLIB_INTEGRATION=1 go test -tags raylib,x11,integration \
  ./internal/render/raymesh -run '^TestRaylibMultisampledSeams$' -v

# Reproduce the formerly missing E1M3 exit-room floor and ceiling.
./build/raydoom -wad DOOM1.WAD -map E1M3 \
  -camera=-600,-1600,89,0 -frames 3 \
  -capture build/raylib-captures/e1m3-exit-room.png
```

On headless Linux, prepend `xvfb-run -a` to windowed tests or captures. The
OpenGL regression checks a foreground cutout against a background wall,
reverses triangle submission order, verifies that unchanged geometry causes
zero uploads, then moves the foreground behind the wall and checks the changed
framebuffer. Native host tests cover fixed-tic movement, render-only simulation
checksums, destination-sector camera physics, opening a real E1M1 door and
changing floor heights.

Texture tests check exact 2x pixel/alpha duplication, unmodified source images
and repeating cutout edge colors. The OpenGL regression checks doubled GPU
image dimensions and mipmaps, exercises all three filters without mesh
uploads, verifies that cutouts retain foreground/background pixels without
dark fringes, and returns to the original nearest-filtered framebuffer.
A minified checkerboard framebuffer check verifies that trilinear and
anisotropic sampling use averaged mip levels, while nearest samples the
original black/white texels.

Lighting checks compare actual framebuffer pixels at three wall distances and
three wall orientations against the existing Doom functions. Pixel-center rays
verify the lighting of a real floor at three distances. They also cover
sector/fullbright comparisons, the powerup override, non-linear uploaded row
ramps and zero geometry uploads when switching lighting. Native host tests
check the WAD-derived ramp, animated E1M3 sector lights and the light-amp blink.

The MSAA framebuffer check draws two coplanar materials sharing a diagonal
edge. It verifies intermediate coverage at the seam and silhouette, no
background samples leaking through the shared edge, unchanged colors inside
both materials, and identical output after reversing submission order.
