# Boom map support

GD-DOOM implements an automatic subset of Boom map features. Load a
classic-format PWAD with `-file`; no compatibility flag is required. The
recorded paths below are verified; complete Boom compatibility is not claimed.

## Implemented

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
- Silent thing teleports (207–208), preserving relative height and rotating
  angle/momentum. Normal teleport freeze timing follows `P_PlayerThink`.
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

## Remaining work

Generalized stairs/crushers and distinct card/skull locked doors, complete independent
floor/ceiling slot handling for legacy actions, friction, wind/pushers,
silent line-to-line teleports/crushers,
translucency, custom `ANIMATED`/`SWITCHES` tables,
GL nodes and complete Boom demo/RNG behavior remain outside this scope.
Maps may load while unsupported actions behave incorrectly.

The no-monsters comparisons establish the recorded movement/progression paths,
not full combat RNG parity, every map route, or large-fight performance. The map
audit detects unknown specials; it does not detect every unsupported interaction.

These changes advance the multiplayer simulation ID to
`gd-doom-authority-dev-8`. Deploy matching clients and servers. Mutable scroller
state extends the authoritative snapshot schema. Save version 30 reads versions
27–29; keyframe version 17 reads versions 14–16. Older snapshots cannot reconstruct
the earlier acceleration history of previously unsupported scrollers.
Voodoo-doll conversion remains limited to single-player sessions.
