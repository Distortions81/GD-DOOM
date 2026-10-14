# In-game 3D mesh experiment

On the `experiments` branch, launch an explicitly selected level with:

```bash
go run . -wad DOOM1.WAD -map E1M1 -mesh-renderer textured -no-monsters
```

Press **Tab** to leave the automap and enter the world view. Normal movement,
use, weapon controls and HUD remain active. **F7** cycles textured → sector
colors → wireframe → classic renderer → textured. The flag also accepts
`sectors` or `wireframe` as the initial view. Omitting it uses normal rendering.

This is a geometry inspection prototype. It rasterizes actual triangles on the
CPU with a depth buffer, backface/frustum clipping, perspective-correct UVs and
masked texture holes, then uploads the frame to Ebiten. Start at modest detail
levels. A separate [native Raylib-Go GPU experiment](raylib-mesh-experiment.md)
draws the same geometry with resident mesh buffers and hardware depth testing.
The world pass currently omits
actors, items, projectiles and the panoramic sky. Sky surfaces reveal the blue
background. Use `-no-monsters` while inspecting geometry because simulation is
still running. Sector light is approximate RGB shading, without faithful
COLORMAP, distance lighting, animation crossfades or powerup color effects.

Floor and ceiling triangles now use the same directed original linedef sides
as the textured automap. Horizontal slabs split at endpoint heights and edge
intersections; intervals with nonzero winding become trapezoids and triangles.
Concave outlines, pillar holes, islands, overlapping regions and opposite
internal sides work without tracing closed rings. This restores E1M3's normal
exit room: its overhanging line endpoint caused the ring tracer to discard the
outer boundary while retaining three inner outlines. Both the room's floor and
ceiling now cover their original wall-defined interior.

No vertices are snapped, boundaries synthesized, or gameplay BSP/cache data
changed. The two renderers share the original edge cache; mesh topology remains
fixed while sector heights update each frame. Empty or isolated sides produce
no plane. Actual triangulation errors retain the old subsector cache fallback
and increment the on-screen **Plane fallbacks** count.

The fill follows the minimap's finite-crossing policy. Open boundary rows do
not establish a complete floor, and zero fallbacks does not prove support for
open-sector rendering tricks. Proven GL-node construction and rendering-sector
handling remain future work for those maps.

Walls are quads split into triangles along original linedefs. Both sidedefs
retain their own materials and inward-facing winding. Portals add upper,
lower and masked middle strips; sky-to-sky upper walls disappear. UVs preserve
texture offsets, row offsets, scrolling and top/bottom pegging. Masked middle
textures repeat across the portal before alpha testing, matching the project's
existing column renderer. This closes E1M3's eight-unit bottom gaps where a
64-unit `BRNSMALC` texture spans a 72-unit opening, while preserving transparent
texels in repeated fences and grates.
Missing textures appear as a magenta checkerboard. Sector heights and current
texture frames are resolved each draw, so doors, lifts and switches can change
the mesh while playing. Horizontal topology is cached for the loaded map.

## Validation

```bash
go test ./internal/render/levelmesh
go test ./internal/doomruntime ./internal/app -run 'TestMeshExperiment|TestRunParseMesh'

# Actual Ebiten framebuffer, with optional PNG captures.
GD_MESH_INTEGRATION=1 GD_MESH_CAPTURE_DIR=build/mesh-captures \
  go test -tags integration ./internal/doomruntime -run '^TestMeshExperimentPresentation$'

# CPU mesh construction and raster cost; excludes upload, HUD and presentation.
go test ./internal/doomruntime -run '^$' -bench '^BenchmarkMeshExperimentE1M1$'

# E1M3 exit room, normal resolution, with before/after PNG captures.
GD_MESH_INTEGRATION=1 GD_MESH_CAPTURE_DIR=build/mesh-captures \
  go test -tags integration ./internal/doomruntime -run '^TestMeshExperimentE1M3ExitRoomPresentation$'

# Audit plane coverage across every map in supplied IWADs.
GD_GEOMETRY_WADS="$PWD/DOOM1.WAD,$PWD/wads/DOOMU.WAD,$PWD/wads/DOOM2.WAD" \
  go test ./internal/doomruntime -run '^TestMeshExperimentMaps$' -v
```

Headless Linux runtime tests need an X display, for example `xvfb-run -a` before
`go test`. The pure `levelmesh` package needs no display. Unit checks cover
concavity, holes, islands, area, overlap, near clipping, winding, depth order,
transparent texels, perspective UVs, wall pegging and closed/open portals.
The E1M1 regression requires complete starting-room coverage and unchanged
simulation checksums across the three mesh views.

The directed-side audit covers 77 map variants and verifies 422,766 bounded
samples against independent winding queries on the original sidedefs, with
no missing or overlapping triangles and no fallback sectors. It reports 342
open-boundary samples separately rather than claiming their unbounded winding
represents valid floor geometry.

The E1M3 regression checks 52,948 exit-room, pillar and exterior samples and
both plane heights and UVs. Independent pixel-center rays verify floor and
ceiling coverage, depth and texture sampling at 1280×800. The actual in-game
framebuffer captures show the room facing the exit stairs, recovering 375,600
floor-region and 86,254 ceiling-region pixels from the old empty background.

Before the directed-side update, the test host (Ryzen 9 7950X) measured the
enclosed E1M1 view at:

| World buffer | Mesh build + CPU raster | Allocations after warmup |
| --- | ---: | ---: |
| 320×200 | 1.4 ms/frame | 0 |
| 640×400 | 4.1 ms/frame | 0 |
| 1280×800 | 15.7 ms/frame | 0 |

These are one-camera CPU measurements, excluding texture upload and game
presentation; larger maps and other viewpoints can cost more.
