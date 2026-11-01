# Boom map support

GD-DOOM implements an automatic subset of Boom map features. Load a
classic-format PWAD with `-file`; no compatibility flag is required. The
recorded paths below are verified; complete Boom compatibility is not claimed.

## Implemented

- Boom variable friction (223, sector bit 256): controller-derived mud/ice
  friction, inertia, speed-dependent mud acceleration, overlapping-floor
  selection, one-tic friction thinker state and icy wall bounces. Normal maps
  keep their original movement path; vanilla demos do not enable Boom friction.
- Wind (224), currents (225) and point push/pull sources (226, things 5001/5002),
  gated by sector bit 512. Forces add in thinker order, follow real ground and
  height-transfer water boundaries, and honor noclip. Point forces use Boom's
  approximate distance/range, cached source position, last-spawned source and
  visibility checks, including height-sector separation. Under Boom rules
  these affect player bodies and voodoo dolls; ordinary monsters are immune.
  MBF's broader actor eligibility and inverse-square point forces are separate.

- Custom `ANIMATED` wall/flat ranges and per-range tic speeds; custom `SWITCHES`
  pairs, episode filtering and table-order selection. Supplied lumps replace
  defaults, including terminator-only tables. Repeat buttons restore one slot
  after 35 tics; pressing an active button does not extend its timer. Resources
  load in the normal, native, headless demo and dedicated-server paths.

- Boom translucency (260): tag-zero local application, tagged transfers,
  named 65,536-byte filters, supplied `TRANMAP` overrides and the generated
  default 66% source filter. Source lighting is applied before table lookup.
  Masked holes stay clear; actors and opaque/translucent walls paint in depth
  order, including crossing walls. CPU and GPU use the same palette lookup.
  RGB shading in Source Port is quantized for this indexed effect. The setup
  references rebuild on load; no additional simulation/save change is needed.
- Height transfers (242): camera-dependent fake floors/ceilings, deep-water
  plane textures, independent model plane lights, model scrolling offsets and
  sky exceptions. The inclusive waterline and one-16.16-unit plane separation
  follow DSDA's `R_FakeFlat`. Moving control heights update every frame;
  collision, actor support heights and saved physical geometry stay real.
- Height-sector sprites clip at the fake floor/ceiling, including partially
  submerged actors and actors completely separated from the camera. Normal
  play uses the average fake floor/ceiling light with classic sprite light
  rows; Boom 201/202 demos retain their original sector-light behavior.
- Named `C_START`/`C_END` colormaps on 242 control sidedefs select bottom,
  middle and top view regions. Selection uses strict height comparisons,
  distinct from the inclusive fake-floor geometry rule. World surfaces,
  sprites, skies and weapons use the active table; CPU/GPU and weapon caches
  update when it changes. Incomplete tables fall back to normal rendering.
- Per-sector sky transfers (271/272): upper sidedef texture, Doom BAM horizontal
  offsets, vertical offsets, optional flipping, sky floors and several skies
  visible together. Tall skies retain their declared height. CPU/GPU share
  per-sky coordinate lookups; Source Port honors its lighting settings.
- Floor/ceiling light transfers (213/261): planes read the control sector's
  current light, including glow/flash effects, while walls retain their own
  sector lighting. Map references are rebuilt on load; restored light caches
  are refreshed. These static features do not change simulation/save formats.

- XNOD and zlib-compressed ZNOD data in the `NODES` lump, including fractional
  16.16 split vertices, 32-bit seg/subsector indices and BSP child references.
  Classic nodes use the same widened traversal. Malformed counts, references,
  cyclic trees and oversized decompression are rejected.
- Silent thing teleports (207–208), switch-activated thing exits (209–210),
  line-to-line teleports (243–244),
  reversed line exits (262–263), monster-only reversed/normal line exits
  (264–267), and monster-only thing exits (268–269). They preserve relative
  height and vertical momentum, rotate facing/horizontal momentum, and retain
  reaction time without teleport fog or teleport sounds. Switch variants retain
  their normal switch animation. Line exits preserve fractional position,
  select the appropriate side with Boom's fixed-point nudges, and measure Z
  above the higher exit floor. First eligible exit order and blocked-exit
  rejection follow DSDA. Normal teleport freeze timing follows `P_PlayerThink`.
- Generalized ceilings (`0x4000`–`0x5fff`): all eight triggers, four speeds,
  eight targets, crushing and trigger/numeric model texture/type changes.
  Changes apply when the mover finishes and survive saves and replication.
- Generalized lifts (`0x3400`–`0x37ff`): all triggers, speeds, waits and targets,
  including perpetual motion, stop/resume and independent floor/ceiling slots.
  Manual lifts retain the activating line's tag when stopped and restored.
- Generalized locked doors accepting either card/skull of a required color,
  any key, or all three colors. Distinct card/skull and all-six-key locks remain
  unsupported. Generalized doors use an independent ceiling slot.
- Voodoo dolls slide using their own collision shape while collision checks
  retain the real player's position.

- Generalized floors (`0x6000`–`0x7fff`): all eight trigger modes, four speeds,
  up/down targets, crushing, and trigger/numeric model texture/type changes.
  Generalized floors use the floor mover slot independently of ceiling motion.
- Basic generalized doors (`0x3c00`–`0x3fff`): triggers, speeds, waits and
  open/close variants used by the recorded paths below. Door lighting
  transfers remain unsupported.
- Static ceiling/floor scrollers (250–251), carry-only conveyors (252), and
  combined floor scrolling/carrying (253). Carry impulses participate in the
  ordered thinker loop; CPU/GPU plane sampling includes the texture offsets.
- Displacement scrollers (245–249) and accelerative scrollers (214–218):
  controller floor-plus-ceiling displacement, persistent acceleration, and
  additive floor/ceiling/wall/carry effects. Wall scrollers (254/255) include
  rotation into the target wall's coordinates, sidedef-derived rates and both
  texture axes. Only the target's first side scrolls. Scrolling sky-control
  sidedefs feed the transferred sky's offsets too.
  Mutable controllers run in thinker order and preserve velocity, previous
  control height and phase across saves and authority snapshots. Arithmetic
  uses Boom's signed 16.16 wrapping. Underwater carry also reaches floating
  actors and missiles, with strict water-height and noclip checks.
- Single-player voodoo dolls: earlier matching starts become physical bodies
  with collision, momentum, player-only walk triggers, pickups and shared
  damage. Their position, health and momentum use existing save fields.
  Multiplayer voodoo ownership and full combat/death fidelity remain unverified.
- Single-player Boom 2.01/2.02 demo headers (201/202). Recording remains in the
  existing vanilla format. Parsing a header does not establish compatibility
  with all options or Boom combat RNG.

- Extended triggers for existing movers and lights: 142–145, 147–149, 152,
  156–164, 167–184, 187–188, and 192–196 (42 additional triggers).
  The exact action table is in
  [boom.go](../internal/mapdata/boom.go). Shoot exits 197–198 were already present.
- Extended W1 triggers clear only after successful activation, so a busy
  sector does not permanently consume them. Vanilla W1 behavior is retained.
- Generalized sector damage (bits 5–6), secrets (bit 7), and low-bit lighting
  and timed doors. Timed door setup consumes the sector special, as in Boom.
  Finding a secret preserves other active flags. Low type 4
  installs both its strobe and 20-point hazard in extended sectors.
- Wall scrolling to the right (85), alongside existing left scrolling (48).
- Unsigned sidedef indices through 65534, with 65535 reserved for a missing
  side. Physics, rendering and binary saves retain these larger indices.
- The last matching player start is used, so maps with duplicate starts do
  not put the player in an earlier voodoo-doll control closet.
- Existing game-mode thing flags (not single-player, not deathmatch, not co-op).

The implementation follows the [Boom editing reference](https://boomref.soulsphere.org/)
and the original extended action dispatch behavior preserved in
[MBF p_spec.c](https://github.com/fragglet/mbf/blob/master/p_spec.c) and
[p_switch.c](https://github.com/fragglet/mbf/blob/master/p_switch.c).

## Try the included map

The original asset-free `testdata/boom/simple.wad` is a single room using Doom II
textures. It has a glowing secret sector with 5-point periodic damage, a
right-scrolling west wall, a north-wall switch (161) that raises the whole room
24 units, and an east-wall exit switch that must be shot (197).

```sh
go run . -wad wads/DOOM2.WAD -file testdata/boom/simple.wad -map MAP01
```

Regenerate its deterministic WAD with `python3 scripts/make_boom_smoke_wad.py`.
The runtime smoke test loads the real WAD, initializes lighting and secret
counts, applies damage, activates the floor, restores its saved state, checks
that the floor carries the player, and triggers the gun exit. These checks do
not replace visual or interactive playtesting.

Audit linedef and sector coverage for individual WADs:

```sh
go run ./cmd/mapaudit testdata/boom/simple.wad
go run ./cmd/mapaudit path/to/addon.wad
```

## Okuplok load check

The local `oku2v31.wad` from `oku2v31.zip` has classic BSP nodes and loads after
correcting unsigned sidedef indices and player-start selection. It contains
38,295 sidedefs, exceeding the old signed 16-bit index range. Its map-data audit
now reports zero unknown linedef or sector specials, after adding its 509
generalized floors, one generalized door and one conveyor. This does not
certify its complete encounter sequence or combat performance.

The checked WAD has SHA-256
`c7dfcdbbaf33b5f7b13c8405a0b36c562c3c7d9e7f63ad48743c999dccb85061`.
This is a startup/render check; progression and large-fight performance are
not established by it.

## Demo verification

`scripts/verify_boom_demos.py` downloads hash-pinned WAD/demo archives, runs
GD-DOOM and DSDA-Doom, and compares every exported reference player position
(X/Y/Z) and angle exactly, plus the level-exit tic. Reference ghost exports
start at completed gametic 1; GD labels that completed command gametic 0.
All five cases are single-player **no-monsters** demos.

| Map | Demo | Reference frames matched | Level time | Demo format |
| --- | --- | ---: | ---: | --- |
| Ancient Aliens MAP02 | `aa02o026.lmp` | 932 / 932 | 26.66 s | Boom 2.02 / complevel 9 |
| Ancient Aliens MAP01 | `aa01o023.lmp` | 835 / 835 | 23.89 s | Boom 2.02 / complevel 9 |
| Sunlust MAP01 | `sl01o016.lmp` | 584 / 584 | 16.71 s | Boom 2.02 / complevel 9 |
| Sunlust MAP02 | `sunlust02nomo48.lmp` | 1,712 / 1,712 | 48.94 s | Boom 2.02 / complevel 9 |
| Sunder MAP01 | `su01o048.lmp` | 1,709 / 1,709 | 48.86 s | Doom 1.9 / complevel 2 |

Ancient Aliens MAP02 exercises a generalized lift and a generalized blue-key
door releasing a voodoo-controlled floor. It exits at leveltime 933.
Sunlust MAP02 contains nine extra player starts, five conveyor lines and
15 generalized floor lines. Its full recorded path stays in sync with the
reference. Sunder is a separate map-loading/movement regression; that demo
does not exercise Boom-specific commands. The reference build was DSDA-Doom
0.27.5. Downloads come from [DSDA](https://dsdarchive.com/); the script pins
archive hashes and writes content/binary hashes, logs, traces, ghosts and
`results.json` under the output directory. WADs and demos are not bundled.

```sh
python3 scripts/verify_boom_demos.py --iwad wads/DOOM2.WAD \
  --reference /path/to/dsda-doom --out /tmp/boom-demo-verification
```

The script requires Go, `xvfb-run`, and DSDA-Doom 0.27.x. Use `--engine` to
reuse a built GD-DOOM binary, or `--case sl02o048` to replay one case. Any
coordinate/angle mismatch, missing frame or exit mismatch fails the run.

The focused tests also cover parameter decoding, floor targets/speeds/model
changes, generalized ceiling targets/crushing/model changes, lift waits and
stasis tags, locked-door key gates, conveyor thinker timing, airborne actors,
a voodoo-triggered floor,
shared damage and binary save restoration. The opt-in renderer test compares
scrolling planes on CPU/GPU in Faithful and Source Port modes:

```sh
xvfb-run -a go test -tags integration ./internal/doomruntime \
  -run '^TestBoomConveyorPlaneRendering$' -count=1
```

Ancient Aliens MAP01 also rendered at six demo viewpoints (tics 1, 80, 130,
300, 500 and 800), at 640×400 with Doom lighting. CPU/GPU output matched pixel
for pixel. These captures verify GD renderer agreement, not DSDA image parity.
A temporary copy of MAP01 with its XNOD payload zlib-compressed into ZNOD
also matched all 835 reference frames and exited at tic 836.
The [XNOD/ZNOD layout](https://github.com/doom-cross-port-collab/udmf/blob/main/formats/bsp.md)
and [DSDA loader](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_setup.c)
provide the format reference.
Ancient Aliens MAP02 additionally rendered at demo tics 90, 110, 500, 550 and
800 at 640×400 with Doom lighting; each CPU/GPU pair matched pixel for pixel.
The generalized mover behavior follows [DSDA p_genlin.c](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_genlin.c).
Decoder tests cover both encodings, fractional
vertices, indices above 65535, truncated payloads, bad counts and cyclic nodes.

## Sky/light screenshot verification

`testdata/boom/transfers.wad` contains original room geometry and a uniform
flat for isolating transferred light from flat UV sampling. Regenerate it with
`python3 scripts/make_boom_transfer_wad.py`. Its ten fixed views cover floor and
ceiling lights, 271/272 flipping, both offsets, multiple skies and sky floors.
Two additional fixed views use the hash-pinned Ancient Aliens WAD above.

The capture harness builds a private copy of DSDA-Doom **0.27.5**, adding only
camera setup, actor/HUD suppression, raw PLAYPAL export and camera logging.
The software BSP/plane/column renderers are unchanged. Camera X/Y/eye Z,
angle, leveltime and output dimensions must match before comparison succeeds.
Only fixed cameras are accepted by this harness; it does not replay demos.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/transfers.wad \
  --scenes testdata/boom/transfer-scenes.json \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-transfer-fixtures

python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad /path/to/aaliens_v1_2.wad \
  --scenes testdata/boom/ancient-aliens-transfer-scenes.json \
  --reference /tmp/boom-transfer-fixtures/reference-build/dsda-doom \
  --out /tmp/boom-transfer-aaliens
```

Building the reference requires CMake, a C/C++ compiler, SDL2/SDL2_mixer, zlib,
libzip and OpenGL development files. Optional music/image libraries are disabled.
`--cmake-arg=-DNAME=value` can select a local dependency prefix. Capturing also
requires Go, Pillow and `xvfb-run`. WADs and reference binaries remain local.

At 320×200, all **15 explicitly checked transfer regions** matched DSDA pixel
for pixel across these 12 views. Every full CPU/GPU pair also matched exactly.
The ten fixture views additionally had identical CPU/GPU output in Source Port
mode with sector lighting enabled. Whole-frame DSDA differences remain:
fixture views differed at about 1.01–2.10% of pixels; the two Ancient Aliens
views differed at 22.99–24.26%, with mean absolute channel error 1.72–1.73/255.
The residual images include wall sampling and boundary differences; these checks
establish the selected transfer effects, not full-frame DSDA raster parity.

Each output directory includes `index.html`, three-way comparison images,
amplified difference images, exact camera logs, source/content/binary hashes and
`results.json`. The manifest's explicit regions use zero differing pixels as
the pass criterion. Whole-frame differences are reported separately.
Implementation references are [DSDA sky planes](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_plane.c)
and [transfer setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_spec.c).

## Height-transfer screenshot verification

Regenerate the original two-room fixture with
`python3 scripts/make_boom_water_wad.py`. MAP01 has a physical floor at -64,
a rendered water surface at 0, and a model ceiling at 128. MAP02/03 exercise
the model floor/ceiling sky exceptions. Distinct uniform flats isolate plane
visibility and lighting from texture UV sampling.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/water.wad \
  --scenes testdata/boom/water-scenes.json \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-water-verification
```

Nine fixed 320×200 views cover the shore across the surface, above/below water,
exactly at the waterline, 1/8 unit to either side, above the fake ceiling, and
both sky exceptions. All **18 explicit plane-interior regions** match DSDA
pixel for pixel on CPU and GPU. Every full CPU/GPU pair matches. These regions
avoid wall boundaries: the lowest land-facing wall edge can differ by one row.
Whole-frame DSDA differences range from 0.50–25.75%, including existing wall
sampling differences that become prominent when facing the shore underwater.
The gallery and difference images retain those differences; the checks establish
fake-plane geometry/material/light selection, not complete raster parity.

The reference probe invalidates the fresh map's wall-flag cache by setting
`gametic=1` without advancing simulation. Otherwise its zero-initialized cache
can skip DSDA's first wall classification. Logged leveltime and camera still
match exactly. The existing 12 sky/light views and 15 transfer regions also
pass with this corrected reference setup.

Focused tests cover exact 16.16 waterline transitions, front/back substitutions,
moving control heights, independent live plane lighting, scrolling offsets,
sky visibility, physical movement and binary save restoration. All five
existing no-monsters demo regressions still match DSDA for **5,772 frames**,
including their exit tics. This render-only addition needs no simulation or
save-version change.

The implementation follows [DSDA's fake-flat rendering](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_bsp.c)
and [height-sector plane marking](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_segs.c).
These original fixed-camera comparisons suppress actors and HUD and cover
world geometry. The additional actor comparisons below enable map sprites.

### Actors and underwater colormaps

`python3 scripts/make_boom_water_actors_wad.py` builds a second original fixture
with uniform indexed sprites: a pillar crossing both model planes and a barrel
entirely below water. It includes three distinct custom light tables and a sky
variant. Uniform sprite colors isolate clipping and light selection from patch
UV rounding. No IWAD artwork is stored in this fixture.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/water-actors.wad \
  --scenes testdata/boom/water-actors-scenes.json --actors --complevel 11 \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-water-actors
```

All **44 checked regions across 19 views** match DSDA pixel for pixel, including
floor/ceiling sprite cuts, the submerged barrel, exact surface/ceiling equality,
all three colormap zones, returning to land, and tinted sky. Every full CPU/GPU pair also matches,
including all 19 views in Source Port mode. Whole-frame DSDA differences remain
0.13–2.89% from wall sampling and raster boundaries; these are reported separately.
The original 21 water/sky/light views and 33 regions still pass, as do the five
demo regressions (5,772 frames and matching exits).

These actor captures explicitly use **MBF compatibility 11**: DSDA deliberately
uses the real sector light for sprites at Boom compatibility 9, retaining an
old Boom lighting bug. Compatibility 11 uses the fake plane-light average,
which GD uses for normal map play. Boom 201/202 demo playback preserves the
older light selection; focused tests cover that distinction. The renderer and
clipping code in the reference remain unchanged. `--actors` keeps map actors
in both engines while continuing to suppress the weapon and HUD.

Focused tests also cover one-fixed-unit transitions, complete sprite rejection,
live plane-light averages, namespace/malformed-table handling, returning to the
default colormap, GPU table invalidation and weapon cache identity. This remains
a presentation change with no new simulation or save format.

## Scroller verification

`python3 scripts/make_boom_scroller_wad.py` regenerates twelve original test
maps, one per remaining scroller special, and their fixed-camera manifest.
An autonomous 30-second door changes a disconnected control sector's ceiling.
Views before, during and after that motion distinguish displacement from
acceleration that continues after the control stops. The wall cases include
a slanted target, horizontal/vertical scrolling and a camera facing the 255
scroller. Patterned flats and an original wall patch expose texture phase.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/scrollers.wad \
  --scenes testdata/boom/scroller-scenes.json --simulate \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-scrollers
```

`--simulate` advances actual stationary-player world tics in both engines,
then selects the fixed rendering camera. Before drawing, it compares exact
16.16 player position/momentum, all sector heights and plane offsets, and all
sidedef offsets. The reference runs its unchanged `P_Ticker`; neither the
controller nor scroller renderer is patched. Without this flag the original
fixed-camera capture behavior remains available.

All **56 fixture views** match this numeric state exactly. All 56 wall check
regions are pixel-exact. Of 112 patterned-plane regions, 106 are pixel-exact
and six differ by at most 1.96% of pixels at UV boundaries, within their
explicit 2% limit. Whole fixture frames differ from DSDA by 1.02–2.25%; their
diagnostic differences remain in the gallery. Every complete CPU/GPU pair
matches, including all 56 views in Source Port mode.

Two additional simulated Ancient Aliens MAP01 views at tics 35/70 use
`testdata/boom/ancient-aliens-scroller-scenes.json` with the hash-pinned WAD.
They also match all recorded numeric state and their sky check regions.
Whole frames retain roughly 22.98% DSDA differences, so these are not claims
of full real-map raster parity. The existing five no-monsters demo regressions
still match all **5,772 frames** and exit tics.

Focused tests cover controller motion/stopping, cancellation between floor
and ceiling changes, additive scrollers, wall rotation/target sides, fixed
wrapping, underwater floating/missile carry, sky-control offsets, and binary
save/keyframe continuation while accelerating. The behavior follows
[DSDA's compatible scroller setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_spec.c)
and [control-scroller updates](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/dsda/scroll.c).

## Translucency verification

`python3 scripts/make_boom_translucency_wad.py` builds two original PWADs and
their scene manifests. The fixtures cover two overlapping masked walls,
transparent holes, actors in front/between/behind the layers, an opaque front
wall, crossing walls, reverse views, asymmetric named filters and an overriding
global `TRANMAP`. No IWAD artwork or generated IWAD-based filter is stored
in these fixtures.

```sh
python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/translucency.wad \
  --scenes testdata/boom/translucency-scenes.json --actors --check-tranmap \
  --reference-source /path/to/clean/dsda-doom-0.27.5 \
  --out /tmp/boom-translucency

python3 scripts/verify_boom_transfers.py --iwad wads/DOOM2.WAD \
  --pwad testdata/boom/translucency-override.wad \
  --scenes testdata/boom/translucency-override-scenes.json \
  --actors --check-tranmap \
  --reference /tmp/boom-translucency/reference-build/dsda-doom \
  --out /tmp/boom-translucency-override
```

Across **30 fixed views**, 86 of 90 checked regions match DSDA pixel for pixel.
Four regions permit one differing column (5% or 6.67% of their narrow regions)
at native raster boundaries; their limits are explicit in the manifests.
The complete CPU/GPU pairs match exactly, including the Source Port captures.
Whole-frame DSDA differences remain below 1% in these fixtures and are shown
in the diagnostic images. These are scoped comparisons, not full raster parity.

`--check-tranmap` exports the reference filter and compares **all 65,536
entries**. Both the generated default and supplied override match. The shader
tests separately exercise every destination/source pair through two asymmetric
layers using unique palette colors. Other focused tests cover tag/setup order,
shared control sides, malformed-table texture fallback, source lighting before
lookup, successive layers, rebuilt save references and ordinary-map fast paths.
Framebuffer RGB is recovered through the renderer's existing palette lookup;
the tests do not establish every duplicate-palette-index/custom-filter case.

The earlier 19 water/actor views and two Ancient Aliens transfer views still
pass their checks. All five existing no-monsters demo comparisons retain their
**5,772 matching frames and exit tics**. The reference's BSP, sprite, plane and
seg renderers are unchanged; the capture probe only adds resource export.
Setup and blending follow [DSDA 260 setup](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/p_setup.c),
[TRANMAP generation](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/dsda/tranmap.c)
and [column compositing](https://github.com/kraflab/dsda-doom/blob/v0.27.5/prboom2/src/r_drawflush.inl).

### Rendering cost

Maps without 260 keep the original cutout path and do not allocate translucency
references or generate a filter. Visible translucent cutouts enable per-column
depth ordering, grouped into adjacent runs. GPU passes reuse buffers and copy
only the affected background columns; gameplay performs no framebuffer readback.
Seven distinct referenced filters fit the current GPU metadata atlas; maps
exceeding that limit use the CPU renderer.

The before/after benchmark uses commit `5305ee5`, 20 warmup frames and **120
measured frames per case**, at fixed cameras with VSync off. It times the world
draw plus synchronized `ReadPixels`, which is benchmark-only. This host uses
Mesa llvmpipe software rendering under Xvfb, so shader-backend numbers do not
predict hardware-GPU gameplay FPS. A repeat vanilla-only run measured:

| MAP01 view | Before p50/p95/p99 ms | After p50/p95/p99 ms |
| --- | --- | --- |
| CPU, 320×200 | 0.763 / 0.848 / 0.920 | 0.871 / 0.981 / 1.118 |
| Shader, 320×200 | 1.636 / 1.817 / 1.839 | 1.677 / 1.863 / 1.892 |
| CPU, 1280×800 | 6.757 / 7.610 / 8.286 | 6.651 / 7.212 / 7.326 |
| Shader, 1280×800 | 8.747 / 9.198 / 9.241 | 8.884 / 9.385 / 9.580 |

The low-resolution CPU increase is about 0.11 ms; this is not a zero-overhead
claim. The separate full run measured the 1280×800 two-layer fixture at
11.845 / 12.361 / 12.694 ms on CPU and 19.123 / 22.362 / 22.903 ms on the shader
backend (p50/p95/p99). With opaque panels, the corresponding p50 values were
5.533 and 7.382 ms. That comparison includes the extra background visibility,
ordering and lookup work required by transparent panels.

`TestBoomFrameTiming` is opt-in and portable to the previous commit for paired
runs. Set `GD_FRAME_TIMING_OUT`, `GD_FRAME_TIMING_IWAD` and
`GD_FRAME_TIMING_FIXTURE` (the capture harness's merged fixture WAD), then run
`go test -tags integration ./internal/doomruntime -run '^TestBoomFrameTiming$'`.
`GD_FRAME_TIMING_ONLY=vanilla-map01` repeats the vanilla views alone. These local
samples are limited; combat, large translucent scenes and hardware GPUs need
their own measurements.

## Custom animation and switch verification

`make_boom_tables_wad.py` builds an original twelve-map fixture with seven
animation speeds (1, 2, 3, 4, 5, 6, 8), non-first frame aliases, overlapping
ranges, duplicate switch entries, absent assets, episode filtering and replaced
Doom defaults. The 91 captures include initial identity tables, frame boundaries,
wraparound, one-shot activation, repeat presses while a button is active, the
35-tic restoration boundary and activation after restoration. Both engines
receive actual use-button commands before advancing their normal world tick.

For every capture, **46 translation aliases**, sidedef names, line specials,
sector lights, live button timers and existing player/plane/offset state match
DSDA-Doom 0.27.5 exactly. All **273 screenshot regions** match DSDA exactly;
the CPU and GPU whole frames are identical. Whole DSDA/GD image differences
remain diagnostic (up to 1.9344% here), including the existing wall/plane raster
boundaries. This does not claim full-image DSDA equality.

The comparison corrected the previous absolute-index animation phase: DSDA
indexes each range from zero. Cache epochs now follow the common divisor of
active animation speeds, retaining the normal eight-tic cache lifetime for Doom
resources. It also corrected special 138 to set full brightness, as DSDA does.
Button state reuses the existing save/keyframe fields; the custom tables rebuild
from the WAD. The compatibility ID advances because slot selection and repeat
press behavior affect simulation.

Reproduce with an owned Doom II IWAD and the private reference capture build:

```sh
python3 scripts/make_boom_tables_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_transfers.py \
  --iwad wads/DOOM2.WAD --pwad testdata/boom/tables.wad \
  --scenes testdata/boom/tables-scenes.json --simulate \
  --reference /path/to/capture-build/dsda-doom --out /tmp/gddoom-boom-tables
```

The capture probe resets animation translations to fresh-process identity
between independent scenes; it does not alter the reference's update or render
logic. `use_tics` is a list of zero-based world tics in the scene manifest.
`table_probes` on the first scene selects the aliases logged for all scenes.
The five existing no-monsters demos still match all **5,772 frames** and level
exit tics. The full Go suite and CPU/GPU integration regression also pass.

## Silent teleport actor verification

`make_boom_teleports_wad.py` builds an original 30-map fixture and 1,456 cases.
The matrix covers all fourteen silent specials, including the existing 207–208,
with **490 player, 532 imp, 420 cacodemon and 14 missile cases**. Source geometry
includes vertical, horizontal and diagonal lines. Exit geometry
includes vertical, horizontal and diagonal lines in both directions, unequal
source/exit lengths, fractional positions, and front-higher, back-higher and
equal exit floors. Ground-level, rising and falling actor states check relative
height and positive/negative/zero vertical momentum.

All **1,456 integer actor-state comparisons match DSDA-Doom 0.27.5, complevel 9**:
position, floor/ceiling, angle, all three momentum components, reaction time,
remaining line special, blocker survival and absence of teleport fog. Negative
cases check monster-only player rejection, back-side crossings, missing exits,
blocked monsters, player telefrags and missile rejection. W1 specials remain
available on failure and clear on success; WR specials remain active.

The reference runs with monsters enabled. Most cases capture immediately after
`P_TryMove` crosses a line or `P_UseSpecialLine` activates a switch, before subsequent friction or gravity changes the
exit state. Forty-two additional cases use actual imp `P_MobjThinker` movement
for 1, 2 and 5 updates; state transitions are delayed to isolate movement from
AI randomness. These verify crossing and continued physical movement, not full
combat/AI RNG parity. The reference's `p_telept.c`, `p_spec.c`, `p_switch.c`,
`p_map.c`, `p_mobj.c` and `p_enemy.c` are byte-identical to the pristine source.

This exposed and fixed a movement difference: Boom momentum probes allow
ledge dropoff while AI walking probes retain their restriction. Normal play
selects that behavior from Boom map features; 201/202 demos select Boom rules,
while vanilla demos retain their original rules. Exit references cache by tag
in map order, so repeated crossings avoid repeated full-map searches. No
additional mutable save fields are needed. Unit tests cover state restoration,
first-exit rejection, zero tags, switch/button lifetime, non-player use restrictions
and the unchanged AI/vanilla ledge gates.

```sh
python3 scripts/make_boom_teleports_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_teleports.py \
  --iwad wads/DOOM2.WAD --reference /path/to/capture-build/dsda-doom \
  --out /tmp/gddoom-boom-teleports
```

Use `--reference-source` with pristine DSDA-Doom 0.27.5 to create a private capture
build instead. The driver saves both CSVs, the input cases, hashes and
`results.json`. The five existing no-monsters demos still match all **5,772
frames** and exit tics; the full Go suite also passes.

## Friction and environmental force verification

`make_boom_environment_wad.py` creates 20 original maps and **1,081 cases**:
441 player, 320 imp and 320 cacodemon cases. The DSDA-Doom 0.27.5 reference
runs at compatibility 9 with monsters enabled. **All 8,474 exported state rows
match exactly**, including X/Y/Z, all momentum components and the raw stored
friction/movement factor at every captured tic.

There are 529 isolated force/friction cases and 552 full-simulation cases.
The isolated cases call the reference's actual friction/pusher thinkers;
full simulation uses `P_Ticker`, player commands and real monster thinkers.
Monster state transitions are delayed to keep AI randomness out of the physics
comparison. This verifies force immunity and physical movement for monsters,
not full combat/AI RNG parity. The reference `p_spec.c`, `p_map.c`, `p_mobj.c`,
`p_user.c` and `p_sight.c` remain byte-identical to the pristine source.

Coverage includes ground, air, submersion, wading and exact body waterlines;
positive/negative controller vectors; mud/ice inertia and acceleration;
additive wind/current/friction; overlapping floors; icy wall collisions;
noclip; enable/disable timing; point-force range, center, diagonals and LOS;
cross-sector forces; and last-source selection. Unit tests additionally check
voodoo versus controlled-body noclip behavior, marker collision properties,
binary snapshot compatibility and identical movement after a checkpoint.

The implementation retains Boom's original, unclamped friction formula and
linear point-force approximation. It does not substitute MBF formulas or make
ordinary monsters respond to Boom wind/current/pushers. Controller references
and thinker orders rebuild from the map. A cached voodoo list avoids walking
all monsters for every friction/force controller. No render path changes are
required.

```sh
python3 scripts/make_boom_environment_wad.py
GOCACHE=/tmp/gd-doom-go-cache python3 scripts/verify_boom_environment.py \
  --iwad wads/DOOM2.WAD --reference /path/to/capture-build/dsda-doom \
  --out /tmp/gddoom-boom-environment
```

`--reference-source` can build a private capture copy of pristine DSDA-Doom
0.27.5. Both CSVs, input cases, hashes and `results.json` are saved. The five
existing demos still match **5,772 frames** and their level exit tics. The full
Go suite and targeted save/authority checks pass.

## Remaining work

Generalized stairs/crushers and distinct card/skull locked doors, complete independent
floor/ceiling slot handling for legacy actions, remaining legacy mover variants,
GL nodes and complete Boom demo/RNG behavior remain outside this scope.
Maps may load while unsupported actions behave incorrectly.

The no-monsters comparisons establish the recorded movement/progression paths,
not full combat RNG parity, every map route, or large-fight performance. The map
audit detects unknown specials; it does not detect every unsupported interaction.

These changes advance the multiplayer simulation ID to
`gd-doom-authority-dev-11`. Deploy matching clients and servers. Stored actor friction and movement factors extend the authoritative schema.
Save version 31 reads versions 28–30; keyframe version 18 reads versions 15–17.
Older snapshots initialize previously unsupported friction state to defaults.
Scroller data from version 30/keyframe 17 remains readable; earlier snapshots
cannot reconstruct the preceding acceleration history of unsupported scrollers.
Voodoo-doll conversion remains limited to single-player sessions.
