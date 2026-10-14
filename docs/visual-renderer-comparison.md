# Visual comparison against classic Doom

`scripts/visual_compare.py` captures the original Linux Doom renderer, GD-DOOM's CPU renderer, and GD-DOOM's GPU renderer at identical fixed cameras. It creates an HTML viewer with three-way images, amplified RGB difference maps, an opacity overlay, and numerical comparisons. Camera manifests can be edited and replayed.

## Run

Requirements: Python 3 with Pillow, Go, `xvfb-run`, a working OpenGL implementation, and a buildable Linux Doom source checkout. The reference build uses its existing Makefile; the bundled workflow's 32-bit build needs GCC multilib and 32-bit X11 development libraries. Supply your own IWAD.

```sh
python3 scripts/visual_compare.py --wad DOOM1.WAD --maps E1M1,E1M2,E1M3
python3 scripts/visual_compare.py --wad wads/DOOM2.WAD --maps MAP01,MAP11,MAP26 --out tmp/visual-compare-doom2
```

The default reference source is `../doom-source/linuxdoom-1.10`. Override it with `--reference-source`. The script copies that checkout to a temporary directory, installs a capture-only entry point, builds it, and removes the copy when finished. It never edits the original checkout or user game configuration. Its patch anchors fail explicitly on unsupported source layouts. At 320×200 the original world rasterizers are unchanged; window polling is disabled for the headless capture. The probe restores Ultimate Doom’s episode sky after `G_InitNew`: Linux 1.10’s `G_DoLoadLevel` incorrectly compares `gamemode` with `gamemission` enum values (`retail == pack_tnt`), overwriting it with SKY1. This setup normalization is recorded in report provenance. After capturing images, the reference emits 384 scalar sampling cases by calling its original `R_MapPlane` and `R_ScaleFromGlobalAngle` and reading `xtoviewangle`. The harness checks those values against GD-DOOM and fails on a mismatch. A checked-in copy also runs without a reference checkout.

Open `tmp/visual-compare/index.html`. Everything needed by the viewer is local; no CDN or service is required. `results.json` records the IWAD hash, GD-DOOM revision and dirty state, hashes of reference rendering sources, camera coordinates, and metrics. Each scene also has a `*-comparison.png` contact sheet.

Use `--lighting` to retain original COLORMAP lighting. Identity-colormap fullbright is the default so lighting does not obscure coordinate differences. Use `--disable-lighting` to exercise GD’s actual `DisableDoomLighting` option against classic with identity colormaps. This also tests the runtime’s separate unlit CPU/shader paths. `--max-gpu-diff` changes the CPU/GPU failure threshold (default 1% of pixels differing by more than two RGB levels). Use `--require-gpu-parity` for exact CPU/GPU RGB equality, including differences of only one level. Classic differences remain diagnostic unless a classic parity assertion is enabled.

Use `--trace-spans` to export each renderer's plane spans (material, height, row, start/end column, 16.16 origin and steps) and wall ranges. Read-only hooks run before the original C rasterizers; the GD trace reconstructs coordinates from the completed frame's actual visplanes and shared sampler. `span-differences.json` compares coordinates and wrapped texel indices, separately counting coverage/material differences. `--require-plane-parity` enables tracing and fails on even a one-unit fixed-point difference at pixels where both renderers draw the same flat at the same height. This assertion does not require equal coverage or lighting. Reanalyze saved captures with `python3 scripts/visual_compare/trace_spans.py REPORT_DIRECTORY`.

Use `--trace-pixels` to record classic's final pixel owner, source palette index, colormap row, sampled offset, and available source bytes. Kinds are 1=opaque wall, 2=flat, 3=masked wall, 4=sky. The reference verifies every traced result against its actual framebuffer before writing the CSV. `pixel-differences.json` separates exact RGB mismatches from proven out-of-buffer reference reads. `--require-bounded-parity` enables this trace and requires exact classic/CPU pixels except those proven reads; unknown source bounds are not exempt. All image metrics still include these pixels. `python3 scripts/visual_compare/trace_pixels.py REPORT_DIRECTORY --require-bounded-parity` checks retained captures.

To replay or adjust specific views:

```sh
python3 scripts/visual_compare.py --wad DOOM1.WAD --maps E1M3 \
  --scenes my-e1m3-scenes.json --out tmp/visual-e1m3
```

Copy entries from `scenes.json` into the input array; retain only maps listed in `--maps`. Coordinates X/Y/Z are map units; `angle` is an unsigned Doom angle (0=east, 1073741824=north). `tic` is the number of completed special updates. Scene names must be unique filename-safe identifiers.

## What is controlled

- Configurable world framebuffer (default native 320×200), raw PLAYPAL colors (GD gamma zero), fixed camera and absolute eye height. Classic display gamma is bypassed because its table zero adds one to low channels; normalizing the palette prevents display brightness from masquerading as coordinate error.
- Identical IWAD, no HUD or weapon, no rendered actors, no camera interpolation.
- Geometry and lighting stay at initial map state. Only texture specials advance; this is a renderer test, not a gameplay/demo synchronization test.
- Classic `P_UpdateSpecials` runs at leveltime 0 through N−1. Animation translations are reset before each independent scene.
- GD-DOOM uses the application's WAD-derived animation sequences and real world draws. GPU fallback fails the capture.
- Automatic views cover spawn orientations, scrolling walls, animated surfaces, sidedef offsets, pegging flags, masked middles, upper/lower tiers, and sky portals when present. Floor-animation cameras use a lower eye height to expose foreground flats. A camera facing a wall does not guarantee every nearby surface is visible; use the viewer and edit the manifest for focused cases.

The synthetic GPU tests additionally compare scrolling against explicit classic-style mutation of the first sidedef, across front/back sides, masked/opaque walls, negative offsets, and wrap boundaries. They require exact CPU/GPU pixel equality for those fixtures. The standalone sky shader suite checks 96 combinations of dimensions, seams, angles, scaling, and filtering against its independent shader reference.

## Coordinate audit findings (2026-10-14)

Fixed shared CPU/GPU errors:

| Area | Error | Correction |
| --- | --- | --- |
| Special 48 | Both sides of a two-sided line scrolled | Apply the displacement only to the first sidedef, as `P_UpdateSpecials` does |
| Back-side walls | U reversed twice, mirroring textures | Advance U along the directed SEG; the WAD already reverses back-side endpoints |
| Floors and ceilings | Flat V followed positive world Y | Use `V = -worldY`, matching `R_MapPlane` |
| Masked middle walls | A short texture repeated vertically across a taller opening | Clip to one texture height, matching `R_DrawMaskedColumn`; apply to generic CPU, optimized CPU, GPU, and experimental mesh paths |
| Faithful animation timing | The first eight-tic frame transition occurred after eight updates | Use the leveltime seen by `P_UpdateSpecials`; transition after update nine and invalidate the flat cache on the same schedule |

The mesh renderer receives the same scrolling, flat orientation, and masked-height corrections. Source Port's interpolated animation timing remains as before.

The old wall parity test reversed back-side U in its own reference and delegated its vertical-anchor reference back to the implementation helper. Those checks now use the directed-seg convention and an independent transcription of classic anchors.

Follow-up parity fixes:

| Area | Correction |
| --- | --- |
| Patch Y offsets | Count covering patches per column. Ignore origin Y for directly referenced single-patch columns; apply origins for composited columns. Indexed and RGBA builds share this logic. Transparent posts still count toward coverage. |
| Animation phase | Build per-alias cycles from absolute WAD texture/flat indices. Include nested flat marker lumps in those indices. Preserve identity translation before the first update and invalidate the animation cache when leaving that initial state. |
| Wall sampling | Faithful uses WAD SEG angles, classic fine-angle tangents, fixed-point distance/U calculations, interpolated fixed-point scales, integer screen rows, and the original reciprocal scale. Generic CPU, optimized masked CPU, and GPU paths share column stepping. |
| Wall boundaries | Faithful uses classic's half-open horizontal range and 20.12 vertical edge stepping. Boundary changes also correct the starting columns of adjoining flat spans. |
| Flat sampling | Faithful computes each span's origin from the original angle/distance tables and uses fixed-point row steps, including signed 32-bit intermediate wrapping near the horizon. |
| Sky sampling | Faithful uses `xtoviewangle` and the original angle-to-sky shift. Source Port keeps its existing projection/filtering. |

Source Port retains its pixel-center grid, interpolation and crossfade timing; texture assembly and WAD-derived animation phases are shared asset corrections.

### Follow-up comparison results

Same camera manifests and IWAD hashes as the preceding audit, at 320×200. Percentages are the mean fraction of pixels per scene with a maximum RGB-channel difference greater than two levels, comparing classic with CPU:

| Sweep | Captures | Before | After | Exact classic matches |
| --- | ---: | ---: | ---: | ---: |
| Doom I: E1M1, E1M2, E1M3 | 96 | 33.621% | 0.956% | 55 |
| Doom II: MAP01, MAP11, MAP26 | 98 | 28.492% | 0.535% | 51 |
| Selected Doom I views, original lighting | 3 | 38.507% | 13.417% | 0 |

CPU and GPU are byte-identical in all 197 captures. All 384 scalar reference cases pass. The full CPU/GPU integration suites also pass for both IWADs; their wider coverage, which includes actors and other resolutions, uses its existing tolerance. The separate sky shader suite passes all 96 cases exactly.

The fullbright reports are `tmp/visual-parity-doom1/index.html` and `tmp/visual-parity-doom2/index.html`; the lighting report is `tmp/visual-parity-lit/index.html`. Earlier `tmp/visual-compare-*` reports remain available as the baseline. E1M3's `offset-line45-side0` TEKWALL5 view now matches classic exactly.

Residual differences remain: the worst fullbright views differ at 7.375% (Doom I) and 7.639% (Doom II) of pixels. Scalar sampling parity does not imply identical visibility, visplane partitioning, or masked-post rasterization; the image reports retain these differences for further investigation. Lighting differences remain visible with the original colormaps. These runs do not establish complete pixel parity.

Nonstandard PWAD behavior still needs dedicated coverage: duplicate texture definitions, unusual flat-marker layouts, non-power-of-two texture widths, and the original opaque column loop's 128-row wrap convention. GD-DOOM continues to support general dimensions and decoded transparency rather than reproducing unsafe reads from malformed patch data.

The built-in IWAD sweeps are representative, not exhaustive coverage of every map, material, save/load state, or PWAD. The reference is the local Linux Doom source checkout, not a DOS executable. Source hashes in each report identify the exact oracle.

### Floor span trace and correction

The small floor error was caused by different span starts feeding classic's quantized angle/distance calculation. For example, in `MAP01-spawn-0-t000` at pixel `(125,150)`, classic's span started at column 97, while GD's started at 109. Both advanced U by 115329 fixed units per pixel, but the different origins produced U values of −10345448 versus −10354944: about 0.145 texel apart, selecting flat texel 1442 versus 1441. Changing a global floor/ceil rounding rule would not correct this cause.

Faithful now retains classic's current floor/ceiling visplane within each subsector. A new subsector selects the first matching plane; an overlap allocates a new split, rather than searching other existing splits for space. Plane checks see each wall's solid-clipped visible fragment, including pass-through walls. BSP occlusion uses the same quantized, half-open column bounds as wall drawing. The additional vertical-face culling shortcut is restricted to Source Port: even a hidden wall face can contribute visible plane ranges and change subsequent clipping. This can process more wall ranges in Faithful, but preserves classic's plane partitioning.

The trace also exposed a harness error: eight decimal places could move the C reference camera by one 16.16 unit when parsed and truncated. Camera serialization now uses 17 significant digits to preserve the exact binary coordinates.

Final reruns with `--require-plane-parity`, using the same manifests and IWAD hashes:

| Sweep | Captures | Previous mean image difference | New mean | Exact classic images |
| --- | ---: | ---: | ---: | ---: |
| Doom I, fullbright | 96 | 0.956% | 0.442% | 71 |
| Doom II, fullbright | 98 | 0.535% | 0.010% | 86 |
| Selected Doom I, original lighting | 3 | 13.417% | 11.776% | 0 |

No scene regressed. All 197 CPU/GPU images are byte-identical. All 3,382,716 shared floor/ceiling pixels have exactly matching fixed-point UVs and texel indices. Coverage/material differences are zero in Doom II and the lit views; 140 pixels remain in Doom I's `E1M2-pegged-line97-side0` view. The largest remaining fullbright image differences are 5.823% in Doom I and 0.267% in Doom II; these runs do not claim complete renderer parity.

Current reports: `tmp/visual-spans-doom1/index.html`, `tmp/visual-spans-doom2/index.html`, and `tmp/visual-spans-lit/index.html`. The preceding `tmp/visual-parity-*` reports remain as the before comparison. The full Go test suite, both IWAD GPU framebuffer suites, 96 sky shader cases, 384 scalar sampling cases per sweep, and five Python harness checks pass. New regressions cover visplane ownership/splitting, clipped ranges, BSP occlusion bounds, fixed-point wrapping, and subtexel errors invisible in RGB comparisons.

## Coverage and lighting follow-up

The next trace started with the worst Doom I views and corrected:

- **Negative composite patch origins:** `R_DrawColumnInCache` reduces the copy count when clipping above the texture, without advancing the source pointer. Preserving this behavior fixes TEKWALL1's first 16 rows and makes the worst scrolling view exact (previously 5.823% different).
- **Light-table quantization:** Faithful truncates sector light to its bucket, quantizes wall scale / plane distance, then performs integer light-table division. The previous interpolated calculation selected different colormap rows. Masked walls now also use directional bias and per-column scale, rather than a constant segment light. Source Port retains its interpolated lighting.
- **Viewport-edge visibility:** Faithful uses `R_CheckBBox`'s corner selection and angle-table clipping. A float frustum check rejected the E1M2 subsector contributing column zero.
- **Masked post coverage:** retain post origins and source bytes, project each post using fixed scale, and sample relative to it with the reciprocal scale. A first fractional coordinate slightly below zero must select byte 127. Save the wall traversal's clip rows for the later masked pass instead of recomputing them from float depth. CPU and GPU share these post bounds.
- **Ceiling and sky coverage:** remove the redundant depth rejection of already-clipped Faithful visplanes, which left black boundary rows. Match classic's sky-height substitution before deciding whether to mark a ceiling; the former extra mark overwrote 27 valid wall pixels in `E1M1-pegged-line26-side0`.
- **Opaque column layout:** Faithful's opaque sampler uses the original 128-row mask and bounded patch/composite bytes, including bytes past a short post within its containing buffer. Masked rendering keeps its separate post stream. Both application asset-loading paths supply this metadata; the test harness uses the same representation.

Final 320×200 reruns use both `--require-plane-parity` and `--require-bounded-parity` on the same six maps, manifests and IWAD hashes:

| Sweep | Captures | Mean image difference (>2 RGB levels) | Entire images exact |
| --- | ---: | ---: | ---: |
| Doom I, fullbright | 96 | 0.001156% | 92 |
| Doom II, fullbright | 98 | 0% | 98 |
| Doom I, original lighting | 96 | 0.001139% | 92 |
| Doom II, original lighting | 98 | 0% | 98 |

All 388 CPU/GPU images are byte-identical. Floor/ceiling coverage, materials and fixed-point UVs match in all captures. There are no bounded-source pixel differences, and no fullbright scene regressed from `tmp/visual-spans-*`.

The remaining Doom I discrepancies are **71 pixels fullbright / 70 lit**, across four scenes: `E1M1-tiers-line79-side0`, `E1M2-spawn-2`, `E1M2-tiers-line9-side0`, and `E1M2-pegged-line97-side0`. The trace proves these are reads beyond the original patch lump or composite allocation. For example, the STEP6 sample at `(123,109)` selects byte 127 with only 96 bytes left in its composite buffer. GD-DOOM deliberately supplies zero for such reads instead of copying unrelated process memory. The traces record 99 out-of-buffer reference reads per Doom I sweep; some happen to produce the same displayed color. These exceptions remain visible in the reports and are the only exclusions from the bounded parity assertion.

Latest reports are `tmp/visual-coverage-doom1/index.html`, `tmp/visual-coverage-doom2/index.html`, `tmp/visual-coverage-doom1-lit/index.html`, and `tmp/visual-coverage-doom2-lit/index.html`. The four lit/fullbright reports include complete pixel and span traces. The reference now exports and validates all 2,816 wall/plane light-table entries, with checked-in scalar fixtures also testing values inside each bucket. Full Go tests, both IWAD GPU integration suites, 96 sky shader cases, and six Python harness tests pass. The worst masked lighting cases and the former black ceiling seams are now exact. This establishes parity for these captures, not exhaustive coverage of every map, resolution or PWAD.

To repeat a full lit sweep with both assertions:

```sh
python3 scripts/visual_compare.py --wad DOOM1.WAD --maps E1M1,E1M2,E1M3 \
  --scenes tmp/visual-spans-doom1/scenes.json --lighting \
  --require-plane-parity --require-bounded-parity --out tmp/visual-coverage-doom1-lit
```

## Expanded map and resolution matrix

```sh
python3 scripts/visual_matrix.py --doom1 wads/DOOMU.WAD --doom2 wads/DOOM2.WAD
python3 scripts/visual_compare.py --wad wads/DOOM2.WAD --maps MAP15,MAP31,MAP32 \
  --resolution 426x240 --lighting --require-gpu-parity --out tmp/visual-widescreen
```

The matrix runs all **36 Ultimate Doom maps and 32 Doom II maps** at 320×200. At 256×128, 400×250, 426×240, and 640×400 it runs eight maps from each game:

- Doom I: E1M1, E1M5, E2M1, E2M8, E3M1, E3M3, E4M1, E4M6.
- Doom II: MAP04, MAP12, MAP15, MAP20, MAP21, MAP26, MAP31, MAP32.

Every combination runs original lighting, identity-colormap fullbright, and the runtime lighting-disabled option. Every scene captures CPU and shaders, with GPU fallback forbidden and exact RGB equality required. Any classic mismatch triggers a focused replay with the bounded-pixel and shared-plane-UV assertions. `tmp/visual-matrix/index.html` links every run; `matrix.json` records coverage, exact-image counts, and validation status. Use `--only doom2-426x240-lit` to replace a particular matrix entry, preserving other results. A normal invocation always recaptures all entries.

### Resolution-adapted classic reference

Vanilla Linux Doom has a fixed 320×200 framebuffer. For other sizes, the private reference build changes framebuffer dimensions, widens visplane row indices and their padding/sentinels when needed, and preserves 320 as the lighting/sky reference width. The original BSP, fixed-point column/span calculations, and 90-degree horizontal projection are retained. Larger and wider captures therefore test a **resolution-adapted classic algorithm**, not a stock executable running at unsupported resolutions. Both image reports and scalar oracles use the requested dimensions. Reports include a `reference_resolution_adapted` provenance flag. Raw framebuffer pixels are compared without display pixel-aspect correction.

### New findings

- **Lower-wall anchors under a shared sky:** unpegged lower textures use `worldtop` after classic substitutes the neighboring sky height. E3M1’s worst spawn view previously differed at 89.380% of pixels.
- **Absent wall tiers:** a height change with no upper/lower texture must preserve classic’s open clip range. Closing it hid the bottom of MAP31’s MIDGRATE and removed thin floor strips in MAP04.
- **Sky floors:** `F_SKY1` floors use the same sky sampling and plane identity as ceilings. MAP32 previously rendered them as ordinary flats.
- **Sky source and scale:** Faithful uses bounded original opaque-column bytes for skies too, including Doom II SKY2’s short post. Vertical stepping truncates `320/width` to 16.16 before sampling; at 400×250, row 130 selects texel 103 rather than 104.
- **Directional lighting:** bias follows the rendered SEG endpoints, including BSP-split segments whose axis differs from the original linedef. This fixes the narrow GRAYVINE wall in MAP31.
- **Subsector ownership:** Faithful chooses its front sector from the subsector’s first SEG, even when a later visible SEG names a different sector. MAP21 contains this case; using the later SEG selected a 256-high ceiling instead of the reference’s 200-high ceiling.
- **Empty trigger lines:** identical-sector lines without a middle texture are skipped as in `R_AddLine`, preserving later visplane coverage.

The expansion also found two harness setup problems: Ultimate Doom’s reference sky enum collision described above, and cameras exactly at Z=0 invoking GD’s body-height fallback. The capture harness now sets the body consistently with the requested eye height and asserts the actual eye coordinate before drawing. These are distinguished from renderer fixes.

## Regression checks

```sh
xvfb-run -a go test ./internal/doomruntime ./internal/render/... -count=1
GD_GPU_INTEGRATION=1 GD_GPU_WAD="$PWD/wads/DOOMU.WAD" xvfb-run -a \
  go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1
GD_GPU_INTEGRATION=1 GD_GPU_WAD="$PWD/wads/DOOM2.WAD" xvfb-run -a \
  go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1
GD_GPU_SKY_INTEGRATION=1 xvfb-run -a go test -tags integration \
  ./internal/doomruntime -run '^TestGPUSkyShaderParity$' -count=1
python3 scripts/test_visual_compare.py
```

Use absolute WAD paths for direct `go test` commands, whose working directory is the package directory.

## Demo playback, death and respawn

```sh
python3 scripts/visual_demo_matrix.py --corpus tmp/compet-n/manifest.json
python3 scripts/visual_compare.py --wad wads/DOOM2.WAD \
  --demo demos/DOOM2-DEMO1.lmp --demo-tics 35,140,350 --death-tics 0,35 \
  --lighting --require-gpu-parity --out tmp/visual-demo-check
```

The demo matrix independently simulates each recording in classic and GD, then
requires identical fixed-point camera coordinates, angle, level time and death
state before comparing the world image. Actors, HUD and player palette effects
are omitted at capture, so this checks the world seen during gameplay rather
than claiming complete sprite/HUD parity. The 25 repository recordings each
supply three normal frames and three 35-tic forced-death continuations. Two
optional COMPET-N recordings add natural death and respawn frames. Each runs
320×200 with original lighting and 426×240 with runtime lighting disabled,
capturing both CPU and shader output: 54 runs and 324 scenes.

These checks exposed and corrected:

- The death camera must lower its height above the actual body to six units;
  it must continue following a sliding/falling corpse or moving floor. Grounded
  camera bob, height recovery and ceiling clipping now follow classic integer
  arithmetic. An airborne eye returns before bob and ceiling clipping. The
  existing bob-free multiplayer camera is retained because prediction uses a
  separate command clock.
- A dead player's collision height is 14 units, including ceiling/door clipping
  and sliding. Using the live height displaced corpses below low ceilings.
- Teleportation preserves the current view height and recovery velocity.
- Fatal damage immediately lowers the weapon, as `P_DropWeapon` does. Allowing
  an attack state to continue consumed RNG after death and shifted subsequent
  movement and random sector lighting in E1M7, MAP28 and MAP22.
- Demo rebirth preserves the selected gamma level. Resetting it changed every
  pixel after respawn despite otherwise matching sampling.
- Classic's saved masked-wall clip range is sufficient. Applying an additional
  approximate depth mask erased boundary pixels in low death-camera views.
- Disabling lighting must retain the original sector-light boundaries when
  assigning Faithful visplanes. Merging them changes quantized span origins and
  texture coordinates even though the output is fullbright.

The reference exports 576 scalar camera cases from its unchanged
`P_DeathThink`/`P_CalcHeight` routines. The checked-in fixture covers negative
floors, a legitimate world eye Z of zero, moving floors, bob, airborne players,
low ceilings, death and live height recovery. These run alongside the 384
sampling cases and 2,816 light-table entries.

Gameplay regression coverage for this change is the 25 repository demos plus
72 COMPET-N recordings selected across all 68 maps and nine categories. All 97
match state and gameplay RNG under the existing first-death-inclusive trace
policy, with semantic handling of unused ceiling fields. The independent visual
matrix additionally checks post-death continuations and respawn. This is a new
sample run, not a rerun of the historical 4,646-demo corpus result.

The final demo/death matrix passes all 54 runs: CPU and shaders are byte-identical
in all 324 captures, and 321 entire images match classic. The other three images
pass the bounded-source pixel and shared-plane-UV gates; their remaining RGB
differences are proven out-of-buffer classic reads. The full Go suite, both IWAD
GPU framebuffer suites, 96 sky shader cases, ten Python harness tests and 15
browser launcher tests also pass. The optional commercial-WAD campaign test now
recognizes wrapped missing-file errors and correctly skips when its WAD is absent.

## Continuous camera bob in Source Port mode

Source Port rendering evaluates bob at fractional simulation time, using
`amplitude * sin(2*pi*(409*t + 0.5)/8192)`. The 409-angle step and half-table-entry
phase match classic's sine-table construction; using an exact 20-tic period
would drift. Base eye height, amplitude and ceiling limit interpolate between
the same tick endpoints used by horizontal movement. Inputs are captured during
`P_CalcHeight`, before body movement and friction change them. The sine itself
is evaluated continuously, rather than linearly blending its sampled heights.

Rendering never changes the fixed-point simulation eye or advances the phase
using display-frame counts. Pauses hold the phase; teleport, load and restart
cut the camera history. Faithful mode retains exact fixed-point tick samples.
Authoritative multiplayer retains its existing bob-free camera and separate
reconciliation/moving-floor presentation. Tests cover 30/60/144/240 Hz sampling,
phase wrapping, long-running clocks, amplitude changes, ceilings, airborne and
dead players, camera cuts, and the shared native renderer camera.

## Weapon and status-bar comparison

Run the focused presentation oracle independently of the world-only harness:

```sh
python3 scripts/presentation_compare.py --wad wads/DOOMU.WAD --out tmp/presentation-doom1
python3 scripts/presentation_compare.py --wad wads/DOOM2.WAD --out tmp/presentation-doom2
```

This compiles another private copy of the reference. It calls the original
`R_DrawPlayerSprites`, `P_CalcHeight`, `P_MovePsprites` and status-bar widgets.
The 320×200 weapon cases cover every available psprite state at three signed,
fractional positions and five sector light levels. Each WAD also gets 72 HUD
fixtures (all faces, health/armor, ammunition, weapon ownership, card/skull
combinations), 520 bob-coordinate checks and a 100-frame sequence spanning
raising, movement, pistol fire, and death lowering. The fixture face is selected
explicitly; this tests face artwork/layout, not its random selection history.

The October 14 audit passes 1,573 Doom I and 1,828 Doom II RGB comparisons
exactly. The report retains classic, GD and amplified difference images and
fails on any differing pixel. Weapon rendering is shared by the CPU and shader
world paths. The native mesh host shares positions, lighting and HUD patch
layout; its textured-quad rasterization is not the classic rasterization oracle.

Corrections from this audit:

- Retain the `A_WeaponReady` horizontal coordinate and fractional vertical bob
  throughout attacks; use the original quadratic momentum amplitude and phase.
- Preserve the pose through saves, keyframes and authoritative player snapshots.
  Save version 28 reads versions 26–27; keyframe version 15 reads versions
  13–14. Versions 26/13 use the legacy pose fallback; older key artwork is
  recovered from collected map items.
  The authoritative binary schema changed, so connected peers need this build.
- Faithful weapon projection now uses classic fixed-point edge/post sampling,
  preserves adjacent patch posts, and uses the original fullscreen anchor.
- Apply sector colormaps to the gun and full brightness to muzzle flashes;
  composite those layers after lighting. Show the death lowering animation and
  keep the weapon down afterward.
- Put yellow/red keys in their original rows, show skull artwork when collected,
  load all six key patches and the minus sign, and match negative-number and
  shotgun-ownership widgets.

Modern Options now contains independent **CAMERA BOB** and **WEAPON BOB** on/off
settings, both on by default. The preferences persist as `disable_camera_bob`
and `disable_weapon_bob` (matching CLI flags use hyphens). They affect
presentation only, preserve weapon raising/lowering, and are ignored in Faithful
mode. Both desktop hosts and the WASM settings path use these preferences.

The expanded power-up/HUD audit adds 64 co-op/deathmatch fixtures covering all
four player backgrounds, positive/negative frags, suicide subtraction, and
number truncation. Eighty weapon frames cover invulnerability, infrared,
invisibility, muzzle flashes, combinations, and expiration boundaries. Ninety-six
flash cases cover damage, bonuses, berserk fading, radiation-suit blinking and
priority, displaying all 256 palette indices. Those run through both the CPU and
Kage flash paths (192 captures). All 3,401 captures across the two WADs match.

Additional corrections:

- Use PLAYPAL channel tables for exact red/gold/green flashes, including classic's
  palette-selection offsets and integer values, in both desktop hosts and WASM.
  Non-palette modern colors interpolate the tables. Missing or nonseparable custom
  PLAYPAL transforms retain the previous overlay fallback.
- Use colormap row 1 for infrared, including muzzle flashes. Invulnerability
  suppresses infrared even during its blink-off ticks.
- Render invisible weapons with ordered row-6 fuzz, including feedback, clipping,
  the original posts in Faithful mode, and the expiration blink. Modern hosts
  use a logical fuzz grid; native scaling floors pixel coordinates before mapping
  that grid so noninteger resolutions do not shift its rows.
- Draw the player's STFB background in multiplayer and replace arms with frags in
  deathmatch. Cache keys now include viewer color, mode and score.
- Store skull-key artwork in player inventory, saves and authoritative replicas;
  share variants with co-op keys and grant all variants at deathmatch spawn/IDKFA.
  Another player's collected map objects no longer alter a player's HUD artwork.

Native framebuffer tests cover 48 palette/death/message composites and 18 weapon
fuzz cases at 320×200, 640×400 and 960×540. The latter compare GPU samples with an
ordered CPU column reference, including simultaneous invulnerability. Run them:

```sh
GD_RAYLIB_OVERLAY_INTEGRATION=1 xvfb-run -a go test -tags 'raylib integration' ./internal/doomruntime -run '^TestNativeHUDOverlayFramebuffersMatchMain$' -count=1
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags 'raylib integration' ./internal/doomruntime -run '^TestNativeWeaponFuzzMatchesClassicColumnSampling$' -count=1
```

The focused classic suite uses raw palette RGB at 320×200. It does not validate
face-selection history, gamma/CRT postprocessing, every custom WAD, or all
world/sprite occlusion combinations. Native textured-quad projection remains
separately tested rather than claimed pixel-identical to classic.

## Modern accuracy contract

Modern is judged by preserved artwork, placement, lighting and gameplay cues.
Its higher resolution, 1.2 pixel-aspect correction, fullscreen weapon lift,
continuous camera motion, interpolated weapon poses and frame crossfades are
intentional. These differences must not move the aim anchor, change brightness
when resizing, remove artwork, or change simulation state/timing.

Run the normalized presentation audit after generating the matching classic
report above:

```sh
python3 scripts/modern_presentation_compare.py --wad wads/DOOMU.WAD --reference tmp/presentation-doom1 --out tmp/modern-doom1 --native
python3 scripts/modern_presentation_compare.py --wad wads/DOOM2.WAD --reference tmp/presentation-doom2 --out tmp/modern-doom2 --native
python3 scripts/test_modern_presentation_compare.py
```

The audit covers representative ready, attack/reload and flash artwork for every
weapon: 16 frames in Doom I and 18 in Doom II, two sector lights, lighting on/off,
a fractional bob pose, and 640×480, 960×540 and 1280×720 output. It captures the
main CPU/shader settings and native host. The weapon overlay is shared between
the main world backends; separate world tests below exercise actual GPU drawing.

Normalization analytically reverses aspect, centering, bottom anchoring and the
eight-unit Modern lift, accounting for classic's half-unit sampling anchor. It
never searches for an alignment that makes the comparison pass. Modern RGB must
occur within one logical texel of its classic location, and the classic
silhouette must remain within that allowance. Coverage includes the resampling
footprint so tiny muzzle-flash flecks cannot disappear solely from picking one
normalization sample. Classic colors absent after downsampling are reported
separately; preserving every original texel is not a Modern requirement.
Regression tests prove that this rule rejects wrong brightness, an eight-unit
anchor error, a missing weapon, and missing internal artwork.

All 576 Doom I and 648 Doom II comparisons pass this contract. This audit found
and corrected three Modern-specific issues:

- Weapon colormaps depended on framebuffer width. Modern now shades the logical
  weapon consistently at any resolution; Faithful projection keeps its own math.
- Native fullscreen weapons omitted the main host's eight-unit lift. Both hosts
  now use one logical vertical placement rule, including interpolated poses.
- Native weapon crossfades multiplied alpha twice. Only the native overlay
  upload converts shared premultiplied pixels to straight alpha, preserving the
  shared images and their Ebiten rendering. A real GPU test checks all 256 alpha
  values over a colored background (one-byte blend-rounding tolerance).

World CPU/GPU comparisons now cover E1M1, E1M5, E2M4, MAP02, MAP11 and MAP26;
Modern runs at 640×400, 960×540 and 1280×720, plus 960×540 with lighting disabled.
They retain multiple viewpoints, close monster/barrel/spectre views, power-up
and gamma changes, and Faithful regressions. The October 14 run passed 216 Modern
views: the largest fraction of pixels differing by more than two RGB levels was
0.411%, below the existing 1% structural-error limit. These are backend-consistency
checks, not proof that Modern's world projection is identical to classic.

```sh
GD_GPU_INTEGRATION=1 GD_GPU_WAD="$PWD/wads/DOOM2.WAD" xvfb-run -a go test -tags integration ./internal/doomruntime -run '^TestGPUFramebufferComparison$' -count=1 -v
GD_RAYLIB_INTEGRATION=1 xvfb-run -a go test -tags 'raylib integration' ./internal/doomruntime -run '^TestNativeModernCrossfadePreservesBrightness$' -count=1 -v
```

The static artwork report excludes gamma/CRT and animation transitions; separate
camera/bob, psprite-state, simulation-invariance, crossfade, native gamma, HUD and
power-up tests cover those behaviors to their stated scopes. This is a bounded
Modern accuracy audit, not an exhaustive gameplay or visual-quality certification.
