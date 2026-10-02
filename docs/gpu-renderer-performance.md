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

## Running the checks

```sh
go test ./internal/doomruntime -run '^TestGPU|^TestMagnifiedSprite' -count=1
go test ./internal/doomruntime -run '^$' -bench '^BenchmarkGPU(Sprite|Wall)Commands$' -count=3
go test ./internal/doomruntime -run '^$' -bench '^BenchmarkGPUCloseSprite' -count=3
GD_GPU_INTEGRATION=1 GD_GPU_WAD=wads/DOOMU.WAD xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1 -v
GD_GPU_INTEGRATION=1 GD_GPU_WAD=wads/DOOM2.WAD xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1 -v
LP_NUM_THREADS=4 GOMAXPROCS=4 xvfb-run -a go test -tags integration ./internal/doomruntime -run '^$' -bench '^BenchmarkGPUIndexedDraw$' -benchtime=1s -count=1
```

Run draw benchmark repetitions in separate processes because Ebitengine starts
its game loop once per process. On a desktop with a working display, omit
`xvfb-run` to measure that display's renderer.

Framebuffer tests compare compressed sprite commands exactly with the original
row commands across seven scales and six sprite modes, including clipping
changes. They also check metadata ID/atlas limits and spectre palette lookup.
Doom E1M1/E1M5 and Doom II MAP11/MAP26 compare against the CPU renderer at four
orientations with gamma and invulnerability changes, plus camera positions 32
and 48 map units from enemies and barrels. All map comparisons must stay within
the existing 1% difference budget. Deterministic randomized tests compare cached
spans with every row across wall, closed-portal, open-portal, clipping-range, and
equal-depth cases.
