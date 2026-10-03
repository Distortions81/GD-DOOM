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
levels; a GPU triangle renderer is a later step. The world pass currently omits
actors, items, projectiles and the panoramic sky. Sky surfaces reveal the blue
background. Use `-no-monsters` while inspecting geometry because simulation is
still running. Sector light is approximate RGB shading, without faithful
COLORMAP, distance lighting, animation crossfades or powerup color effects.

Floor and ceiling triangles come from the original sector boundary rings,
including concave outlines, holes and disconnected islands. Vertical slabs
split these rings into trapezoids and triangles. This avoids gaps and spills
between cached BSP subsector polygons: the initial E1M1 screenshot had 1,049
uncovered pixels despite every subsector having triangles. Sector-ring planes
cover all 256,000 pixels in the enclosed starting-room view at 640×400.

Sectors with unavailable or unsupported rings fall back to the existing
subsector triangle cache, including its hole-fill patches. The on-screen
**Plane fallbacks** count exposes this limitation; zero fallbacks is not a
guarantee that an arbitrary malformed level is watertight. The gameplay
renderer and its polygon cache are unchanged by this alternate plane builder.

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

# Report fallback sectors across every map in supplied IWADs.
GD_GEOMETRY_WADS=DOOM1.WAD,wads/DOOMU.WAD,wads/DOOM2.WAD \
  go test ./internal/doomruntime -run '^TestMeshExperimentMaps$' -v
```

Headless Linux runtime tests need an X display, for example `xvfb-run -a` before
`go test`. The pure `levelmesh` package needs no display. Unit checks cover
concavity, holes, islands, area, overlap, near clipping, winding, depth order,
transparent texels, perspective UVs, wall pegging and closed/open portals.
The E1M1 regression requires complete starting-room coverage and unchanged
simulation checksums across the three mesh views.

The initial audit covered 77 map variants: all nine shareware levels used
sector-ring planes throughout. Ultimate Doom required seven fallback sectors
across four levels; Doom II required 23 across ten levels. These counts identify
remaining inspection targets rather than proving complete geometry coverage.
The full Go suite, the opt-in Ebiten presentation check, and the WebAssembly
build passed.

On the test host (Ryzen 9 7950X), the enclosed E1M1 view measured:

| World buffer | Mesh build + CPU raster | Allocations after warmup |
| --- | ---: | ---: |
| 320×200 | 1.4 ms/frame | 0 |
| 640×400 | 4.1 ms/frame | 0 |
| 1280×800 | 15.7 ms/frame | 0 |

These are one-camera CPU measurements, excluding texture upload and game
presentation; larger maps and other viewpoints can cost more.
