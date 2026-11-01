# Boom map support

GD-DOOM implements an automatic subset of Boom map features. Load a
classic-format PWAD with `-file`; no compatibility flag is required. The
recorded paths below are verified; complete Boom compatibility is not claimed.

## Implemented

- Generalized floors (`0x6000`–`0x7fff`): all eight trigger modes, four speeds,
  up/down targets, crushing, and trigger/numeric model texture/type changes.
  Generalized floors use the floor mover slot independently of ceiling motion.
- Basic generalized doors (`0x3c00`–`0x3fff`): triggers, speeds, waits and
  open/close variants used by the recorded paths below. Locked generalized
  doors and door lighting transfers remain unsupported.
- Static ceiling/floor scrollers (250–251), carry-only conveyors (252), and
  combined floor scrolling/carrying (253). Carry impulses participate in the
  ordered thinker loop; CPU/GPU plane sampling includes the texture offsets.
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
All three cases are single-player **no-monsters** demos.

| Map | Demo | Reference frames matched | Level time | Demo format |
| --- | --- | ---: | ---: | --- |
| Sunlust MAP01 | `sl01o016.lmp` | 584 / 584 | 16.71 s | Boom 2.02 / complevel 9 |
| Sunlust MAP02 | `sunlust02nomo48.lmp` | 1,712 / 1,712 | 48.94 s | Boom 2.02 / complevel 9 |
| Sunder MAP01 | `su01o048.lmp` | 1,709 / 1,709 | 48.86 s | Doom 1.9 / complevel 2 |

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
changes, conveyor thinker timing, airborne actors, a voodoo-triggered floor,
shared damage and binary save restoration. The opt-in renderer test compares
scrolling planes on CPU/GPU in Faithful and Source Port modes:

```sh
xvfb-run -a go test -tags integration ./internal/doomruntime \
  -run '^TestBoomConveyorPlaneRendering$' -count=1
```

## Remaining work

Generalized ceilings/lifts/stairs/crushers and locked doors, complete independent
floor/ceiling slot handling for legacy actions, displacement/accelerative and
wall scrollers, friction, wind/pushers, silent teleports/crushers, height/light
transfers, deep water, translucency, custom `ANIMATED`/`SWITCHES` tables,
extended nodes, and complete Boom demo/RNG behavior remain outside this scope.
Ancient Aliens MAP01 was excluded from replay verification because its XNOD
nodes are unsupported. Maps may load while unsupported actions behave incorrectly.

The no-monsters comparisons establish the recorded movement/progression paths,
not full combat RNG parity, every map route, or large-fight performance. The map
audit detects unknown specials; it does not detect every unsupported interaction.

These changes advance the multiplayer simulation ID to
`gd-doom-authority-dev-5`. Deploy matching clients and servers. The serialized
layouts remain unchanged; new movers and single-player dolls reuse existing
sector, thing, health and momentum fields. Voodoo-doll conversion is currently
limited to single-player sessions.
