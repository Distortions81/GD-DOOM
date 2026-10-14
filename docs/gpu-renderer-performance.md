# GPU renderer optimization checks

## Faithful mode

GPU world rendering is enabled by default in both modes. Faithful walls, planes,
masked textures, and sprites resolve texture indices through the WAD's 256-entry
COLORMAP rows and active gamma palette. Fullbright sky sampling uses the same
CPU column/row coordinates, uploaded as a small lookup image. Low detail copies
each even framebuffer column into the next column in a GPU presentation pass.
`-gpu-renderer=false` retains the software renderer. GPU spectre fuzz uses the
classic logical grid and row-six COLORMAP. Sprites, masked textures, and spectres
interleave by depth; feedback interrupted by wall clipping remains approximate.

Faithful software sprites now respect COLORMAP rows instead of the RGB shade
table, and indexed sky sampling respects the active gamma palette. Integration
checks verify every world pixel belongs to that palette, all 256 indices across
33 deliberately non-linear COLORMAP rows at three gamma settings, and low-detail
column equality. Doom/Doom II high- and low-detail comparisons remain within the
existing 1% pixel difference budget, including nearby spectres (maximum observed
Faithful difference 0.300%).

## Geometry runs

Sprite commands carry a visible rectangle, starting texture coordinates, and
texture steps. Consecutive rows with identical visible spans share a rectangle
when shader stepping selects the same texels as the original row commands.
Clipping changes and float rounding boundaries start a new run. This applies to
ordinary sprites, flipped sprites, debug overlays, and teleport puffs.

Spectres use opaque vertical posts on a fixed 320×200 grid. Each post carries
its logical column, starting row, and starting position in Doom's 50-entry fuzz
table. The shader reconstructs neighbor offsets and the short chains of repeated
darkening from the original downward, in-place draw loop. It uses the WAD's
COLORMAP row six in both modes when available. An exact active-palette hash
recognizes palette colors across gamma changes; Modern RGB backgrounds fall
back to the existing nearest-palette lookup.

Fuzz shading runs on a 320×200 GPU layer, then a cheap nearest-neighbor pass
scales it to the framebuffer while retaining native wall/portal clipping. The
mask, grain, sequence, and expensive shader work therefore remain independent
of output resolution. No framebuffer readback or per-pixel phase upload is used.
The original reference is [id Software's R_DrawFuzzColumn](https://github.com/id-Software/DOOM/blob/master/linuxdoom-1.10/r_draw.c).

Pixel checks compare the logical effect with the software path at 320×200,
640×400, and 1920×1080, including flipped masks, transparent gaps, phase wrap,
four-offset feedback chains, and multiple gamma settings. They also assert
that the larger outputs reproduce the same logical 320×200 pixels.

Frames containing spectres paint the sorted cutout queue from back to front.
The GPU groups consecutive ordinary cutouts into a pass and takes a fresh
background snapshot for each intervening spectre. This prevents a spectre behind
an enemy from blurring that enemy, and lets overlapping spectres sample one
another in the correct order. Software drawing disables front-to-back coverage
rejection for these frames. Frames without fuzz retain that optimization.
Regression checks cover enemies in front of and behind a spectre, alpha holes,
multiple overlapping spectres, and both modes at two resolutions.

A synthetic screen-filling opaque spectre uses 320 logical post quads plus one
presentation quad (65,484 vertex/index bytes) at all three tested sizes. CPU
preparation measured 0.330 ms at 320×200, 0.352 ms at 1920×1080, and 0.361 ms at
3840×2160, with zero allocations after warmup. Clipping and mask holes can add
posts; these measurements are isolated preparation costs, not whole-game FPS
or the cost of multiple ordered spectre snapshots.

Adjacent wall columns share geometry when their texture column, vertical
sampling, lighting, animation, and bounds match. Solid fills and sky copies
also coalesce along either axis. Command order and atlas page boundaries remain
intact. Varying perspective and lighting on walls and planes still use the
existing CPU decisions; this change does not reconstruct those across a surface.

The command benchmark reports logical vertex/index payload: each quad contains
four 48-byte vertices and six 2-byte indices. It excludes driver overhead and
resident texture data. Results below are synthetic cases, not whole-game FPS.

October 2, 2026, Go 1.26.6, Ryzen 9 7950X; sprite timings are medians of three runs:

| Sprite height | Original quads | Run quads | Original payload | Run payload | Original preparation | Run preparation |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 200 | 200 | 1 | 40,800 B | 204 B | 7.56 µs | 2.40 µs |
| 1080 | 1080 | 2 | 220,320 B | 408 B | 40.50 µs | 10.83 µs |

For 1,920 wall columns with matching vertical sampling and lighting, repeats of
8 columns reduce geometry from 1,920 to 240 quads (87.5% fewer payload bytes).
Repeats of 64 columns produce 30 quads (98.4% fewer). All command benchmarks
allocate zero bytes after warmup.

## Close-up visibility spans

With gameplay's per-column wall/portal buffers present, the original sprite
command path rescanned every covered column for every screen row. A CPU profile
of a screen-filling nearby sprite attributed 99% of samples to that scan. The
empty-buffer command benchmark above did not expose this cost.

GPU sprite commands and sprite plane occluders now collect the rows where a
nearer wall or portal changes visibility. Horizontal spans are computed at those
boundaries and reused between them. Closed columns and strict depth comparisons
remain unchanged. Incomplete buffers retain the original generic row path.

| Synthetic close-up workload | Original time | Cached-span time |
| --- | ---: | ---: |
| Sprite commands, 1920×1080 | 1.325 ms | 0.00593 ms |
| Sprite commands, 3840×2160 | 5.512 ms | 0.0118 ms |
| Sprite plane occluders, 3840×2160 | 5.803 ms | 0.0169 ms |

The original sprite-command values are one profiled run; cached values and
plane-occluder values are medians of three runs. All allocate zero bytes after
warmup. These measure CPU visibility work, not total frame time or GPU fill rate.

The expanded close-up comparisons also exposed overlapping texture-row bounds
in the CPU magnification path. It now respects the pixel-center row lookup;
a regression test checks fractional-scale sampling against the texture directly.

## Packed texture metadata

Texture metadata occupies two RGB texels instead of four RG texels. Each texel
packs an 11-bit atlas coordinate and an 11-bit dimension minus one. Values fit
exactly in shader floats and retain the supported 2,048-pixel atlas dimensions.
The second half of the metadata image stores the spectre palette lookup.

The draw benchmark waits for shader completion with a readback. These readbacks
and their allocations are benchmark-only. Mesa 26.1.3 llvmpipe uses software
rendering, so these results require confirmation on hardware GPUs and browsers.

The following 1080p results isolate metadata packing, before geometry run changes.
Baseline and packed binaries alternated for three runs, with `GOMAXPROCS=4`,
`LP_NUM_THREADS=4`, CPU affinity 0–7, and a one-second benchmark duration:

| Draw | Baseline median | Packed median | Time reduction |
| --- | ---: | ---: | ---: |
| Planes | 29.64 ms | 26.09 ms | 12.0% |
| Walls | 47.24 ms | 40.47 ms | 14.3% |
| Sprites | 10.42 ms | 8.68 ms | 16.6% |

## Vertex metadata and sky work

Unblended world draws now carry the two packed atlas-coordinate/dimension pairs
in the existing vertex channels. The fragment shader no longer reads two
metadata texels for each ordinary wall, plane, sprite, or Faithful sky pixel.
Vertices remain 48 bytes, and geometry, batch order, lighting, masks, and texture
coordinates are unchanged. Animated blends retain the original metadata lookup;
solid fills, sky copies, and spectre passes retain their existing encodings.
GPU comparisons exercise both encodings at texture-ID, atlas, and dimension
limits, including 2,048-pixel textures.

The final candidate was compared with a frozen binary from `3399c93` on
October 9, 2026, using Go 1.26.6, a Ryzen 9 7950X, and Mesa 26.1.3 llvmpipe
(LLVM 22.1.5). Three separate-process pairs alternated baseline/candidate,
candidate/baseline, then baseline/candidate. Each used a one-second benchmark,
`GOMAXPROCS=4`, `LP_NUM_THREADS=4`, CPU affinity 0–7, and Xvfb. These synthetic
Modern-mode draws isolate the ordinary indexed world path; they do not time
Faithful COLORMAP shading, sky, or fuzz.

| World draw | Baseline median | Vertex metadata median | Time reduction |
| --- | ---: | ---: | ---: |
| Planes, 320×200 | 1.026 ms | 0.895 ms | 12.8% |
| Walls, 320×200 | 1.528 ms | 1.255 ms | 17.8% |
| Sprites, 320×200 | 0.346 ms | 0.303 ms | 12.6% |
| Planes, 1920×1080 | 28.666 ms | 24.971 ms | 12.9% |
| Walls, 1920×1080 | 43.952 ms | 36.361 ms | 17.3% |
| Sprites, 1920×1080 | 9.848 ms | 7.723 ms | 21.6% |

Shaders were compiled and warmed before timing, and each draw synchronized with
a readback. Timings include that transfer and benchmark-only allocations.
All runs were retained, including a transient slowdown in the first candidate
process: at 1080p, candidate ranges were 23.990–33.326 ms for planes,
35.997–48.429 ms for walls, and 7.680–11.258 ms for sprites. These medians show
software-renderer microbenchmark results, not hardware-GPU or whole-game FPS
guarantees.

CPU command preparation was checked separately with three 200 ms repetitions
per binary. Heavily merged wall commands added about 3 µs: 24.782 → 27.670 µs
for eight-column repeats and 20.669 → 23.694 µs for 64-column repeats. The
measured sprite and wall cases retained zero allocations and identical quad
counts and vertex/index payloads.

Both complete native framebuffer suites passed for the world-metadata and
hidden-sky candidate. All 316 saved CPU/GPU images from Doom and Doom II matched
the frozen baseline pixel-for-pixel, including the real-map views and synthetic
cutout-order captures. This comparison is stricter than merely remaining within
the existing 1% CPU/GPU difference budget.

Frames with no visible sky commands skip the full-screen sky backdrop. The
snapshot lifecycle still supports indoor → outdoor → indoor transitions and
ordered spectres. The optional sharp sky path shares identical sample positions
while retaining the original filter arithmetic. The default GPU world path uses
point sky sampling, so this sample reuse applies to the separate sharp path.

The sky shader's original source is retained as an integration-only reference.
Its parity test compares every channel across 96 cases covering point/sharp
modes, wrapped seams, odd output sizes, non-power-of-two textures, a 1×1 texture,
and nonzero source-image origins. Both paths retain the original safe sampler;
switching to the unchecked sampler introduced small channel-rounding differences
on WebGL. At the shader source level, the sharp filter reduces texture-sampling
calls from 80 to 56. Compiler and driver optimizations determine the resulting
hardware work. The final safe-sampler version independently passes all 96 cases
byte-for-byte on native OpenGL and browser WebGL. Indoor frames avoid the sky
draw entirely.

Final safe-sampler measurements on October 9, 2026 used the same Go 1.26.6,
Ryzen 9 7950X, Mesa 26.1.3 llvmpipe, four Go/Mesa threads, and CPU affinity 0–7
as the world checks. Medians below retain all three separate-process runs, with
one-second benchmarks and variant order reversed between runs. Each draw was
warmed and synchronized with a benchmark-only readback.

| Sky draw | Original median | Cached-sample median | Time reduction |
| --- | ---: | ---: | ---: |
| Point, 320×200 | 0.195 ms | 0.199 ms | −2.5% |
| Sharp, 320×200 | 1.242 ms | 1.140 ms | 8.2% |
| Point, 1920×1080 | 5.316 ms | 5.294 ms | 0.4% |
| Sharp, 1920×1080 | 37.911 ms | 34.343 ms | 9.4% |

The point-sampling code is unchanged; its small timing variation does not
establish a performance gain. All three 1080p sharp-filter runs improved:
original times ranged from 37.307–38.315 ms and cached times from
33.905–34.503 ms. These software-renderer measurements include readback and do
not predict hardware-GPU performance or whole-game FPS.

## Running the checks

```sh
go test ./internal/doomruntime -run '^TestGPU' -count=1
go test ./internal/doomruntime -run '^TestMagnifiedSprite' -count=1
go test ./internal/doomruntime -run '^$' -bench '^BenchmarkGPU(Sprite|Wall)Commands$' -count=3
go test ./internal/doomruntime -run '^$' -bench '^BenchmarkGPUCloseSprite' -count=3
GD_GPU_INTEGRATION=1 GD_GPU_WAD=wads/DOOMU.WAD xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1 -v
GD_GPU_INTEGRATION=1 GD_GPU_WAD=wads/DOOM2.WAD xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1 -v
LP_NUM_THREADS=4 GOMAXPROCS=4 xvfb-run -a go test -tags integration ./internal/doomruntime -run '^$' -bench '^BenchmarkGPUIndexedDraw$' -benchtime=1s -count=1
GD_GPU_SKY_INTEGRATION=1 xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUSkyShaderParity$' -count=1 -v
LP_NUM_THREADS=4 GOMAXPROCS=4 xvfb-run -a go test -tags integration ./internal/doomruntime -run '^$' -bench '^BenchmarkGPUSkyDraw$' -benchtime=1s -count=1
```

Run draw benchmark repetitions in separate processes because Ebitengine starts
its game loop once per process. On a desktop with a working display, omit
`xvfb-run` to measure that display's renderer.

Keep the GPU and magnified-sprite unit groups in separate processes as shown;
their existing fixtures share global palette state and depend on initialization
order when selected together.

Framebuffer tests compare compressed sprite commands exactly with the original
row commands across seven scales and six sprite modes, including clipping
changes. They also check metadata ID/atlas limits and spectre palette lookup.
Doom E1M1/E1M5 and Doom II MAP11/MAP26 compare against the CPU renderer at four
orientations with gamma and invulnerability changes, plus camera positions 32
and 48 map units from enemies and barrels. All map comparisons must stay within
the existing 1% difference budget. Deterministic randomized tests compare cached
spans with every row across wall, closed-portal, open-portal, clipping-range, and
equal-depth cases.

For browser WebGL checks of the indexed world shader, build the integration test
binary and serve only its temporary fixture directory:

```sh
mkdir -p /tmp/gddoom-shader-webgl
GOOS=js GOARCH=wasm go test -c -tags integration \
  -o /tmp/gddoom-shader-webgl/doomruntime.test.wasm ./internal/doomruntime
cp internal/doomruntime/shaders/testdata/gpu_shader_probe.html /tmp/gddoom-shader-webgl/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /tmp/gddoom-shader-webgl/
python3 -m http.server 18082 --bind 127.0.0.1 --directory /tmp/gddoom-shader-webgl
```

Open `http://127.0.0.1:18082/gpu_shader_probe.html` and select the indexed world
or sky suite. Reload between suites. This uses
real browser shader compilation and pixel readback, including palette, cutout,
animation, metadata precision, and sky visibility checks. It does not claim
whole-game FPS measurements. Run each Ebitengine integration test in its own
process or page. The indexed world suite and the final safe-sampler sky suite
both pass these browser WebGL checks.

The full framebuffer suite also exposes a pre-existing WebGL spectre scaling
discrepancy at 1920×1080: the clean `3399c93` baseline and optimized shader both
produce the same first failing fuzz case. The indexed browser fixture isolates
the world paths changed here; the full native suite continues to check spectre
feedback and draw order.
