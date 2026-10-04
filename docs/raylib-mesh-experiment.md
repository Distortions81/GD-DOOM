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
- R reloads the level at its normal player start. Escape exits.

This is a single-level geometry experiment with monsters disabled and an
invulnerable player. Movement, collision, sector thinkers, doors, lifts, pickups
and weapon state use the existing simulation. Actors, items, weapon sprites,
panoramic sky, audio, menus, automap, saves and campaign transitions have not
been ported to this host. A simple numerical HUD shows health, armor and ammo.
Sky surfaces reveal the background. Lighting uses per-sector RGB shading;
faithful palette and distance lighting remain future work.

Use `-width`, `-height`, `-fps` (0 means uncapped) and `-mode` to adjust the view.
The camera retains the CPU experiment's horizontal FOV and pixel aspect.

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
